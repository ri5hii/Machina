package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strconv"
)

type Config struct {
	Version     string `json:"version"`
	Port        int    `json:"port"`
	WorkerCount int    `json:"workerCount"`
}

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		commandHelp(args)
		return
	}

	switch args[0] {
	case "start", "s":
		commandStart()
		return
	case "help", "h":
		commandHelp(args)
		return
	case "description", "d":
		commandDescription()
		commandHelp(args)
		return
	case "config":
		err := commandConfig(args)
		if err != nil {
			fmt.Println(err)
		}
		return
	case "version", "v":

	default:
		fmt.Printf("Unknown command: %s", args[0])
		commandHelp(args)
		os.Exit(1)
	}
	slog.Info("Starting the application")
}

func commandHelp(args []string) {
	helpString := `
	Machina — asynchronous job execution engine
	
	Usage: machina <command> [flags]
	
	Commands:
	  start                              Start the server and engine
	  status <id>                        Get the status of a job
	  config                             View/Change config values
	  submit <job-name> <input> <output> Submit a job
	  jobs                               List all jobs
	  health                             Check server health
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
	if len(configArgs)%2 != 0 {
		return fmt.Errorf("Not enough arguments")
	}

	data, err := os.ReadFile("config.json")
	if err != nil {
		return fmt.Errorf("Config file is missing")
	}
	var config Config
	err = json.Unmarshal(data, &config)
	if err := json.Unmarshal(data, &config); err != nil {
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

func commandStart() {

}

func writeConfigJSON(config Config) error {
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
