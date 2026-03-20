package api

import (
	"net/http"
	"encoding/json"
)

type SubmitRequest struct {
	Type    string `json:"type"`
	Payload map[string]any `json:"payload"`
}

type SubmitResponse struct {
	ID string `json:"id"`
	Status string `json:"status"`
}

func (s *Server) SubmitHandler(w http.ResponseWriter, r *http.Request) {
	var req SubmitRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	
	if req.Type == "" {
		http.Error(w, "field 'type' is required", http.StatusBadRequest)
		return
	}
}