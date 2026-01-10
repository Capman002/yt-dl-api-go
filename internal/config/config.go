// Package config provides configuration loading and validation.
package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all configuration values for the application.
type Config struct {
	// Server
	Port     string
	Env      string
	LogLevel string

	// CORS
	AllowedOrigins []string

	// Turnstile
	TurnstileSecretKey string
	TurnstileSkip      bool

	// Rate Limiting
	RateLimitRPM   int
	RateLimitBurst int

	// Worker Pool
	MaxWorkers   int
	MaxQueueSize int

	// R2 Storage
	R2AccountID       string
	R2AccessKeyID     string
	R2SecretAccessKey string
	R2BucketName      string
	R2PublicURL       string

	// File Settings
	MaxFileSize        int64
	MaxDuration        int
	PresignedURLExpiry time.Duration

	// Cleanup
	LocalCleanupInterval time.Duration
	R2CleanupInterval    time.Duration
	R2MaxFileAge         time.Duration

	// Paths
	TempDir string
	DataDir string
}

// Load loads configuration from environment variables.
func Load() (*Config, error) {
	// Load .env file if it exists (ignore error if not found)
	if err := godotenv.Load(); err != nil {
		slog.Debug("No .env file found, using environment variables")
	}

	cfg := &Config{
		// Server
		Port:     getEnv("PORT", "8080"),
		Env:      getEnv("ENV", "development"),
		LogLevel: getEnv("LOG_LEVEL", "info"),

		// CORS
		AllowedOrigins: strings.Split(getEnv("ALLOWED_ORIGINS", "http://localhost:3000"), ","),

		// Turnstile
		TurnstileSecretKey: getEnv("TURNSTILE_SECRET_KEY", ""),
		TurnstileSkip:      getEnvBool("TURNSTILE_SKIP", false),

		// Rate Limiting
		RateLimitRPM:   getEnvInt("RATE_LIMIT_RPM", 5),
		RateLimitBurst: getEnvInt("RATE_LIMIT_BURST", 2),

		// Worker Pool
		MaxWorkers:   getEnvInt("MAX_WORKERS", 3),
		MaxQueueSize: getEnvInt("MAX_QUEUE_SIZE", 10),

		// R2 Storage
		R2AccountID:       getEnv("R2_ACCOUNT_ID", ""),
		R2AccessKeyID:     getEnv("R2_ACCESS_KEY_ID", ""),
		R2SecretAccessKey: getEnv("R2_SECRET_ACCESS_KEY", ""),
		R2BucketName:      getEnv("R2_BUCKET_NAME", ""),
		R2PublicURL:       getEnv("R2_PUBLIC_URL", ""),

		// File Settings
		MaxFileSize:        getEnvInt64("MAX_FILE_SIZE", 524288000), // 500MB
		MaxDuration:        getEnvInt("MAX_DURATION", 1800),         // 30 minutes
		PresignedURLExpiry: time.Duration(getEnvInt("PRESIGNED_URL_EXPIRY", 15)) * time.Minute,

		// Cleanup
		LocalCleanupInterval: time.Duration(getEnvInt("LOCAL_CLEANUP_INTERVAL", 5)) * time.Minute,
		R2CleanupInterval:    time.Duration(getEnvInt("R2_CLEANUP_INTERVAL", 30)) * time.Minute,
		R2MaxFileAge:         time.Duration(getEnvInt("R2_MAX_FILE_AGE", 60)) * time.Minute,

		// Paths
		TempDir: getEnv("TEMP_DIR", "./tmp"),
		DataDir: getEnv("DATA_DIR", "./data"),
	}

	return cfg, nil
}

// IsDevelopment returns true if running in development mode.
func (c *Config) IsDevelopment() bool {
	return c.Env == "development"
}

// IsProduction returns true if running in production mode.
func (c *Config) IsProduction() bool {
	return c.Env == "production"
}

// Helper functions

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvInt64(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.ParseInt(value, 10, 64); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}
