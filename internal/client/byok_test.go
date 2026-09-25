package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

type byokRoundTripper func(*http.Request) (*http.Response, error)

func (f byokRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func byokResponse(name any) map[string]any {
	return map[string]any{
		"id":                     "11111111-2222-3333-4444-555555555555",
		"provider":               "openai",
		"workspace_id":           "550e8400-e29b-41d4-a716-446655440000",
		"label":                  "sk-...AbCd",
		"name":                   name,
		"disabled":               false,
		"is_fallback":            false,
		"is_required":            true,
		"is_byok_only":           true,
		"declared_zdr":           true,
		"allowed_models":         nil,
		"allowed_api_key_hashes": []string{},
		"allowed_user_ids":       []string{"user_1"},
		"sort_order":             0,
		"created_at":             "2025-08-24T10:30:00Z",
	}
}

func TestBYOKCredentialClientCRUDAndNullableFields(t *testing.T) {
	t.Parallel()

	var requests []struct {
		Method string
		Path   string
		Body   map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entry := struct {
			Method string
			Path   string
			Body   map[string]any
		}{Method: r.Method, Path: r.URL.Path}
		if r.Method == http.MethodPost || r.Method == http.MethodPatch {
			if err := json.NewDecoder(r.Body).Decode(&entry.Body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
		}
		requests = append(requests, entry)
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodDelete {
			_ = json.NewEncoder(w).Encode(map[string]bool{"deleted": true})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": byokResponse(nil)})
	}))
	defer server.Close()

	cl := New("management-token", server.URL, "test-agent", time.Second)
	credential, err := cl.CreateBYOKCredential(context.Background(), BYOKCreateRequest{
		Key:                 "raw-provider-secret",
		Provider:            "openai",
		Name:                BYOKString("production"),
		AllowedModels:       BYOKNullStringSlice(),
		AllowedAPIKeyHashes: BYOKStringSlice([]string{strings.Repeat("a", 64)}),
		DeclaredZDR:         BYOKBool(true),
	})
	if err != nil {
		t.Fatalf("CreateBYOKCredential() error = %v", err)
	}
	if credential.Name != nil {
		t.Fatalf("Name = %v, want nil", *credential.Name)
	}
	if credential.AllowedModels != nil {
		t.Fatalf("AllowedModels = %#v, want nil for response null", credential.AllowedModels)
	}
	if credential.AllowedAPIKeyHashes == nil || len(credential.AllowedAPIKeyHashes) != 0 {
		t.Fatalf("AllowedAPIKeyHashes = %#v, want non-nil empty slice", credential.AllowedAPIKeyHashes)
	}
	if !credential.IsRequired || !credential.IsBYOKOnly || credential.DeclaredZDR == nil || !*credential.DeclaredZDR {
		t.Fatalf("new API fields not decoded: %#v", credential)
	}

	if _, err := cl.GetBYOKCredential(context.Background(), credential.ID); err != nil {
		t.Fatalf("GetBYOKCredential() error = %v", err)
	}
	rotated := "rotated-provider-secret"
	falseValue := false
	if _, err := cl.UpdateBYOKCredential(context.Background(), credential.ID, BYOKUpdateRequest{
		Key:                 &rotated,
		Name:                BYOKNullString(),
		AllowedModels:       BYOKStringSlice([]string{}),
		AllowedAPIKeyHashes: BYOKNullStringSlice(),
		IsRequired:          &falseValue,
		DeclaredZDR:         BYOKNullBool(),
	}); err != nil {
		t.Fatalf("UpdateBYOKCredential() error = %v", err)
	}
	if err := cl.DeleteBYOKCredential(context.Background(), credential.ID); err != nil {
		t.Fatalf("DeleteBYOKCredential() error = %v", err)
	}

	if got, want := len(requests), 4; got != want {
		t.Fatalf("requests = %d, want %d", got, want)
	}
	if got, want := requests[0].Method+" "+requests[0].Path, "POST /byok"; got != want {
		t.Fatalf("create request = %q, want %q", got, want)
	}
	if got := requests[0].Body["allowed_models"]; got != nil {
		t.Fatalf("create allowed_models = %#v, want explicit null", got)
	}
	if got := requests[0].Body["allowed_api_key_hashes"]; !reflect.DeepEqual(got, []any{strings.Repeat("a", 64)}) {
		t.Fatalf("create allowed_api_key_hashes = %#v", got)
	}
	if got := requests[0].Body["declared_zdr"]; got != true {
		t.Fatalf("create declared_zdr = %#v, want true", got)
	}
	if got, want := requests[2].Method+" "+requests[2].Path, "PATCH /byok/11111111-2222-3333-4444-555555555555"; got != want {
		t.Fatalf("update request = %q, want %q", got, want)
	}
	if got := requests[2].Body["key"]; got != rotated {
		t.Fatalf("rotation key = %#v, want supplied key", got)
	}
	if got := requests[2].Body["name"]; got != nil {
		t.Fatalf("update name = %#v, want explicit null", got)
	}
	if got := requests[2].Body["allowed_api_key_hashes"]; got != nil {
		t.Fatalf("update allowed_api_key_hashes = %#v, want explicit null", got)
	}
	if got := requests[2].Body["declared_zdr"]; got != nil {
		t.Fatalf("update declared_zdr = %#v, want explicit null", got)
	}
	if got := requests[2].Body["allowed_models"]; !reflect.DeepEqual(got, []any{}) {
		t.Fatalf("update allowed_models = %#v, want explicit empty list", got)
	}
	if _, exists := requests[2].Body["allowed_user_ids"]; exists {
		t.Fatal("omitted PATCH field allowed_user_ids was sent")
	}
}

func TestListBYOKCredentialsFiltersAndPaginates(t *testing.T) {
	t.Parallel()
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/byok" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("workspace_id"); got != "workspace-1" {
			t.Fatalf("workspace_id = %q", got)
		}
		if got := r.URL.Query().Get("provider"); got != "openai" {
			t.Fatalf("provider = %q", got)
		}
		if got := r.URL.Query().Get("limit"); got != "100" {
			t.Fatalf("limit = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("offset") == "0" {
			items := make([]map[string]any, 100)
			for i := range items {
				items[i] = byokResponse("credential")
				items[i]["id"] = "credential-" + string(rune(i+1))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": items, "total_count": 101})
			return
		}
		if got := r.URL.Query().Get("offset"); got != "100" {
			t.Fatalf("offset = %q, want 100", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{byokResponse("last")}, "total_count": 101})
	}))
	defer server.Close()

	workspaceID, provider := "workspace-1", "openai"
	items, err := New("token", server.URL, "", time.Second).ListBYOKCredentials(context.Background(), &workspaceID, &provider)
	if err != nil {
		t.Fatalf("ListBYOKCredentials() error = %v", err)
	}
	if got, want := len(items), 101; got != want {
		t.Fatalf("items = %d, want %d", got, want)
	}
	if got, want := calls, 2; got != want {
		t.Fatalf("calls = %d, want %d", got, want)
	}
}

