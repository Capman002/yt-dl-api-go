// Package handler provides HTTP handlers for the API.
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Downloader defines the interface for video downloading.
type Downloader interface {
	Download(ctx context.Context, videoURL string) (filePath string, err error)
}

// Storage defines the interface for file storage.
type Storage interface {
	Upload(ctx context.Context, filePath string) (publicURL string, err error)
	Cleanup(filePath string) error
}

// Handler holds dependencies for HTTP handlers.
type Handler struct {
	dl    Downloader
	store Storage
}

// New creates a new Handler.
func New(dl Downloader, store Storage) *Handler {
	return &Handler{dl: dl, store: store}
}

// DownloadRequest is the expected JSON body for POST /api/download.
type DownloadRequest struct {
	URL string `json:"url"`
}

// DownloadResponse is the JSON response for successful downloads.
type DownloadResponse struct {
	DownloadURL string `json:"download_url"`
	Title       string `json:"title,omitempty"`
}

// ErrorResponse is the standard error response format.
type ErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
}

// Allowed domains for video downloads (security whitelist).
var allowedDomains = []string{
	"youtube.com", "youtu.be", "www.youtube.com", "m.youtube.com",
	"tiktok.com", "www.tiktok.com", "vm.tiktok.com",
	"instagram.com", "www.instagram.com",
	"twitter.com", "x.com", "www.twitter.com",
	"facebook.com", "www.facebook.com", "fb.watch",
	"vimeo.com", "www.vimeo.com",
	"dailymotion.com", "www.dailymotion.com",
	"twitch.tv", "www.twitch.tv", "clips.twitch.tv",
	"reddit.com", "www.reddit.com", "v.redd.it",
	"pinterest.com", "www.pinterest.com", "pin.it",
}

// Health handles GET /api/health.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// Options handles preflight CORS requests.
func (h *Handler) Options(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// Download handles POST /api/download.
func (h *Handler) Download(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	// Parse request
	var req DownloadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorJSON(w, "Invalid JSON body", "INVALID_JSON", http.StatusBadRequest)
		return
	}

	// Validate URL
	if err := h.validateURL(req.URL); err != nil {
		h.errorJSON(w, err.Error(), "INVALID_URL", http.StatusBadRequest)
		return
	}

	slog.Info("Download requested", "url", req.URL, "ip", r.RemoteAddr)

	// Download video
	filePath, err := h.dl.Download(ctx, req.URL)
	if err != nil {
		slog.Error("Download failed", "error", err, "url", req.URL)
		h.handleDownloadError(w, err)
		return
	}
	defer h.store.Cleanup(filePath)

	// Upload to storage
	publicURL, err := h.store.Upload(ctx, filePath)
	if err != nil {
		slog.Error("Upload failed", "error", err)
		h.errorJSON(w, "Failed to upload video", "UPLOAD_ERROR", http.StatusInternalServerError)
		return
	}

	slog.Info("Download completed", "url", req.URL, "download_url", publicURL)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(DownloadResponse{DownloadURL: publicURL})
}

// validateURL checks if the URL is valid and from an allowed domain.
func (h *Handler) validateURL(rawURL string) error {
	if rawURL == "" {
		return errors.New("URL is required")
	}

	// Basic URL validation
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return errors.New("Invalid URL format")
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("URL must use http or https")
	}

	// Check against whitelist
	host := strings.ToLower(parsed.Host)
	host = strings.TrimPrefix(host, "www.")

	allowed := false
	for _, domain := range allowedDomains {
		d := strings.TrimPrefix(domain, "www.")
		if host == d || strings.HasSuffix(host, "."+d) {
			allowed = true
			break
		}
	}

	if !allowed {
		return errors.New("Domain not supported")
	}

	// Block suspicious patterns (command injection prevention)
	suspicious := regexp.MustCompile(`[;&|$\x60\\]`)
	if suspicious.MatchString(rawURL) {
		return errors.New("URL contains invalid characters")
	}

	return nil
}

// handleDownloadError maps download errors to appropriate HTTP responses.
func (h *Handler) handleDownloadError(w http.ResponseWriter, err error) {
	msg := err.Error()

	switch {
	case strings.Contains(msg, "duration"):
		h.errorJSON(w, "Video exceeds maximum duration (30 minutes)", "DURATION_EXCEEDED", http.StatusBadRequest)
	case strings.Contains(msg, "filesize") || strings.Contains(msg, "file size"):
		h.errorJSON(w, "Video exceeds maximum file size (500MB)", "SIZE_EXCEEDED", http.StatusBadRequest)
	case strings.Contains(msg, "unavailable") || strings.Contains(msg, "private"):
		h.errorJSON(w, "Video is unavailable or private", "VIDEO_UNAVAILABLE", http.StatusNotFound)
	case strings.Contains(msg, "timed out"):
		h.errorJSON(w, "Download timed out", "TIMEOUT", http.StatusGatewayTimeout)
	default:
		h.errorJSON(w, "Failed to download video", "DOWNLOAD_ERROR", http.StatusInternalServerError)
	}
}

// errorJSON writes a JSON error response.
func (h *Handler) errorJSON(w http.ResponseWriter, message, code string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{Error: message, Code: code})
}
