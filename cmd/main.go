package main

import (
	"encoding/json"
	"fmt"

	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"os/signal"
	"strconv"
	"syscall"

	"github.com/ri5hii/Machina/internal/api"
	"github.com/ri5hii/Machina/internal/engine"
	"github.com/ri5hii/Machina/internal/jobs"
	"github.com/ri5hii/Machina/internal/registry"
	"github.com/ri5hii/Machina/internal/storage"
)

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		commandHelp(args)
		return
	}

	switch args[0] {
	case "start", "s":
		slog.Info("Starting the application")
		err := commandStart()
		if err != nil {
			fmt.Printf("Error starting the application: %v", err)
		}
		return
	case "help":
		commandHelp(args)
		return
	case "description":
		commandDescription()
		commandHelp(args)
		return
	case "config":
		err := commandConfig(args)
		if err != nil {
			fmt.Println(err)
		}
		return
	case "health":
		err := commandHealth()
		if err != nil {
			fmt.Printf("Error connecting to server: %v", err)
		}
	case "status":
		err := commandStatus(args)
		if err != nil {
			fmt.Println(err)
		}
		return
	case "version":

	default:
		fmt.Printf("Unknown command: %s", args[0])
		commandHelp(args)
		os.Exit(1)
	}
}

func commandHelp(args []string) {
	helpString := `
	Machina — asynchronous job execution engine

	Usage: machina <command> [flags]

	Commands:
	  start                              Start the server and engine
	  status <id>                        Get the status of a job
	  submit <job-name> <input> <output> Submit a job
	  jobs                               List all jobs
	  health                             Check server health
	  config                             View/Change config values
	  version                            Print version
	  help [command]                     Show help for a command

	Run 'machina help <command>' for flags and examples.`

	startString := `
	Usage: machina start [flags]

	Flags:
	  --port        <port>   Listen port            (default: 8080, env: PORT)
	  --log-level   <level>  DEBUG|INFO|WARN|ERROR  (default: INFO,  env: LOG_LEVEL)
	  --workers     <n>      Worker goroutine count (default: 4,    env: WORKER_COUNT)
	  --queue-size  <n>      Bounded queue capacity (default: 100,  env: QUEUE_SIZE)

	Example:
	  machina start --port 9090 --workers 8 --queue-size 200`

	healthString := `
	Usage: machina health [--port]

	Example:
  		machina health --port 9090`

	configString := `
	Usage: machina config [flags]

	Flags:
	  --port            <port>   Listen port            (default: 8080, json: port)
	  --workerCount     <n>      Worker goroutine count (default: 4,    json: workerCount)`

	statusString := `
	Usage: machina status <id> [flags]

	Flags:
	  --watch              Continuously poll the server for status updates
	                      until the job reaches a terminal state (succeeded|failed).
	  --interval <secs>    Polling interval in seconds when using --watch
	                      (default: 2)

	Examples:
	  machina status d3adb33f
	  machina status d3adb33f --watch
	  machina status d3adb33f --watch --interval 5`

	if len(args) == 1 {
		fmt.Print(helpString)
	} else if len(args) == 2 {
		switch args[1] {
		case "start":
			fmt.Print(startString)
		case "health":
			fmt.Print(healthString)
		case "config":
			fmt.Print(configString)
		case "status":
			fmt.Print(statusString)
		default:
			fmt.Printf("Invalid flag %s", args[1])
		}
	}
}

func commandDescription() {
	descriptionString := `
	Machina — asynchronous job execution engine

	Description:
	Machina is a concurrent job execution engine for Go. It decouples work submission from
	work execution, giving you a structured runtime for asynchronous, resource-controlled processing.`

	fmt.Print(descriptionString)
}

func commandConfig(args []string) error {
	configArgs := args[1:]
	if len(configArgs) == 0 {
		config, err := readConfigJSON()
		if err != nil {
			return fmt.Errorf("Error reading Config file")
		}
		fmt.Println("Version:", config.Version)
		fmt.Println("Port:", config.Port)
		fmt.Println("Worker count:", config.WorkerCount)
		fmt.Println("Queue size:", config.QueueSize)
		return nil
	}
	if len(configArgs)%2 != 0 {
		return fmt.Errorf("Not enough arguments")
	}

	data, err := os.ReadFile("config.json")
	if err != nil {
		return fmt.Errorf("Config file is missing")
	}
	var config api.Config
	err = json.Unmarshal(data, &config)
	if err != nil {
		return fmt.Errorf("invalid config file")
	}

	var errors []error
	var updated bool

	for i := 0; i < len(configArgs); i += 2 {
		switch configArgs[i] {
		case "--port":
			port, err := strconv.Atoi(configArgs[i+1])
			if err != nil {
				errors = append(errors, fmt.Errorf("invalid port: %s (must be a number)", configArgs[i+1]))
				continue
			}
			if port != 8080 && (port < 49152 || port > 65535) {
				errors = append(errors, fmt.Errorf("port must be 8080 or between 49152-65535"))
				continue
			}
			config.Port = port
			fmt.Printf("Port value set to: %d\n", port)
			updated = true
		case "--workerCount":
			workerCount, err := strconv.Atoi(configArgs[i+1])
			if err != nil {
				errors = append(errors, fmt.Errorf("invalid worker count: %s (must be a number)", configArgs[i+1]))
				continue
			}
			if workerCount < 4 || workerCount > 10 {
				errors = append(errors, fmt.Errorf("Worker count can't be set to: %d (must be between 4 and 10)", workerCount))
				continue
			}
			config.WorkerCount = workerCount
			fmt.Printf("Worker count set to: %d\n", workerCount)
			updated = true
		case "--queueSize":
			QueueSize, err := strconv.Atoi(configArgs[i+1])
			if err != nil {
				errors = append(errors, fmt.Errorf("Invalid queue size: %s (must be a number)", configArgs[i+1]))
				continue
			}
			if QueueSize < 4 || QueueSize > 100 {
				errors = append(errors, fmt.Errorf("Queue size can't be set to: %d (must be between 4 and 100)", QueueSize))
				continue
			}
			config.QueueSize = QueueSize
			fmt.Printf("Queue size set to: %d\n", QueueSize)
			updated = true

		default:
			errors = append(errors, fmt.Errorf("invalid config flag: %s", configArgs[i]))
			continue
		}
	}

	if updated {
		if err := writeConfigJSON(config); err != nil {
			return fmt.Errorf("failed to write config: %w", err)
		}
	}

	if len(errors) > 0 {
		for e := 0; e < len(errors); e++ {
			fmt.Println(errors[e])
		}
		return nil
	}

	return nil
}

