package api

import (
	"encoding/json"
	"net/http"

	"github.com/ri5hii/Machina/internal/engine"
)

type response struct {
	ServerStatus string `json:"server-status"`
	EngineStatus string `json:"engine-status"`
	Port         string       `json:"port"`
	Version      string       `json:"version"`
}

func (server *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	serverStatus := server.status.Load().(string)
	engineStatus := engine.EngineStatusInfo(server.eng)
	response := response{
		ServerStatus: serverStatus,
		EngineStatus: engineStatus,
		Port:         server.http.Addr,
		Version:      server.version,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
