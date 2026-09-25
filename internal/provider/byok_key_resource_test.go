package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/cloudopsworks/terraform-provider-openrouter/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func byokTestSchema(t *testing.T) resourceschema.Schema {
	t.Helper()
	var response frameworkresource.SchemaResponse
	NewBYOKKeyResource().Schema(context.Background(), frameworkresource.SchemaRequest{}, &response)
	if response.Diagnostics.HasError() {
		t.Fatal(response.Diagnostics)
	}
	return response.Schema
}
func TestBYOKKeySchemaWriteOnlyAndLifecycle(t *testing.T) {
	schema := byokTestSchema(t)
	key := schema.Attributes["key_wo"].(resourceschema.StringAttribute)
	if !key.Optional || !key.WriteOnly || !key.Sensitive {
		t.Fatal("key_wo must be optional, write-only, and sensitive")
	}
	provider := schema.Attributes["provider_slug"].(resourceschema.StringAttribute)
	if len(provider.PlanModifiers) == 0 {
		t.Fatal("provider_slug must require replacement")
	}
	workspace := schema.Attributes["workspace_id"].(resourceschema.StringAttribute)
	if !workspace.Optional || !workspace.Computed || len(workspace.PlanModifiers) < 2 {
		t.Fatal("workspace_id must be optional/computed with lifecycle modifiers")
	}
	for _, name := range []string{"name", "declared_zdr", "allowed_models", "allowed_api_key_hashes", "allowed_user_ids"} {
		switch a := schema.Attributes[name].(type) {
		case resourceschema.StringAttribute:
			if !a.Optional || a.Computed {
				t.Fatalf("%s must be optional-only", name)
			}
		case resourceschema.BoolAttribute:
			if !a.Optional || a.Computed {
				t.Fatalf("%s must be optional-only", name)
			}
		case resourceschema.SetAttribute:
			if !a.Optional || a.Computed {
				t.Fatalf("%s must be optional-only", name)
			}
		}
	}
}
func TestBYOKKeyValidation(t *testing.T) {
	ctx := context.Background()
	base := byokKeyResourceModel{ProviderSlug: types.StringValue("openai"), KeyWO: types.StringValue("secret"), IsFallback: types.BoolValue(false), IsBYOKOnly: types.BoolValue(false), AllowedModels: types.SetNull(types.StringType), AllowedAPIKeyHashes: types.SetNull(types.StringType), AllowedUserIDs: types.SetNull(types.StringType)}
	if d := validateBYOKModel(ctx, base, base, true); d.HasError() {
		t.Fatal(d)
	}
	bad := base
	bad.IsFallback = types.BoolValue(true)
	bad.IsBYOKOnly = types.BoolValue(true)
	if d := validateBYOKModel(ctx, bad, bad, true); !d.HasError() {
		t.Fatal("expected fallback/byok_only diagnostic")
	}
	bad = base
	bad.AllowedAPIKeyHashes = types.SetValueMust(types.StringType, []attr.Value{})
	if d := validateBYOKModel(ctx, bad, bad, true); !d.HasError() {
		t.Fatal("expected empty hashes diagnostic")
	}
	bad = base
	bad.AllowedAPIKeyHashes = types.SetValueMust(types.StringType, []attr.Value{types.StringValue("bad")})
	if d := validateBYOKModel(ctx, bad, bad, true); !d.HasError() {
		t.Fatal("expected malformed hash diagnostic")
	}
}
func TestFlattenBYOKKeyPreservesNullAndNoSecret(t *testing.T) {
	state, diags := flattenBYOKKeyModel(context.Background(), client.BYOKCredential{ID: "id", Provider: "openai", WorkspaceID: "ws", Label: "label"}, byokKeyResourceModel{})
	if diags.HasError() {
		t.Fatal(diags)
	}
	if !state.KeyWO.IsNull() || state.KeyWOVersion.ValueInt64() != 0 {
		t.Fatal("write-only key must be absent and version initialized to zero")
	}
	if !state.Name.IsNull() || !state.DeclaredZDR.IsNull() || !state.AllowedModels.IsNull() || !state.AllowedAPIKeyHashes.IsNull() || !state.AllowedUserIDs.IsNull() {
		t.Fatal("API nulls must remain Terraform nulls")
	}
}

