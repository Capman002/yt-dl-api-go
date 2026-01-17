// Package downloader provides a minimal wrapper around yt-dlp.
package downloader

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Downloader wraps yt-dlp with security constraints.
type Downloader struct {
	tempDir     string
	maxDuration int
	maxFileSize int64
}

// New creates a new Downloader.
func New(tempDir string, maxDuration int, maxFileSize int64) *Downloader {
	os.MkdirAll(tempDir, 0755)
	return &Downloader{
		tempDir:     tempDir,
		maxDuration: maxDuration,
		maxFileSize: maxFileSize,
	}
}

// Download downloads a video from the given URL and returns the file path.
func (d *Downloader) Download(ctx context.Context, videoURL string) (string, error) {
	// Generate unique output filename
	timestamp := time.Now().UnixNano()
	outputTemplate := filepath.Join(d.tempDir, fmt.Sprintf("%d_%%(id)s.%%(ext)s", timestamp))

	// Build yt-dlp arguments with security constraints
	args := []string{
		"--no-playlist",
		"--max-filesize", fmt.Sprintf("%d", d.maxFileSize),
		"--match-filter", fmt.Sprintf("duration<%d", d.maxDuration),
		"-f", "bestvideo[height<=1080][ext=mp4]+bestaudio[ext=m4a]/best[height<=1080][ext=mp4]/best",
		"-o", outputTemplate,
		"--no-cache-dir",
		"--socket-timeout", "30",
		"--retries", "3",
		"--print", "after_move:filepath",
		videoURL,
	}

	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		outputStr := string(output)

		// Check for specific error conditions
		if strings.Contains(outputStr, "Video unavailable") {
			return "", errors.New("video is unavailable or private")
		}
		if strings.Contains(outputStr, "duration<") && strings.Contains(outputStr, "skipping") {
			return "", errors.New("video exceeds maximum duration limit")
		}
		if strings.Contains(outputStr, "filesize") {
			return "", errors.New("video exceeds maximum file size limit")
		}
		if ctx.Err() == context.DeadlineExceeded {
			return "", errors.New("download timed out")
		}

		return "", fmt.Errorf("yt-dlp error: %s", truncate(outputStr, 200))
	}

	// Extract file path from output (last non-empty line)
	filePath := extractFilePath(string(output), d.tempDir, timestamp)
	if filePath == "" {
		return "", errors.New("could not determine downloaded file path")
	}

	// Verify file exists
	if _, err := os.Stat(filePath); err != nil {
		return "", fmt.Errorf("downloaded file not found: %w", err)
	}

	return filePath, nil
}

// extractFilePath finds the downloaded file path from yt-dlp output.
func extractFilePath(output, tempDir string, timestamp int64) string {
	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Try to find the printed filepath (from --print after_move:filepath)
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" && !strings.HasPrefix(line, "[") && strings.Contains(line, string(filepath.Separator)) {
			if _, err := os.Stat(line); err == nil {
				return line
			}
		}
	}

	// Fallback: find by pattern
	pattern := filepath.Join(tempDir, fmt.Sprintf("%d_*", timestamp))
	matches, _ := filepath.Glob(pattern)
	if len(matches) > 0 {
		return matches[0]
	}

	return ""
}

// truncate shortens a string for error messages.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
