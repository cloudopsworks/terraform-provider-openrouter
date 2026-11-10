package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"sync"
	"testing"

	fp "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

type observabilityProtocolProvider struct{ fp.Provider }

func (p *observabilityProtocolProvider) Resources(context.Context) []func() frameworkresource.Resource {
	return []func() frameworkresource.Resource{NewObservabilityDestinationResource}
}
func observabilityProtocolFactories() map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){"openrouter": providerserver.NewProtocol6WithError(&observabilityProtocolProvider{New("test")()})}
}

type observabilitySecretPlanCheck struct{}

func (observabilitySecretPlanCheck) CheckPlan(_ context.Context, req plancheck.CheckPlanRequest, resp *plancheck.CheckPlanResponse) {
	// Ephemeral variables keep source literals out of serialized configuration;
	// inspect the entire plan, not just a single sensitive marker.
	data, err := json.Marshal(req.Plan)
	if err != nil {
		resp.Error = err
		return
	}
	for _, secret := range []string{"submitted-secret-canary", "rotated-secret-canary", "secret-canary-response", "masked-canary"} {
		if strings.Contains(string(data), secret) {
			resp.Error = fmt.Errorf("write-only configuration appeared in plan")
			return
		}
	}
}
func observabilitySecretStateCheck(state *terraform.State) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	for _, secret := range []string{"submitted-secret-canary", "rotated-secret-canary", "secret-canary-response", "masked-canary"} {
		if strings.Contains(string(data), secret) {
			return fmt.Errorf("write-only configuration appeared in state")
		}
	}
	return nil
}
func TestObservabilityDestinationProtocol(t *testing.T) {
	if _, err := exec.LookPath("terraform"); err != nil {
		t.Skip("terraform executable required for local protocol test")
	}
	t.Setenv("TF_VAR_destination_config", `{"secretKey":"submitted-secret-canary"}`)
	var mu sync.Mutex
	var remote map[string]any
	var configWrites []string
	deletes := 0
	metadataWrites := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/key" {
			_, _ = w.Write([]byte(`{"data":{"label":"local-management","is_management_key":true}}`))
			return
		}
		switch r.Method {
		case http.MethodPost, http.MethodPatch:
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
				http.Error(w, "bad body", 400)
				return
			}
			if r.Method == http.MethodPost {
				remote = obsResponse()
				if body["enabled"] != false || body["privacy_mode"] != true {
					t.Error("unsafe provider create defaults")
				}
				remote["filter_rules"] = map[string]any{"groups": []any{}}
				remote["broadcast_generation_cost_allocation"] = true
			} else {
				metadataWrites++
				for _, field := range []string{"type", "workspace_id", "filter_rules", "broadcast_generation_cost_allocation"} {
					if _, ok := body[field]; ok {
						t.Errorf("metadata PATCH unexpectedly owns %s", field)
					}
				}
			}
			if config, ok := body["config"]; ok {
				encoded, _ := json.Marshal(config)
				configWrites = append(configWrites, string(encoded))
			}
			for k, v := range body {
				if k != "config" {
					remote[k] = v
				}
			}
			// Reads may expose plaintext or a masked value; neither can enter state.
			remote["config"] = map[string]any{"secretKey": "secret-canary-response", "publicKey": "****...masked-canary"}
		case http.MethodGet:
			if remote == nil {
				http.Error(w, "missing", 404)
				return
			}
		case http.MethodDelete:
			deletes++
			remote = nil
			_, _ = w.Write([]byte(`{"deleted":true}`))
			return
		default:
			http.Error(w, "unexpected method", 500)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": remote})
	}))
	defer server.Close()
	config := func(name string, revision int, writeConfig bool, hashes bool) string {
		args := fmt.Sprintf("config_wo_revision = %d", revision)
		if writeConfig {
			args += "\nconfig_wo = var.destination_config"
		}
		if hashes {
			args += "\napi_key_hashes = [\"" + strings.Repeat("a", 64) + "\"]"
		}
		return fmt.Sprintf(`
variable "destination_config" {
 type = string
 sensitive = true
 ephemeral = true
}
provider "openrouter" {
 api_key = "local-management"
 base_url = %q
}
resource "openrouter_observability_destination" "test" {
 workspace_id = %q
 type = "langfuse"
 name = %q
 %s
}
`, server.URL, obsWorkspace, name, args)
	}
	check := resource.ComposeTestCheckFunc(resource.TestCheckResourceAttr("openrouter_observability_destination.test", "enabled", "false"), resource.TestCheckResourceAttr("openrouter_observability_destination.test", "privacy_mode", "true"), observabilitySecretStateCheck)
	step := func(cfg string) resource.TestStep {
		return resource.TestStep{Config: cfg, Check: check, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{observabilitySecretPlanCheck{}}}}
	}
	initial := config("initial", 1, true, true)
	updated := config("updated", 1, true, false)
	imported := config("imported", 1, false, false)
	rotated := config("rotated", 2, true, false)
	resource.UnitTest(t, resource.TestCase{ProtoV6ProviderFactories: observabilityProtocolFactories(), Steps: []resource.TestStep{
		step(initial),
		{Config: initial, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan(), observabilitySecretPlanCheck{}}}},
		{Config: updated, PreConfig: func() { t.Setenv("TF_VAR_destination_config", `{"secretKey":"rotated-secret-canary"}`) }, Check: check},
		{ResourceName: "openrouter_observability_destination.test", ImportState: true, ImportStateId: obsID, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"config_wo_revision"}},
		step(imported),
		step(rotated),
		{Config: rotated, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan(), observabilitySecretPlanCheck{}}}},
	}})
	mu.Lock()
	defer mu.Unlock()
	if len(configWrites) != 2 || !strings.Contains(configWrites[0], "submitted-secret-canary") || !strings.Contains(configWrites[1], "rotated-secret-canary") {
		t.Fatal("configuration must be sent only on create and revision change")
	}
	if metadataWrites != 3 || deletes != 1 {
		t.Fatalf("metadata PATCH count=%d deletes=%d", metadataWrites, deletes)
	}
}

