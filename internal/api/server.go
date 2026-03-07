package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/ri5hii/Machina/internal/engine"
	"github.com/ri5hii/Machina/internal/registry"
	"github.com/ri5hii/Machina/internal/storage"
)

type Server struct {
	http     *http.Server
	logger   *slog.Logger
	eng      *engine.Engine
	store    *storage.Store
	registry *registry.Registry
	version  string
}

type Config struct {
	Port    string
	Version string
}

func New(cfg Config, eng *engine.Engine, store *storage.Store, reg *registry.Registry, log *slog.Logger) *Server {
	s := &Server{
		logger:   log,
		eng:      eng,
		store:    store,
		registry: reg,
		version:  cfg.Version,
	}
	s.http = &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      s.routes(),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}
	return s
}

func (s *Server) Start() {
	s.logger.Info("starting server", "addr", s.http.Addr)
	go func() {
		if err := s.http.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("server error", "error", err)
		}
	}()
}

func (s *Server) Handler() http.Handler {
	return s.http.Handler
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("server shutting down", "addr", s.http.Addr)
	return s.http.Shutdown(ctx)
}
