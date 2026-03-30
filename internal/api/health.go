package api

import (
	"encoding/json"
	"net/http"
)

type response struct {
	ServerStatus string `json:"serverStatus"`
	EngineStatus string `json:"engineStatus"`
	Port         string `json:"port"`
	Version      string `json:"version"`
}

// healthHandler reports server and engine state for readiness checks.
func (server *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	serverStatus := server.status.Load().(string)
	engineStatus := server.eng.EngineStatusInfo()
	response := response{
		ServerStatus: serverStatus,
		EngineStatus: engineStatus,
		Port:         server.http.Addr,
		Version:      server.version,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
