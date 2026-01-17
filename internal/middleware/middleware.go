// Package middleware provides HTTP middleware for the API.
package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Logger logs HTTP requests.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(wrapped, r)
		slog.Info("HTTP request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.statusCode,
			"duration", time.Since(start).String(),
			"ip", getIP(r),
		)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// CORS handles Cross-Origin Resource Sharing.
func CORS(next http.Handler, allowedOrigins []string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowed := false

		for _, o := range allowedOrigins {
			if o == "*" || o == origin {
				allowed = true
				break
			}
		}

		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Turnstile-Token")
			w.Header().Set("Access-Control-Max-Age", "86400")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// RateLimit limits requests per IP.
func RateLimit(next http.Handler, requestsPerMinute int) http.Handler {
	type client struct {
		count    int
		lastSeen time.Time
	}
	var (
		mu      sync.Mutex
		clients = make(map[string]*client)
	)

	// Cleanup old entries every minute
	go func() {
		for range time.Tick(time.Minute) {
			mu.Lock()
			cutoff := time.Now().Add(-time.Minute)
			for ip, c := range clients {
				if c.lastSeen.Before(cutoff) {
					delete(clients, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip rate limiting for health checks
		if r.URL.Path == "/api/health" {
			next.ServeHTTP(w, r)
			return
		}

		ip := getIP(r)
		mu.Lock()
		c, exists := clients[ip]
		if !exists {
			c = &client{}
			clients[ip] = c
		}

		// Reset if more than a minute has passed
		if time.Since(c.lastSeen) > time.Minute {
			c.count = 0
		}

		c.count++
		c.lastSeen = time.Now()
		count := c.count
		mu.Unlock()

		if count > requestsPerMinute {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Rate limit exceeded",
				"code":  "RATE_LIMIT",
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Turnstile verifies Cloudflare Turnstile tokens.
func Turnstile(next http.Handler, secretKey string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip for non-POST requests and health checks
		if r.Method != http.MethodPost || r.URL.Path == "/api/health" {
			next.ServeHTTP(w, r)
			return
		}

		token := r.Header.Get("X-Turnstile-Token")
		if token == "" {
			errorJSON(w, "Turnstile token required", "TURNSTILE_MISSING", http.StatusBadRequest)
			return
		}

		if !verifyTurnstile(token, secretKey, getIP(r)) {
			errorJSON(w, "Invalid Turnstile token", "TURNSTILE_INVALID", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func verifyTurnstile(token, secretKey, ip string) bool {
	resp, err := http.PostForm("https://challenges.cloudflare.com/turnstile/v0/siteverify",
		url.Values{
			"secret":   {secretKey},
			"response": {token},
			"remoteip": {ip},
		})
	if err != nil {
		slog.Error("Turnstile verification failed", "error", err)
		return false
	}
	defer resp.Body.Close()

	var result struct {
		Success bool `json:"success"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false
	}
	return result.Success
}

func getIP(r *http.Request) string {
	// Check common proxy headers
	if ip := r.Header.Get("CF-Connecting-IP"); ip != "" {
		return ip
	}
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.Split(xff, ",")[0]
	}
	return strings.Split(r.RemoteAddr, ":")[0]
}

func errorJSON(w http.ResponseWriter, message, code string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message, "code": code})
}
