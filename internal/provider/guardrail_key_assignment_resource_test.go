package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/cloudopsworks/terraform-provider-openrouter/internal/client"
)

func TestGuardrailKeyAssignmentResourceSchema(t *testing.T) {
	t.Parallel()

	var resp frameworkresource.SchemaResponse
	NewGuardrailKeyAssignmentResource().Schema(context.Background(), frameworkresource.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected schema diagnostics: %s", resp.Diagnostics)
	}

	for _, name := range []string{"guardrail_id", "key_hash"} {
		attr, ok := resp.Schema.Attributes[name].(resourceschema.StringAttribute)
		if !ok || !attr.Required {
			t.Fatalf("%s must be a required string attribute, got %#v", name, resp.Schema.Attributes[name])
		}
	}
	guardrailID := resp.Schema.Attributes["guardrail_id"].(resourceschema.StringAttribute)
	if len(guardrailID.PlanModifiers) != 0 {
		t.Fatal("guardrail_id must update in place to avoid an unguarded replacement window")
	}
	keyHash := resp.Schema.Attributes["key_hash"].(resourceschema.StringAttribute)
	_, requiresReplace := applyStringPlanModifiersForTest(keyHash.PlanModifiers, types.StringValue("new-key"), types.StringValue("new-key"), types.StringValue("old-key"))
	if !requiresReplace {
		t.Fatal("key_hash must require replacement when changed")
	}
	id := resp.Schema.Attributes["id"].(resourceschema.StringAttribute)
	if !id.Computed {
		t.Fatal("id must be computed")
	}
}

func TestGuardrailKeyAssignmentCompositeIdentity(t *testing.T) {
	t.Parallel()

	const guardrailID = "guardrail-1"
	const keyHash = "key-1"
	id := guardrailKeyAssignmentID(guardrailID, keyHash)
	if id != "guardrail-1/key-1" {
		t.Fatalf("id = %q", id)
	}
	gotGuardrailID, gotKeyHash, err := parseGuardrailKeyAssignmentImportID(id)
	if err != nil {
		t.Fatalf("parse import ID: %v", err)
	}
	if gotGuardrailID != guardrailID || gotKeyHash != keyHash {
		t.Fatalf("parse import ID = (%q, %q), want (%q, %q)", gotGuardrailID, gotKeyHash, guardrailID, keyHash)
	}
	if _, _, err := parseGuardrailKeyAssignmentImportID("not-composite"); err == nil {
		t.Fatal("expected malformed composite identity error")
	}
}

func TestContainsKeyHash(t *testing.T) {
	t.Parallel()
	if !containsKeyHash([]string{"key-1", "key-2"}, "key-2") {
		t.Fatal("expected owned key to be found")
	}
	if containsKeyHash([]string{"key-1"}, "key-2") {
		t.Fatal("unowned key must not be considered assigned")
	}
}

func TestGuardrailKeyAssignmentCreateDoesNotRemoveOnFailedVerification(t *testing.T) {
	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		methods = append(methods, req.Method+" "+req.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case http.MethodPost:
			if req.URL.Path == "/guardrails/guardrail-1/assignments/keys/remove" {
				t.Fatal("failed verification must not remove an assignment")
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{}})
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}, "total_count": 0})
		}
	}))
	defer server.Close()

	resource := &guardrailKeyAssignmentResource{client: client.New("token", server.URL, "test", time.Second)}
	var schemaResp frameworkresource.SchemaResponse
	resource.Schema(context.Background(), frameworkresource.SchemaRequest{}, &schemaResp)
	plan := tfsdk.Plan{Schema: schemaResp.Schema}
	model := guardrailKeyAssignmentResourceModel{GuardrailID: types.StringValue("guardrail-1"), KeyHash: types.StringValue("key-1"), ID: types.StringUnknown()}
	if diags := plan.Set(context.Background(), &model); diags.HasError() {
		t.Fatalf("set plan: %s", diags)
	}
	state := tfsdk.State{Schema: schemaResp.Schema}
	resp := frameworkresource.CreateResponse{State: state}
	resource.Create(context.Background(), frameworkresource.CreateRequest{Plan: plan}, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("failed verification must return diagnostics")
	}
	if got, want := methods, []string{"GET /guardrails/assignments/keys", "POST /guardrails/guardrail-1/assignments/keys", "GET /guardrails/guardrail-1/assignments/keys"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("requests = %#v, want %#v", got, want)
	}
	var persisted guardrailKeyAssignmentResourceModel
	if diags := resp.State.Get(context.Background(), &persisted); diags.HasError() {
		t.Fatalf("get persisted state: %s", diags)
	}
	if persisted.ID.ValueString() != "guardrail-1/key-1" {
		t.Fatalf("state ID = %q, want composite identity", persisted.ID.ValueString())
	}
}