func TestBYOKKeyImportInitializesWriteOnlyVersion(t *testing.T) {
	schema := byokTestSchema(t)
	response := frameworkresource.ImportStateResponse{State: tfsdk.State{Schema: schema}}
	(&byokKeyResource{}).ImportState(context.Background(), frameworkresource.ImportStateRequest{ID: "11111111-2222-3333-4444-555555555555"}, &response)
	if response.Diagnostics.HasError() {
		t.Fatal(response.Diagnostics)
	}
	var state byokKeyResourceModel
	if diagnostics := response.State.Get(context.Background(), &state); diagnostics.HasError() {
		t.Fatal(diagnostics)
	}
	if state.ID.ValueString() != "11111111-2222-3333-4444-555555555555" || state.KeyWOVersion.ValueInt64() != 0 {
		t.Fatalf("unexpected imported state: %#v", state)
	}
	invalid := frameworkresource.ImportStateResponse{State: tfsdk.State{Schema: schema}}
	(&byokKeyResource{}).ImportState(context.Background(), frameworkresource.ImportStateRequest{ID: "not-a-uuid"}, &invalid)
	if !invalid.Diagnostics.HasError() {
		t.Fatal("expected invalid import ID diagnostic")
	}
}

func TestBYOKKeyDeleteAcceptsConfirmedDeletion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodDelete:
			_ = json.NewEncoder(w).Encode(map[string]any{"deleted": true})
		case http.MethodGet:
			http.Error(w, "missing", http.StatusNotFound)
		default:
			t.Fatalf("unexpected method %s", req.Method)
		}
	}))
	defer server.Close()
	schema := byokTestSchema(t)
	state := tfsdk.State{Schema: schema}
	model := byokKeyResourceModel{ID: types.StringValue("id"), KeyWO: types.StringNull(), KeyWOVersion: types.Int64Value(0), ProviderSlug: types.StringValue("openai"), WorkspaceID: types.StringValue("ws"), Name: types.StringNull(), DeclaredZDR: types.BoolNull(), AllowedModels: types.SetNull(types.StringType), AllowedAPIKeyHashes: types.SetNull(types.StringType), AllowedUserIDs: types.SetNull(types.StringType), Disabled: types.BoolValue(false), IsFallback: types.BoolValue(false), IsRequired: types.BoolValue(false), IsBYOKOnly: types.BoolValue(false), Label: types.StringValue("label"), SortOrder: types.Int64Value(0), CreatedAt: types.StringValue("now")}
	if diagnostics := state.Set(context.Background(), &model); diagnostics.HasError() {
		t.Fatal(diagnostics)
	}
	response := frameworkresource.DeleteResponse{}
	(&byokKeyResource{client: client.New("token", server.URL, "", time.Second)}).Delete(context.Background(), frameworkresource.DeleteRequest{State: state}, &response)
	if response.Diagnostics.HasError() {
		t.Fatal(response.Diagnostics)
	}
}

