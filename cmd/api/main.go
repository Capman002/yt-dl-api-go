// Package main is the entry point for the yt-dl-api-go application.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/emanuelef/yt-dl-api-go/internal/config"
	"github.com/emanuelef/yt-dl-api-go/internal/infra/fs"
	"github.com/emanuelef/yt-dl-api-go/internal/infra/r2"
	"github.com/emanuelef/yt-dl-api-go/internal/infra/sqlite"
	"github.com/emanuelef/yt-dl-api-go/internal/service/downloader"
	"github.com/emanuelef/yt-dl-api-go/internal/service/queue"
	"github.com/emanuelef/yt-dl-api-go/internal/transport/http"
	"github.com/emanuelef/yt-dl-api-go/internal/transport/http/middleware"
	"github.com/emanuelef/yt-dl-api-go/pkg/logger"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Setup logger
	logFormat := "text"
	if cfg.IsProduction() {
		logFormat = "json"
	}
	logger.Setup(&logger.Config{
		Level:  cfg.LogLevel,
		Format: logFormat,
	})

	slog.Info("Starting yt-dl-api-go",
		"env", cfg.Env,
		"port", cfg.Port,
	)

	// Create context with cancellation for graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Initialize database
	repo, err := sqlite.NewRepository(cfg.DataDir)
	if err != nil {
		slog.Error("Failed to initialize database", "error", err)
		os.Exit(1)
	}
	defer repo.Close()

	// Initialize downloader
	dl := downloader.New(&downloader.Config{
		MaxFileSize: cfg.MaxFileSize,
		MaxDuration: cfg.MaxDuration,
		OutputDir:   cfg.TempDir,
		Timeout:     10 * time.Minute,
	})

	// Check if yt-dlp is available
	if err := dl.CheckYtDlp(); err != nil {
		slog.Warn("yt-dlp not found, downloads will fail", "error", err)
	}

	// Initialize R2 client (optional)
	var r2Client *r2.Client
	if cfg.R2AccountID != "" && cfg.R2AccessKeyID != "" {
		r2Client, err = r2.NewClient(ctx, &r2.Config{
			AccountID:       cfg.R2AccountID,
			AccessKeyID:     cfg.R2AccessKeyID,
			SecretAccessKey: cfg.R2SecretAccessKey,
			BucketName:      cfg.R2BucketName,
			PublicURL:       cfg.R2PublicURL,
		})
		if err != nil {
			slog.Warn("Failed to initialize R2 client, files will be stored locally", "error", err)
		}
	} else {
		slog.Info("R2 not configured, files will be stored locally")
	}

	// Initialize handlers
	handlers := http.NewHandlers(repo, nil, dl, r2Client) // dispatcher will be set after

	// Initialize dispatcher (worker pool)
	dispatcher := queue.NewDispatcher(cfg.MaxWorkers, cfg.MaxQueueSize, handlers.ProcessJob)
	dispatcher.Start(ctx)
	defer dispatcher.Stop()

	// Update handlers with dispatcher
	handlers = http.NewHandlers(repo, dispatcher, dl, r2Client)

	// Initialize rate limiter
	rateLimiter := middleware.NewRateLimiter(&middleware.RateLimitConfig{
		RequestsPerMinute: cfg.RateLimitRPM,
		Burst:             cfg.RateLimitBurst,
		CleanupInterval:   10 * time.Minute,
	})
	defer rateLimiter.Stop()

	// Initialize file cleaner
	cleaner := fs.NewCleaner(&fs.CleanerConfig{
		LocalDir:      cfg.TempDir,
		LocalMaxAge:   30 * time.Minute,
		LocalInterval: cfg.LocalCleanupInterval,
		R2Client:      r2Client,
		R2MaxAge:      cfg.R2MaxFileAge,
		R2Interval:    cfg.R2CleanupInterval,
	})
	cleaner.Start(ctx)
	defer cleaner.Stop()

	// Create router
	router := http.NewRouter(cfg, handlers, rateLimiter)

	// Create and start server
	server := http.NewServer(":"+cfg.Port, router)

	// Start server in goroutine
	go func() {
		slog.Info("HTTP server starting", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != httpError(err) {
			slog.Error("HTTP server error", "error", err)
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()

	slog.Info("Shutting down gracefully...")

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Shutdown HTTP server
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("HTTP server shutdown error", "error", err)
	}

	slog.Info("Server stopped")
}

// httpError checks if the error is http.ErrServerClosed
func httpError(err error) error {
	if err.Error() == "http: Server closed" {
		return err
	}
	return nil
}
