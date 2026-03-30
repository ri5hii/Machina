package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/ri5hii/Machina/internal/engine"
	"github.com/ri5hii/Machina/internal/registry"
	"github.com/ri5hii/Machina/internal/storage"
)

const (
	Uninitialized = "uninitialized"
	Initialized   = "initialized"
	Running       = "running"
	Shutdown      = "shutdown"
)

type Config struct {
	Version     string `json:"version"`
	Port        int    `json:"port"`
	WorkerCount int    `json:"workerCount"`
	QueueSize   int    `json:"queuesize"`
}

type Server struct {
	http     *http.Server
	logger   *slog.Logger
	eng      *engine.Engine
	store    *storage.JobStore
	registry *registry.Registry
	version  string
	status   atomic.Value
}

func New(config Config, eng *engine.Engine, store *storage.JobStore, log *slog.Logger, reg *registry.Registry) *Server {
	server := &Server{
		logger:   log,
		eng:      eng,
		store:    store,
		version:  config.Version,
		registry: reg,
	}
	server.http = &http.Server{
		Addr:         ":" + strconv.Itoa(config.Port),
		Handler:      server.routes(),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}
	server.status.Store(Initialized)
	return server
}

func (server *Server) Handler() http.Handler {
	return server.http.Handler
}

func (server *Server) Start() error {
	go func() {
		//handle the err when the server is shutdown, shouldnt log it in that case
		err := server.http.ListenAndServe()
		if err != nil {
			if err == http.ErrServerClosed {
				return
			}
			server.logger.Error("Server error", "Error", err)
		}
	}()
	server.logger.Info("Server status", "Status", Running)
	server.status.Store(Running)
	server.logger.Info("Server listening at port", "Port", server.http.Addr)
	return nil
}

func (server *Server) Shutdown(ctx context.Context) error {
	server.logger.Info("Server status", "Status", Shutdown)
	server.status.Store(Shutdown)
	server.logger.Info("Server disconnected from port", "Port", server.http.Addr)
	return server.http.Shutdown(ctx)
}

func HttpGET(url string) ([]byte, int, error) {
	response, err := http.Get(url)
	if err != nil {
		return nil, 0, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, 0, err
	}
	return body, response.StatusCode, err
}

func HttpPOST(url string, payload any) ([]byte, int, error) {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, err
	}

	response, err := http.Post(url, "application/json", bytes.NewReader(payloadJSON))
	if err != nil {
		return nil, 0, err
	}

	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	return body, response.StatusCode, err
}
