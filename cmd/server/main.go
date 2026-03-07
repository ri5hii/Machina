package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ri5hii/Machina/internal/api"
	"github.com/ri5hii/Machina/internal/engine"
	"github.com/ri5hii/Machina/internal/registry"
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
		version = "0.2.0"
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
	case "WARN":
		level = slog.LevelWarn
	case "ERROR":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}

func fatalf(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format, a...)
	os.Exit(1)
}

func printJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(v)
}

func serverURL(port string) string {
	return "http://localhost:" + port
}

func httpGet(url string) ([]byte, int, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	return body, resp.StatusCode, err
}

func httpPost(url string, payload any) ([]byte, int, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, err
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	return body, resp.StatusCode, err
}

var jobPayload = map[string]func(input, output string) map[string]any{
	"image-resize": func(input, output string) map[string]any {
		return map[string]any{"folder_path": input, "output_path": output}
	},
	"file-encrypt": func(input, output string) map[string]any {
		return map[string]any{"folder_path": input, "output_path": output}
	},
	"csv-transform": func(input, output string) map[string]any {
		return map[string]any{"input_path": input, "output_path": output}
	},
}

var jobTypeName = map[string]string{
	"image-resize":  "image_resize",
	"file-encrypt":  "file_encrypt",
	"csv-transform": "csv_transform",
}

func commandStart(args []string) {
	cfg := loadConfig()

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--port":
			cfg.Port = requireNext(args, &i, "--port")
		case "--log-level":
			cfg.LogLevel = requireNext(args, &i, "--log-level")
		case "--workers":
			cfg.WorkerCount = requireNextInt(args, &i, "--workers")
		case "--queue-size":
			cfg.QueueSize = requireNextInt(args, &i, "--queue-size")
		default:
			fatalf("unknown flag %q\n", args[i])
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger := setupLogger(cfg)
	store := storage.New()
	eng := engine.New(logger, store, cfg.WorkerCount, cfg.QueueSize)

	reg := registry.New()
	registry.RegisterAll(reg)

	srv := api.New(api.Config{Port: cfg.Port, Version: cfg.Version}, eng, store, reg, logger)

	eng.Start(ctx)
	srv.Start()

	fmt.Printf("Machina %s  port=:%s  workers=%d  queue=%d\n",
		cfg.Version, cfg.Port, cfg.WorkerCount, cfg.QueueSize)

	<-ctx.Done()
	fmt.Println("\nshutting down…")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown error", "error", err)
	}
	eng.Shutdown()
}

func commandSubmit(args []string) {
	if len(args) < 3 {
		commandHelp("submit")
		os.Exit(1)
	}

	jobName := args[0]
	inputPath := args[1]
	outputPath := args[2]

	port := "8080"
	for i := 3; i < len(args); i++ {
		if args[i] == "--port" {
			port = requireNext(args, &i, "--port")
		} else {
			fatalf("unknown flag %q\n", args[i])
		}
	}

	builder, ok := jobPayload[jobName]
	if !ok {
		fatalf("unknown job name %q — valid names: image-resize, file-encrypt, csv-transform\n", jobName)
	}
	typeName := jobTypeName[jobName]

	body, code, err := httpPost(serverURL(port)+"/jobs", map[string]any{
		"type":    typeName,
		"payload": builder(inputPath, outputPath),
	})
	if err != nil {
		fatalf("could not reach server: %v\n", err)
	}
	if code != http.StatusAccepted {
		fatalf("server returned %d: %s\n", code, strings.TrimSpace(string(body)))
	}

	var resp map[string]any
	json.Unmarshal(body, &resp)
	printJSON(resp)
}

func commandStatus(args []string) {
	if len(args) == 0 || strings.HasPrefix(args[0], "--") {
		fatalf("usage: machina status <job-id> [--port 8080] [--watch]\n")
	}

	id := args[0]
	port := "8080"
	watch := false

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--port":
			port = requireNext(args, &i, "--port")
		case "--watch":
			watch = true
		default:
			fatalf("unknown flag %q\n", args[i])
		}
	}

	url := serverURL(port) + "/jobs/" + id

	for {
		body, code, err := httpGet(url)
		if err != nil {
			fatalf("could not reach server: %v\n", err)
		}
		if code == http.StatusNotFound {
			fatalf("job %q not found\n", id)
		}
		if code != http.StatusOK {
			fatalf("server returned %d: %s\n", code, strings.TrimSpace(string(body)))
		}

		var resp map[string]any
		json.Unmarshal(body, &resp)

		if watch {
			fmt.Print("\033[2J\033[H")
		}
		printJSON(resp)

		if !watch {
			break
		}
		status, _ := resp["status"].(string)
		if status == "completed" || status == "failed" {
			break
		}
		time.Sleep(1 * time.Second)
	}
}

