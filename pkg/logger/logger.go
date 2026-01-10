// Package logger provides structured logging configuration.
package logger

import (
	"log/slog"
	"os"
	"strings"
)

// Config holds logger configuration.
type Config struct {
	Level  string // debug, info, warn, error
	Format string // json, text
}

// Setup configures the global logger.
func Setup(cfg *Config) {
	// Parse level
	var level slog.Level
	switch strings.ToLower(cfg.Level) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	// Create handler based on format
	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: level == slog.LevelDebug,
	}

	switch strings.ToLower(cfg.Format) {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, opts)
	default:
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	// Set as default logger
	logger := slog.New(handler)
	slog.SetDefault(logger)
}

// SetupDevelopment configures the logger for development.
func SetupDevelopment() {
	Setup(&Config{
		Level:  "debug",
		Format: "text",
	})
}

// SetupProduction configures the logger for production.
func SetupProduction() {
	Setup(&Config{
		Level:  "info",
		Format: "json",
	})
}
