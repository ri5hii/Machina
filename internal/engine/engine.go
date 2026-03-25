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

func (eng *Engine) Start(ctx context.Context) {
	eng.status.Store(Running)
	eng.workerPool.Start(ctx)

	eng.logger.Info("engine started", "status", Running)
}

func (eng *Engine) Shutdown() {
	eng.status.Store(Shutdown)
	eng.logger.Info("shutting down engine", "status", Shutdown)
	eng.closeOnce.Do(func() {
		close(eng.queue)
	})
	eng.workerPool.Wait()
	eng.logger.Info("engine stopped", "status", Shutdown)
}

func (eng *Engine) SubmitJob(job jobs.JobRunType) (string, error) {
	if eng.EngineStatusInfo() != Running {
		eng.logger.Warn("engine is not running", "status", eng.EngineStatusInfo())
		return "", fmt.Errorf("engine is not running")
	}

	id := fmt.Sprintf("%d", time.Now().UnixNano())
	eng.store.Add(id, job)

	select {
	case eng.queue <- jobs.JobSubmission{JobID: id, Job: job}:
		eng.logger.Info("job queued", "jobID", id)
		return id, nil
	default:
		eng.store.SetStatus(id, storage.StatusFailed)
		eng.store.SetError(id, fmt.Errorf("queue is full"))
		return "", fmt.Errorf("queue is full")
	}
}

func (eng *Engine) EngineStatusInfo() string {
	engineStatus := eng.status.Load().(string)
	return engineStatus
}
