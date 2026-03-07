package api

import (
	"encoding/json"
	"net/http"
	"time"
)

type listRecord struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	Error     string    `json:"error,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (s *Server) listHandler(w http.ResponseWriter, r *http.Request) {
	records := s.store.List()

	resp := make([]listRecord, len(records))
	for i, rec := range records {
		lr := listRecord{
			ID:        rec.ID,
			Status:    rec.Status,
			CreatedAt: rec.CreatedAt,
			UpdatedAt: rec.UpdatedAt,
		}
		if rec.Err != nil {
			lr.Error = rec.Err.Error()
		}
		resp[i] = lr
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
