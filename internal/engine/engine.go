package engine

import (
	"log/slog"
	"sync"
	"sync/atomic"
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
	closeOnce sync.Once
}

func New(log *slog.Logger) *Engine {
	eng := &Engine{
		logger: log,
	}
	eng.status.Store(Initialized)
	return eng
}

func (eng *Engine) Start() {
	eng.status.Store(Running)
	eng.logger.Info("engine started: ", "status: ", Running)
}

func (eng *Engine) Shutdown() {
	eng.status.Store(Shutdown)
	eng.logger.Info("engine stopped: ", "status: ", Shutdown)
}