func byokCompleteModel(key types.String, version int64) byokKeyResourceModel {
	return byokKeyResourceModel{ID: types.StringUnknown(), KeyWO: key, KeyWOVersion: types.Int64Value(version), ProviderSlug: types.StringValue("openai"), WorkspaceID: types.StringValue("11111111-2222-3333-4444-555555555555"), Name: types.StringNull(), DeclaredZDR: types.BoolNull(), AllowedModels: types.SetNull(types.StringType), AllowedAPIKeyHashes: types.SetNull(types.StringType), AllowedUserIDs: types.SetNull(types.StringType), Disabled: types.BoolValue(false), IsFallback: types.BoolValue(false), IsRequired: types.BoolValue(false), IsBYOKOnly: types.BoolValue(false), Label: types.StringUnknown(), SortOrder: types.Int64Unknown(), CreatedAt: types.StringUnknown()}
}
func byokTestPlan(t *testing.T, schema resourceschema.Schema, model byokKeyResourceModel) tfsdk.Plan {
	t.Helper()
	plan := tfsdk.Plan{Schema: schema}
	if d := plan.Set(context.Background(), &model); d.HasError() {
		t.Fatal(d)
	}
	return plan
}
func byokTestConfig(t *testing.T, schema resourceschema.Schema, model byokKeyResourceModel) tfsdk.Config {
	t.Helper()
	plan := byokTestPlan(t, schema, model)
	return tfsdk.Config{Schema: schema, Raw: plan.Raw}
}
func byokAPI() map[string]any {
	return map[string]any{"id": "11111111-2222-3333-4444-555555555555", "provider": "openai", "workspace_id": "11111111-2222-3333-4444-555555555555", "label": "masked", "name": nil, "declared_zdr": nil, "disabled": false, "is_fallback": false, "is_required": false, "is_byok_only": false, "allowed_models": nil, "allowed_api_key_hashes": nil, "allowed_user_ids": nil, "sort_order": 0, "created_at": "now"}
}
func TestBYOKKeyCreateUsesConfigWriteOnlyKeyAndExcludesState(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/byok" {
			t.Fatalf("request %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		_ = json.NewEncoder(w).Encode(map[string]any{"data": byokAPI()})
	}))
	defer server.Close()
	schema := byokTestSchema(t)
	configModel := byokCompleteModel(types.StringValue("super-secret"), 1)
	planModel := byokCompleteModel(types.StringNull(), 1)
	response := frameworkresource.CreateResponse{State: tfsdk.State{Schema: schema}}
	(&byokKeyResource{client: client.New("token", server.URL, "", time.Second)}).Create(context.Background(), frameworkresource.CreateRequest{Config: byokTestConfig(t, schema, configModel), Plan: byokTestPlan(t, schema, planModel)}, &response)
	if response.Diagnostics.HasError() {
		t.Fatal(response.Diagnostics)
	}
	if body["key"] != "super-secret" {
		t.Fatalf("key request = %#v", body["key"])
	}
	var state byokKeyResourceModel
	if d := response.State.Get(context.Background(), &state); d.HasError() {
		t.Fatal(d)
	}
	if !state.KeyWO.IsNull() {
		t.Fatal("raw key entered state")
	}
}
func TestBYOKKeyRead404RemovesState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "missing", http.StatusNotFound) }))
	defer server.Close()
	schema := byokTestSchema(t)
	model := byokCompleteModel(types.StringNull(), 0)
	model.ID = types.StringValue("11111111-2222-3333-4444-555555555555")
	model.Label = types.StringValue("masked")
	model.SortOrder = types.Int64Value(0)
	model.CreatedAt = types.StringValue("now")
	state := tfsdk.State{Schema: schema}
	if d := state.Set(context.Background(), &model); d.HasError() {
		t.Fatal(d)
	}
	response := frameworkresource.ReadResponse{State: state}
	(&byokKeyResource{client: client.New("token", server.URL, "", time.Second)}).Read(context.Background(), frameworkresource.ReadRequest{State: state}, &response)
	if response.Diagnostics.HasError() || !response.State.Raw.IsNull() {
		t.Fatalf("diagnostics=%s state=%#v", response.Diagnostics, response.State.Raw)
	}
}

func byokState(t *testing.T, schema resourceschema.Schema, model byokKeyResourceModel) tfsdk.State {
	t.Helper()
	state := tfsdk.State{Schema: schema}
	if d := state.Set(context.Background(), &model); d.HasError() {
		t.Fatal(d)
	}
	return state
}
func TestBYOKKeyUpdateSparsePatchAndRotation(t *testing.T) {
	var bodies []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("method %s", r.Method)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		bodies = append(bodies, body)
		_ = json.NewEncoder(w).Encode(map[string]any{"data": byokAPI()})
	}))
	defer server.Close()
	schema := byokTestSchema(t)
	stateModel := byokCompleteModel(types.StringNull(), 1)
	stateModel.ID = types.StringValue("11111111-2222-3333-4444-555555555555")
	stateModel.Name = types.StringValue("old name")
	stateModel.Label = types.StringValue("masked")
	stateModel.SortOrder = types.Int64Value(0)
	stateModel.CreatedAt = types.StringValue("now")
	state := byokState(t, schema, stateModel)
	// Clearing a nullable field is a sparse PATCH and must not contain a raw key.
	clearPlan := stateModel
	clearPlan.Name = types.StringNull()
	clearPlan.KeyWO = types.StringNull()
	clearConfig := clearPlan
	clearResponse := frameworkresource.UpdateResponse{State: tfsdk.State{Schema: schema}}
	resource := &byokKeyResource{client: client.New("token", server.URL, "", time.Second)}
	resource.Update(context.Background(), frameworkresource.UpdateRequest{State: state, Plan: byokTestPlan(t, schema, clearPlan), Config: byokTestConfig(t, schema, clearConfig)}, &clearResponse)
	if clearResponse.Diagnostics.HasError() {
		t.Fatal(clearResponse.Diagnostics)
	}
	if _, ok := bodies[0]["key"]; ok {
		t.Fatal("unrelated PATCH sent raw key")
	}
	if value, ok := bodies[0]["name"]; !ok || value != nil {
		t.Fatalf("clear patch=%#v", bodies[0])
	}
	// Incrementing the companion version is the only action that rotates the raw key.
	rotatePlan := clearPlan
	rotatePlan.KeyWOVersion = types.Int64Value(2)
	rotateConfig := rotatePlan
	rotateConfig.KeyWO = types.StringValue("rotated-secret")
	rotateResponse := frameworkresource.UpdateResponse{State: tfsdk.State{Schema: schema}}
	resource.Update(context.Background(), frameworkresource.UpdateRequest{State: state, Plan: byokTestPlan(t, schema, rotatePlan), Config: byokTestConfig(t, schema, rotateConfig)}, &rotateResponse)
	if rotateResponse.Diagnostics.HasError() {
		t.Fatal(rotateResponse.Diagnostics)
	}
	if got := bodies[1]["key"]; got != "rotated-secret" {
		t.Fatalf("rotation key=%#v body=%#v", got, bodies[1])
	}
}

