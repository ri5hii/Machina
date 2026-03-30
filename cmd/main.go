package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"

	"context"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"os/exec"
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
		failCommand(commandStart(args[1:]))
		return
	case "shutdown":
		failCommand(commandShutdown(args[1:]))
		return
	case "health":
		failCommand(commandHealth(args[1:]))
		return
	case "submit":
		commandSubmit(args[1:])
		return
	case "status":
		failCommand(commandStatus(args[1:]))
		return
	case "list":
		failCommand(commandList(args[1:]))
		return
	case "register":
		failCommand(commandRegister(args[1:]))
		return
	case "unregister":
		failCommand(commandUnregister(args[1:]))
		return
	case "types":
		failCommand(commandTypes(args[1:]))
		return
	case "profile":
		failCommand(commandProfile(args[1:]))
		return
	case "config":
		failCommand(commandConfig(args[1:]))
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
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", args[0])
		commandHelp(args)
		os.Exit(1)
	}
}

func commandHelp(args []string) {
	helpString := `
	Machina - asynchronous job execution engine

	Usage: machina <command> [flags]

	Commands:
	  start       Start the server and engine
	  shutdown    Shutdown the server and engine
	  health      Check server health
	  submit      Submit a job
	  status      Get the status of a job
	  list        List all jobs
	  register    Create and register a new job
	  unregister  Remove a registered job
	  profile     List available job scaffolding profiles
	  types       List registered job types
	  config      View/change config values
	  version     Print version
	  help        Show help for a command

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
	Usage: machina shutdown [--port]

	Example:
	  machina shutdown --port 9090`

	healthString := `
	Usage: machina health [--port]

	Description:
	  Check the Machina server health endpoint and print the JSON response.

	Flags:
	  --port <port>  Server port override (default: config.json port or 8080 fallback)

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

	Description:
	  Fetch a job by id from the Machina server and print the JSON status.

	Flags:
	  --watch              Continuously poll the server for status updates
	                       until the job reaches a terminal state (completed|failed).
	  --interval <secs>    Polling interval in seconds when using --watch
	                       (default: 2)
	  --port <port>        Server port override (default: config.json port or 8080 fallback)

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
	  --status <status>    Filter jobs by status (pending|running|completed|failed)
	                       (example: --status running)
	  --port <port>        Server port override (default: config.json port or 8080 fallback)

	Examples:
	  machina list
	  machina list --status running`

	registerString := `
	Usage: machina register <profile> <job-name>

	Description:
	  Create a new job source file from a profile template, open it in the
	  default editor, then save it into internal/jobs and auto-register it in
	  the payload constructor registry.

	Profiles:
	  batch       BatchProcessingJob scaffold
	  singleRun   SingleRunJob scaffold

	Examples:
	  machina register batch image_resize
	  machina register singleRun thumbnail_cleanup`

	unregisterString := `
	Usage: machina unregister <job-name>

	Description:
	  Remove a registered job type from the registry and delete its job source
	  file from internal/jobs.

	Example:
	  machina unregister image_resize`

	profileString := `
	Usage: machina profile

	Description:
	  Print the job scaffolding profiles available to the Machina generator.
	  The output is a JSON array of profile names.

	Flags:
	  None

	Example:
	  machina profile`

	typesString := `
	Usage: machina types

	Description:
	  Print the registered job types available to the Machina runtime.
	  The output is a JSON array of registered job type names.

	Flags:
	  None

	Example:
	  machina types`

	configString := `
	Usage: machina config [flags]

	Description:
	  Print the current config when called without flags, or update config.json
	  values when called with flag/value pairs.

	Flags:
	  --port         <port>  Listen port stored in config.json
	  --workers      <n>     Worker count stored in config.json
	  --queue-size   <n>     Queue size stored in config.json

	Examples:
	  machina config
	  machina config --port 9090
	  machina config --workers 6 --queue-size 50`

	if len(args) == 1 {
		printHelpBlock(helpString)
	} else if len(args) == 2 {
		switch args[1] {
		case "start":
			printHelpBlock(startString)
		case "shutdown":
			printHelpBlock(shutdownString)
		case "health":
			printHelpBlock(healthString)
		case "submit":
			printHelpBlock(submitString)
		case "status":
			printHelpBlock(statusString)
		case "list":
			printHelpBlock(listString)
		case "register":
			printHelpBlock(registerString)
		case "unregister":
			printHelpBlock(unregisterString)
		case "profile":
			printHelpBlock(profileString)
		case "types":
			printHelpBlock(typesString)
		case "config":
			printHelpBlock(configString)
		default:
			fmt.Printf("unknown help topic: %s\n", args[1])
		}
	}
}

