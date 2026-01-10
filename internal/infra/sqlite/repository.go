// Package sqlite provides SQLite database operations.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/emanuelef/yt-dl-api-go/internal/domain"
	_ "modernc.org/sqlite"
)

// Repository provides database operations for jobs.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new Repository with the given database path.
func NewRepository(dataDir string) (*Repository, error) {
	// Ensure data directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	dbPath := filepath.Join(dataDir, "jobs.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool for SQLite (single writer)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(time.Hour)

	// Enable WAL mode and other optimizations
	if err := configureDB(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to configure database: %w", err)
	}

	// Create schema
	if err := createSchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create schema: %w", err)
	}

	slog.Info("Database initialized", "path", dbPath)

	return &Repository{db: db}, nil
}

// configureDB applies SQLite optimizations.
func configureDB(db *sql.DB) error {
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA cache_size=10000",
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			return fmt.Errorf("failed to execute %s: %w", pragma, err)
		}
	}

	return nil
}

// createSchema creates the database tables.
func createSchema(db *sql.DB) error {
	schema := `
		CREATE TABLE IF NOT EXISTS jobs (
			id TEXT PRIMARY KEY,
			url TEXT NOT NULL,
			title TEXT,
			status TEXT DEFAULT 'pending',
			file_key TEXT,
			file_path TEXT,
			download_url TEXT,
			progress INTEGER DEFAULT 0,
			error TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			completed_at DATETIME
		);

		CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
		CREATE INDEX IF NOT EXISTS idx_jobs_created ON jobs(created_at);
	`

	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	return nil
}

// Close closes the database connection.
func (r *Repository) Close() error {
	return r.db.Close()
}

// Create inserts a new job into the database.
func (r *Repository) Create(ctx context.Context, job *domain.Job) error {
	query := `
		INSERT INTO jobs (id, url, title, status, file_key, file_path, download_url, progress, error, created_at, completed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(ctx, query,
		job.ID,
		job.URL,
		job.Title,
		job.Status,
		job.FileKey,
		job.FilePath,
		job.DownloadURL,
		job.Progress,
		job.Error,
		job.CreatedAt,
		job.CompletedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create job: %w", err)
	}

	return nil
}

// GetByID retrieves a job by its ID.
func (r *Repository) GetByID(ctx context.Context, id string) (*domain.Job, error) {
	query := `
		SELECT id, url, title, status, file_key, file_path, download_url, progress, error, created_at, completed_at
		FROM jobs
		WHERE id = ?
	`

	job := &domain.Job{}
	var fileKey, filePath, downloadURL, errorMsg sql.NullString
	var completedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&job.ID,
		&job.URL,
		&job.Title,
		&job.Status,
		&fileKey,
		&filePath,
		&downloadURL,
		&job.Progress,
		&errorMsg,
		&job.CreatedAt,
		&completedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	job.FileKey = fileKey.String
	job.FilePath = filePath.String
	job.DownloadURL = downloadURL.String
	job.Error = errorMsg.String
	if completedAt.Valid {
		job.CompletedAt = &completedAt.Time
	}

	return job, nil
}

// Update updates an existing job.
func (r *Repository) Update(ctx context.Context, job *domain.Job) error {
	query := `
		UPDATE jobs
		SET title = ?, status = ?, file_key = ?, file_path = ?, download_url = ?, progress = ?, error = ?, completed_at = ?
		WHERE id = ?
	`

	result, err := r.db.ExecContext(ctx, query,
		job.Title,
		job.Status,
		job.FileKey,
		job.FilePath,
		job.DownloadURL,
		job.Progress,
		job.Error,
		job.CompletedAt,
		job.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update job: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("job not found: %s", job.ID)
	}

	return nil
}

// UpdateProgress updates only the progress of a job.
func (r *Repository) UpdateProgress(ctx context.Context, id string, progress int) error {
	query := `UPDATE jobs SET progress = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, progress, id)
	return err
}

// UpdateStatus updates only the status of a job.
func (r *Repository) UpdateStatus(ctx context.Context, id string, status domain.JobStatus) error {
	query := `UPDATE jobs SET status = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, status, id)
	return err
}

// ListPending returns all pending jobs.
func (r *Repository) ListPending(ctx context.Context) ([]*domain.Job, error) {
	return r.listByStatus(ctx, domain.JobStatusPending)
}

// ListProcessing returns all processing jobs.
func (r *Repository) ListProcessing(ctx context.Context) ([]*domain.Job, error) {
	return r.listByStatus(ctx, domain.JobStatusProcessing)
}

// listByStatus returns jobs with the given status.
func (r *Repository) listByStatus(ctx context.Context, status domain.JobStatus) ([]*domain.Job, error) {
	query := `
		SELECT id, url, title, status, file_key, file_path, download_url, progress, error, created_at, completed_at
		FROM jobs
		WHERE status = ?
		ORDER BY created_at ASC
	`

	rows, err := r.db.QueryContext(ctx, query, status)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*domain.Job

	for rows.Next() {
		job := &domain.Job{}
		var fileKey, filePath, downloadURL, errorMsg sql.NullString
		var completedAt sql.NullTime

		err := rows.Scan(
			&job.ID,
			&job.URL,
			&job.Title,
			&job.Status,
			&fileKey,
			&filePath,
			&downloadURL,
			&job.Progress,
			&errorMsg,
			&job.CreatedAt,
			&completedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan job: %w", err)
		}

		job.FileKey = fileKey.String
		job.FilePath = filePath.String
		job.DownloadURL = downloadURL.String
		job.Error = errorMsg.String
		if completedAt.Valid {
			job.CompletedAt = &completedAt.Time
		}

		jobs = append(jobs, job)
	}

	return jobs, rows.Err()
}

// DeleteOlderThan deletes jobs older than the specified duration.
func (r *Repository) DeleteOlderThan(ctx context.Context, age time.Duration) (int64, error) {
	threshold := time.Now().Add(-age)

	query := `DELETE FROM jobs WHERE created_at < ?`
	result, err := r.db.ExecContext(ctx, query, threshold)
	if err != nil {
		return 0, fmt.Errorf("failed to delete old jobs: %w", err)
	}

	return result.RowsAffected()
}

// Count returns the total number of jobs.
func (r *Repository) Count(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM jobs").Scan(&count)
	return count, err
}

// CountByStatus returns the number of jobs with the given status.
func (r *Repository) CountByStatus(ctx context.Context, status domain.JobStatus) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM jobs WHERE status = ?", status).Scan(&count)
	return count, err
}
