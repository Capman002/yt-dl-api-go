// Package queue provides a worker pool for processing download jobs.
package queue

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/emanuelef/yt-dl-api-go/internal/domain"
)

var (
	// ErrQueueFull is returned when the job queue is at capacity.
	ErrQueueFull = errors.New("job queue is full")
	// ErrDispatcherStopped is returned when trying to enqueue after dispatcher is stopped.
	ErrDispatcherStopped = errors.New("dispatcher has been stopped")
)

// JobProcessor is the function signature for processing a job.
type JobProcessor func(ctx context.Context, job *domain.Job)

// ProgressCallback is called when a job's progress updates.
type ProgressCallback func(jobID string, progress int, status string)

// Dispatcher manages a pool of workers that process download jobs.
type Dispatcher struct {
	jobChan    chan *domain.Job
	workerWg   sync.WaitGroup
	numWorkers int
	processor  JobProcessor
	progressCb ProgressCallback
	stopped    atomic.Bool
	stopCh     chan struct{}
	mu         sync.Mutex
}

// NewDispatcher creates a new Dispatcher with the given configuration.
func NewDispatcher(numWorkers, queueSize int, processor JobProcessor) *Dispatcher {
	if numWorkers < 1 {
		numWorkers = 1
	}
	if queueSize < 1 {
		queueSize = 10
	}

	return &Dispatcher{
		jobChan:    make(chan *domain.Job, queueSize),
		numWorkers: numWorkers,
		processor:  processor,
		stopCh:     make(chan struct{}),
	}
}

// SetProgressCallback sets the callback for progress updates.
func (d *Dispatcher) SetProgressCallback(cb ProgressCallback) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.progressCb = cb
}

// Start starts the worker pool.
func (d *Dispatcher) Start(ctx context.Context) {
	slog.Info("Starting dispatcher",
		"workers", d.numWorkers,
		"queue_size", cap(d.jobChan),
	)

	for i := 0; i < d.numWorkers; i++ {
		d.workerWg.Add(1)
		go d.worker(ctx, i)
	}
}

// worker processes jobs from the job channel.
func (d *Dispatcher) worker(ctx context.Context, id int) {
	defer d.workerWg.Done()

	slog.Debug("Worker started", "worker_id", id)

	for {
		select {
		case job, ok := <-d.jobChan:
			if !ok {
				slog.Debug("Worker stopping (channel closed)", "worker_id", id)
				return
			}

			slog.Debug("Worker processing job",
				"worker_id", id,
				"job_id", job.ID,
				"url", job.URL,
			)

			if d.processor != nil {
				d.processor(ctx, job)
			}

		case <-ctx.Done():
			slog.Debug("Worker stopping (context canceled)", "worker_id", id)
			return

		case <-d.stopCh:
			slog.Debug("Worker stopping (stop signal)", "worker_id", id)
			return
		}
	}
}

// Enqueue adds a job to the queue.
// Returns ErrQueueFull if the queue is at capacity.
func (d *Dispatcher) Enqueue(job *domain.Job) error {
	if d.stopped.Load() {
		return ErrDispatcherStopped
	}

	select {
	case d.jobChan <- job:
		slog.Debug("Job enqueued",
			"job_id", job.ID,
			"queue_size", len(d.jobChan),
		)
		return nil
	default:
		slog.Warn("Queue is full",
			"job_id", job.ID,
			"queue_size", len(d.jobChan),
		)
		return ErrQueueFull
	}
}

// Stop gracefully stops the dispatcher.
func (d *Dispatcher) Stop() {
	if d.stopped.Swap(true) {
		return // Already stopped
	}

	slog.Info("Stopping dispatcher...")

	// Signal workers to stop
	close(d.stopCh)

	// Close job channel to unblock workers
	close(d.jobChan)

	// Wait for all workers to finish
	d.workerWg.Wait()

	slog.Info("Dispatcher stopped")
}

// QueueSize returns the current number of jobs in the queue.
func (d *Dispatcher) QueueSize() int {
	return len(d.jobChan)
}

// QueueCapacity returns the maximum capacity of the queue.
func (d *Dispatcher) QueueCapacity() int {
	return cap(d.jobChan)
}

// IsFull returns true if the queue is at capacity.
func (d *Dispatcher) IsFull() bool {
	return len(d.jobChan) >= cap(d.jobChan)
}

// WorkerCount returns the number of workers.
func (d *Dispatcher) WorkerCount() int {
	return d.numWorkers
}
