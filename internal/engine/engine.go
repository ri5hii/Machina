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
	logger    *slog.Logger
	status    atomic.Value
	queue     chan jobs.Submission
	store     *storage.Store
	pool      *worker.WorkerPool
	closeOnce sync.Once
}

func New(log *slog.Logger, store *storage.Store, workerCount int, queueSize int) *Engine {
	queue := make(chan jobs.Submission, queueSize)
	eng := &Engine{
		logger: log,
		queue:  queue,
		store:  store,
		pool:   worker.New(workerCount, queue, store, log),
	}
	eng.status.Store(Initialized)
	return eng
}

func (eng *Engine) Start(ctx context.Context) {
	eng.status.Store(Running)
	eng.pool.Start(ctx)
	eng.logger.Info("engine started", "status", Running)
}

func (eng *Engine) Shutdown() {
	eng.status.Store(Shutdown)
	eng.logger.Info("shutting down engine", "status", Shutdown)
	eng.closeOnce.Do(func() {
		close(eng.queue)
	})
	eng.pool.Wait()
	eng.logger.Info("engine stopped", "status", Shutdown)
}

func (eng *Engine) SubmitJob(job jobs.Runnable) (string, error) {
	if eng.StatusInfo() != Running {
		eng.logger.Warn("engine is not running", "status", eng.StatusInfo())
		return "", fmt.Errorf("engine is not running")
	}

	id := fmt.Sprintf("%d", time.Now().UnixNano())
	eng.store.Add(id, job)

	select {
	case eng.queue <- jobs.Submission{ID: id, Job: job}:
		eng.logger.Info("job queued", "jobID", id)
		return id, nil
	default:
		eng.store.SetStatus(id, storage.StatusFailed)
		eng.store.SetError(id, fmt.Errorf("queue is full"))
		return "", fmt.Errorf("queue is full")
	}
}

func (eng *Engine) StatusInfo() string {
	return eng.status.Load().(string)
}
