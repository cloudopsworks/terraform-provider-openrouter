package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func workspaceBudgetResponse(interval any, limit float64) map[string]any {
	return map[string]any{"data": map[string]any{
		"id": "budget-1", "workspace_id": "workspace-uuid", "limit_usd": limit,
		"reset_interval": interval, "created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z",
	}}
}

func TestWorkspaceBudgetClientListAndPut(t *testing.T) {
	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		methods = append(methods, request.Method+" "+request.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch request.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{workspaceBudgetResponse(nil, 10)["data"]}})
		case http.MethodPut:
			if request.URL.Path != "/workspaces/workspace-slug/budgets/lifetime" {
				t.Fatalf("unexpected PUT path: %s", request.URL.Path)
			}
			_ = json.NewEncoder(w).Encode(workspaceBudgetResponse(nil, 11))
		}
	}))
	defer server.Close()

	client := New("token", server.URL, "test", time.Second)
	budgets, err := client.ListWorkspaceBudgets(context.Background(), "workspace-slug")
	if err != nil || len(budgets) != 1 || budgets[0].ResetInterval != "lifetime" {
		t.Fatalf("ListWorkspaceBudgets() = %#v, %v", budgets, err)
	}
	budget, err := client.PutWorkspaceBudget(context.Background(), "workspace-slug", "lifetime", 11)
	if err != nil || budget.ResetInterval != "lifetime" {
		t.Fatalf("PutWorkspaceBudget() = %#v, %v", budget, err)
	}
	if got, want := strings.Join(methods, ","), "GET /workspaces/workspace-slug/budgets,PUT /workspaces/workspace-slug/budgets/lifetime"; got != want {
		t.Fatalf("methods = %q, want %q", got, want)
	}
}

func TestPutWorkspaceBudgetRejectsWrongOrInvalidResponse(t *testing.T) {
	for name, response := range map[string]map[string]any{
		"wrong interval":   workspaceBudgetResponse("weekly", 10),
		"invalid limit":    workspaceBudgetResponse("daily", 0),
		"unknown interval": workspaceBudgetResponse("yearly", 10),
		"missing id": func() map[string]any {
			r := workspaceBudgetResponse("daily", 10)
			r["data"].(map[string]any)["id"] = ""
			return r
		}(),
	} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(response)
			}))
			defer server.Close()
			if _, err := New("token", server.URL, "", time.Second).PutWorkspaceBudget(context.Background(), "workspace", "daily", 10); err == nil {
				t.Fatal("expected invalid response error")
			}
		})
	}
}

func TestListWorkspaceBudgetsRejectsInvalidResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{workspaceBudgetResponse("daily", -1)["data"]}})
	}))
	defer server.Close()
	if _, err := New("token", server.URL, "", time.Second).ListWorkspaceBudgets(context.Background(), "workspace"); err == nil {
		t.Fatal("expected invalid list response error")
	}
}

func TestPutWorkspaceBudgetRejectsMissingResetInterval(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
			"id": "budget", "workspace_id": "workspace", "limit_usd": 10,
		}})
	}))
	defer server.Close()
	if _, err := New("token", server.URL, "", time.Second).PutWorkspaceBudget(context.Background(), "workspace", "daily", 10); err == nil {
		t.Fatal("expected missing reset_interval error")
	}
}