func TestObservabilityDestinationImportedMetadataProtocol(t *testing.T) {
	if _, err := exec.LookPath("terraform"); err != nil {
		t.Skip("terraform executable required for local protocol test")
	}
	var mu sync.Mutex
	remote := obsResponse()
	patches, deletes := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/key" {
			_, _ = w.Write([]byte(`{"data":{"label":"local-management","is_management_key":true}}`))
			return
		}
		switch r.Method {
		case http.MethodPatch:
			patches++
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if _, ok := body["config"]; ok {
				t.Error("imported metadata-only update submitted configuration")
			}
			for k, v := range body {
				remote[k] = v
			}
		case http.MethodDelete:
			deletes++
			_, _ = w.Write([]byte(`{"deleted":true}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": remote})
	}))
	defer server.Close()
	cfg := fmt.Sprintf(`
provider "openrouter" {
 api_key = "local-management"
 base_url = %q
}
resource "openrouter_observability_destination" "test" {
 workspace_id = %q
 type = "langfuse"
 name = "imported-metadata"
}
`, server.URL, obsWorkspace)
	resource.UnitTest(t, resource.TestCase{ProtoV6ProviderFactories: observabilityProtocolFactories(), Steps: []resource.TestStep{
		{Config: cfg, ResourceName: "openrouter_observability_destination.test", ImportState: true, ImportStateId: obsID, ImportStatePersist: true, ImportStateCheck: func(states []*terraform.InstanceState) error {
			if len(states) != 1 || states[0].Attributes["config_wo_revision"] != "0" || states[0].Attributes["config_wo"] != "" {
				return fmt.Errorf("import must establish zero revision without configuration")
			}
			return nil
		}},
		{Config: cfg, Check: observabilitySecretStateCheck, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{observabilitySecretPlanCheck{}}}},
		{Config: cfg, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
	}})
	mu.Lock()
	defer mu.Unlock()
	if patches != 1 || deletes != 1 {
		t.Fatalf("imported metadata lifecycle PATCH=%d DELETE=%d", patches, deletes)
	}
}

func TestObservabilityDestinationPlanWithComputedSetElements(t *testing.T) {
	if _, err := exec.LookPath("terraform"); err != nil {
		t.Skip("terraform executable required for local protocol test")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/key" {
			t.Errorf("plan must not mutate or read a destination: %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected operation", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"data":{"label":"local-management","is_management_key":true}}`))
	}))
	defer server.Close()
	cfg := fmt.Sprintf(`
provider "openrouter" {
 api_key = "local-management"
 base_url = %q
}
resource "terraform_data" "hash" { input = %q }
resource "terraform_data" "region" { input = "global" }
resource "openrouter_observability_destination" "test" {
 workspace_id = %q
 type = "langfuse"
 name = "computed-filter"
 api_key_hashes = [terraform_data.hash.output]
 regions = [terraform_data.region.output]
 config_wo = jsonencode({ secretKey = "local-only-test-value" })
 config_wo_revision = 1
}
`, server.URL, strings.Repeat("a", 64), obsWorkspace)
	resource.UnitTest(t, resource.TestCase{ProtoV6ProviderFactories: observabilityProtocolFactories(), Steps: []resource.TestStep{
		{Config: cfg, PlanOnly: true, ExpectNonEmptyPlan: true},
	}})
}
