// Package main is the entry point for the video downloader API.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/emanuelef/yt-dl-api-go/internal/downloader"
	"github.com/emanuelef/yt-dl-api-go/internal/handler"
	"github.com/emanuelef/yt-dl-api-go/internal/middleware"
	"github.com/emanuelef/yt-dl-api-go/internal/storage"
)

// Config holds all application configuration.
type Config struct {
	Port               string
	AllowedOrigins     []string
	TurnstileSecret    string
	TurnstileSkip      bool
	RateLimitPerMinute int
	R2AccountID        string
	R2AccessKeyID      string
	R2SecretAccessKey  string
	R2BucketName       string
	R2PublicURL        string
	MaxDurationSeconds int
	MaxFileSizeBytes   int64
	TempDir            string
}

func main() {
	cfg := loadConfig()

	// Setup structured logging
	logLevel := slog.LevelInfo
	if os.Getenv("LOG_LEVEL") == "debug" {
		logLevel = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})))

	// Initialize components
	dl := downloader.New(cfg.TempDir, cfg.MaxDurationSeconds, cfg.MaxFileSizeBytes)

	var store handler.Storage
	if cfg.R2AccountID != "" {
		r2, err := storage.NewR2(context.Background(), cfg.R2AccountID, cfg.R2AccessKeyID, cfg.R2SecretAccessKey, cfg.R2BucketName, cfg.R2PublicURL)
		if err != nil {
			slog.Warn("R2 not configured, using local storage", "error", err)
			store = storage.NewLocal(cfg.TempDir)
		} else {
			store = r2
		}
	} else {
		store = storage.NewLocal(cfg.TempDir)
	}

	h := handler.New(dl, store)

	// Build middleware chain
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", h.Health)
	mux.HandleFunc("POST /api/download", h.Download)
	mux.HandleFunc("OPTIONS /api/download", h.Options)

	// Apply middleware (order matters: outermost first)
	var httpHandler http.Handler = mux
	httpHandler = middleware.RateLimit(httpHandler, cfg.RateLimitPerMinute)
	if !cfg.TurnstileSkip {
		httpHandler = middleware.Turnstile(httpHandler, cfg.TurnstileSecret)
	}
	httpHandler = middleware.CORS(httpHandler, cfg.AllowedOrigins)
	httpHandler = middleware.Logger(httpHandler)

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      httpHandler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 10 * time.Minute, // Long for video downloads
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		slog.Info("Server starting", "port", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	server.Shutdown(ctx)
}

func loadConfig() *Config {
	return &Config{
		Port:               getEnv("PORT", "8080"),
		AllowedOrigins:     splitEnv("ALLOWED_ORIGINS", []string{"*"}),
		TurnstileSecret:    os.Getenv("TURNSTILE_SECRET_KEY"),
		TurnstileSkip:      os.Getenv("TURNSTILE_SKIP") == "true",
		RateLimitPerMinute: getEnvInt("RATE_LIMIT_RPM", 10),
		R2AccountID:        os.Getenv("R2_ACCOUNT_ID"),
		R2AccessKeyID:      os.Getenv("R2_ACCESS_KEY_ID"),
		R2SecretAccessKey:  os.Getenv("R2_SECRET_ACCESS_KEY"),
		R2BucketName:       getEnv("R2_BUCKET_NAME", "video-downloads"),
		R2PublicURL:        os.Getenv("R2_PUBLIC_URL"),
		MaxDurationSeconds: getEnvInt("MAX_DURATION_SECONDS", 1800),
		MaxFileSizeBytes:   int64(getEnvInt("MAX_FILE_SIZE_MB", 500)) * 1024 * 1024,
		TempDir:            getEnv("TEMP_DIR", "./tmp"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		var i int
		if _, err := fmt.Sscanf(v, "%d", &i); err == nil {
			return i
		}
	}
	return fallback
}

func splitEnv(key string, fallback []string) []string {
	if v := os.Getenv(key); v != "" {
		return strings.Split(v, ",")
	}
	return fallback
}
