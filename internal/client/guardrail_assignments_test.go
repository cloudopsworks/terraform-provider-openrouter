package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

func TestGuardrailKeyAssignmentsClient(t *testing.T) {
	t.Parallel()

	var requests []struct {
		Method string
		Path   string
		Body   map[string][]string
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entry := struct {
			Method string
			Path   string
			Body   map[string][]string
		}{Method: r.Method, Path: r.URL.Path}
		if r.Method == http.MethodPost {
			if err := json.NewDecoder(r.Body).Decode(&entry.Body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
		}
		requests = append(requests, entry)
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			if got := r.URL.Query().Get("limit"); got != "100" {
				t.Fatalf("limit = %q, want 100", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]string{{"key_hash": "key-1"}, {"hash": "key-2"}}, "total_count": 2})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{}})
	}))
	defer server.Close()

	cl := New("token", server.URL, "test-agent", time.Second)
	keys, err := cl.ListGuardrailKeyAssignments(context.Background(), "guardrail-1")
	if err != nil {
		t.Fatalf("ListGuardrailKeyAssignments() error = %v", err)
	}
	if want := []string{"key-1", "key-2"}; !reflect.DeepEqual(keys, want) {
		t.Fatalf("keys = %#v, want %#v", keys, want)
	}
	if err := cl.AssignGuardrailKeys(context.Background(), "guardrail-1", []string{"key-1"}); err != nil {
		t.Fatalf("AssignGuardrailKeys() error = %v", err)
	}
	if err := cl.RemoveGuardrailKeys(context.Background(), "guardrail-1", []string{"key-1"}); err != nil {
		t.Fatalf("RemoveGuardrailKeys() error = %v", err)
	}
	if len(requests) != 3 {
		t.Fatalf("requests = %d, want 3", len(requests))
	}
	if got, want := requests[1].Path, "/guardrails/guardrail-1/assignments/keys"; got != want {
		t.Fatalf("assign path = %q, want %q", got, want)
	}
	if got, want := requests[2].Path, "/guardrails/guardrail-1/assignments/keys/remove"; got != want {
		t.Fatalf("remove path = %q, want %q", got, want)
	}
	for _, request := range requests[1:] {
		if got, want := request.Body["key_hashes"], []string{"key-1"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("request body = %#v, want key_hashes %#v", request.Body, want)
		}
	}
}

func TestListGuardrailKeyAssignmentsPaginates(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		offset := r.URL.Query().Get("offset")
		if offset == "0" {
			items := make([]map[string]string, 100)
			for i := range items {
				items[i] = map[string]string{"key_hash": "key" + string(rune(i))}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": items, "total_count": 101})
			return
		}
		if offset != "100" {
			t.Fatalf("offset = %q, want 100", offset)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]string{{"key_hash": "last"}}, "total_count": 101})
	}))
	defer server.Close()

	keys, err := New("token", server.URL, "test-agent", time.Second).ListGuardrailKeyAssignments(context.Background(), "guardrail-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 101 {
		t.Fatalf("keys = %d, want 101", len(keys))
	}
}

func TestListGuardrailKeyAssignmentBindingsPaginates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/guardrails/assignments/keys" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if r.URL.Query().Get("offset") == "0" {
			items := make([]map[string]string, 100)
			for i := range items {
				items[i] = map[string]string{"guardrail_id": "guardrail", "key_hash": fmt.Sprintf("key-%d", i)}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": items, "total_count": 101})
			return
		}
		if r.URL.Query().Get("offset") != "100" {
			t.Fatalf("offset=%s", r.URL.Query().Get("offset"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]string{{"guardrail_id": "guardrail", "key_hash": "last"}}, "total_count": 101})
	}))
	defer server.Close()
	items, err := New("token", server.URL, "", time.Second).ListGuardrailKeyAssignmentBindings(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 101 {
		t.Fatalf("items=%d", len(items))
	}
}
