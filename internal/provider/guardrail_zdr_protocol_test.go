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
)

type guardrailZDRProtocolProvider struct{ fp.Provider }

func (p *guardrailZDRProtocolProvider) Resources(context.Context) []func() frameworkresource.Resource {
	return []func() frameworkresource.Resource{NewGuardrailResource}
}

// Exercise Terraform's actual optional+computed planning and post-apply consistency
// checks against the API's aggregate-to-provider ZDR fanout behavior.
func TestGuardrailZDRProtocol(t *testing.T) {
	if _, err := exec.LookPath("terraform"); err != nil {
		t.Skip("terraform executable required for local protocol test")
	}
	fields := []string{"enforce_zdr_anthropic", "enforce_zdr_google", "enforce_zdr_openai", "enforce_zdr_other"}
	var mu sync.Mutex
	var remote map[string]any
	var writes []map[string]any
	deletes := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/key" {
			_, _ = w.Write([]byte(`{"data":{"label":"local-management","is_management_key":true}}`))
			return
		}
		if r.URL.Path != "/guardrails" && r.URL.Path != "/guardrails/local-guardrail" {
			t.Errorf("unexpected API path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case http.MethodPost, http.MethodPatch:
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
				http.Error(w, "invalid body", http.StatusBadRequest)
				return
			}
			writes = append(writes, body)
			if r.Method == http.MethodPost {
				remote = map[string]any{"id": "local-guardrail", "created_at": "2026-11-10T00:00:00Z"}
				for _, field := range fields {
					remote[field] = false
				}
			}
			if aggregate, ok := body["enforce_zdr"]; ok {
				for _, field := range fields {
					remote[field] = aggregate
				}
			}
			for key, value := range body {
				if key != "enforce_zdr" {
					remote[key] = value
				}
			}
			// The current API returns only provider flags, not the legacy aggregate.
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
			t.Errorf("unexpected API method: %s", r.Method)
			http.Error(w, "unexpected method", http.StatusMethodNotAllowed)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": remote})
	}))
	defer server.Close()
	config := func(name, zdr string) string {
		return fmt.Sprintf(`
provider "openrouter" {
 api_key = "local-management"
 base_url = %q
}
resource "openrouter_guardrail" "test" {
 name = %q
 workspace_id = "local-workspace"
 %s
}
`, server.URL, name, zdr)
	}
	checkFlags := func(values ...string) resource.TestCheckFunc {
		checks := make([]resource.TestCheckFunc, 0, len(fields))
		for i, field := range fields {
			checks = append(checks, resource.TestCheckResourceAttr("openrouter_guardrail.test", field, values[i]))
		}
		return resource.ComposeAggregateTestCheckFunc(checks...)
	}
	allFalse := checkFlags("false", "false", "false", "false")
	allTrue := checkFlags("true", "true", "true", "true")
	noOp := func(cfg string) resource.TestStep {
		return resource.TestStep{Config: cfg, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}}
	}
	initial := config("initial", "enforce_zdr = false")
	enabled := config("initial", "enforce_zdr = true")
	disabled := config("initial", "enforce_zdr = false")
	mixedFalse := config("mixed-false", "enforce_zdr = false\nenforce_zdr_anthropic = false")
	mixed := config("initial", "enforce_zdr = true\nenforce_zdr_anthropic = true")
	perProvider := config("per-provider", "enforce_zdr_google = false\nenforce_zdr_other = false")
	renamed := config("renamed", "enforce_zdr_google = false\nenforce_zdr_other = false")
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"openrouter": providerserver.NewProtocol6WithError(&guardrailZDRProtocolProvider{New("test")()}),
		},
		Steps: []resource.TestStep{
			{Config: initial, Check: allFalse},
			noOp(initial),
			{Config: enabled, Check: allTrue},
			noOp(enabled),
			{Config: disabled, Check: allFalse},
			noOp(disabled),
			{Config: mixedFalse, Check: allFalse},
			noOp(mixedFalse),
			{Config: mixed, Check: allTrue},
			noOp(mixed),
			{Config: perProvider, Check: checkFlags("true", "false", "true", "false")},
			{Config: renamed, Check: checkFlags("true", "false", "true", "false")},
			noOp(renamed),
		},
	})
	mu.Lock()
	defer mu.Unlock()
	if len(writes) != 7 || deletes != 1 {
		t.Fatalf("API writes=%d deletes=%d, want 7 writes and one delete", len(writes), deletes)
	}
	for i, write := range writes {
		for _, field := range fields {
			_, sent := write[field]
			wantSent := ((i == 3 || i == 4) && strings.HasSuffix(field, "anthropic")) || (i >= 5 && (strings.HasSuffix(field, "google") || strings.HasSuffix(field, "other")))
			if sent != wantSent {
				t.Errorf("write %d field %s present=%t, want %t: only configured ZDR fields should be sent", i, field, sent, wantSent)
			}
		}
		if _, sent := write["enforce_zdr"]; sent != (i < 5) {
			t.Errorf("write %d aggregate presence=%t, want %t", i, sent, i < 5)
		}
	}
}
