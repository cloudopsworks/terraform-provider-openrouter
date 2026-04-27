// internal/api/handlers.go
package api

import (
	"encoding/json"
	"net/http"
	"time"
)

// Response represents the standard API response
type Response struct {
	Message string `json:"message"`
	Time    string `json:"time,omitempty"`
}

// HelloHandler handles requests to the /hello endpoint
func HelloHandler(w http.ResponseWriter, r *http.Request) {
	resp := Response{
		Message: "Hello, World!",
		Time:    time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HealthCheckHandler handles requests to the /health endpoint
func HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	resp := map[string]string{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}
