package api

import (
	"encoding/json"
	"net/http"
)

type response struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	resp := response{
		Status:  s.eng.StatusInfo(),
		Version: s.version,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
