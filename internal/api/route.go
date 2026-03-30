package api

import (
	"net/http"
)

// routes wires the server endpoints used by the CLI and shell tests.
func (server *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", server.healthHandler)
	mux.HandleFunc("GET /jobs", server.listHandler)
	mux.HandleFunc("POST /jobs", server.submitHandler)
	mux.HandleFunc("GET /jobs/{id}", server.statusHandler)
	mux.HandleFunc("POST /shutdown", server.handleShutdown)

	return mux
}
