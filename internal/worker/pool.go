package worker

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/ri5hii/Machina/internal/jobs"
	"github.com/ri5hii/Machina/internal/storage"
)

type WorkerPool struct {
	workerCount int
	queue       <-chan jobs.Submission
	store       *storage.Store
	logger      *slog.Logger
	waitgroup   sync.WaitGroup
}

func New(workerCount int, queue <-chan jobs.Submission, store *storage.Store, log *slog.Logger) *WorkerPool {
	return &WorkerPool{
		workerCount: workerCount,
		queue:       queue,
		store:       store,
		logger:      log,
	}
}

func (wp *WorkerPool) Wait() {
	wp.waitgroup.Wait()
}

func (wp *WorkerPool) Start(ctx context.Context) {
	wp.logger.Info("starting worker pool", "workerCount", wp.workerCount)
	wp.waitgroup.Add(wp.workerCount)
	for i := 0; i < wp.workerCount; i++ {
		go func(workerID int) {
			defer wp.waitgroup.Done()
			wp.logger.Info("worker started", "workerID", workerID)
			wp.worker(ctx, workerID)
			wp.logger.Info("worker stopped", "workerID", workerID)
		}(i)
	}
}

func (wp *WorkerPool) worker(ctx context.Context, workerID int) {
	for {
		select {
		case <-ctx.Done():
			wp.logger.Info("worker shutting down via context", "workerID", workerID)
			return

		case submission, ok := <-wp.queue:
			if !ok {
				wp.logger.Info("queue closed, worker exiting", "workerID", workerID)
				return
			}

			wp.logger.Info("job started", "workerID", workerID, "jobID", submission.ID)
			wp.store.SetStatus(submission.ID, storage.StatusRunning)

			result, err := wp.safeExecute(ctx, submission, workerID)
			if err != nil {
				wp.store.SetError(submission.ID, err)
				wp.store.SetStatus(submission.ID, storage.StatusFailed)
				wp.logger.Error("job failed", "workerID", workerID, "jobID", submission.ID, "error", err)
				continue
			}

			wp.store.SetResult(submission.ID, result)
			wp.store.SetStatus(submission.ID, storage.StatusCompleted)
			wp.logger.Info("job completed", "workerID", workerID, "jobID", submission.ID)
		}
	}
}

func (wp *WorkerPool) safeExecute(ctx context.Context, submission jobs.Submission, workerID int) (result any, err error) {
	defer func() {
		 r := recover();
		if r != nil {
			err = fmt.Errorf("job panicked: %v", r)
			wp.logger.Error("panic recovered in job execution",
				"workerID", workerID,
				"jobID", submission.ID,
				"panic", r,
			)
		}
	}()

	switch j := submission.Job.(type) {
	case jobs.BatchProcessingJob:
		return executeBatch(ctx, j, submission.ID, wp.logger)
	case jobs.Job:
		return j.Run(ctx)
	default:
		return nil, fmt.Errorf("job %q does not implement Job or BatchProcessingJob", submission.ID)
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

	eg, egCtx := errgroup.WithContext(ctx)

	batchStart := time.Now()

	for i, chunk := range chunks {
		i, chunk := i, chunk
		eg.Go(func() error {
			chunkStart := time.Now()
			logger.Info("chunk started",
				"jobID", jobID,
				"chunk", i,
				"items", len(chunk),
			)

			partial, err := job.RunBatch(egCtx, chunk)
			if err != nil {
				return fmt.Errorf("chunk %d failed: %w", i, err)
			}
			partials[i] = partial

			logger.Info("chunk done",
				"jobID", jobID,
				"chunk", i,
				"items", len(chunk),
				"duration", time.Since(chunkStart).Round(time.Microsecond).String(),
			)
			return nil
		})
	}
	
	err = eg.Wait()
	if err != nil {
		return nil, fmt.Errorf("batch run failed: %w", err)
	}

	logger.Info("batch complete",
		"jobID", jobID,
		"totalItems", len(items),
		"totalChunks", len(chunks),
		"duration", time.Since(batchStart).Round(time.Microsecond).String(),
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
