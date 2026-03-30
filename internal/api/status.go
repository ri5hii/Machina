package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type StatusResponse struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	Result    any       `json:"result"`
	Error     string    `json:"error"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (s *Server) statusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "405 Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/jobs/")
	if id == "" {
		http.Error(w, "job id is required", http.StatusBadRequest)
		return
	}

	record, ok := s.store.Get(id)
	if !ok {
		http.Error(w, "job not found:"+id, http.StatusNotFound)
		return
	}

	resp := StatusResponse{
		ID:        record.RecordID,
		Status:    record.JobStatus,
		Result:    record.Result,
		CreatedAt: record.CreatedAt,
		UpdatedAt: record.UpdatedAt,
	}
	if record.Err != nil {
		resp.Error = record.Err.Error()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
