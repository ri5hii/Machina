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

func (s *Server) SubmitHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "405 Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

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

	ConstructedPayload, ok := s.registry.GetPayloadConstructor(req.Type)
	if !ok {
		http.Error(w, "unknown job type: "+req.Type, http.StatusBadRequest)
		return
	}

	job, err := ConstructedPayload(req.Payload)
	if err != nil {
		http.Error(w, "invalid payload: "+err.Error(), http.StatusBadRequest)
		return
	}

	id, err := s.eng.SubmitJob(job)
	if err != nil {
		s.logger.Error("failed to submit job", "type", req.Type, "error", err)
		http.Error(w, "service unavailable: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	s.logger.Info("job submitted", "id", id, "type", req.Type)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(SubmitResponse{ID: id, Status: "pending"})
}