func TestBYOKKeyDelete404IsIdempotent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) { http.Error(w, "missing", http.StatusNotFound) }))
	defer server.Close()
	schema := byokTestSchema(t)
	model := byokCompleteModel(types.StringNull(), 0)
	model.ID, model.Label, model.SortOrder, model.CreatedAt = types.StringValue("11111111-2222-3333-4444-555555555555"), types.StringValue("masked"), types.Int64Value(0), types.StringValue("now")
	state := byokState(t, schema, model)
	response := frameworkresource.DeleteResponse{}
	(&byokKeyResource{client: client.New("token", server.URL, "", time.Second)}).Delete(context.Background(), frameworkresource.DeleteRequest{State: state}, &response)
	if response.Diagnostics.HasError() {
		t.Fatal(response.Diagnostics)
	}
}

func TestBYOKKeyCreateMalformed2xxRecoversByRemoteIdentity(t *testing.T) {
	calls := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		if r.Method == http.MethodPost {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "11111111-2222-3333-4444-555555555555"}})
			return
		}
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": byokAPI()})
			return
		}
		t.Fatalf("unexpected request %s", r.Method)
	}))
	defer server.Close()
	schema := byokTestSchema(t)
	config := byokCompleteModel(types.StringValue("secret"), 1)
	plan := byokCompleteModel(types.StringNull(), 1)
	response := frameworkresource.CreateResponse{State: tfsdk.State{Schema: schema}}
	(&byokKeyResource{client: client.New("token", server.URL, "", time.Second)}).Create(context.Background(), frameworkresource.CreateRequest{Config: byokTestConfig(t, schema, config), Plan: byokTestPlan(t, schema, plan)}, &response)
	if response.Diagnostics.HasError() {
		t.Fatal(response.Diagnostics)
	}
	var state byokKeyResourceModel
	if d := response.State.Get(context.Background(), &state); d.HasError() {
		t.Fatal(d)
	}
	if state.ID.ValueString() != "11111111-2222-3333-4444-555555555555" || !state.KeyWO.IsNull() {
		t.Fatalf("unsafe recovery state %#v", state)
	}
	if !reflect.DeepEqual(calls, []string{"POST /byok", "GET /byok/11111111-2222-3333-4444-555555555555"}) {
		t.Fatalf("calls=%#v", calls)
	}
}
func TestBYOKKeyUpdateMismatched2xxRecoversKnownIdentity(t *testing.T) {
	known := "11111111-2222-3333-4444-555555555555"
	calls := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		if r.Method == http.MethodPatch {
			wrong := byokAPI()
			wrong["id"] = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
			_ = json.NewEncoder(w).Encode(map[string]any{"data": wrong})
			return
		}
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": byokAPI()})
			return
		}
		t.Fatalf("unexpected %s", r.Method)
	}))
	defer server.Close()
	schema := byokTestSchema(t)
	stateModel := byokCompleteModel(types.StringNull(), 1)
	stateModel.ID = types.StringValue(known)
	stateModel.Label = types.StringValue("masked")
	stateModel.SortOrder = types.Int64Value(0)
	stateModel.CreatedAt = types.StringValue("now")
	state := byokState(t, schema, stateModel)
	plan := stateModel
	plan.KeyWOVersion = types.Int64Value(2)
	config := plan
	config.KeyWO = types.StringValue("rotate")
	response := frameworkresource.UpdateResponse{State: tfsdk.State{Schema: schema}}
	(&byokKeyResource{client: client.New("token", server.URL, "", time.Second)}).Update(context.Background(), frameworkresource.UpdateRequest{State: state, Plan: byokTestPlan(t, schema, plan), Config: byokTestConfig(t, schema, config)}, &response)
	if response.Diagnostics.HasError() {
		t.Fatal(response.Diagnostics)
	}
	var recovered byokKeyResourceModel
	if d := response.State.Get(context.Background(), &recovered); d.HasError() {
		t.Fatal(d)
	}
	if recovered.ID.ValueString() != known || recovered.KeyWOVersion.ValueInt64() != 2 {
		t.Fatalf("identity/version changed: %#v", recovered)
	}
	if !reflect.DeepEqual(calls, []string{"PATCH /byok/" + known, "GET /byok/" + known}) {
		t.Fatalf("calls=%#v", calls)
	}
}

