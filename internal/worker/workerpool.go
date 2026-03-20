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

func New(workerCount int, queue chan jobs.JobSubmission, store *storage.JobStore, logger *slog.Logger) *WorkerPool {
	return &WorkerPool{
		workerCount: workerCount,
		queue:       queue,
		store:       store,
		logger:      logger,
	}
}

func (pool *WorkerPool) Start(ctx context.Context) {
	pool.logger.Info("Starting worker pool", "worker count", pool.workerCount)
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
			pool.logger.Info("worker shutting down via context", "Worker ID", WorkerID)
			return
		case submission, ok := <-pool.queue:
			if !ok {
				pool.logger.Info("queue closed, worker exiting", "Worker ID", WorkerID)
				return
			}

			pool.logger.Info("job started", "Worker ID", WorkerID, "Job ID", submission.JobID)
			pool.store.SetStatus(submission.JobID, storage.StatusRunning)

			result, err := pool.SafeExecute(ctx, submission, WorkerID)
			if err != nil {
				pool.store.SetError(submission.JobID, err)
				pool.store.SetStatus(submission.JobID, storage.StatusFailed)
				pool.logger.Error("job failed", "Worker ID", WorkerID, "Job ID", submission.JobID, "error", err)

				continue
			}

			pool.store.SetResult(submission.JobID, result)
			pool.store.SetStatus(submission.JobID, storage.StatusCompleted)
			pool.logger.Info("job completed", "Worker ID", WorkerID, "Job ID", submission.JobID)
		}
	}
}

func (pool *WorkerPool) SafeExecute(ctx context.Context, submission jobs.JobSubmission, workerID int) (result any, err error) {
	defer func() {
		r := recover()
		if r != nil {
			err = fmt.Errorf("Job panicked: %v", r)
			pool.logger.Error("panic recovered in job execution", "Worker ID", workerID, "Job ID", submission.JobID, "panic", r)
		}
	}()

	switch JobType := submission.Job.(type) {
	case jobs.BatchProcessingJob:
		return executeBatch(ctx, JobType, submission.JobID, pool.logger)
	case jobs.ParallelProcessingJob:
		return JobType.Run(ctx)
	default:
		return nil, fmt.Errorf("job %q is not implemented in any job profile", JobType)
	}
}

func executeBatch(ctx context.Context, job jobs.BatchProcessingJob, jobID string, logger *slog.Logger) (any, error) {
	items, err := job.Scan()
	if err != nil {
		return nil, fmt.Errorf("batch scan failed: %w", err)
	}

	if len(items) == 0 {
		return job.Aggregate(nil)
	}

	chunks := partition(items, job.ChunkSize())
	logger.Info("batch dispatching chunks",
		"jobID", jobID,
		"totalItems", len(items),
		"chunkSize", job.ChunkSize(),
		"totalChunks", len(chunks),
	)

	partials := make([]any, len(chunks))

	errGroup, errGroupCtx := errgroup.WithContext(ctx)

	BatchStartTime := time.Now()

	for i := 0; i < len(chunks); i++ {
		chunk := chunks[i]
		errGroup.Go(func() error {
			chunkStartTime := time.Now()
			logger.Info("chunk started",
				"jobID", jobID,
				"chunk", i,
				"items", len(chunk),
				"time", chunkStartTime,
			)

			partial, err := job.RunBatch(errGroupCtx, chunk)
			if err != nil {
				return fmt.Errorf("chunk %d failed: %w", i, err)
			}

			partials[i] = partial

			logger.Info("chunk done",
				"jobID", jobID,
				"chunk", i,
				"items", len(chunk),
				"duration", time.Since(chunkStartTime).Round(time.Microsecond).String(),
			)
			return nil
		})
	}

	err = errGroup.Wait()
	if err != nil {
		return nil, fmt.Errorf("batch run failed: %w", err)
	}

	logger.Info("batch done",
		"jobID", jobID,
		"total items", len(items),
		"total chunks", len(chunks),
		"duration", time.Since(BatchStartTime).Round(time.Microsecond).String(),
	)

	result, err := job.Aggregate(partials)
	if err != nil {
		return nil, fmt.Errorf("batch aggregate failed: %w", err)
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