func commandJobs(args []string) {
	port := "8080"
	filterStatus := ""

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--port":
			port = requireNext(args, &i, "--port")
		case "--status":
			filterStatus = requireNext(args, &i, "--status")
		default:
			fatalf("unknown flag %q\n", args[i])
		}
	}

	body, code, err := httpGet(serverURL(port) + "/jobs")
	if err != nil {
		fatalf("could not reach server: %v\n", err)
	}
	if code != http.StatusOK {
		fatalf("server returned %d: %s\n", code, strings.TrimSpace(string(body)))
	}

	var jobs []map[string]any
	if err := json.Unmarshal(body, &jobs); err != nil {
		fatalf("unexpected response: %v\n", err)
	}

	if filterStatus != "" {
		filtered := jobs[:0]
		for _, j := range jobs {
			if j["status"] == filterStatus {
				filtered = append(filtered, j)
			}
		}
		jobs = filtered
	}

	if len(jobs) == 0 {
		fmt.Println("no jobs found")
		return
	}
	printJSON(jobs)
}

func commandHealth(args []string) {
	port := "8080"
	for i := 0; i < len(args); i++ {
		if args[i] == "--port" {
			port = requireNext(args, &i, "--port")
		} else {
			fatalf("unknown flag %q\n", args[i])
		}
	}

	body, code, err := httpGet(serverURL(port) + "/health")
	if err != nil {
		fatalf("could not reach server: %v\n", err)
	}
	if code != http.StatusOK {
		fatalf("server returned %d: %s\n", code, strings.TrimSpace(string(body)))
	}
	var resp map[string]any
	json.Unmarshal(body, &resp)
	printJSON(resp)
}

func commandVersion() {
	fmt.Println(loadConfig().Version)
}

func requireNext(args []string, i *int, flag string) string {
	*i++
	if *i >= len(args) {
		fatalf("%s requires a value\n", flag)
	}
	return args[*i]
}

func requireNextInt(args []string, i *int, flag string) int {
	v := requireNext(args, i, flag)
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		fatalf("%s requires a positive integer, got %q\n", flag, v)
	}
	return n
}

func commandHelp(topic string) {
	switch topic {
	case "start":
		fmt.Print(`
Usage: machina start [flags]

Flags:
  --port        <port>   Listen port           (default: 8080, env: PORT)
  --log-level   <level>  DEBUG|INFO|WARN|ERROR  (default: INFO,  env: LOG_LEVEL)
  --workers     <n>      Worker goroutine count (default: 4,    env: WORKER_COUNT)
  --queue-size  <n>      Bounded queue capacity (default: 100,  env: QUEUE_SIZE)

Example:
  machina start --port 9090 --workers 8 --queue-size 200
`)

	case "submit":
		fmt.Print(`
Usage: machina submit <job-name> <input-path> <output-path> [--port 8080]

Job names:
  image-resize   Resize all images in a folder
  file-encrypt   Encrypt all files in a folder
  csv-transform  Apply a row transform to a CSV file

Examples:
  machina submit image-resize  /data/photos   /data/resized
  machina submit file-encrypt  /data/secrets  /data/encrypted
  machina submit csv-transform /data/in.csv   /data/out.csv
`)

	case "status":
		fmt.Print(`
Usage: machina status <job-id> [--port 8080] [--watch]

  --watch   Poll every second until the job reaches a terminal state

Examples:
  machina status 1718000000000000000
  machina status 1718000000000000000 --watch
`)

	case "jobs":
		fmt.Print(`
Usage: machina jobs [--port 8080] [--status pending|running|completed|failed]

Examples:
  machina jobs
  machina jobs --status failed
`)

	case "health":
		fmt.Print(`
Usage: machina health [--port 8080]

Example:
  machina health --port 9090
`)

	default:
		fmt.Print(`
Machina — asynchronous job execution engine

Usage: machina <command> [flags]

Commands:
  start                              Start the server and engine
  submit <job-name> <input> <output> Submit a job
  status <id>                        Get the status of a job
  jobs                               List all jobs
  health                             Check server health
  version                            Print version
  help   [command]                   Show help for a command

Run 'machina help <command>' for flags and examples.
`)
	}
}

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		commandHelp("")
		return
	}

	switch args[0] {
	case "start":
		commandStart(args[1:])
	case "submit":
		commandSubmit(args[1:])
	case "status":
		commandStatus(args[1:])
	case "jobs":
		commandJobs(args[1:])
	case "health":
		commandHealth(args[1:])
	case "version":
		commandVersion()
	case "help":
		commandHelp(strings.Join(args[1:], " "))
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", args[0])
		commandHelp("")
		os.Exit(1)
	}
}