func TestBYOKKeyUpdatePatchFailureDoesNotRecover(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; http.Error(w, "bad", http.StatusBadRequest) }))
	defer server.Close()
	schema := byokTestSchema(t)
	m := byokCompleteModel(types.StringNull(), 1)
	m.ID = types.StringValue("11111111-2222-3333-4444-555555555555")
	m.Label = types.StringValue("masked")
	m.SortOrder = types.Int64Value(0)
	m.CreatedAt = types.StringValue("now")
	state := byokState(t, schema, m)
	plan := m
	plan.KeyWOVersion = types.Int64Value(2)
	config := plan
	config.KeyWO = types.StringValue("secret")
	response := frameworkresource.UpdateResponse{State: tfsdk.State{Schema: schema}}
	(&byokKeyResource{client: client.New("token", server.URL, "", time.Second)}).Update(context.Background(), frameworkresource.UpdateRequest{State: state, Plan: byokTestPlan(t, schema, plan), Config: byokTestConfig(t, schema, config)}, &response)
	if !response.Diagnostics.HasError() || calls != 1 {
		t.Fatalf("diagnostics=%s calls=%d", response.Diagnostics, calls)
	}
}
func TestBYOKKeyReadRejectsForeignIdentity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		foreign := byokAPI()
		foreign["id"] = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
		_ = json.NewEncoder(w).Encode(map[string]any{"data": foreign})
	}))
	defer server.Close()
	schema := byokTestSchema(t)
	m := byokCompleteModel(types.StringNull(), 0)
	m.ID = types.StringValue("11111111-2222-3333-4444-555555555555")
	m.Label = types.StringValue("masked")
	m.SortOrder = types.Int64Value(0)
	m.CreatedAt = types.StringValue("now")
	state := byokState(t, schema, m)
	response := frameworkresource.ReadResponse{State: state}
	(&byokKeyResource{client: client.New("token", server.URL, "", time.Second)}).Read(context.Background(), frameworkresource.ReadRequest{State: state}, &response)
	if !response.Diagnostics.HasError() {
		t.Fatal("expected foreign identity diagnostic")
	}
}
func TestBYOKKeyCreateRejectsInvalidResponseIdentity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bad := byokAPI()
		bad["id"] = "not-a-uuid"
		_ = json.NewEncoder(w).Encode(map[string]any{"data": bad})
	}))
	defer server.Close()
	schema := byokTestSchema(t)
	config := byokCompleteModel(types.StringValue("secret"), 1)
	plan := byokCompleteModel(types.StringNull(), 1)
	response := frameworkresource.CreateResponse{State: tfsdk.State{Schema: schema}}
	(&byokKeyResource{client: client.New("token", server.URL, "", time.Second)}).Create(context.Background(), frameworkresource.CreateRequest{Config: byokTestConfig(t, schema, config), Plan: byokTestPlan(t, schema, plan)}, &response)
	if !response.Diagnostics.HasError() {
		t.Fatal("expected invalid identity diagnostic")
	}
}