func commandStart() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	config, err := readConfigJSON()
	if err != nil {
		return fmt.Errorf("Error reading Config file")
	}

	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	store := storage.NewStore()
	queue := make(chan jobs.JobSubmission, config.QueueSize)
	eng := engine.New(log, queue, store, config.WorkerCount)
	reg := registry.New()
	reg.RegisterJob()
	server := api.New(config, eng, store, log, reg)

	eng.Start(ctx)
	server.Start()

	<-ctx.Done()
	fmt.Println("Shutting down…")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = server.Shutdown(shutdownCtx)
	if err != nil {
		return fmt.Errorf("Server shutdown error: %w", err)
	}
	eng.Shutdown()
	return nil
}

func commandHealth() error {
	config, err := readConfigJSON()
	if err != nil {
		return fmt.Errorf("Error reading Config file")
	}

	url := "http://localhost:" + strconv.Itoa(config.Port) + "/health"
	response, statusCode, err := api.HttpGET(url)
	if err != nil {
		return fmt.Errorf("Couldn't reach server: %v", err)
	}
	if statusCode != http.StatusOK {
		return fmt.Errorf("Server returned %d\n", statusCode)
	}
	var health map[string]any
	json.Unmarshal(response, &health)
	printJSON(health)
	return nil
}

func commandStatus(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("No job ID provided")
	}

	config, err := readConfigJSON()
	if err != nil {
		return fmt.Errorf("Error reading Config file")
	}

	JobID := args[1]
	url := "http://localhost:" + strconv.Itoa(config.Port) + "/jobs/" + JobID

	statusFlags := args[2:]
	var watch bool
	interval := 2 * time.Second

	for i := 0; i < len(statusFlags); i++ {
		switch statusFlags[i] {
		case "--watch":
			watch = true
		case "--interval":
			if i+1 >= len(statusFlags) {
				return fmt.Errorf("Not enough arguments for --interval")
			}
			secs, err := strconv.Atoi(statusFlags[i+1])
			if err != nil {
				return fmt.Errorf("Invalid interval: %s (must be a number)", statusFlags[i+1])
			}
			if secs <= 0 {
				return fmt.Errorf("Interval must be greater than 0")
			}
			interval = time.Duration(secs) * time.Second
			i++
		default:
			return fmt.Errorf("invalid status flag: %s", statusFlags[i])
		}
	}

	if watch {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := pollURL(ctx, url, interval)
		if err != nil {
			return err
		}
		return nil
	}

	response, statusCode, err := api.HttpGET(url)
	if err != nil {
		return fmt.Errorf("Status error: %v", err)
	}
	if statusCode != http.StatusOK {
		responseString := string(response)

		return fmt.Errorf("Server returned %d \n%s", statusCode, responseString)
	}

	var status map[string]any
	json.Unmarshal(response, &status)
	printJSON(status)
	return nil
}

func readConfigJSON() (api.Config, error) {
	data, err := os.ReadFile("config.json")
	if err != nil {
		return api.Config{}, err
	}
	var config api.Config
	err = json.Unmarshal(data, &config)
	if err != nil {
		return api.Config{}, err
	}
	return config, nil
}

func writeConfigJSON(config api.Config) error {
	updated, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	err = os.WriteFile("config.json", updated, 0644)
	if err != nil {
		return err
	}
	return nil
}

func printJSON(content any) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	encoder.Encode(content)
}

func pollURL(ctx context.Context, url string, interval time.Duration) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		response, statusCode, err := api.HttpGET(url)
		if err != nil {
			return fmt.Errorf("Status error: %v", err)
		}
		if statusCode != http.StatusOK {
			responseString := string(response)
			return fmt.Errorf("Server returned %d \n%s", statusCode, responseString)
		}

		var status map[string]any
		json.Unmarshal(response, &status)
		printJSON(status)

		if s, ok := status["status"].(string); ok {
			if s == "succeeded" || s == "failed" {
				return nil
			}
		}

		select {
		case <-ctx.Done():
			fmt.Println("Stopping poll")
			return nil
		case <-ticker.C:

		}
	}
}
