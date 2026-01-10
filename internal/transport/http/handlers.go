// Package http provides HTTP handlers and router configuration.
package http

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/emanuelef/yt-dl-api-go/internal/domain"
	"github.com/emanuelef/yt-dl-api-go/internal/infra/r2"
	"github.com/emanuelef/yt-dl-api-go/internal/infra/sqlite"
	"github.com/emanuelef/yt-dl-api-go/internal/service/downloader"
	"github.com/emanuelef/yt-dl-api-go/internal/service/queue"
	"github.com/emanuelef/yt-dl-api-go/internal/transport/http/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Handlers contains all HTTP handlers and their dependencies.
type Handlers struct {
	repo       *sqlite.Repository
	dispatcher *queue.Dispatcher
	downloader *downloader.Downloader
	r2Client   *r2.Client
}

// NewHandlers creates a new Handlers instance.
func NewHandlers(repo *sqlite.Repository, dispatcher *queue.Dispatcher, dl *downloader.Downloader, r2Client *r2.Client) *Handlers {
	return &Handlers{
		repo:       repo,
		dispatcher: dispatcher,
		downloader: dl,
		r2Client:   r2Client,
	}
}

// HealthHandler handles GET /api/health requests.
func (h *Handlers) HealthHandler(w http.ResponseWriter, r *http.Request) {
	response := &domain.HealthResponse{
		Status:    "ok",
		QueueSize: h.dispatcher.QueueSize(),
		Workers:   h.dispatcher.WorkerCount(),
	}

	writeJSON(w, http.StatusOK, response)
}

// DownloadHandler handles POST /api/download requests.
func (h *Handlers) DownloadHandler(w http.ResponseWriter, r *http.Request) {
	// Parse request body
	var req domain.DownloadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "INVALID_BODY")
		return
	}

	// Validate URL
	if err := middleware.ValidateURL(req.URL); err != nil {
		slog.Warn("URL validation failed",
			"url", req.URL,
			"error", err,
			"ip", middleware.GetClientIP(r),
		)
		writeError(w, http.StatusBadRequest, err.Error(), "INVALID_URL")
		return
	}

	// Check if queue is full
	if h.dispatcher.IsFull() {
		writeError(w, http.StatusServiceUnavailable, "server is busy, please try again later", "QUEUE_FULL")
		return
	}

	// Create job
	jobID := uuid.New().String()
	job := domain.NewJob(jobID, middleware.NormalizeURL(req.URL))

	// Save to database
	if err := h.repo.Create(r.Context(), job); err != nil {
		slog.Error("Failed to create job",
			"error", err,
			"job_id", jobID,
		)
		writeError(w, http.StatusInternalServerError, "failed to create job", "DB_ERROR")
		return
	}

	// Enqueue job
	if err := h.dispatcher.Enqueue(job); err != nil {
		slog.Error("Failed to enqueue job",
			"error", err,
			"job_id", jobID,
		)
		// Update job status to error
		job.MarkError("failed to enqueue job")
		h.repo.Update(r.Context(), job)

		writeError(w, http.StatusServiceUnavailable, "server is busy, please try again later", "QUEUE_FULL")
		return
	}

	slog.Info("Download job created",
		"job_id", jobID,
		"url", req.URL,
		"ip", middleware.GetClientIP(r),
	)

	// Return job ID immediately
	writeJSON(w, http.StatusAccepted, &domain.DownloadResponse{
		JobID: jobID,
	})
}

// StatusHandler handles GET /api/status/{job_id} requests.
func (h *Handlers) StatusHandler(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "job_id")
	if jobID == "" {
		writeError(w, http.StatusBadRequest, "job_id is required", "MISSING_JOB_ID")
		return
	}

	// Validate UUID format
	if _, err := uuid.Parse(jobID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid job_id format", "INVALID_JOB_ID")
		return
	}

	// Get job from database
	job, err := h.repo.GetByID(r.Context(), jobID)
	if err != nil {
		slog.Error("Failed to get job",
			"error", err,
			"job_id", jobID,
		)
		writeError(w, http.StatusInternalServerError, "failed to get job status", "DB_ERROR")
		return
	}

	if job == nil {
		writeError(w, http.StatusNotFound, "job not found", "JOB_NOT_FOUND")
		return
	}

	writeJSON(w, http.StatusOK, job.ToStatusResponse())
}

// ProcessJob processes a download job (called by the worker pool).
func (h *Handlers) ProcessJob(ctx context.Context, job *domain.Job) {
	slog.Info("Processing job",
		"job_id", job.ID,
		"url", job.URL,
	)

	// Update status to processing
	job.MarkProcessing()
	if err := h.repo.Update(ctx, job); err != nil {
		slog.Error("Failed to update job status",
			"error", err,
			"job_id", job.ID,
		)
	}

	// Progress callback
	progressCb := func(progress int, status string) {
		job.UpdateProgress(progress)
		if err := h.repo.UpdateProgress(ctx, job.ID, progress); err != nil {
			slog.Debug("Failed to update progress",
				"error", err,
				"job_id", job.ID,
			)
		}
	}

	// Download the video
	videoInfo, filePath, err := h.downloader.Download(ctx, job.URL, progressCb)
	if err != nil {
		slog.Error("Download failed",
			"error", err,
			"job_id", job.ID,
			"url", job.URL,
		)
		job.MarkError(err.Error())
		h.repo.Update(ctx, job)
		return
	}

	// Update job with video info
	if videoInfo != nil {
		job.Title = videoInfo.Title
	}
	job.FilePath = filePath

	// Upload to R2 if client is configured
	if h.r2Client != nil {
		fileKey := job.ID + "/" + job.Title
		if err := h.r2Client.Upload(ctx, filePath, fileKey); err != nil {
			slog.Error("Failed to upload to R2",
				"error", err,
				"job_id", job.ID,
			)
			// Fall back to local file (or error depending on requirements)
			job.MarkError("failed to upload file")
			h.repo.Update(ctx, job)
			return
		}

		job.FileKey = fileKey

		// Generate presigned URL
		downloadURL, err := h.r2Client.GeneratePresignedURL(ctx, fileKey, 15) // 15 minutes
		if err != nil {
			slog.Error("Failed to generate presigned URL",
				"error", err,
				"job_id", job.ID,
			)
			job.MarkError("failed to generate download URL")
			h.repo.Update(ctx, job)
			return
		}

		job.MarkDone(downloadURL)

		// Clean up local file after upload
		h.downloader.Cleanup(filePath)
	} else {
		// No R2 client, keep local file
		job.MarkDone(filePath)
	}

	if err := h.repo.Update(ctx, job); err != nil {
		slog.Error("Failed to update job",
			"error", err,
			"job_id", job.ID,
		)
	}

	slog.Info("Job completed",
		"job_id", job.ID,
		"title", job.Title,
	)
}

// Helper functions

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("Failed to encode JSON response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, message, code string) {
	writeJSON(w, status, &domain.ErrorResponse{
		Error: message,
		Code:  code,
	})
}
