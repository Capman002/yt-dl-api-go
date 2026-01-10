// Package middleware provides HTTP middleware functions.
package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/emanuelef/yt-dl-api-go/pkg/safeclient"
)

const (
	turnstileVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"
	turnstileHeader    = "X-Turnstile-Token"
)

// TurnstileResponse represents the response from Cloudflare Turnstile API.
type TurnstileResponse struct {
	Success     bool     `json:"success"`
	ChallengeTS string   `json:"challenge_ts,omitempty"`
	Hostname    string   `json:"hostname,omitempty"`
	ErrorCodes  []string `json:"error-codes,omitempty"`
	Action      string   `json:"action,omitempty"`
	CData       string   `json:"cdata,omitempty"`
}

// TurnstileConfig holds configuration for Turnstile middleware.
type TurnstileConfig struct {
	SecretKey string
	Skip      bool // Skip verification (for development)
}

// VerifyTurnstile verifies a Turnstile token with Cloudflare's API.
func VerifyTurnstile(ctx context.Context, token, secretKey, remoteIP string) (bool, error) {
	if token == "" {
		return false, fmt.Errorf("turnstile token is empty")
	}

	if secretKey == "" {
		return false, fmt.Errorf("turnstile secret key is not configured")
	}

	// Create form data
	formData := fmt.Sprintf("secret=%s&response=%s&remoteip=%s", secretKey, token, remoteIP)

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, turnstileVerifyURL, strings.NewReader(formData))
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Use safe HTTP client
	client := safeclient.NewSafeHTTPClientWithTimeout(10 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to verify turnstile: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse response
	var result TurnstileResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return false, fmt.Errorf("failed to parse response: %w", err)
	}

	if !result.Success {
		slog.Warn("Turnstile verification failed",
			"error_codes", result.ErrorCodes,
			"hostname", result.Hostname,
		)
		return false, nil
	}

	return true, nil
}

// TurnstileMiddleware creates a middleware that verifies Turnstile tokens.
func TurnstileMiddleware(config *TurnstileConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip verification if configured
			if config.Skip {
				slog.Debug("Turnstile verification skipped (development mode)")
				next.ServeHTTP(w, r)
				return
			}

			// Get client IP
			remoteIP := getClientIP(r)

			// Get token from header first, then from request body
			token := r.Header.Get(turnstileHeader)

			// If not in header, try to get from query or body
			if token == "" {
				token = r.URL.Query().Get("turnstile")
			}

			// For POST requests, we might need to parse the body
			// But we'll handle this in the handler since we need the body for the URL too

			if token == "" {
				http.Error(w, `{"error":"turnstile token required","code":"TURNSTILE_MISSING"}`, http.StatusBadRequest)
				return
			}

			// Verify token
			ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
			defer cancel()

			valid, err := VerifyTurnstile(ctx, token, config.SecretKey, remoteIP)
			if err != nil {
				slog.Error("Turnstile verification error",
					"error", err,
					"ip", remoteIP,
				)
				http.Error(w, `{"error":"turnstile verification failed","code":"TURNSTILE_ERROR"}`, http.StatusInternalServerError)
				return
			}

			if !valid {
				slog.Warn("Invalid Turnstile token",
					"ip", remoteIP,
				)
				http.Error(w, `{"error":"invalid turnstile token","code":"TURNSTILE_INVALID"}`, http.StatusForbidden)
				return
			}

			// Token is valid, proceed
			next.ServeHTTP(w, r)
		})
	}
}

// getClientIP extracts the real client IP from the request.
// It checks Cloudflare headers first, then standard proxy headers.
func getClientIP(r *http.Request) string {
	// Cloudflare's real IP header
	if ip := r.Header.Get("CF-Connecting-IP"); ip != "" {
		return ip
	}

	// Standard proxy headers
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}

	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		// X-Forwarded-For can contain multiple IPs, take the first one
		if idx := strings.Index(forwarded, ","); idx != -1 {
			return strings.TrimSpace(forwarded[:idx])
		}
		return strings.TrimSpace(forwarded)
	}

	// Fall back to RemoteAddr
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}

// GetClientIP is exported for use in other packages.
func GetClientIP(r *http.Request) string {
	return getClientIP(r)
}
