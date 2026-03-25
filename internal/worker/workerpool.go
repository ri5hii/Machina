package worker

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/ri5hii/Machina/internal/jobs"
	"github.com/ri5hii/Machina/internal/storage"
	"golang.org/x/sync/errgroup"
)

type WorkerPool struct {
	workerCount int
	queue       <-chan jobs.JobSubmission
	store       *storage.JobStore
	logger      *slog.Logger
	waitGroup   sync.WaitGroup
}

func New(workerCount int, queue <-chan jobs.JobSubmission, store *storage.JobStore, logger *slog.Logger) *WorkerPool {
	return &WorkerPool{
		workerCount: workerCount,
		queue:       queue,
		store:       store,
		logger:      logger,
	}
}

func (pool *WorkerPool) Start(ctx context.Context) {
	pool.logger.Info("Starting worker pool", "Worker count", pool.workerCount)
	pool.waitGroup.Add(pool.workerCount)
	for i := 0; i < pool.workerCount; i++ {
		go func(WorkerID int) {
			defer pool.waitGroup.Done()

			pool.logger.Info("Worker initiated", "Worker ID", WorkerID)
			pool.worker(ctx, WorkerID)
			pool.logger.Info("Worker stopped", "Worker ID", WorkerID)
		}(i)
	}
}

func (pool *WorkerPool) Wait() {
	pool.waitGroup.Wait()
}

func (pool *WorkerPool) worker(ctx context.Context, WorkerID int) {
	for {
		select {
		case <-ctx.Done():
			pool.logger.Info("Worker shutting down via context", "Worker ID", WorkerID)
			return
		case submission, ok := <-pool.queue:
			if !ok {
				pool.logger.Info("Queue closed, worker exiting", "Worker ID", WorkerID)
				return
			}

			pool.logger.Info("Job started", "Worker ID", WorkerID, "Job ID", submission.JobID)
			pool.store.SetStatus(submission.JobID, storage.StatusRunning)

			result, err := pool.SafeExecute(ctx, submission, WorkerID)
			if err != nil {
				pool.store.SetError(submission.JobID, err)
				pool.store.SetStatus(submission.JobID, storage.StatusFailed)
				pool.logger.Error("Job failed", "Worker ID", WorkerID, "Job ID", submission.JobID, "Error", err)

				continue
			}

			pool.store.SetResult(submission.JobID, result)
			pool.store.SetStatus(submission.JobID, storage.StatusCompleted)
			pool.logger.Info("Job completed", "Worker ID", WorkerID, "Job ID", submission.JobID)
		}
	}
}

func (pool *WorkerPool) SafeExecute(ctx context.Context, submission jobs.JobSubmission, workerID int) (result any, err error) {
	defer func() {
		r := recover()
		if r != nil {
			err = fmt.Errorf("Job panicked: %v", r)
			pool.logger.Error("Panic recovered in job execution", "Worker ID", workerID, "Job ID", submission.JobID, "Panic", r)
		}
	}()

	switch JobType := submission.Job.(type) {
	case jobs.BatchProcessingJob:
		return executeBatch(ctx, JobType, submission.JobID, pool.logger)
	case jobs.ParallelProcessingJob:
		return JobType.Run(ctx)
	default:
		return nil, fmt.Errorf("Job %q is not implemented in any job profile", JobType)
	}
}

func executeBatch(ctx context.Context, job jobs.BatchProcessingJob, jobID string, logger *slog.Logger) (any, error) {
	items, err := job.Scan()
	if err != nil {
		return nil, fmt.Errorf("Batch scan failed: %w", err)
	}

	if len(items) == 0 {
		return job.Aggregate(nil)
	}

	chunks := partition(items, job.ChunkSize())
	logger.Info("Batch dispatching chunks",
		"JobID", jobID,
		"TotalItems", len(items),
		"ChunkSize", job.ChunkSize(),
		"TotalChunks", len(chunks),
	)

	partials := make([]any, len(chunks))

	errGroup, errGroupCtx := errgroup.WithContext(ctx)

	BatchStartTime := time.Now()

	for i := 0; i < len(chunks); i++ {
		x, chunk := i, chunks[i]
		errGroup.Go(func() error {
			chunkStartTime := time.Now()
			logger.Info("Chunk started",
				"JobID", jobID,
				"Chunk", x,
				"Items", len(chunk),
				"Time", chunkStartTime,
			)

			partial, err := job.RunBatch(errGroupCtx, chunk)
			if err != nil {
				return fmt.Errorf("Chunk %d failed: %w", x, err)
			}

			partials[x] = partial

			logger.Info("Chunk done",
				"JobID", jobID,
				"Chunk", x,
				"Items", len(chunk),
				"Duration", time.Since(chunkStartTime).Round(time.Microsecond).String(),
			)
			return nil
		})
	}

	err = errGroup.Wait()
	if err != nil {
		return nil, fmt.Errorf("Batch run failed: %w", err)
	}

	logger.Info("Batch done",
		"JobID", jobID,
		"Total items", len(items),
		"Total chunks", len(chunks),
		"Duration", time.Since(BatchStartTime).Round(time.Microsecond).String(),
	)

	result, err := job.Aggregate(partials)
	if err != nil {
		return nil, fmt.Errorf("Batch aggregate failed: %w", err)
	}

	return result, nil
}

func partition(items []jobs.Item, chunkSize int) [][]jobs.Item {
	if chunkSize <= 0 {
		return [][]jobs.Item{items}
	}

	var chunks [][]jobs.Item
	for chunkSize < len(items) {
		items, chunks = items[chunkSize:], append(chunks, items[:chunkSize])
	}
	return append(chunks, items)
}
