package worker

import (
	"fmt"
	"log/slog"
	"sync"
	"context"

	"github.com/ri5hii/Machina/internal/jobs"
	"github.com/ri5hii/Machina/internal/storage"
)

type WorkerPool struct {
	workerCount int
	queue <-chan jobs.Submission
	store *storage.Store
	logger *slog.Logger
	waitgroup sync.WaitGroup
}

func New(workerCount int, queue <-chan jobs.Submission, store *storage.Store, log *slog.Logger) *WorkerPool {
	return &WorkerPool {
		workerCount: workerCount,
		queue: queue,
		store: store,
		logger: log,
	}
}

func(wp *WorkerPool) Start(ctx context.Context) {
	wp.logger.Info("starting worker pool", "workerCount", wp.workerCount)
	wp.waitgroup.Add(wp.workerCount)
	for i := 0; i < wp.workerCount; i++ {
		go func(workerID int) {
			defer wp.waitgroup.Done()
			wp.logger.Info("worker started", "workerID", workerID)
			wp.worker(ctx, workerID)
		}(i)
	}	
}

func (wp *WorkerPool) worker(ctx context.Context, workerID int) {
    for {
        select {
        case <-ctx.Done():
            wp.logger.Info("worker shutting down", "workerID", workerID)
            return
        case submission, ok := <-wp.queue:
            if !ok {
                wp.logger.Info("queue closed", "workerID", workerID)
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
        if r := recover(); r != nil {
            err = fmt.Errorf("job %s panicked: %v", submission.ID, r)
            wp.logger.Error("panic in job execution", "workerID", workerID, "jobID", submission.ID, "panic", r)
        }
    }()
    return submission.Job.Run(ctx)
}