// Package domain contains the core business entities and types.
package domain

import (
	"time"
)

// JobStatus represents the current state of a download job.
type JobStatus string

const (
	JobStatusPending    JobStatus = "pending"
	JobStatusProcessing JobStatus = "processing"
	JobStatusDone       JobStatus = "done"
	JobStatusError      JobStatus = "error"
)

// Job represents a download job in the system.
type Job struct {
	ID          string     `json:"id"`
	URL         string     `json:"url"`
	Title       string     `json:"title,omitempty"`
	Status      JobStatus  `json:"status"`
	FileKey     string     `json:"-"` // R2 object key (internal use)
	FilePath    string     `json:"-"` // Local file path (internal use)
	DownloadURL string     `json:"download_url,omitempty"`
	Progress    int        `json:"progress"` // 0-100
	Error       string     `json:"error,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// NewJob creates a new Job with the given URL.
func NewJob(id, url string) *Job {
	return &Job{
		ID:        id,
		URL:       url,
		Status:    JobStatusPending,
		Progress:  0,
		CreatedAt: time.Now().UTC(),
	}
}

// MarkProcessing updates the job status to processing.
func (j *Job) MarkProcessing() {
	j.Status = JobStatusProcessing
}

// MarkDone updates the job status to done with the download URL.
func (j *Job) MarkDone(downloadURL string) {
	j.Status = JobStatusDone
	j.DownloadURL = downloadURL
	j.Progress = 100
	now := time.Now().UTC()
	j.CompletedAt = &now
}

// MarkError updates the job status to error with the error message.
func (j *Job) MarkError(err string) {
	j.Status = JobStatusError
	j.Error = err
	now := time.Now().UTC()
	j.CompletedAt = &now
}

// UpdateProgress updates the job progress percentage.
func (j *Job) UpdateProgress(progress int) {
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	j.Progress = progress
}

// VideoInfo contains metadata about a video.
type VideoInfo struct {
	Title       string  `json:"title"`
	Duration    float64 `json:"duration"` // in seconds
	Thumbnail   string  `json:"thumbnail,omitempty"`
	Filesize    int64   `json:"filesize,omitempty"`
	Filename    string  `json:"filename,omitempty"`
	Extractor   string  `json:"extractor,omitempty"`
	WebpageURL  string  `json:"webpage_url,omitempty"`
	Description string  `json:"description,omitempty"`
}

// DownloadRequest represents a request to download a video.
type DownloadRequest struct {
	URL       string `json:"url"`
	Turnstile string `json:"turnstile,omitempty"`
}

// DownloadResponse represents the response after enqueuing a download.
type DownloadResponse struct {
	JobID string `json:"job_id"`
}

// StatusResponse represents the response for a job status check.
type StatusResponse struct {
	ID          string     `json:"id"`
	Status      JobStatus  `json:"status"`
	Progress    int        `json:"progress"`
	Title       string     `json:"title,omitempty"`
	DownloadURL string     `json:"download_url,omitempty"`
	Error       string     `json:"error,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// HealthResponse represents the response for a health check.
type HealthResponse struct {
	Status    string `json:"status"`
	QueueSize int    `json:"queue_size"`
	Workers   int    `json:"workers"`
}

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}

// ToStatusResponse converts a Job to a StatusResponse.
func (j *Job) ToStatusResponse() *StatusResponse {
	return &StatusResponse{
		ID:          j.ID,
		Status:      j.Status,
		Progress:    j.Progress,
		Title:       j.Title,
		DownloadURL: j.DownloadURL,
		Error:       j.Error,
		CreatedAt:   j.CreatedAt,
		CompletedAt: j.CompletedAt,
	}
}
