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

func testActivityItem(endpoint string) map[string]any {
	return map[string]any{"date": "2026-11-08", "model": "test/model", "model_permaslug": "test/model-version", "endpoint_id": endpoint, "provider_name": "Test", "workspace_id": "ws", "usage": json.Number("0.000000000000000123456789"), "byok_usage_inference": 0, "requests": 1, "prompt_tokens": 10, "completion_tokens": 2, "reasoning_tokens": 0, "cached_tokens": 20}
}
func TestActivitySnapshotFiltersAndOrdering(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodGet || r.URL.Path != "/activity" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		for k, v := range map[string]string{"date": "2026-11-08", "workspace_id": "ws", "api_key_hash": "hash", "user_id": "user", "group_by": "workspace"} {
			if r.URL.Query().Get(k) != v {
				t.Errorf("query %s=%q", k, r.URL.Query().Get(k))
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{testActivityItem("z"), testActivityItem("a")}})
	}))
	defer server.Close()
	result, err := New("local", server.URL, "", time.Second).GetActivity(context.Background(), ActivityOptions{Date: "2026-11-08", WorkspaceID: "ws", APIKeyHash: "hash", UserID: "user", GroupByWorkspace: true})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || len(result) != 2 || result[0].EndpointID != "a" || result[0].Usage.String() != "0.000000000000000123456789" || *result[0].CachedTokens != 20 {
		t.Fatalf("snapshot semantics lost: %+v", result)
	}
}
func TestActivityMalformedResponses(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"wrong date", func(m map[string]any) { m["date"] = "2026-11-07" }},
		{"missing model", func(m map[string]any) { delete(m, "model") }},
		{"null counter", func(m map[string]any) { m["requests"] = nil }},
		{"negative counter", func(m map[string]any) { m["prompt_tokens"] = -1 }},
		{"null spend", func(m map[string]any) { m["usage"] = nil }},
		{"quoted spend", func(m map[string]any) { m["usage"] = "0.1" }},
		{"missing spend", func(m map[string]any) { delete(m, "usage") }},
		{"missing grouped workspace", func(m map[string]any) { delete(m, "workspace_id") }},
		{"unexpected workspace", func(m map[string]any) { m["workspace_id"] = "other" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				m := testActivityItem("endpoint")
				tt.mutate(m)
				_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{m}})
			}))
			defer server.Close()
			if _, err := New("local", server.URL, "", time.Second).GetActivity(context.Background(), ActivityOptions{Date: "2026-11-08", WorkspaceID: "ws", GroupByWorkspace: true}); err == nil {
				t.Fatal("accepted malformed activity")
			}
		})
	}
	for _, body := range []string{`{}`, `{"data":null}`} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
			defer server.Close()
			if _, err := New("local", server.URL, "", time.Second).GetActivity(context.Background(), ActivityOptions{Date: "2026-11-08"}); err == nil {
				t.Fatal("accepted malformed envelope")
			}
		})
	}
}
func TestActivityBoundErrorsAndEmpty(t *testing.T) {
	if err := ValidateActivityDate("2026-02-30"); err == nil {
		t.Fatal("accepted impossible date")
	}
	if err := ValidateActivityDate("2026-2-01"); err == nil {
		t.Fatal("accepted noncanonical date")
	}
	if err := ValidateActivityDate("2024-02-29"); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("user_id") == "empty" {
			fmt.Fprint(w, `{"data":[]}`)
			return
		}
		if r.URL.Query().Get("user_id") == "forbidden" {
			w.WriteHeader(403)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{testActivityItem("a"), testActivityItem("b")}})
	}))
	defer server.Close()
	c := New("local", server.URL, "", time.Second)
	if _, err := c.GetActivity(context.Background(), ActivityOptions{Date: "2026-11-08", MaxItems: 1}); err == nil {
		t.Fatal("silently exceeded state bound")
	}
	if _, err := c.GetActivity(context.Background(), ActivityOptions{Date: "2026-11-08", MaxItems: 10001}); err == nil {
		t.Fatal("accepted invalid bound")
	}
	if _, err := c.GetActivity(context.Background(), ActivityOptions{Date: "2026-11-08", UserID: "forbidden"}); err == nil {
		t.Fatal("swallowed HTTP error")
	}
	items, err := c.GetActivity(context.Background(), ActivityOptions{Date: "2026-11-08", UserID: "empty"})
	if err != nil || items == nil || len(items) != 0 {
		t.Fatalf("empty activity is valid: %+v %v", items, err)
	}
}
