// Package middleware provides HTTP middleware functions.
package middleware

import (
	"log/slog"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimitConfig holds configuration for rate limiting.
type RateLimitConfig struct {
	RequestsPerMinute int           // Maximum requests per minute per IP
	Burst             int           // Maximum burst size
	CleanupInterval   time.Duration // How often to clean up old entries
}

// DefaultRateLimitConfig returns the default rate limit configuration.
func DefaultRateLimitConfig() *RateLimitConfig {
	return &RateLimitConfig{
		RequestsPerMinute: 5,
		Burst:             2,
		CleanupInterval:   10 * time.Minute,
	}
}

// visitor represents a rate-limited visitor.
type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimiter manages rate limiting per IP address.
type RateLimiter struct {
	config   *RateLimitConfig
	visitors map[string]*visitor
	mu       sync.RWMutex
	stopCh   chan struct{}
}

// NewRateLimiter creates a new RateLimiter with the given configuration.
func NewRateLimiter(config *RateLimitConfig) *RateLimiter {
	if config == nil {
		config = DefaultRateLimitConfig()
	}

	rl := &RateLimiter{
		config:   config,
		visitors: make(map[string]*visitor),
		stopCh:   make(chan struct{}),
	}

	// Start cleanup goroutine
	go rl.cleanupOldEntries()

	return rl
}

// Stop stops the rate limiter's cleanup goroutine.
func (rl *RateLimiter) Stop() {
	close(rl.stopCh)
}

// getVisitor retrieves or creates a rate limiter for the given IP.
func (rl *RateLimiter) getVisitor(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.visitors[ip]
	if !exists {
		// Calculate rate: requests per minute -> per second
		ratePerSecond := rate.Limit(float64(rl.config.RequestsPerMinute) / 60.0)
		limiter := rate.NewLimiter(ratePerSecond, rl.config.Burst)

		rl.visitors[ip] = &visitor{
			limiter:  limiter,
			lastSeen: time.Now(),
		}
		return limiter
	}

	// Update last seen time
	v.lastSeen = time.Now()
	return v.limiter
}

// cleanupOldEntries periodically removes old entries from the visitors map.
func (rl *RateLimiter) cleanupOldEntries() {
	ticker := time.NewTicker(rl.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.cleanup()
		case <-rl.stopCh:
			return
		}
	}
}

// cleanup removes visitors that haven't been seen recently.
func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	threshold := time.Now().Add(-rl.config.CleanupInterval)
	deleted := 0

	for ip, v := range rl.visitors {
		if v.lastSeen.Before(threshold) {
			delete(rl.visitors, ip)
			deleted++
		}
	}

	if deleted > 0 {
		slog.Debug("Rate limiter cleanup",
			"deleted", deleted,
			"remaining", len(rl.visitors),
		)
	}
}

// Allow checks if a request from the given IP is allowed.
func (rl *RateLimiter) Allow(ip string) bool {
	limiter := rl.getVisitor(ip)
	return limiter.Allow()
}

// VisitorCount returns the number of tracked visitors.
func (rl *RateLimiter) VisitorCount() int {
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	return len(rl.visitors)
}

// RateLimitMiddleware creates a middleware that enforces rate limiting.
func RateLimitMiddleware(rl *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := getClientIP(r)

			if !rl.Allow(ip) {
				slog.Warn("Rate limit exceeded",
					"ip", ip,
					"path", r.URL.Path,
				)

				w.Header().Set("Retry-After", "60")
				w.Header().Set("X-RateLimit-Remaining", "0")
				http.Error(w, `{"error":"rate limit exceeded","code":"RATE_LIMIT"}`, http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// StrictRateLimitMiddleware creates a stricter rate limiting middleware
// that's specifically for download endpoints.
func StrictRateLimitMiddleware(requestsPerMinute, burst int) func(http.Handler) http.Handler {
	rl := NewRateLimiter(&RateLimitConfig{
		RequestsPerMinute: requestsPerMinute,
		Burst:             burst,
		CleanupInterval:   5 * time.Minute,
	})

	return RateLimitMiddleware(rl)
}
