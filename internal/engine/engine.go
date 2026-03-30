package engine

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ri5hii/Machina/internal/jobs"
	"github.com/ri5hii/Machina/internal/storage"
	"github.com/ri5hii/Machina/internal/worker"
)

const (
	Uninitialized = "uninitialized"
	Initialized   = "initialized"
	Running       = "running"
	Shutdown      = "shutdown"
)

type Engine struct {
	logger     *slog.Logger
	status     atomic.Value
	store      *storage.JobStore
	queue      chan jobs.JobSubmission
	workerPool *worker.WorkerPool
	closeOnce  sync.Once
}

// New assembles an engine with its queue, store, and worker pool dependencies.
func New(log *slog.Logger, queue chan jobs.JobSubmission, store *storage.JobStore, workerCount int) *Engine {
	eng := &Engine{
		logger:     log,
		store:      store,
		queue:      queue,
		workerPool: worker.New(workerCount, queue, store, log),
	}
	eng.status.Store(Initialized)
	return eng
}

// Start marks the engine as running and starts worker consumption.
func (eng *Engine) Start(ctx context.Context) {
	eng.status.Store(Running)
	eng.workerPool.Start(ctx)

	eng.logger.Info("Engine started", "Status", Running)
}

// Shutdown closes the queue once and waits for workers to finish draining.
func (eng *Engine) Shutdown() {
	eng.logger.Info("Shutting down engine", "Status", Shutdown)
	eng.closeOnce.Do(func() {
		close(eng.queue)
	})
	eng.workerPool.Wait()
	eng.status.Store(Shutdown)
	eng.logger.Info("Engine stopped", "Status", Shutdown)
}

// SubmitJob stores a pending job and enqueues it without blocking the caller.
func (eng *Engine) SubmitJob(job jobs.JobRunType) (string, error) {
	if eng.EngineStatusInfo() != Running {
		eng.logger.Warn("Engine is not running", "Status", eng.EngineStatusInfo())
		return "", fmt.Errorf("Engine is not running")
	}

	id := fmt.Sprintf("%d", time.Now().UnixNano())
	eng.store.Add(id, job)

	select {
	case eng.queue <- jobs.JobSubmission{JobID: id, Job: job}:
		eng.logger.Info("Job queued", "JobID", id)
		return id, nil
	default:
		// Fast failure keeps API latency bounded even when workers are saturated.
		eng.store.SetStatus(id, storage.StatusFailed)
		eng.store.SetError(id, fmt.Errorf("Queue is full"))
		return "", fmt.Errorf("Queue is full")
	}
}

// EngineStatusInfo returns the current engine lifecycle state for health and guards.
func (eng *Engine) EngineStatusInfo() string {
	engineStatus := eng.status.Load().(string)
	return engineStatus
}
