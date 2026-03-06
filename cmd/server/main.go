package main

import (
	"log/slog"
	"os"
	"os/signal"
	"context"
	"time"
	"syscall"
	"strings"

	"github.com/ri5hii/Machina/internal/api"
	"github.com/ri5hii/Machina/internal/engine"
)

type Config struct {
	Port     string
	Version  string
	LogLevel string
}

func loadConfig() Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	version := os.Getenv("VERSION")
	if version == "" {
		version = "0.1.0"
	}
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "DEFAULT"
	}

	return Config{
		Port:     port,
		Version:  version,
		LogLevel: logLevel,
	}
}

func setupLogger(cfg Config) *slog.Logger {
	var level slog.Level
	switch strings.ToUpper(cfg.LogLevel) {
	case "DEBUG":
		level = slog.LevelDebug
	case "INFO":
		level = slog.LevelInfo
	case "WARN":
		level = slog.LevelWarn
	case "ERROR":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})

	return slog.New(handler)
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	
	cfg := loadConfig()
	logger := setupLogger(cfg)
	eng := engine.New(logger)
	srv := api.New(api.Config{
					Port: cfg.Port,
					Version: cfg.Version,
				}, eng, logger)
	
	eng.Start()
	srv.Start()

	<-ctx.Done()
	logger.Info("shutting down server and engine")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown error", "error", err)
	}
	eng.Shutdown()
	
}
