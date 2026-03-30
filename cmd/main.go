package main

import (
	"encoding/json"
	"fmt"

	"context"
	"log/slog"
	"net/http"
	"os"
	"strings"
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
	case "start":
		slog.Info("Starting the application")
		err := commandStart(args[1:])
		if err != nil {
			fmt.Printf("Error starting the application: %v", err)
		}
		return
	case "shutdown":
		err := commandShutdown()
		if err != nil {
			fmt.Println(err)
		}
		return
	case "health":
		err := commandHealth()
		if err != nil {
			fmt.Printf("Error connecting to server: %v", err)
		}
		return
	case "submit":
		commandSubmit(args[1:])
		return
	case "status":
		err := commandStatus(args)
		if err != nil {
			fmt.Println(err)
		}
		return
	case "list":
		err := commandList(args)
		if err != nil {
			fmt.Println(err)
		}
		return
	case "config":
		err := commandConfig(args)
		if err != nil {
			fmt.Println(err)
		}
		return
	case "version":

	case "help":
		commandHelp(args)
		return
	case "description":
		commandDescription()
		commandHelp(args)
		return
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
	  shutdown							 Shutdown the server and engine
	  health                             Check server health
	  submit 							 Submit a job
	  status                             Get the status of a job
	  list                               List all jobs
	  profile							 List of job Profiles available
	  config                             View/Change config values
	  version                            Print version
	  help 			                     Show help for a command

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

	shutdownString := `
    Usage: machina shutdown`

	healthString := `
	Usage: machina health [--port]

	Example:
  		machina health --port 9090`

	submitString := `
	Usage: machina submit <job> <input> <output> [flags]

	Description:
	  Submit a job to the Machina server. The command sends the job type and
	  payload to the server and prints the accepted job response as JSON.

	Jobs:
	  file-encrypt   Encrypt files from an input folder into an output folder
	  csv-transform  Transform a CSV file into an output CSV file

	Flags:
	  --port <port>  Server port (default: config.json port or 8080 fallback)

	Examples:
	  machina submit file-encrypt ./input ./encrypted
	  machina submit csv-transform ./input.csv ./output.csv
	  machina submit csv-transform ./input.csv ./output.csv --port 9090`

	statusString := `
	Usage: machina status <id> [flags]

	Flags:
	  --watch              Continuously poll the server for status updates
	                       until the job reaches a terminal state (completed|failed).
	  --interval <secs>    Polling interval in seconds when using --watch
	                      (default: 5)
	  --port <port>        Server port override

	Examples:
	  machina status d3adb33f
	  machina status d3adb33f --watch
	  machina status d3adb33f --watch --interval 5`

	listString := `
	Usage: machina list [flags]

	Description:
	  List all jobs currently stored on the server. The command fetches the job
	  list from the server and prints a JSON array of job records. Each record
	  contains: id, status, error (if any), createdAt and updatedAt timestamps.

	Flags:
	  --status <status>    Filter jobs by status (pending|running|succeeded|failed)
	                       (example: --status running)

	Examples:
	  machina list
	  machina list --status running`

	configString := `
	Usage: machina config [flags]

	Flags:
	  --port            <port>   Listen port            (default: 8080, json: port)
	  --workerCount     <n>      Worker goroutine count (default: 4,    json: workerCzount)`

	if len(args) == 1 {
		fmt.Print(helpString)
	} else if len(args) == 2 {
		switch args[1] {
		case "start":
			fmt.Print(startString)
		case "shutdown":
			fmt.Print(shutdownString)
		case "health":
			fmt.Print(healthString)
		case "submit":
			fmt.Print(submitString)
		case "status":
			fmt.Print(statusString)
		case "list":
			fmt.Print(listString)
		case "config":
			fmt.Print(configString)
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

func commandStart(args []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	config, err := readConfigJSON()
	if err != nil {
		return fmt.Errorf("Error reading Config file")
	}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--port":
			if i+1 >= len(args) {
				return fmt.Errorf("Not enough arguments for --port")
			}
			port, err := strconv.Atoi(args[i+1])
			if err != nil {
				return fmt.Errorf("invalid port: %s (must be a number)", args[i+1])
			}
			config.Port = port
			i++
		case "--workers":
			if i+1 >= len(args) {
				return fmt.Errorf("Not enough arguments for --workers")
			}
			workers, err := strconv.Atoi(args[i+1])
			if err != nil {
				return fmt.Errorf("invalid workers: %s (must be a number)", args[i+1])
			}
			config.WorkerCount = workers
			i++
		case "--queue-size":
			if i+1 >= len(args) {
				return fmt.Errorf("Not enough arguments for --queue-size")
			}
			queueSize, err := strconv.Atoi(args[i+1])
			if err != nil {
				return fmt.Errorf("invalid queue size: %s (must be a number)", args[i+1])
			}
			config.QueueSize = queueSize
			i++
		default:
			return fmt.Errorf("invalid start flag: %s", args[i])
		}
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

func commandShutdown() error {
	config, err := readConfigJSON()
	if err != nil {
		return fmt.Errorf("Error reading config file")
	}

	url := "http://localhost:" + strconv.Itoa(config.Port) + "/shutdown"

	resp, statusCode, err := api.HttpPOST(url, nil)
	if err != nil {
		return fmt.Errorf("Failed to send shutdown request: %v", err)
	}

	if statusCode != http.StatusOK {
		return fmt.Errorf("Server returned %d: %s", statusCode, string(resp))
	}

	fmt.Println("Shutdown signal sent successfully")
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

var jobPayload = map[string]func(string, string) map[string]any{
	"file-encrypt": func(inputPath, outputPath string) map[string]any {
		return map[string]any{
			"folder_path": inputPath,
			"output_path": outputPath,
		}
	},
	"csv-transform": func(inputPath, outputPath string) map[string]any {
		return map[string]any{
			"input_path":  inputPath,
			"output_path": outputPath,
		}
	},
}

var jobTypeName = map[string]string{
	"file-encrypt":  "file_encrypt",
	"csv-transform": "csv_transform",
}

func commandSubmit(args []string) {
	if len(args) < 3 {
		commandHelp([]string{"help", "submit"})
		os.Exit(1)
	}

	jobName := args[0]
	inputPath := args[1]
	outputPath := args[2]

	port := "8080"
	if config, err := readConfigJSON(); err == nil && config.Port != 0 {
		port = strconv.Itoa(config.Port)
	}

	for i := 3; i < len(args); i++ {
		if args[i] == "--port" {
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "missing value for --port\n")
				os.Exit(1)
			}
			i++
			port = args[i]
		} else {
			fmt.Fprintf(os.Stderr, "unknown flag %q\n", args[i])
			os.Exit(1)
		}
	}

	builder, ok := jobPayload[jobName]
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown job name %q; valid names: file-encrypt, csv-transform\n", jobName)
		os.Exit(1)
	}

	typeName := jobTypeName[jobName]
	body, code, err := api.HttpPOST("http://localhost:"+port+"/jobs", map[string]any{
		"type":    typeName,
		"payload": builder(inputPath, outputPath),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not reach server: %v\n", err)
		os.Exit(1)
	}
	if code != http.StatusAccepted {
		fmt.Fprintf(os.Stderr, "server returned %d: %s\n", code, strings.TrimSpace(string(body)))
		os.Exit(1)
	}

	var resp map[string]any
	if err := json.Unmarshal(body, &resp); err != nil {
		fmt.Fprintf(os.Stderr, "invalid response: %v\n", err)
		os.Exit(1)
	}

	printJSON(resp)
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
	port := config.Port

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
		case "--port":
			if i+1 >= len(statusFlags) {
				return fmt.Errorf("Not enough arguments for --port")
			}
			p, err := strconv.Atoi(statusFlags[i+1])
			if err != nil {
				return fmt.Errorf("Invalid port: %s (must be a number)", statusFlags[i+1])
			}
			port = p
			i++
		default:
			return fmt.Errorf("invalid status flag: %s", statusFlags[i])
		}
	}

	url := "http://localhost:" + strconv.Itoa(port) + "/jobs/" + JobID

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

func commandList(args []string) error {
	config, err := readConfigJSON()
	if err != nil {
		return fmt.Errorf("Error reading Config file")
	}

	listFlags := args[1:]
	var statusFilter string

	for i := 0; i < len(listFlags); i++ {
		switch listFlags[i] {
		case "--status":
			if i+1 >= len(listFlags) {
				return fmt.Errorf("Not enough arguments for --status")
			}
			statusFilter = strings.ToLower(strings.TrimSpace(listFlags[i+1]))
			i++
		default:
			return fmt.Errorf("invalid list flag: %s", listFlags[i])
		}
	}

	baseURL := "http://localhost:" + strconv.Itoa(config.Port) + "/jobs"

	response, statusCode, err := api.HttpGET(baseURL)
	if err != nil {
		return fmt.Errorf("List error: %v", err)
	}
	if statusCode != http.StatusOK {
		responseString := string(response)
		return fmt.Errorf("Server returned %d \n%s", statusCode, responseString)
	}

	var list []map[string]any
	if err := json.Unmarshal(response, &list); err != nil {
		return fmt.Errorf("invalid response: %v", err)
	}

	if statusFilter != "" {
		filtered := make([]map[string]any, 0)
		for _, rec := range list {
			if s, ok := rec["status"].(string); ok {
				if strings.ToLower(s) == statusFilter {
					filtered = append(filtered, rec)
				}
			}
		}
		list = filtered
	}

	printJSON(list)
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
			if s == "succeeded" || s == "completed" || s == "failed" {
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
