package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cloudopsworks/terraform-provider-openrouter/internal/client"
	frameworkdatasource "github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func byokKeysTestSchema(t *testing.T) datasourceschema.Schema {
	t.Helper()
	var response frameworkdatasource.SchemaResponse
	NewBYOKKeysDataSource().Schema(context.Background(), frameworkdatasource.SchemaRequest{}, &response)
	if response.Diagnostics.HasError() {
		t.Fatal(response.Diagnostics)
	}
	return response.Schema
}
func TestBYOKKeysDataSourceSchemaDoesNotExposeKey(t *testing.T) {
	schema := byokKeysTestSchema(t)
	items := schema.Attributes["items"].(datasourceschema.ListNestedAttribute)
	if _, ok := items.NestedObject.Attributes["key"]; ok {
		t.Fatal("data source must never expose raw BYOK keys")
	}
	if !schema.Attributes["workspace_id"].(datasourceschema.StringAttribute).Optional || !schema.Attributes["provider_slug"].(datasourceschema.StringAttribute).Optional {
		t.Fatal("filters must be optional")
	}
}
func TestBYOKDataSourceItemTypeHasNoSecret(t *testing.T) {
	types := byokDataSourceItemTypes()
	if _, ok := types["key"]; ok {
		t.Fatal("data source item type must not expose a raw key")
	}
	if _, ok := types["label"]; !ok {
		t.Fatal("data source item type must include metadata")
	}
}

func TestBYOKKeysDataSourceReadFiltersAndPreservesNullCollections(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet || req.URL.Path != "/byok" || req.URL.Query().Get("workspace_id") != "ws" || req.URL.Query().Get("provider") != "openai" {
			t.Fatalf("request %s", req.URL.String())
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]any{"id": "id", "provider": "openai", "workspace_id": "ws", "label": "masked", "name": nil, "declared_zdr": nil, "disabled": false, "is_fallback": false, "is_required": false, "is_byok_only": false, "allowed_models": nil, "allowed_api_key_hashes": nil, "allowed_user_ids": nil, "sort_order": 0, "created_at": "now"}}, "total_count": 1})
	}))
	defer server.Close()
	schema := byokKeysTestSchema(t)
	configModel := byokKeysDataSourceModel{WorkspaceID: types.StringValue("ws"), ProviderSlug: types.StringValue("openai"), TotalCount: types.Int64Unknown(), Items: types.ListUnknown(types.ObjectType{AttrTypes: byokDataSourceItemTypes()})}
	raw := tfsdk.Plan{Schema: schema}
	if d := raw.Set(context.Background(), &configModel); d.HasError() {
		t.Fatal(d)
	}
	config := tfsdk.Config{Schema: schema, Raw: raw.Raw}
	response := frameworkdatasource.ReadResponse{State: tfsdk.State{Schema: schema}}
	(&byokKeysDataSource{client: client.New("token", server.URL, "", time.Second)}).Read(context.Background(), frameworkdatasource.ReadRequest{Config: config}, &response)
	if response.Diagnostics.HasError() {
		t.Fatal(response.Diagnostics)
	}
	var state byokKeysDataSourceModel
	if d := response.State.Get(context.Background(), &state); d.HasError() {
		t.Fatal(d)
	}
	if state.TotalCount.ValueInt64() != 1 {
		t.Fatalf("count=%d", state.TotalCount.ValueInt64())
	}
	elements := state.Items.Elements()
	if len(elements) != 1 {
		t.Fatalf("items=%#v", elements)
	}
	item := elements[0].(types.Object).Attributes()
	for _, name := range []string{"allowed_models", "allowed_api_key_hashes", "allowed_user_ids"} {
		if !item[name].(types.List).IsNull() {
			t.Fatalf("%s must preserve API null", name)
		}
	}
}
