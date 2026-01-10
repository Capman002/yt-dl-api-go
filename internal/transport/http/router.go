// Package http provides HTTP handlers and router configuration.
package http

import (
	"net/http"
	"time"

	"github.com/emanuelef/yt-dl-api-go/internal/config"
	"github.com/emanuelef/yt-dl-api-go/internal/transport/http/middleware"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// NewRouter creates a new chi router with all routes and middleware configured.
func NewRouter(cfg *config.Config, handlers *Handlers, rateLimiter *middleware.RateLimiter) http.Handler {
	r := chi.NewRouter()

	// Basic middleware (applied to all routes)
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Timeout(30 * time.Second))

	// Compression
	r.Use(chimiddleware.Compress(5))

	// CORS configuration
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Turnstile-Token"},
		ExposedHeaders:   []string{"X-Request-ID", "X-RateLimit-Remaining", "Retry-After"},
		AllowCredentials: false,
		MaxAge:           300, // 5 minutes
	}))

	// Health check (no rate limiting)
	r.Get("/api/health", handlers.HealthHandler)

	// API routes with rate limiting
	r.Route("/api", func(r chi.Router) {
		// Apply rate limiting to all API routes
		r.Use(middleware.RateLimitMiddleware(rateLimiter))

		// Status endpoint (lighter rate limiting)
		r.Get("/status/{job_id}", handlers.StatusHandler)

		// Download endpoint with Turnstile verification
		r.Group(func(r chi.Router) {
			// Turnstile middleware
			if !cfg.TurnstileSkip {
				r.Use(middleware.TurnstileMiddleware(&middleware.TurnstileConfig{
					SecretKey: cfg.TurnstileSecretKey,
					Skip:      cfg.TurnstileSkip,
				}))
			}

			r.Post("/download", handlers.DownloadHandler)
		})
	})

	// Catch-all for undefined routes
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusNotFound, &map[string]string{
			"error": "not found",
			"code":  "NOT_FOUND",
		})
	})

	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusMethodNotAllowed, &map[string]string{
			"error": "method not allowed",
			"code":  "METHOD_NOT_ALLOWED",
		})
	})

	return r
}

// NewServer creates a new HTTP server with optimized timeouts.
func NewServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
}
