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

const observabilityTestID = "11111111-2222-3333-4444-555555555555"

func observabilityResponse() map[string]any {
	return map[string]any{"id": observabilityTestID, "workspace_id": "550e8400-e29b-41d4-a716-446655440000", "type": "langfuse", "name": "test", "enabled": false, "privacy_mode": true, "sampling_rate": 1, "regions": []string{"global"}, "api_key_hashes": nil, "broadcast_generation_cost": false, "broadcast_generation_identity": false, "broadcast_generation_request_context": false, "created_at": "2026-11-09T00:00:00Z", "updated_at": "2026-11-09T00:00:00Z", "config": map[string]any{"secretKey": "response-secret-canary", "publicKey": "pk-l...masked"}}
}
func TestObservabilityClientCRUDAndSecretBoundary(t *testing.T) {
	var bodies []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodDelete {
			_, _ = w.Write([]byte(`{"deleted":true}`))
			return
		}
		if r.Method == http.MethodPost || r.Method == http.MethodPatch {
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			bodies = append(bodies, body)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": observabilityResponse()})
	}))
	defer server.Close()
	cl := New("local", server.URL, "test", time.Second)
	body := ObservabilityDestinationWrite{WorkspaceID: "550e8400-e29b-41d4-a716-446655440000", Type: "langfuse", Name: "test", PrivacyMode: true, SamplingRate: 1, Regions: []string{"global"}, Config: json.RawMessage(`{"secretKey":"request-secret-canary"}`)}
	got, err := cl.CreateObservabilityDestination(context.Background(), body)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(got)
	if strings.Contains(string(encoded), "secret") || strings.Contains(string(encoded), "masked") {
		t.Fatal("provider configuration retained in client metadata")
	}
	if _, err := cl.GetObservabilityDestination(context.Background(), observabilityTestID); err != nil {
		t.Fatal(err)
	}
	body.Config = nil
	if _, err := cl.UpdateObservabilityDestination(context.Background(), observabilityTestID, body); err != nil {
		t.Fatal(err)
	}
	if err := cl.DeleteObservabilityDestination(context.Background(), observabilityTestID); err != nil {
		t.Fatal(err)
	}
	if len(bodies) != 2 {
		t.Fatalf("write count=%d", len(bodies))
	}
	if bodies[0]["enabled"] != false || bodies[0]["privacy_mode"] != true || bodies[0]["api_key_hashes"] != nil {
		t.Fatal("false/privacy/null semantics lost")
	}
	if _, ok := bodies[1]["config"]; ok {
		t.Fatal("metadata update resent config")
	}
	for _, field := range []string{"type", "workspace_id", "filter_rules", "broadcast_generation_cost_allocation"} {
		if _, ok := bodies[1][field]; ok {
			t.Fatalf("PATCH must omit %s", field)
		}
	}
}
func TestObservabilityClientMalformedAndMismatchedResponses(t *testing.T) {
	for _, mutation := range []struct {
		name   string
		change func(map[string]any)
	}{
		{"null boolean", func(m map[string]any) { m["enabled"] = nil }},
		{"missing privacy", func(m map[string]any) { delete(m, "privacy_mode") }},
		{"null region", func(m map[string]any) { m["regions"] = []any{nil} }},
		{"null hash", func(m map[string]any) { m["api_key_hashes"] = []any{nil} }},
		{"mismatched id", func(m map[string]any) { m["id"] = "different" }},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				m := observabilityResponse()
				mutation.change(m)
				_ = json.NewEncoder(w).Encode(map[string]any{"data": m})
			}))
			defer server.Close()
			cl := New("local", server.URL, "test", time.Second)
			if got, err := cl.GetObservabilityDestination(context.Background(), observabilityTestID); err == nil || got != nil {
				t.Fatal("malformed response accepted")
			}
		})
	}
}
func TestObservabilityClientSensitiveErrors(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodGet, http.MethodPatch, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(500)
				_, _ = w.Write([]byte(`{"error":{"message":"secret-canary (404)"}}`))
			}))
			defer server.Close()
			cl := New("local", server.URL, "test", time.Second)
			var err error
			switch method {
			case http.MethodPost:
				var got *ObservabilityDestination
				got, err = cl.CreateObservabilityDestination(context.Background(), ObservabilityDestinationWrite{})
				if got != nil {
					t.Fatal("ordinary API failure supplied recovery identity")
				}
			case http.MethodGet:
				_, err = cl.GetObservabilityDestination(context.Background(), observabilityTestID)
			case http.MethodPatch:
				_, err = cl.UpdateObservabilityDestination(context.Background(), observabilityTestID, ObservabilityDestinationWrite{})
			case http.MethodDelete:
				err = cl.DeleteObservabilityDestination(context.Background(), observabilityTestID)
			}
			if err == nil || strings.Contains(err.Error(), "canary") || !strings.Contains(err.Error(), "(500)") {
				t.Fatalf("unsafe error: %v", err)
			}
		})
	}
}
func TestObservabilityDeleteRequiresConfirmation(t *testing.T) {
	for _, body := range []string{`{}`, `{"deleted":null}`, `{"deleted":false}`, ``} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) }))
			defer server.Close()
			if err := New("local", server.URL, "test", time.Second).DeleteObservabilityDestination(context.Background(), observabilityTestID); err == nil {
				t.Fatal("unconfirmed delete accepted")
			}
		})
	}
}

func TestObservabilityCreateRetainsOnlyScopedIdentityForMalformedMetadata(t *testing.T) {
	for _, test := range []struct {
		name    string
		mutate  func(map[string]any)
		trusted bool
	}{
		{"missing metadata", func(m map[string]any) { delete(m, "enabled") }, true},
		{"null metadata", func(m map[string]any) { m["privacy_mode"] = nil }, true},
		{"wrong workspace", func(m map[string]any) { m["workspace_id"] = "other" }, false},
		{"wrong type", func(m map[string]any) { m["type"] = "webhook" }, false},
		{"missing identity", func(m map[string]any) { delete(m, "id") }, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			posts := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				posts++
				m := observabilityResponse()
				test.mutate(m)
				_ = json.NewEncoder(w).Encode(map[string]any{"data": m})
			}))
			defer server.Close()
			got, err := New("local", server.URL, "test", time.Second).CreateObservabilityDestination(context.Background(), ObservabilityDestinationWrite{WorkspaceID: "550e8400-e29b-41d4-a716-446655440000", Type: "langfuse"})
			if err == nil || (got != nil) != test.trusted || posts != 1 {
				t.Fatalf("recovery trusted=%t got=%v err=%v posts=%d", test.trusted, got, err, posts)
			}
			data, _ := json.Marshal(got)
			if strings.Contains(string(data)+err.Error(), "secret") || strings.Contains(string(data)+err.Error(), "masked") {
				t.Fatal("sensitive response configuration retained")
			}
			if got != nil && (got.ID != observabilityTestID || got.Name != nil || got.CreatedAt != "") {
				t.Fatal("partial metadata retained instead of identity only")
			}
		})
	}
}
