// Package downloader provides a secure wrapper for yt-dlp.
package downloader

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/emanuelef/yt-dl-api-go/internal/domain"
)

// Buffer pool for reducing allocations when reading stdout.
var bufPool = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, 32*1024) // 32KB buffer
		return &buf
	},
}

// Downloader configuration options.
type Config struct {
	MaxFileSize int64         // Maximum file size in bytes
	MaxDuration int           // Maximum duration in seconds
	OutputDir   string        // Directory for downloaded files
	Timeout     time.Duration // Maximum time for a download
	YtDlpPath   string        // Path to yt-dlp binary
	FFmpegPath  string        // Path to ffmpeg binary (optional)
}

// DefaultConfig returns the default downloader configuration.
func DefaultConfig() *Config {
	return &Config{
		MaxFileSize: 500 * 1024 * 1024, // 500MB
		MaxDuration: 1800,              // 30 minutes
		OutputDir:   "./tmp",
		Timeout:     10 * time.Minute,
		YtDlpPath:   "yt-dlp",
		FFmpegPath:  "ffmpeg",
	}
}

// Downloader wraps yt-dlp with security constraints.
type Downloader struct {
	config *Config
	mu     sync.Mutex
}

// New creates a new Downloader with the given configuration.
func New(config *Config) *Downloader {
	if config == nil {
		config = DefaultConfig()
	}
	return &Downloader{
		config: config,
	}
}

// ProgressCallback is called with progress updates during download.
type ProgressCallback func(progress int, status string)

// Download downloads a video from the given URL.
// It returns the video info and the path to the downloaded file.
func (d *Downloader) Download(ctx context.Context, url string, progressCb ProgressCallback) (*domain.VideoInfo, string, error) {
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(ctx, d.config.Timeout)
	defer cancel()

	// Ensure output directory exists
	if err := os.MkdirAll(d.config.OutputDir, 0755); err != nil {
		return nil, "", fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate a unique filename
	timestamp := time.Now().UnixNano()
	outputTemplate := filepath.Join(d.config.OutputDir, fmt.Sprintf("%d_%%(id)s.%%(ext)s", timestamp))

	// Build yt-dlp command with security flags
	args := d.buildArgs(url, outputTemplate)

	cmd := exec.CommandContext(ctx, d.config.YtDlpPath, args...)

	// Capture stdout for progress and JSON output
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, "", fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, "", fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return nil, "", fmt.Errorf("failed to start yt-dlp: %w", err)
	}

	// Parse output
	var videoInfo *domain.VideoInfo
	var downloadedFile string
	var progressRegex = regexp.MustCompile(`\[download\]\s+(\d+\.?\d*)%`)
	var filenameRegex = regexp.MustCompile(`\[download\] Destination: (.+)`)
	var alreadyDownloadedRegex = regexp.MustCompile(`\[download\] (.+) has already been downloaded`)
	// Capture merged file from ffmpeg
	var mergerRegex = regexp.MustCompile(`\[Merger\] Merging formats into "(.+)"`)
	// Capture final file after post-processing (ExtractAudio, etc)
	var moveFileRegex = regexp.MustCompile(`\[MoveFiles\] Moving file "(.+)" to "(.+)"`)
	// Capture ffmpeg output file
	var ffmpegRegex = regexp.MustCompile(`\[ffmpeg\] Destination: (.+)`)

	// Read stdout in a goroutine
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)

		for scanner.Scan() {
			line := scanner.Text()

			// Try to parse as JSON (video info)
			if strings.HasPrefix(line, "{") {
				var info domain.VideoInfo
				if err := json.Unmarshal([]byte(line), &info); err == nil {
					videoInfo = &info
				}
				continue
			}

			// Parse progress
			if matches := progressRegex.FindStringSubmatch(line); len(matches) > 1 {
				if progress, err := strconv.ParseFloat(matches[1], 64); err == nil {
					if progressCb != nil {
						progressCb(int(progress), "downloading")
					}
				}
			}

			// Parse filename from download destination
			if matches := filenameRegex.FindStringSubmatch(line); len(matches) > 1 {
				downloadedFile = strings.TrimSpace(matches[1])
			}

			// Check for already downloaded
			if matches := alreadyDownloadedRegex.FindStringSubmatch(line); len(matches) > 1 {
				downloadedFile = strings.TrimSpace(matches[1])
			}

			// Capture merged file (overrides previous, this is the final file)
			if matches := mergerRegex.FindStringSubmatch(line); len(matches) > 1 {
				downloadedFile = strings.TrimSpace(matches[1])
			}

			// Capture ffmpeg destination (for audio extraction, etc)
			if matches := ffmpegRegex.FindStringSubmatch(line); len(matches) > 1 {
				downloadedFile = strings.TrimSpace(matches[1])
			}

			// Capture final file after MoveFiles post-processor (last resort, most accurate)
			if matches := moveFileRegex.FindStringSubmatch(line); len(matches) > 2 {
				downloadedFile = strings.TrimSpace(matches[2]) // Use destination, not source
			}
		}
	}()

	// Read stderr for errors
	var stderrOutput strings.Builder
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			stderrOutput.WriteString(scanner.Text())
			stderrOutput.WriteString("\n")
		}
	}()

	// Wait for output processing
	wg.Wait()

	// Wait for command to complete
	if err := cmd.Wait(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, "", errors.New("download timed out")
		}
		if ctx.Err() == context.Canceled {
			return nil, "", errors.New("download was canceled")
		}

		errOutput := stderrOutput.String()
		if strings.Contains(errOutput, "Video unavailable") {
			return nil, "", errors.New("video is unavailable or private")
		}
		if strings.Contains(errOutput, "is not a valid URL") {
			return nil, "", errors.New("invalid video URL")
		}

		return nil, "", fmt.Errorf("yt-dlp error: %s", errOutput)
	}

	// Notify completion
	if progressCb != nil {
		progressCb(100, "complete")
	}

	// Find the downloaded file if not captured
	if downloadedFile == "" && videoInfo != nil && videoInfo.Filename != "" {
		downloadedFile = filepath.Join(d.config.OutputDir, videoInfo.Filename)
	}

	// If we still don't have the file, try to find it by pattern
	if downloadedFile == "" {
		pattern := filepath.Join(d.config.OutputDir, fmt.Sprintf("%d_*", timestamp))
		matches, _ := filepath.Glob(pattern)
		if len(matches) > 0 {
			downloadedFile = matches[0]
		}
	}

	if downloadedFile == "" {
		return nil, "", errors.New("could not determine downloaded file path")
	}

	// Verify file exists
	if _, err := os.Stat(downloadedFile); err != nil {
		return nil, "", fmt.Errorf("downloaded file not found: %w", err)
	}

	return videoInfo, downloadedFile, nil
}

