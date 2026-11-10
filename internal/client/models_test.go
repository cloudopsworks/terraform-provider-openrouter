package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testPublicModel() map[string]any {
	return map[string]any{"id": "test/resolved", "canonical_slug": "test/canonical", "name": "Test", "description": nil, "created": 123, "context_length": nil, "pricing": map[string]any{"prompt": "0.000000000000000000123456789", "completion": "0.1", "request": "0"}, "architecture": map[string]any{"input_modalities": []string{"text"}, "output_modalities": []string{"text"}}, "supported_parameters": []string{}}
}
func TestModelLookup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/model/test/alias:free" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": testPublicModel()})
	}))
	defer server.Close()
	model, err := New("local", server.URL, "", time.Second).GetModel(context.Background(), "test/alias:free")
	if err != nil {
		t.Fatal(err)
	}
	if model.ID != "test/resolved" || model.ContextLength != nil || model.Pricing.Prompt != "0.000000000000000000123456789" || model.SupportedParameters == nil {
		t.Fatalf("lookup lost alias, nullable or decimal semantics: %+v", model)
	}
	for _, slug := range []string{"", "test", "test/model/extra", "/model", "test/..", "test/model?bad", "test/model#bad", "test/ model"} {
		if _, _, err := SplitModelSlug(slug); err == nil {
			t.Errorf("accepted malformed slug %q", slug)
		}
	}
}
func TestModelMalformedResponses(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"missing name", func(m map[string]any) { delete(m, "name") }},
		{"missing created", func(m map[string]any) { delete(m, "created") }},
		{"null pricing", func(m map[string]any) { m["pricing"] = nil }},
		{"numeric pricing", func(m map[string]any) { m["pricing"].(map[string]any)["prompt"] = 0.01 }},
		{"null prompt", func(m map[string]any) { m["pricing"].(map[string]any)["prompt"] = nil }},
		{"missing context", func(m map[string]any) { delete(m, "context_length") }},
		{"null array", func(m map[string]any) { m["supported_parameters"] = nil }},
		{"null array entry", func(m map[string]any) { m["supported_parameters"] = []any{nil} }},
		{"null input entry", func(m map[string]any) { m["architecture"].(map[string]any)["input_modalities"] = []any{nil} }},
		{"negative context", func(m map[string]any) { m["context_length"] = -1 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := testPublicModel()
			tt.mutate(m)
			b, _ := json.Marshal(m)
			var result Model
			if err := json.Unmarshal(b, &result); err == nil {
				t.Fatalf("accepted invalid model %s", b)
			}
		})
	}
	for _, body := range []string{`{}`, `{"data":null}`, `{"data":[]}`} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
			defer server.Close()
			if _, err := New("local", server.URL, "", time.Second).GetModel(context.Background(), "test/model"); err == nil {
				t.Fatal("accepted missing model response")
			}
		})
	}
}
func TestModelsPageBoundedAndFiltered(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodGet || r.URL.Path != "/models" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		for k, v := range map[string]string{"offset": "2", "limit": "1", "q": "test model", "input_modalities": "text,image", "output_modalities": "all"} {
			if r.URL.Query().Get(k) != v {
				t.Errorf("query %s=%q, want %q", k, r.URL.Query().Get(k), v)
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{testPublicModel()}, "total_count": 10, "links": map[string]any{"next": "https://untrusted.invalid/steal"}})
	}))
	defer server.Close()
	result, err := New("local", server.URL, "", time.Second).GetModelsPage(context.Background(), ModelsPageOptions{Offset: 2, Limit: 1, Search: "test model", InputModalities: "text,image", OutputModalities: "all"})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || len(result.Data) != 1 || *result.TotalCount != 10 {
		t.Fatalf("bounded page mismatch: calls%d page%+v", calls, result)
	}
	for _, o := range []ModelsPageOptions{{Offset: -1, Limit: 1}, {Limit: 0}, {Limit: 1001}} {
		if _, err := New("local", server.URL, "", time.Second).GetModelsPage(context.Background(), o); err == nil {
			t.Error("accepted invalid page bound")
		}
	}
}
func TestModelsPageMalformedAndErrors(t *testing.T) {
	for _, body := range []string{`{}`, `{"data":null,"total_count":1,"links":{"next":null}}`, `{"data":[],"total_count":null,"links":{"next":null}}`, `{"data":[],"total_count":0,"links":{}}`, `{"data":[],"total_count":-1,"links":{"next":null}}`} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
			defer server.Close()
			if _, err := New("local", server.URL, "", time.Second).GetModelsPage(context.Background(), ModelsPageOptions{Limit: 1}); err == nil {
				t.Fatal("accepted malformed page")
			}
		})
	}
	for _, status := range []int{401, 403, 404, 429, 500} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
				fmt.Fprint(w, `{"error":{"message":"test failure"}}`)
			}))
			defer server.Close()
			c := New("local", server.URL, "", time.Second)
			if _, err := c.GetModel(context.Background(), "test/model"); err == nil || !strings.Contains(err.Error(), fmt.Sprintf("(%d)", status)) {
				t.Fatalf("model error not surfaced: %v", err)
			}
			if _, err := c.GetModelsPage(context.Background(), ModelsPageOptions{Limit: 1}); err == nil {
				t.Fatal("catalog error swallowed")
			}
		})
	}
}