func TestGuardrailKeyAssignmentCreatePreflightCollisionDoesNotAssign(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet || req.URL.Path != "/guardrails/assignments/keys" {
			t.Fatalf("collision preflight must not assign; got %s %s", req.Method, req.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]string{{"guardrail_id": "other-guardrail", "key_hash": "key-1"}}, "total_count": 1})
	}))
	defer server.Close()

	r := &guardrailKeyAssignmentResource{client: client.New("token", server.URL, "test", time.Second)}
	if err := r.preflightCreate(context.Background(), "guardrail-1", "key-1"); err == nil {
		t.Fatal("expected collision error")
	}
}

func TestGuardrailKeyAssignmentInputValidation(t *testing.T) {
	t.Parallel()
	for _, input := range [][2]string{{" ", "key"}, {"guardrail", "\t"}} {
		if err := validateGuardrailKeyAssignmentInputs(input[0], input[1]); err == nil {
			t.Fatalf("expected validation error for %#v", input)
		}
	}
}

func assignmentTestSchema(t *testing.T) resourceschema.Schema {
	t.Helper()
	var resp frameworkresource.SchemaResponse
	NewGuardrailKeyAssignmentResource().Schema(context.Background(), frameworkresource.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	return resp.Schema
}
func assignmentTestState(t *testing.T, schema resourceschema.Schema, guardrail, key string) tfsdk.State {
	t.Helper()
	state := tfsdk.State{Schema: schema}
	if diags := state.Set(context.Background(), &guardrailKeyAssignmentResourceModel{ID: types.StringValue(guardrailKeyAssignmentID(guardrail, key)), GuardrailID: types.StringValue(guardrail), KeyHash: types.StringValue(key)}); diags.HasError() {
		t.Fatal(diags)
	}
	return state
}
func assignmentTestPlan(t *testing.T, schema resourceschema.Schema, guardrail, key string) tfsdk.Plan {
	t.Helper()
	plan := tfsdk.Plan{Schema: schema}
	if diags := plan.Set(context.Background(), &guardrailKeyAssignmentResourceModel{ID: types.StringUnknown(), GuardrailID: types.StringValue(guardrail), KeyHash: types.StringValue(key)}); diags.HasError() {
		t.Fatal(diags)
	}
	return plan
}

func TestGuardrailKeyAssignmentUpdateMovesInPlaceAndPersistsTargetOnVerifyFailure(t *testing.T) {
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		calls = append(calls, req.Method+" "+req.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch req.URL.Path {
		case "/guardrails/assignments/keys":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]string{{"guardrail_id": "old", "key_hash": "key"}}, "total_count": 1})
		case "/guardrails/new/assignments/keys":
			if req.Method == http.MethodPost {
				_ = json.NewEncoder(w).Encode(map[string]any{})
			} else {
				_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}, "total_count": 0})
			}
		default:
			t.Fatalf("unexpected %s", req.URL.Path)
		}
	}))
	defer server.Close()
	schema := assignmentTestSchema(t)
	r := &guardrailKeyAssignmentResource{client: client.New("t", server.URL, "", time.Second)}
	state := assignmentTestState(t, schema, "old", "key")
	resp := frameworkresource.UpdateResponse{State: tfsdk.State{Schema: schema}}
	r.Update(context.Background(), frameworkresource.UpdateRequest{State: state, Plan: assignmentTestPlan(t, schema, "new", "key")}, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected verify error")
	}
	var got guardrailKeyAssignmentResourceModel
	if d := resp.State.Get(context.Background(), &got); d.HasError() {
		t.Fatal(d)
	}
	if got.ID.ValueString() != "new/key" {
		t.Fatalf("state=%q want new/key", got.ID.ValueString())
	}
	want := []string{"GET /guardrails/assignments/keys", "POST /guardrails/new/assignments/keys", "GET /guardrails/new/assignments/keys"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls=%#v", calls)
	}
}

