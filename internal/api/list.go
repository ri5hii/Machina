package api

import (
	"encoding/json"
	"net/http"
	"time"
)

type ListResponse struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	Error     string    `json:"error,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (s *Server) listHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "405 Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	records := s.store.List()

	response := make([]ListResponse, len(records))
	for i, record := range records {
		lr := ListResponse{
			ID:        record.RecordID,
			Status:    record.JobStatus,
			CreatedAt: record.CreatedAt,
			UpdatedAt: record.UpdatedAt,
		}
		if record.Err != nil {
			lr.Error = record.Err.Error()
		}

		response[i] = lr
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
