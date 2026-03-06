package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/ri5hii/Machina/internal/engine"
	"github.com/ri5hii/Machina/internal/storage"
)

type Server struct {
	http    *http.Server
	logger  *slog.Logger
	eng     *engine.Engine
	store   *storage.Store
	version string
}

type Config struct {
	Port    string
	Version string
}

func New(cfg Config, eng *engine.Engine, store *storage.Store, log *slog.Logger) *Server {
	server := &Server{
		logger: log,
		eng:    eng,
		store:  store,
	}
	server.http = &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      server.routes(),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  10 * time.Second,
	}
	server.version = cfg.Version
	return server
}

func (s *Server) Start() {
	s.logger.Info("starting server", "addr", s.http.Addr)
	go func() {
		err := s.http.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			s.logger.Error("server error", "error", err)
		}
	}()

}

func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("server shutting down", "addr", s.http.Addr)
	err := s.http.Shutdown(ctx)

	return err
}
