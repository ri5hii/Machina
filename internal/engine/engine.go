package engine

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"

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
	queue      <-chan jobs.JobSubmission
	workerPool *worker.WorkerPool
	closeOnce  sync.Once
}

func New(log *slog.Logger, queue <-chan jobs.JobSubmission, store *storage.JobStore, workerCount int) *Engine {
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
	eng.logger.Info("engine stopped", "status", Shutdown)
}

func (eng *Engine) SubmitJob(job jobs.JobSubmission) {
	//might need to take job and queue it and send off to the workerpool where it gets wired according to jobType

}

func (eng *Engine) EngineStatusInfo() string {
	engineStatus := eng.status.Load().(string)
	return engineStatus
}
