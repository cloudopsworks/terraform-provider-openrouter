// internal/server/server.go
package server

import (
	"context"
	"hello-service/internal/api"
	"net/http"
	"time"
)

// Server represents the HTTP server
type Server struct {
	server *http.Server
}

// NewServer creates a new HTTP server
func NewServer(port string) (*Server, error) {
	mux := http.NewServeMux()

	// Register handlers
	mux.HandleFunc("GET /hello", api.HelloHandler)
	mux.HandleFunc("GET /health", api.HealthCheckHandler)

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	return &Server{server: srv}, nil
}

// Start starts the HTTP server
func (s *Server) Start() error {
	return s.server.ListenAndServe()
}

// Stop gracefully stops the HTTP server
func (s *Server) Stop(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}