func commandDescription() {
	descriptionString := `
	Machina — asynchronous job execution engine

	Description:
	Machina is a concurrent job execution engine for Go. It decouples work submission from
	work execution, giving you a structured runtime for asynchronous, resource-controlled processing.`

	printHelpBlock(descriptionString)
}

func commandConfig(args []string) error {
	if len(args) == 0 {
		config, err := readConfigJSON()
		if err != nil {
			return fmt.Errorf("error reading config file")
		}
		fmt.Println("Version:", config.Version)
		fmt.Println("Port:", config.Port)
		fmt.Println("Worker count:", config.WorkerCount)
		fmt.Println("Queue size:", config.QueueSize)
		return nil
	}
	if len(args)%2 != 0 {
		return fmt.Errorf("missing value for %s", args[len(args)-1])
	}

	data, err := os.ReadFile("config.json")
	if err != nil {
		return fmt.Errorf("config file is missing")
	}
	var config api.Config
	err = json.Unmarshal(data, &config)
	if err != nil {
		return fmt.Errorf("invalid config file")
	}

	var errors []error
	var updated bool

	for i := 0; i < len(args); i += 2 {
		switch args[i] {
		case "--port":
			port, err := strconv.Atoi(args[i+1])
			if err != nil {
				errors = append(errors, fmt.Errorf("invalid port: %s (must be a number)", args[i+1]))
				continue
			}
			if port != 8080 && (port < 49152 || port > 65535) {
				errors = append(errors, fmt.Errorf("port must be 8080 or between 49152-65535"))
				continue
			}
			config.Port = port
			fmt.Printf("port set to: %d\n", port)
			updated = true
		case "--workers", "--workerCount":
			workerCount, err := strconv.Atoi(args[i+1])
			if err != nil {
				errors = append(errors, fmt.Errorf("invalid worker count: %s (must be a number)", args[i+1]))
				continue
			}
			if workerCount < 4 || workerCount > 10 {
				errors = append(errors, fmt.Errorf("worker count must be between 4 and 10"))
				continue
			}
			config.WorkerCount = workerCount
			fmt.Printf("worker count set to: %d\n", workerCount)
			updated = true
		case "--queue-size", "--queueSize":
			queueSize, err := strconv.Atoi(args[i+1])
			if err != nil {
				errors = append(errors, fmt.Errorf("invalid queue size: %s (must be a number)", args[i+1]))
				continue
			}
			if queueSize < 4 || queueSize > 100 {
				errors = append(errors, fmt.Errorf("queue size must be between 4 and 100"))
				continue
			}
			config.QueueSize = queueSize
			fmt.Printf("queue size set to: %d\n", queueSize)
			updated = true

		default:
			errors = append(errors, fmt.Errorf("invalid config flag: %s", args[i]))
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
		return fmt.Errorf("error reading config file")
	}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--port":
			if i+1 >= len(args) {
				return fmt.Errorf("missing value for --port")
			}
			port, err := strconv.Atoi(args[i+1])
			if err != nil {
				return fmt.Errorf("invalid port: %s (must be a number)", args[i+1])
			}
			config.Port = port
			i++
		case "--workers":
			if i+1 >= len(args) {
				return fmt.Errorf("missing value for --workers")
			}
			workers, err := strconv.Atoi(args[i+1])
			if err != nil {
				return fmt.Errorf("invalid workers: %s (must be a number)", args[i+1])
			}
			config.WorkerCount = workers
			i++
		case "--queue-size":
			if i+1 >= len(args) {
				return fmt.Errorf("missing value for --queue-size")
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
	reg.RegisterJobs()
	server := api.New(config, eng, store, log, reg)

	eng.Start(ctx)
	server.Start()

	<-ctx.Done()
	fmt.Println("Shutting down…")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = server.Shutdown(shutdownCtx)
	if err != nil {
		return fmt.Errorf("server shutdown error: %w", err)
	}
	eng.Shutdown()
	return nil
}

func commandShutdown(args []string) error {
	port, remaining, err := parsePortFlag(args, defaultPort())
	if err != nil {
		return err
	}
	if len(remaining) > 0 {
		return fmt.Errorf("invalid shutdown flag: %s", remaining[0])
	}

	url := "http://localhost:" + strconv.Itoa(port) + "/shutdown"

	resp, statusCode, err := api.HttpPOST(url, nil)
	if err != nil {
		return fmt.Errorf("failed to send shutdown request: %v", err)
	}

	if statusCode != http.StatusOK {
		return fmt.Errorf("server returned %d: %s", statusCode, string(resp))
	}

	fmt.Println("Shutdown signal sent successfully")
	return nil
}

func commandHealth(args []string) error {
	port, remaining, err := parsePortFlag(args, defaultPort())
	if err != nil {
		return err
	}
	if len(remaining) > 0 {
		return fmt.Errorf("invalid health flag: %s", remaining[0])
	}

	url := "http://localhost:" + strconv.Itoa(port) + "/health"
	response, statusCode, err := api.HttpGET(url)
	if err != nil {
		return fmt.Errorf("could not reach server: %v", err)
	}
	if statusCode != http.StatusOK {
		return fmt.Errorf("server returned %d", statusCode)
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

	port, remaining, err := parsePortFlag(args[3:], defaultPort())
	if err != nil {
		failCommand(err)
	}
	if len(remaining) > 0 {
		failCommand(fmt.Errorf("unknown flag %q", remaining[0]))
	}

	builder, ok := jobPayload[jobName]
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown job name %q; valid names: file-encrypt, csv-transform\n", jobName)
		os.Exit(1)
	}

	typeName := jobTypeName[jobName]
	body, code, err := api.HttpPOST("http://localhost:"+strconv.Itoa(port)+"/jobs", map[string]any{
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
	if len(args) == 0 {
		return fmt.Errorf("missing job id")
	}

	jobID := args[0]
	port, statusFlags, err := parsePortFlag(args[1:], defaultPort())
	if err != nil {
		return err
	}
	var watch bool
	interval := 2 * time.Second

	for i := 0; i < len(statusFlags); i++ {
		switch statusFlags[i] {
		case "--watch":
			watch = true
		case "--interval":
			if i+1 >= len(statusFlags) {
				return fmt.Errorf("missing value for --interval")
			}
			secs, err := strconv.Atoi(statusFlags[i+1])
			if err != nil {
				return fmt.Errorf("invalid interval: %s (must be a number)", statusFlags[i+1])
			}
			if secs <= 0 {
				return fmt.Errorf("interval must be greater than 0")
			}
			interval = time.Duration(secs) * time.Second
			i++
		default:
			return fmt.Errorf("invalid status flag: %s", statusFlags[i])
		}
	}

	url := "http://localhost:" + strconv.Itoa(port) + "/jobs/" + jobID

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
		return fmt.Errorf("status request failed: %v", err)
	}
	if statusCode != http.StatusOK {
		responseString := string(response)
		return fmt.Errorf("server returned %d: %s", statusCode, responseString)
	}

	var status map[string]any
	json.Unmarshal(response, &status)
	printJSON(status)
	return nil
}

func commandList(args []string) error {
	port, listFlags, err := parsePortFlag(args, defaultPort())
	if err != nil {
		return err
	}
	var statusFilter string

	for i := 0; i < len(listFlags); i++ {
		switch listFlags[i] {
		case "--status":
			if i+1 >= len(listFlags) {
				return fmt.Errorf("missing value for --status")
			}
			statusFilter = strings.ToLower(strings.TrimSpace(listFlags[i+1]))
			i++
		default:
			return fmt.Errorf("invalid list flag: %s", listFlags[i])
		}
	}

	baseURL := "http://localhost:" + strconv.Itoa(port) + "/jobs"

	response, statusCode, err := api.HttpGET(baseURL)
	if err != nil {
		return fmt.Errorf("list request failed: %v", err)
	}
	if statusCode != http.StatusOK {
		responseString := string(response)
		return fmt.Errorf("server returned %d: %s", statusCode, responseString)
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

func commandProfile(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("profile does not accept flags")
	}

	printJSON([]string{"batch", "singleRun"})
	return nil
}

func commandTypes(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("types does not accept flags")
	}

	jobTypes, err := registeredJobTypes()
	if err != nil {
		return err
	}
	printJSON(jobTypes)
	return nil
}

func commandRegister(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: machina register <profile> <job-name>")
	}

	profile := strings.ToLower(strings.TrimSpace(args[0]))
	jobName := normalizeJobName(args[1])
	if jobName == "" {
		return fmt.Errorf("invalid job name: %q", args[1])
	}

	if profile != "batch" && profile != "singlerun" {
		return fmt.Errorf("unknown profile %q; valid profiles: batch, singleRun", profile)
	}
	if profile == "singlerun" {
		profile = "singleRun"
	}

	profiles, err := registeredJobTypes()
	if err != nil {
		return err
	}
	for _, existing := range profiles {
		if existing == jobName {
			return fmt.Errorf("job %q is already registered", jobName)
		}
	}

	jobFilePath := filepath.Join("internal", "jobs", jobName+".go")
	if _, err := os.Stat(jobFilePath); err == nil {
		return fmt.Errorf("job file already exists: %s", jobFilePath)
	} else if !os.IsNotExist(err) {
		return err
	}

	spec := newJobSpec(profile, jobName)
	template := buildJobTemplate(spec)

	tempFile, err := os.CreateTemp("", spec.FileName+"-*.go")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	if _, err := tempFile.WriteString(template); err != nil {
		tempFile.Close()
		return fmt.Errorf("failed to write temp template: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp template: %w", err)
	}

	if err := openEditor(tempPath); err != nil {
		return err
	}

	content, err := os.ReadFile(tempPath)
	if err != nil {
		return fmt.Errorf("failed to read edited template: %w", err)
	}
	if len(bytes.TrimSpace(content)) == 0 {
		return fmt.Errorf("edited file is empty")
	}

	if err := os.WriteFile(jobFilePath, content, 0o644); err != nil {
		return fmt.Errorf("failed to save job file: %w", err)
	}

	if err := appendRegistryConstructor(spec); err != nil {
		return err
	}

	if err := gofmtFiles(jobFilePath, filepath.Join("internal", "registry", "payloadConstructor.go")); err != nil {
		return err
	}

	fmt.Printf("Created %s\n", jobFilePath)
	fmt.Printf("Registered %s using profile %s\n", jobName, profile)
	return nil
}

func commandUnregister(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: machina unregister <job-name>")
	}

	jobName := normalizeJobName(args[0])
	if jobName == "" {
		return fmt.Errorf("invalid job name: %q", args[0])
	}

	profiles, err := registeredJobTypes()
	if err != nil {
		return err
	}
	found := false
	for _, existing := range profiles {
		if existing == jobName {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("job %q is not registered", jobName)
	}

	spec := newJobSpec("singleRun", jobName)
	if err := removeRegistryConstructor(spec); err != nil {
		return err
	}

	jobFilePath := filepath.Join("internal", "jobs", jobName+".go")
	if err := os.Remove(jobFilePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete job file: %w", err)
	}

	if err := gofmtFiles(filepath.Join("internal", "registry", "payloadConstructor.go")); err != nil {
		return err
	}

	fmt.Printf("Unregistered %s\n", jobName)
	if _, err := os.Stat(jobFilePath); os.IsNotExist(err) {
		fmt.Printf("Deleted %s\n", jobFilePath)
	}
	return nil
}

func openEditor(path string) error {
	editor, err := resolveEditor()
	if err != nil {
		return err
	}

	parts := strings.Fields(editor)
	if len(parts) == 0 {
		return fmt.Errorf("no editor configured")
	}
	cmd := exec.Command(parts[0], append(parts[1:], path)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editor failed: %w", err)
	}
	return nil
}

func resolveEditor() (string, error) {
	for _, key := range []string{"VISUAL", "EDITOR"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value, nil
		}
	}

	for _, candidate := range []string{"nano", "vim", "vi"} {
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("no editor found; set $EDITOR or $VISUAL")
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

func printHelpBlock(text string) {
	lines := strings.Split(strings.Trim(text, "\n"), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimPrefix(line, "\t")
	}
	fmt.Println(strings.Join(lines, "\n"))
}

func pollURL(ctx context.Context, url string, interval time.Duration) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		response, statusCode, err := api.HttpGET(url)
		if err != nil {
			return fmt.Errorf("status request failed: %v", err)
		}
		if statusCode != http.StatusOK {
			responseString := string(response)
			return fmt.Errorf("server returned %d: %s", statusCode, responseString)
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
			fmt.Println("stopping poll")
			return nil
		case <-ticker.C:

		}
	}
}
