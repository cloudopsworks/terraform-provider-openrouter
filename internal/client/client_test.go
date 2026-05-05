package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestListAPIKeysPaginates(t *testing.T) {
	t.Parallel()

	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.URL.RawQuery)
		if got := r.URL.Query().Get("workspace_id"); got != "ws-1" {
			t.Fatalf("expected workspace_id=ws-1, got %q", got)
		}
		if got := r.URL.Query().Get("include_disabled"); got != "true" {
			t.Fatalf("expected include_disabled=true, got %q", got)
		}

		offset := r.URL.Query().Get("offset")
		var resp any
		switch offset {
		case "0":
			firstPage := make([]map[string]any, 0, 100)
			for i := 0; i < 100; i++ {
				firstPage = append(firstPage, map[string]any{
					"hash":                  fmt.Sprintf("hash-%d", i),
					"name":                  fmt.Sprintf("key-%d", i),
					"label":                 fmt.Sprintf("Key %d", i),
					"disabled":              false,
					"include_byok_in_limit": false,
					"usage":                 0.0,
					"usage_daily":           0.0,
					"usage_weekly":          0.0,
					"usage_monthly":         0.0,
					"byok_usage":            0.0,
					"byok_usage_daily":      0.0,
					"byok_usage_weekly":     0.0,
					"byok_usage_monthly":    0.0,
					"created_at":            "2026-01-01T00:00:00Z",
					"updated_at":            "2026-01-01T00:00:00Z",
					"workspace_id":          "ws-1",
				})
			}
			resp = map[string]any{
				"data":        firstPage,
				"total_count": 101,
			}
		case "100":
			resp = map[string]any{
				"data": []map[string]any{
					{"hash": "hash-2", "name": "key-2", "label": "Key 2", "disabled": false, "include_byok_in_limit": false, "usage": 0.0, "usage_daily": 0.0, "usage_weekly": 0.0, "usage_monthly": 0.0, "byok_usage": 0.0, "byok_usage_daily": 0.0, "byok_usage_weekly": 0.0, "byok_usage_monthly": 0.0, "created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z", "workspace_id": "ws-1"},
				},
				"total_count": 101,
			}
		default:
			t.Fatalf("unexpected offset %q", offset)
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer server.Close()

	cl := New("token", server.URL, "test-agent", time.Second)
	workspaceID := "ws-1"
	items, err := cl.ListAPIKeys(context.Background(), &workspaceID, true)
	if err != nil {
		t.Fatalf("ListAPIKeys() error = %v", err)
	}
	if len(items) != 101 {
		t.Fatalf("expected 101 items, got %d", len(items))
	}
	if len(calls) != 2 {
		t.Fatalf("expected 2 paginated requests, got %d", len(calls))
	}
}

func TestListWorkspacesPaginates(t *testing.T) {
	t.Parallel()

	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.URL.RawQuery)
		offset := r.URL.Query().Get("offset")
		var resp any
		switch offset {
		case "0":
			firstPage := make([]map[string]any, 0, 100)
			for i := 0; i < 100; i++ {
				firstPage = append(firstPage, map[string]any{
					"id":                                  fmt.Sprintf("ws-%d", i),
					"name":                                fmt.Sprintf("Workspace %d", i),
					"slug":                                fmt.Sprintf("workspace-%d", i),
					"created_at":                          "2026-01-01T00:00:00Z",
					"is_data_discount_logging_enabled":    false,
					"is_observability_broadcast_enabled":  false,
					"is_observability_io_logging_enabled": false,
				})
			}
			resp = map[string]any{
				"data":        firstPage,
				"total_count": 101,
			}
		case "100":
			resp = map[string]any{
				"data": []map[string]any{
					{"id": "ws-2", "name": "Workspace 2", "slug": "workspace-2", "created_at": "2026-01-01T00:00:00Z", "is_data_discount_logging_enabled": false, "is_observability_broadcast_enabled": false, "is_observability_io_logging_enabled": false},
				},
				"total_count": 101,
			}
		default:
			t.Fatalf("unexpected offset %q", offset)
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer server.Close()

	cl := New("token", server.URL, "test-agent", time.Second)
	items, err := cl.ListWorkspaces(context.Background())
	if err != nil {
		t.Fatalf("ListWorkspaces() error = %v", err)
	}
	if len(items) != 101 {
		t.Fatalf("expected 101 items, got %d", len(items))
	}
	if len(calls) != 2 {
		t.Fatalf("expected 2 paginated requests, got %d", len(calls))
	}
}

func TestGetWorkspaceFailsFastOnMissingRequiredFields(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id":   "",
				"name": "Workspace 1",
				"slug": "workspace-1",
			},
		}); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer server.Close()

	cl := New("token", server.URL, "test-agent", time.Second)
	if _, err := cl.GetWorkspace(context.Background(), "ws-1"); err == nil {
		t.Fatal("expected validation error for missing required id")
	}
}

func TestCreateAPIKeyFailsFastOnMissingSecret(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"hash":                  "hash-1",
				"name":                  "service",
				"label":                 "Service",
				"disabled":              false,
				"include_byok_in_limit": false,
				"usage":                 0.0,
				"usage_daily":           0.0,
				"usage_weekly":          0.0,
				"usage_monthly":         0.0,
				"byok_usage":            0.0,
				"byok_usage_daily":      0.0,
				"byok_usage_weekly":     0.0,
				"byok_usage_monthly":    0.0,
				"created_at":            "2026-01-01T00:00:00Z",
				"updated_at":            "2026-01-01T00:00:00Z",
			},
			"key": "",
		}); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer server.Close()

	cl := New("token", server.URL, "test-agent", time.Second)
	if _, err := cl.CreateAPIKey(context.Background(), APIKeyCreateRequest{Name: "service"}); err == nil {
		t.Fatal("expected validation error for missing created secret key")
	}
}

func TestListGuardrailsPassesWorkspaceFilter(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("workspace_id"); got != "ws-1" {
			t.Fatalf("expected workspace_id=ws-1, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"id":           "gr-1",
					"name":         "prod",
					"workspace_id": "ws-1",
					"created_at":   "2026-01-01T00:00:00Z",
				},
			},
			"total_count": 1,
		}); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer server.Close()

	cl := New("token", server.URL, "test-agent", time.Second)
	workspaceID := "ws-1"
	items, err := cl.ListGuardrails(context.Background(), &workspaceID)
	if err != nil {
		t.Fatalf("ListGuardrails() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 guardrail, got %d", len(items))
	}
}
