package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ri5hii/Machina/internal/api"
	"github.com/ri5hii/Machina/internal/engine"
	"github.com/ri5hii/Machina/internal/storage"
)

type Config struct {
	Port        string
	Version     string
	LogLevel    string
	WorkerCount int
	QueueSize   int
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
		logLevel = "INFO"
	}

	workerCount := 4
	if v := os.Getenv("WORKER_COUNT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			workerCount = n
		}
	}

	queueSize := 100
	if v := os.Getenv("QUEUE_SIZE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			queueSize = n
		}
	}

	return Config{
		Port:        port,
		Version:     version,
		LogLevel:    logLevel,
		WorkerCount: workerCount,
		QueueSize:   queueSize,
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

func commandHelp() {
	helpText := `
Machina:
Usage: machina [options]

Options:
  start                       Start the server
  help                        Show this help message
  version                     Show version information
  --port <port>               Set the server port (default: 8080)
  --log-level <level>         Set log level (DEBUG, INFO, WARN, ERROR)
`
	fmt.Println(helpText)
}

func commandStart(cfg Config) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger := setupLogger(cfg)
	store := storage.New()
	eng := engine.New(logger, store, cfg.WorkerCount, cfg.QueueSize)
	srv := api.New(api.Config{
		Port:    cfg.Port,
		Version: cfg.Version,
	}, eng, store, logger)

	eng.Start(ctx)
	srv.Start()

	<-ctx.Done()
	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown error", "error", err)
	}
	eng.Shutdown()
}

func commandVersion() {
	cfg := loadConfig()
	fmt.Println(cfg.Version)
}

func main() {
	if len(os.Args) < 2 {
		commandHelp()
		return
	}

	switch os.Args[1] {
	case "start":
		cfg := loadConfig()
		args := os.Args[2:]
		for i := 0; i < len(args); i++ {
			switch args[i] {
			case "--port":
				if i+1 < len(args) {
					cfg.Port = args[i+1]
					i++
				} else {
					fmt.Println("Error: --port flag requires a value")
					return
				}
			case "--log-level":
				if i+1 < len(args) {
					cfg.LogLevel = args[i+1]
					i++
				} else {
					fmt.Println("Error: --log-level flag requires a value")
					return
				}
			default:
				fmt.Println("Unknown option:", args[i])
				commandHelp()
				return
			}
		}
		commandStart(cfg)
	case "help":
		commandHelp()
	case "version":
		commandVersion()
	default:
		fmt.Println("Unknown command:", os.Args[1])
		commandHelp()
	}
}
