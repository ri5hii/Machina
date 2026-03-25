package api

import (
	"net/http"
)

func (server *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", server.healthHandler)
	mux.HandleFunc("/jobs", server.SubmitHandler)

	return mux
}
