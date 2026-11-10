package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"sync"
	"testing"

	fp "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

type apiKeyCreateProtocolProvider struct{ fp.Provider }

func (p *apiKeyCreateProtocolProvider) Resources(context.Context) []func() frameworkresource.Resource {
	return []func() frameworkresource.Resource{NewAPIKeyResource}
}

func TestAPIKeyCreateDisabledProtocol(t *testing.T) {
	if _, err := exec.LookPath("terraform"); err != nil {
		t.Skip("terraform executable required for local protocol test")
	}
	for _, tt := range []struct {
		name     string
		config   string
		disabled string
		patches  int
	}{
		{name: "disabled", config: "disabled = true", disabled: "true", patches: 1},
		{name: "enabled", config: "disabled = false", disabled: "false"},
		{name: "omitted", disabled: "false"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var mu sync.Mutex
			var remote map[string]any
			creates, patches, deletes := 0, 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/key" {
					_, _ = w.Write([]byte(`{"data":{"label":"local-management","is_management_key":true}}`))
					return
				}
				if r.URL.Path != "/keys" && r.URL.Path != "/keys/local-key-hash" {
					t.Errorf("unexpected path: %s", r.URL.Path)
					http.NotFound(w, r)
					return
				}
				switch r.Method {
				case http.MethodPost:
					creates++
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
						http.Error(w, "invalid body", http.StatusBadRequest)
						return
					}
					if _, sent := body["disabled"]; sent {
						t.Error("POST must not send undocumented disabled field")
					}
					if body["limit"] != float64(0) {
						t.Error("POST must preserve explicit zero spending limit")
					}
					remote = body
					remote["hash"] = "local-key-hash"
					remote["disabled"] = false
					remote["created_at"] = "2026-11-10T00:00:00Z"
					_ = json.NewEncoder(w).Encode(map[string]any{"data": remote, "key": "local-created-secret"})
					return
				case http.MethodPatch:
					patches++
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
						http.Error(w, "invalid body", http.StatusBadRequest)
						return
					}
					if len(body) != 1 || body["disabled"] != true {
						t.Errorf("post-create PATCH must only disable the key, got %v", body)
					}
					remote["disabled"] = true
				case http.MethodGet:
					if remote == nil {
						http.NotFound(w, r)
						return
					}
				case http.MethodDelete:
					deletes++
					remote = nil
					_, _ = w.Write([]byte(`{"deleted":true}`))
					return
				default:
					t.Errorf("unexpected method: %s", r.Method)
					http.Error(w, "unexpected method", http.StatusMethodNotAllowed)
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"data": remote})
			}))
			defer server.Close()
			config := fmt.Sprintf(`
provider "openrouter" {
 api_key = "local-management"
 base_url = %q
}
resource "openrouter_api_key" "test" {
 name = "zero-limit-key"
 limit = 0
 %s
}
`, server.URL, tt.config)
			check := resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("openrouter_api_key.test", "disabled", tt.disabled),
				resource.TestCheckResourceAttr("openrouter_api_key.test", "limit", "0"),
				resource.TestCheckResourceAttr("openrouter_api_key.test", "key", "local-created-secret"),
			)
			resource.UnitTest(t, resource.TestCase{
				ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
					"openrouter": providerserver.NewProtocol6WithError(&apiKeyCreateProtocolProvider{New("test")()}),
				},
				Steps: []resource.TestStep{
					{Config: config, Check: check},
					{Config: config, Check: check, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
				},
			})
			mu.Lock()
			defer mu.Unlock()
			if creates != 1 || patches != tt.patches || deletes != 1 {
				t.Fatalf("API calls: creates=%d patches=%d deletes=%d, want 1/%d/1", creates, patches, deletes, tt.patches)
			}
		})
	}
}
