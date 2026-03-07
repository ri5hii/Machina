package api

import (
	"encoding/json"
	"net/http"
)

type submitRequest struct {
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload"`
}

type SubmitResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func (s *Server) submitHandler(w http.ResponseWriter, r *http.Request) {
	var req submitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.Type == "" {
		http.Error(w, "field 'type' is required", http.StatusBadRequest)
		return
	}

	payloadConstructor, ok := s.registry.GetPayloadConstructor(req.Type)
	if !ok {
		http.Error(w, "unknown job type: "+req.Type, http.StatusBadRequest)
		return
	}

	job, err := payloadConstructor(req.Payload)
	if err != nil {
		http.Error(w, "invalid payload: "+err.Error(), http.StatusBadRequest)
		return
	}

	type validatable interface {
		Validate() error
	}
	if v, ok := job.(validatable); ok {
		if err := v.Validate(); err != nil {
			http.Error(w, "validation failed: "+err.Error(), http.StatusBadRequest)
			return
		}
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