func TestGuardrailKeyAssignmentDeleteAfterExternalMoveDoesNotRemove(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("must not remove externally moved assignment: %s", req.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}, "total_count": 0})
	}))
	defer server.Close()
	schema := assignmentTestSchema(t)
	r := &guardrailKeyAssignmentResource{client: client.New("t", server.URL, "", time.Second)}
	state := assignmentTestState(t, schema, "old", "key")
	resp := frameworkresource.DeleteResponse{State: state}
	r.Delete(context.Background(), frameworkresource.DeleteRequest{State: state}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
}

func TestGuardrailKeyAssignmentReadMissingRemovesState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}, "total_count": 0})
	}))
	defer server.Close()
	schema := assignmentTestSchema(t)
	r := &guardrailKeyAssignmentResource{client: client.New("t", server.URL, "", time.Second)}
	state := assignmentTestState(t, schema, "old", "key")
	resp := frameworkresource.ReadResponse{State: state}
	r.Read(context.Background(), frameworkresource.ReadRequest{State: state}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if !resp.State.Raw.IsNull() {
		t.Fatal("missing assignment must remove state")
	}
}

func TestGuardrailKeyAssignmentRead404RemovesState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) { http.Error(w, "missing", http.StatusNotFound) }))
	defer server.Close()
	schema := assignmentTestSchema(t)
	r := &guardrailKeyAssignmentResource{client: client.New("t", server.URL, "", time.Second)}
	state := assignmentTestState(t, schema, "old", "key")
	resp := frameworkresource.ReadResponse{State: state}
	r.Read(context.Background(), frameworkresource.ReadRequest{State: state}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if !resp.State.Raw.IsNull() {
		t.Fatal("404 must remove state")
	}
}

func TestGuardrailKeyAssignmentDeleteVerifiesRemoval(t *testing.T) {
	var calls []string
	getCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		calls = append(calls, req.Method+" "+req.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		if req.Method == http.MethodPost {
			_ = json.NewEncoder(w).Encode(map[string]any{})
			return
		}
		getCount++
		if getCount == 1 {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]string{{"key_hash": "key"}}, "total_count": 1})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}, "total_count": 0})
	}))
	defer server.Close()
	schema := assignmentTestSchema(t)
	r := &guardrailKeyAssignmentResource{client: client.New("t", server.URL, "", time.Second)}
	state := assignmentTestState(t, schema, "old", "key")
	resp := frameworkresource.DeleteResponse{State: state}
	r.Delete(context.Background(), frameworkresource.DeleteRequest{State: state}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	want := []string{"GET /guardrails/old/assignments/keys", "POST /guardrails/old/assignments/keys/remove", "GET /guardrails/old/assignments/keys"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls=%#v", calls)
	}
}

func TestGuardrailKeyAssignmentDeleteReportsUnverifiedRemoval(t *testing.T) {
	getCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if req.Method == http.MethodPost {
			_ = json.NewEncoder(w).Encode(map[string]any{})
			return
		}
		getCount++
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]string{{"key_hash": "key"}}, "total_count": 1})
	}))
	defer server.Close()
	schema := assignmentTestSchema(t)
	r := &guardrailKeyAssignmentResource{client: client.New("t", server.URL, "", time.Second)}
	state := assignmentTestState(t, schema, "old", "key")
	resp := frameworkresource.DeleteResponse{State: state}
	r.Delete(context.Background(), frameworkresource.DeleteRequest{State: state}, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected failed postcondition diagnostic")
	}
}
