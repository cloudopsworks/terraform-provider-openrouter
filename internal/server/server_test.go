// internal/server/server_test.go
package server

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestServerStartAndStop(t *testing.T) {
	// Use a random port to avoid conflicts
	server, err := NewServer("0")
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Start the server in a goroutine
	go func() {
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			t.Errorf("Server failed unexpectedly: %v", err)
		}
	}()

	// Give the server time to start
	time.Sleep(100 * time.Millisecond)

	// Gracefully shut down the server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Stop(ctx); err != nil {
		t.Fatalf("Failed to stop server: %v", err)
	}
}
