package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"reflect"
	"sync"
	"testing"

	"github.com/cloudopsworks/terraform-provider-openrouter/internal/client"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

// These tests use the real Terraform engine and protocol server, but all API
// calls are confined to a local fake. They never require OpenRouter credentials.
func TestPolicyAssignmentResourceProtocol(t *testing.T) {
	if os.Getenv("TF_ACC_TERRAFORM_PATH") == "" {
		binary, err := exec.LookPath("terraform")
		if err != nil {
			t.Skip("Terraform executable is required for local protocol tests")
		}
		t.Setenv("TF_ACC_TERRAFORM_PATH", binary)
	}
	for _, subject := range []struct{ suffix, attribute, endpoint string }{
		{"api_key_policy_assignment", "key_hash", "api-keys"},
		{"member_policy_assignment", "member_id", "members"},
	} {
		t.Run(subject.suffix, func(t *testing.T) {
			var mu sync.Mutex
			current := client.PolicyAssignmentState{PolicyIDs: []string{}, WorkspaceDefaultPolicyIDs: []string{"inherited"}}
			assignments, removals := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				if req.Header.Get("Authorization") != "Bearer local-management-key" {
					t.Errorf("unexpected authorization header")
					http.Error(w, "unauthorized", 401)
					return
				}
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/key":
					_, _ = w.Write([]byte(`{"data":{"label":"local-key","is_management_key":true}}`))
				case req.Method == http.MethodGet && (req.URL.Path == "/guardrails/policies/budget/policy" || req.URL.Path == "/guardrails/policies/budget/replacement-policy"):
					if req.URL.Query().Get("workspace_id") != "workspace" {
						t.Errorf("policy query=%s", req.URL.RawQuery)
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": req.URL.Path[len("/guardrails/policies/budget/"):], "policy_type": "budget", "reference": nil, "is_workspace_default": false}})
				case req.URL.Path == "/guardrails/policies/assignments/"+subject.endpoint+"/subject":
					if req.Method == http.MethodGet {
						if req.URL.Query().Get("workspace_id") != "workspace" || req.URL.Query().Get("policy_type") != "budget" {
							t.Errorf("assignment query=%s", req.URL.RawQuery)
						}
						_ = json.NewEncoder(w).Encode(map[string]any{"data": current})
						return
					}
					if req.Method != http.MethodPut {
						t.Errorf("unexpected method=%s", req.Method)
						http.Error(w, "bad method", 405)
						return
					}
					var body client.ChangePolicyAssignmentRequest
					if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
						t.Error(err)
						http.Error(w, "invalid json", 400)
						return
					}
					if !reflect.DeepEqual(body.ExpectedAssignment, current) {
						t.Errorf("CAS token=%#v want %#v", body.ExpectedAssignment, current)
						http.Error(w, "CAS conflict", 409)
						return
					}
					if body.WorkspaceID != "workspace" || body.PolicyType != "budget" || (body.PolicyID != "policy" && body.PolicyID != "replacement-policy") {
						t.Errorf("request=%#v", body)
						http.Error(w, "wrong identity", 400)
						return
					}
					if body.Assigned {
						assignments++
						current.PolicyIDs = []string{body.PolicyID}
					} else {
						removals++
						current.PolicyIDs = []string{}
					}
					_, _ = w.Write([]byte(`{"data":true}`))
				default:
					t.Errorf("unexpected API request %s %s", req.Method, req.URL)
					http.Error(w, "unexpected request", 404)
				}
			}))
			defer server.Close()
			address := "openrouter_" + subject.suffix + ".test"
			config := func(policyID string) string {
				return fmt.Sprintf(`
provider "openrouter" {
 api_key = "local-management-key"
 base_url = %q
}
resource "openrouter_%s" "test" {
 workspace_id = "workspace"
 policy_type = "budget"
 policy_id = %q
 %s = "subject"
}
`, server.URL, subject.suffix, policyID, subject.attribute)
			}
			resource.UnitTest(t, resource.TestCase{
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_11_0)},
				CheckDestroy: func(_ *terraform.State) error {
					mu.Lock()
					defer mu.Unlock()
					if len(current.PolicyIDs) != 0 || current.OptedOut || !reflect.DeepEqual(current.WorkspaceDefaultPolicyIDs, []string{"inherited"}) {
						return fmt.Errorf("destroy changed unrelated state: %#v", current)
					}
					if assignments != 2 || removals != 2 {
						return fmt.Errorf("unexpected mutation counts: assignments=%d removals=%d", assignments, removals)
					}
					return nil
				},
				Steps: []resource.TestStep{
					{Config: config("policy"), Check: resource.ComposeAggregateTestCheckFunc(
						resource.TestCheckResourceAttr(address, "id", "workspace/budget/subject/policy"),
						resource.TestCheckResourceAttr(address, "workspace_id", "workspace"),
						resource.TestCheckResourceAttr(address, "policy_type", "budget"),
						resource.TestCheckResourceAttr(address, "policy_id", "policy"),
						resource.TestCheckResourceAttr(address, subject.attribute, "subject"),
					)},
					{Config: config("policy"), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
					{ResourceName: address, ImportState: true, ImportStateId: "workspace/budget/subject/policy", ImportStateVerify: true},
					{Config: config("replacement-policy"), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(address, plancheck.ResourceActionDestroyBeforeCreate)}}},
				},
			})
			// The replacement and final destroy must leave no managed direct binding.
			mu.Lock()
			defer mu.Unlock()
			if len(current.PolicyIDs) != 0 {
				t.Errorf("assignment remained after destroy: %#v", current)
			}
		})
	}
}
