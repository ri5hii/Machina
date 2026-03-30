package api

import (
	"context"
	"net/http"
	"time"
)

func (s *Server) handleShutdown(w http.ResponseWriter, r *http.Request) {
	go func() {
		time.Sleep(100 * time.Millisecond)
		s.logger.Info("Shutdown requested via API")

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		s.Shutdown(ctx)
		s.eng.Shutdown()
	}()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"message":"shutdown initiated"}`))
}
