package api

import (
	"net/http"
)

func (s * Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.healthHandler)
	
	return mux
}