// GetVideoInfo retrieves video metadata without downloading.
func (d *Downloader) GetVideoInfo(ctx context.Context, url string) (*domain.VideoInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	args := []string{
		"--no-download",
		"--print-json",
		"--no-playlist",
		"--no-warnings",
		url,
	}

	cmd := exec.CommandContext(ctx, d.config.YtDlpPath, args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get video info: %w", err)
	}

	var info domain.VideoInfo
	if err := json.Unmarshal(output, &info); err != nil {
		return nil, fmt.Errorf("failed to parse video info: %w", err)
	}

	return &info, nil
}

// buildArgs constructs the yt-dlp command arguments with security constraints.
func (d *Downloader) buildArgs(url, outputTemplate string) []string {
	args := []string{
		// Security flags
		"--no-playlist",                                           // Don't download playlists
		"--max-filesize", fmt.Sprintf("%d", d.config.MaxFileSize), // Limit file size
		"--match-filter", fmt.Sprintf("duration<%d", d.config.MaxDuration), // Limit duration

		// Output flags
		"--newline",    // Print progress on new lines
		"--print-json", // Output video info as JSON
		"-o", outputTemplate,

		// Quality settings (best quality up to 1080p)
		"-f", "bestvideo[height<=1080][ext=mp4]+bestaudio[ext=m4a]/best[height<=1080][ext=mp4]/best",

		// Embed metadata
		"--embed-metadata",
		"--embed-thumbnail",

		// Network settings
		"--socket-timeout", "30",
		"--retries", "3",

		// Disable cache for security
		"--no-cache-dir",

		// Final URL
		url,
	}

	// Add ffmpeg path if specified
	if d.config.FFmpegPath != "" {
		args = append([]string{"--ffmpeg-location", d.config.FFmpegPath}, args...)
	}

	return args
}

// Cleanup removes a downloaded file.
func (d *Downloader) Cleanup(filePath string) error {
	if filePath == "" {
		return nil
	}

	// Security check: ensure file is within output directory
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return err
	}

	absOutputDir, err := filepath.Abs(d.config.OutputDir)
	if err != nil {
		return err
	}

	if !strings.HasPrefix(absPath, absOutputDir) {
		return errors.New("cannot delete file outside output directory")
	}

	return os.Remove(filePath)
}

// CheckYtDlp verifies that yt-dlp is installed and accessible.
func (d *Downloader) CheckYtDlp() error {
	cmd := exec.Command(d.config.YtDlpPath, "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("yt-dlp not found or not executable: %w", err)
	}
	return nil
}
