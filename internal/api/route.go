package api

import (
	"net/http"
)

func (server *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", server.healthHandler)
	mux.HandleFunc("GET /jobs", server.listHandler)
	mux.HandleFunc("POST /jobs", server.submitHandler)
	mux.HandleFunc("/jobs/{id}", server.statusHandler)

	return mux
}