func TestBYOKCredentialRejectsInvalidResponsesAndRedactsKey(t *testing.T) {
	t.Parallel()
	const secret = "provider-secret-must-not-escape"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/byok" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"code":400,"message":"invalid ` + secret + `"}}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"provider": "openai"}})
	}))
	defer server.Close()
	cl := New("token", server.URL, "", time.Second)
	_, err := cl.CreateBYOKCredential(context.Background(), BYOKCreateRequest{Key: secret, Provider: "openai"})
	if err == nil {
		t.Fatal("CreateBYOKCredential() error = nil, want API error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("error leaks raw BYOK key: %v", err)
	}
	if !strings.Contains(err.Error(), "sensitive request") {
		t.Fatalf("error = %v, want safe sensitive error", err)
	}
	_, err = cl.GetBYOKCredential(context.Background(), "credential")
	if err == nil || !strings.Contains(err.Error(), "missing required field id") {
		t.Fatalf("GetBYOKCredential() error = %v, want response validation failure", err)
	}
}

func TestBYOKCredentialInputValidation(t *testing.T) {
	t.Parallel()
	cl := New("token", "http://127.0.0.1:1", "", time.Second)
	if _, err := cl.CreateBYOKCredential(context.Background(), BYOKCreateRequest{Provider: "openai"}); err == nil {
		t.Fatal("empty create key succeeded")
	}
	if _, err := cl.UpdateBYOKCredential(context.Background(), "", BYOKUpdateRequest{}); err == nil {
		t.Fatal("empty update id succeeded")
	}
	if err := cl.DeleteBYOKCredential(context.Background(), ""); err == nil {
		t.Fatal("empty delete id succeeded")
	}
	if _, err := cl.CreateBYOKCredential(context.Background(), BYOKCreateRequest{
		Key:                 "key",
		Provider:            "openai",
		AllowedAPIKeyHashes: BYOKStringSlice([]string{}),
	}); err == nil {
		t.Fatal("empty allowed_api_key_hashes succeeded")
	}
	if _, err := cl.CreateBYOKCredential(context.Background(), BYOKCreateRequest{
		Key:                 "key",
		Provider:            "openai",
		AllowedAPIKeyHashes: BYOKStringSlice(nil),
	}); err == nil {
		t.Fatal("nil explicit allowed_api_key_hashes succeeded")
	}
}

func TestBYOKCredentialSensitiveDecodeErrorDoesNotLeakBody(t *testing.T) {
	t.Parallel()
	const secret = "provider-secret-in-malformed-response"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":"` + secret + `"}`))
	}))
	defer server.Close()

	_, err := New("token", server.URL, "", time.Second).GetBYOKCredential(context.Background(), "credential")
	if err == nil {
		t.Fatal("GetBYOKCredential() error = nil, want decode failure")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("decode error leaks sensitive body: %v", err)
	}
	if got, want := err.Error(), "failed to decode OpenRouter sensitive response"; got != want {
		t.Fatalf("decode error = %q, want %q", got, want)
	}
}

func TestBYOKCredentialSensitiveTransportErrorDoesNotLeakKey(t *testing.T) {
	t.Parallel()
	const secret = "provider-secret-in-transport-error"
	cl := New("token", "https://example.test", "", time.Second)
	cl.httpClient = &http.Client{Transport: byokRoundTripper(func(*http.Request) (*http.Response, error) {
		return nil, errors.New(secret)
	})}

	_, err := cl.CreateBYOKCredential(context.Background(), BYOKCreateRequest{Key: secret, Provider: "openai"})
	if err == nil {
		t.Fatal("CreateBYOKCredential() error = nil, want transport failure")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("transport error leaks raw BYOK key: %v", err)
	}
	if got, want := err.Error(), "OpenRouter sensitive request failed"; got != want {
		t.Fatalf("transport error = %q, want %q", got, want)
	}
}
