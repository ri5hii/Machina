package api

import (
	"encoding/json"
	"net/http"
)

type SubmitRequest struct {
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload"`
}

type SubmitResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// submitHandler validates a job submission and queues it for execution.
func (s *Server) submitHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "405 Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SubmitRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.Type == "" {
		http.Error(w, "Field 'type' is required", http.StatusBadRequest)
		return
	}

	ConstructedPayload, ok := s.registry.GetPayloadConstructor(req.Type)
	if !ok {
		http.Error(w, "Unknown job type: "+req.Type, http.StatusBadRequest)
		return
	}

	job, err := ConstructedPayload(req.Payload)
	if err != nil {
		http.Error(w, "Invalid payload: "+err.Error(), http.StatusBadRequest)
		return
	}

	id, err := s.eng.SubmitJob(job)
	if err != nil {
		s.logger.Error("Failed to submit job", "Type", req.Type, "error", err)
		http.Error(w, "Service unavailable: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	s.logger.Info("Job submitted", "id", id, "type", req.Type)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(SubmitResponse{ID: id, Status: "pending"})
}
