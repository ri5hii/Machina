package engine

import (
	"log/slog"
	"sync/atomic"
)

const (
	Uninitialized = "uninitialized"
	Initialized   = "initialized"
	Running       = "running"
	Shutdown      = "shutdown"
)

type Engine struct {
	logger *slog.Logger
	status atomic.Value
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
	eng.logger.Info("starting engine", "status", Running)
}

func (eng *Engine) Shutdown() {
	eng.status.Store(Shutdown)
	eng.logger.Info("shutting down engine", "status", Shutdown)
}

func (eng *Engine) StatusInfo() string {
	return eng.status.Load().(string)
}
