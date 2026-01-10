// Package fs provides filesystem cleanup operations.
package fs

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/emanuelef/yt-dl-api-go/internal/infra/r2"
)

// Cleaner handles automated cleanup of files.
type Cleaner struct {
	localDir      string
	localMaxAge   time.Duration
	localInterval time.Duration

	r2Client   *r2.Client
	r2MaxAge   time.Duration
	r2Interval time.Duration

	stopCh chan struct{}
}

// CleanerConfig holds configuration for the cleaner.
type CleanerConfig struct {
	LocalDir      string
	LocalMaxAge   time.Duration
	LocalInterval time.Duration

	R2Client   *r2.Client
	R2MaxAge   time.Duration
	R2Interval time.Duration
}

// NewCleaner creates a new Cleaner.
func NewCleaner(cfg *CleanerConfig) *Cleaner {
	return &Cleaner{
		localDir:      cfg.LocalDir,
		localMaxAge:   cfg.LocalMaxAge,
		localInterval: cfg.LocalInterval,
		r2Client:      cfg.R2Client,
		r2MaxAge:      cfg.R2MaxAge,
		r2Interval:    cfg.R2Interval,
		stopCh:        make(chan struct{}),
	}
}

// Start starts the cleanup goroutines.
func (c *Cleaner) Start(ctx context.Context) {
	// Start local cleanup
	if c.localDir != "" && c.localInterval > 0 {
		go c.startLocalCleanup(ctx)
	}

	// Start R2 cleanup
	if c.r2Client != nil && c.r2Interval > 0 {
		go c.startR2Cleanup(ctx)
	}
}

// Stop stops the cleanup goroutines.
func (c *Cleaner) Stop() {
	close(c.stopCh)
}

// startLocalCleanup runs periodic local file cleanup.
func (c *Cleaner) startLocalCleanup(ctx context.Context) {
	slog.Info("Starting local cleanup",
		"dir", c.localDir,
		"max_age", c.localMaxAge,
		"interval", c.localInterval,
	)

	ticker := time.NewTicker(c.localInterval)
	defer ticker.Stop()

	// Run immediately on start
	c.cleanupLocal()

	for {
		select {
		case <-ticker.C:
			c.cleanupLocal()
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		}
	}
}

// cleanupLocal removes old local files.
func (c *Cleaner) cleanupLocal() {
	threshold := time.Now().Add(-c.localMaxAge)
	deleted := 0

	err := filepath.Walk(c.localDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and the directory itself
		if info.IsDir() {
			return nil
		}

		// Check if file is old enough
		if info.ModTime().Before(threshold) {
			if err := os.Remove(path); err != nil {
				slog.Warn("Failed to delete local file",
					"path", path,
					"error", err,
				)
			} else {
				deleted++
			}
		}

		return nil
	})

	if err != nil {
		slog.Error("Local cleanup error",
			"dir", c.localDir,
			"error", err,
		)
	}

	if deleted > 0 {
		slog.Info("Local cleanup completed",
			"deleted", deleted,
			"max_age", c.localMaxAge,
		)
	}
}

// startR2Cleanup runs periodic R2 file cleanup.
func (c *Cleaner) startR2Cleanup(ctx context.Context) {
	slog.Info("Starting R2 cleanup",
		"max_age", c.r2MaxAge,
		"interval", c.r2Interval,
	)

	ticker := time.NewTicker(c.r2Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.cleanupR2(ctx)
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		}
	}
}

// cleanupR2 removes old files from R2.
func (c *Cleaner) cleanupR2(ctx context.Context) {
	deleted, err := c.r2Client.DeleteOlderThan(ctx, c.r2MaxAge)
	if err != nil {
		slog.Error("R2 cleanup error", "error", err)
		return
	}

	if deleted > 0 {
		slog.Info("R2 cleanup completed",
			"deleted", deleted,
			"max_age", c.r2MaxAge,
		)
	}
}

// CleanupLocalNow performs an immediate local cleanup.
func (c *Cleaner) CleanupLocalNow() {
	c.cleanupLocal()
}

// CleanupR2Now performs an immediate R2 cleanup.
func (c *Cleaner) CleanupR2Now(ctx context.Context) {
	c.cleanupR2(ctx)
}
