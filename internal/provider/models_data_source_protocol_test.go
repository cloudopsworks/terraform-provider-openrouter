package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"regexp"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	fp "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	tfresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// Isolated registration lets these tests run independently of integration edits
// to provider.go and proves no resource mutation can be requested.
type readonlyProtocolProvider struct{ fp.Provider }

func (p *readonlyProtocolProvider) DataSources(context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{NewModelDataSource, NewModelsDataSource, NewActivityDataSource}
}
func (p *readonlyProtocolProvider) Resources(context.Context) []func() resource.Resource { return nil }
func readonlyProtocolFactories() map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){"openrouter": providerserver.NewProtocol6WithError(&readonlyProtocolProvider{New("test")()})}
}

func protocolModel(id string) map[string]any {
	return map[string]any{"id": id, "canonical_slug": id, "name": "Local model", "description": nil, "created": 123, "context_length": nil, "knowledge_cutoff": nil, "expiration_date": nil, "pricing": map[string]any{"prompt": "0.000000000000000000123456789", "completion": "0.1"}, "architecture": map[string]any{"input_modalities": []string{"text"}, "output_modalities": []string{"text"}}, "supported_parameters": []string{"temperature"}}
}
func protocolActivity(endpoint string) map[string]any {
	return map[string]any{"date": "2026-11-08", "model": "test/model", "model_permaslug": "test/model-v1", "endpoint_id": endpoint, "provider_name": "Local", "workspace_id": "ws", "usage": 0.015, "byok_usage_inference": 0, "requests": 1, "prompt_tokens": 5, "completion_tokens": 2, "reasoning_tokens": 0, "cached_tokens": 10}
}

func TestReadOnlyDataSourcesProtocol(t *testing.T) {
	if _, err := exec.LookPath("terraform"); err != nil {
		t.Skip("terraform executable required for local protocol test")
	}
	var writes, reads atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writes.Add(1)
			http.Error(w, "mutation forbidden", 500)
			return
		}
		reads.Add(1)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/key":
			fmt.Fprint(w, `{"data":{"label":"local-management","is_management_key":true}}`)
		case "/model/test/alias:free":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": protocolModel("test/resolved")})
		case "/models":
			if r.URL.Query().Get("offset") != "2" || r.URL.Query().Get("limit") != "2" || r.URL.Query().Get("q") != "test" || r.URL.Query().Get("output_modalities") != "all" {
				t.Errorf("catalog filters lost: %s", r.URL.RawQuery)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{protocolModel("test/z"), protocolModel("test/a")}, "total_count": 9, "links": map[string]any{"next": "/models?offset=4&limit=2"}})
		case "/activity":
			if r.URL.Query().Get("date") != "2026-11-08" || r.URL.Query().Get("workspace_id") != "ws" || r.URL.Query().Get("group_by") != "workspace" {
				t.Errorf("activity filters lost: %s", r.URL.RawQuery)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{protocolActivity("z"), protocolActivity("a")}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	config := fmt.Sprintf(`
provider "openrouter" {
 api_key = "local-management"
 base_url = %q
}
data "openrouter_model" "single" { slug = "test/alias:free" }
data "openrouter_models" "page" {
 offset = 2
 limit = 2
 search = "test"
 output_modalities = "all"
}
data "openrouter_activity" "usage" {
 date = "2026-11-08"
 workspace_id = "ws"
 group_by_workspace = true
}
`, server.URL)
	tfresource.UnitTest(t, tfresource.TestCase{ProtoV6ProviderFactories: readonlyProtocolFactories(), Steps: []tfresource.TestStep{
		{Config: config, Check: tfresource.ComposeAggregateTestCheckFunc(
			tfresource.TestCheckResourceAttr("data.openrouter_model.single", "id", "test/resolved"),
			tfresource.TestCheckResourceAttr("data.openrouter_model.single", "slug", "test/alias:free"),
			tfresource.TestCheckResourceAttr("data.openrouter_model.single", "pricing.prompt", "0.000000000000000000123456789"),
			tfresource.TestCheckNoResourceAttr("data.openrouter_model.single", "context_length"),
			tfresource.TestCheckResourceAttr("data.openrouter_models.page", "total_count", "9"),
			tfresource.TestCheckResourceAttr("data.openrouter_models.page", "has_more", "true"),
			tfresource.TestCheckResourceAttr("data.openrouter_models.page", "items.0.id", "test/a"),
			tfresource.TestCheckResourceAttr("data.openrouter_activity.usage", "total_count", "2"),
			tfresource.TestCheckResourceAttr("data.openrouter_activity.usage", "max_items", "1000"),
			tfresource.TestCheckResourceAttr("data.openrouter_activity.usage", "items.0.endpoint_id", "a"),
			tfresource.TestCheckResourceAttr("data.openrouter_activity.usage", "items.0.usage", "0.015"),
		)},
		{Config: config, ConfigPlanChecks: tfresource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
	}})
	if writes.Load() != 0 || reads.Load() == 0 {
		t.Fatalf("read-only contract: writes=%d reads=%d", writes.Load(), reads.Load())
	}
}
func TestReadOnlyDataSourceConfigValidation(t *testing.T) {
	if _, err := exec.LookPath("terraform"); err != nil {
		t.Skip("terraform executable required for local protocol test")
	}
	for _, tt := range []struct{ name, config, pattern string }{
		{"slug", `data "openrouter_model" "bad" { slug = "bad" }`, "Invalid model slug"},
		{"limit", `data "openrouter_models" "bad" { limit = 1001 }`, "Invalid model catalog page"},
		{"date", `data "openrouter_activity" "bad" { date = "2026-02-30" }`, "Invalid activity date"},
		{"bound", `data "openrouter_activity" "bad" {
 date = "2026-11-08"
 max_items = 0
 }`, "Invalid activity state bound"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tfresource.UnitTest(t, tfresource.TestCase{ProtoV6ProviderFactories: readonlyProtocolFactories(), Steps: []tfresource.TestStep{{Config: "provider \"openrouter\" { api_key = \"local-management\" }\n" + tt.config, PlanOnly: true, ExpectError: regexp.MustCompile(tt.pattern)}}})
		})
	}
}
