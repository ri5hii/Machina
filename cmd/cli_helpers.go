package main

import (
	"fmt"
	"os"
	"strconv"
)

func defaultPort() int {
	config, err := readConfigJSON()
	if err == nil && config.Port != 0 {
		return config.Port
	}
	return 8080
}

func parsePortFlag(args []string, port int) (int, []string, error) {
	remaining := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--port":
			if i+1 >= len(args) {
				return 0, nil, fmt.Errorf("missing value for --port")
			}
			parsed, err := strconv.Atoi(args[i+1])
			if err != nil {
				return 0, nil, fmt.Errorf("invalid port: %s (must be a number)", args[i+1])
			}
			port = parsed
			i++
		default:
			remaining = append(remaining, args[i])
		}
	}
	return port, remaining, nil
}

func failCommand(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
