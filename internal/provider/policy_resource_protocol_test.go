package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// Exercise the actual Terraform protocol against a local API, including no-op
// plans and import verification. No OpenRouter credentials or live API are used.
func TestPolicyResourceProtocol(t *testing.T) {
	if _, err := exec.LookPath("terraform"); err != nil {
		t.Skip("terraform executable required for local protocol test")
	}
	for _, kind := range []string{"budget", "model_availability"} {
		t.Run(kind, func(t *testing.T) {
			var mu sync.Mutex
			var remote map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/key" {
					_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"label": "local-management", "is_management_key": true}})
					return
				}
				switch r.Method {
				case http.MethodPost, http.MethodPut:
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
						http.Error(w, "bad", 400)
						return
					}
					remote = map[string]any{"id": "policy", "name": body["name"], "policy_type": kind, "config": body["config"], "member_ids": []string{"external-member"}, "api_key_hashes": []string{"external-key"}}
				case http.MethodDelete:
					remote = nil
					w.WriteHeader(http.StatusNoContent)
					return
				case http.MethodGet:
					if remote == nil {
						http.Error(w, "missing", 404)
						return
					}
				default:
					http.Error(w, "unexpected", 500)
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"data": remote})
			}))
			defer server.Close()
			initial := `{ limit_usd = 25, reset_interval = "monthly", include_byok_in_budgets = false }`
			updated := `{ limit_usd = null, reset_interval = null, include_byok_in_budgets = true }`
			if kind != "budget" {
				initial = `{ allowed_models = ["openai/gpt-4o"], allowed_data_regions = ["eu"], enforce_zdr_xai = false }`
				updated = `{ allowed_models = [], enforce_zdr_xai = true }`
			}
			config := func(body string) string {
				return fmt.Sprintf(`
provider "openrouter" {
 api_key = "local-management"
 base_url = %q
}
resource "openrouter_%s_policy" "test" {
 workspace_id = "ws"
 name = "policy name"
 config = %s
}
`, server.URL, kind, body)
			}
			address := "openrouter_" + kind + "_policy.test"
			resource.UnitTest(t, resource.TestCase{ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){"openrouter": providerserver.NewProtocol6WithError(New("test")())}, Steps: []resource.TestStep{
				{Config: config(initial)},
				{Config: config(initial), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
				{ResourceName: address, ImportState: true, ImportStateId: "ws/policy", ImportStateVerify: true},
				{Config: config(updated)},
				{Config: config(updated), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
				{ResourceName: address, ImportState: true, ImportStateId: "ws/policy", ImportStateVerify: true},
			}})
		})
	}
}
