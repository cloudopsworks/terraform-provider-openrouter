package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/cloudopsworks/terraform-provider-openrouter/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func policyAssignmentTestSetup(t *testing.T, r *policyAssignmentResource) (resourceschema.Schema, policyAssignmentModel, tfsdk.State, tfsdk.Plan) {
	t.Helper()
	var response resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &response)
	model := policyAssignmentModel{ID: types.StringValue("ws/budget/subject/policy"), WorkspaceID: types.StringValue("ws"), PolicyType: types.StringValue("budget"), SubjectID: types.StringValue("subject"), PolicyID: types.StringValue("policy")}
	state := tfsdk.State{Schema: response.Schema}
	if d := r.writeModel(context.Background(), &state, model); d.HasError() {
		t.Fatal(d)
	}
	plan := tfsdk.Plan{Schema: response.Schema, Raw: state.Raw}
	if d := plan.SetAttribute(context.Background(), path.Root("id"), types.StringUnknown()); d.HasError() {
		t.Fatal(d)
	}
	return response.Schema, model, state, plan
}
func policyAssignmentTestResource(member bool) *policyAssignmentResource {
	if member {
		return NewMemberPolicyAssignmentResource().(*policyAssignmentResource)
	}
	return NewAPIKeyPolicyAssignmentResource().(*policyAssignmentResource)
}
func assignmentState(policyIDs, defaults []string, optedOut bool) client.PolicyAssignmentState {
	return client.PolicyAssignmentState{PolicyIDs: policyIDs, WorkspaceDefaultPolicyIDs: defaults, OptedOut: optedOut}
}
func TestPolicyAssignmentResourcesLifecycle(t *testing.T) {
	for _, member := range []bool{false, true} {
		t.Run(map[bool]string{false: "api-key", true: "member"}[member], func(t *testing.T) {
			ctx := context.Background()
			r := policyAssignmentTestResource(member)
			schema, _, _, plan := policyAssignmentTestSetup(t, r)
			current := assignmentState([]string{}, []string{"inherited"}, false)
			changes := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if strings.HasPrefix(req.URL.Path, "/guardrails/policies/budget/") {
					_, _ = w.Write([]byte(`{"data":{"id":"policy","policy_type":"budget"}}`))
					return
				}
				if req.URL.Path != "/guardrails/policies/assignments/"+r.subjectType+"/subject" {
					t.Errorf("path=%s", req.URL.Path)
				}
				if req.Method == http.MethodGet {
					_ = json.NewEncoder(w).Encode(map[string]any{"data": current})
					return
				}
				var body client.ChangePolicyAssignmentRequest
				if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if !reflect.DeepEqual(body.ExpectedAssignment, current) {
					t.Errorf("CAS token=%#v want %#v", body.ExpectedAssignment, current)
				}
				changes++
				if body.Assigned {
					current.PolicyIDs = []string{"policy"}
				} else {
					current.PolicyIDs = []string{}
				}
				_, _ = w.Write([]byte(`{"data":true}`))
			}))
			defer server.Close()
			r.client = client.New("token", server.URL, "", time.Second)
			created := resource.CreateResponse{State: tfsdk.State{Schema: schema}}
			r.Create(ctx, resource.CreateRequest{Plan: plan}, &created)
			if created.Diagnostics.HasError() {
				t.Fatal(created.Diagnostics)
			}
			model, d := r.readModel(ctx, created.State)
			if d.HasError() || model.ID.ValueString() != "ws/budget/subject/policy" {
				t.Fatalf("model=%#v diagnostics=%s", model, d)
			}
			read := resource.ReadResponse{State: created.State}
			r.Read(ctx, resource.ReadRequest{State: created.State}, &read)
			if read.Diagnostics.HasError() {
				t.Fatal(read.Diagnostics)
			}
			imported := resource.ImportStateResponse{State: tfsdk.State{Schema: schema}}
			r.ImportState(ctx, resource.ImportStateRequest{ID: model.ID.ValueString()}, &imported)
			if imported.Diagnostics.HasError() {
				t.Fatal(imported.Diagnostics)
			}
			deleted := resource.DeleteResponse{State: created.State}
			r.Delete(ctx, resource.DeleteRequest{State: created.State}, &deleted)
			if deleted.Diagnostics.HasError() {
				t.Fatal(deleted.Diagnostics)
			}
			if len(current.PolicyIDs) != 0 || current.OptedOut || changes != 2 {
				t.Fatalf("state=%#v changes=%d", current, changes)
			}
			drifted := resource.ReadResponse{State: created.State}
			r.Read(ctx, resource.ReadRequest{State: created.State}, &drifted)
			if drifted.Diagnostics.HasError() || !drifted.State.Raw.IsNull() {
				t.Fatalf("read state=%v diagnostics=%s", drifted.State.Raw, drifted.Diagnostics)
			}
		})
	}
}
func TestPolicyAssignmentCreateRefusesExistingState(t *testing.T) {
	for index, current := range []client.PolicyAssignmentState{
		assignmentState([]string{"policy"}, []string{}, false),
		assignmentState([]string{"other"}, []string{}, false),
		assignmentState([]string{}, []string{"policy"}, false),
		assignmentState([]string{}, []string{}, true),
	} {
		t.Run(fmt.Sprintf("case-%d", index), func(t *testing.T) {
			r := policyAssignmentTestResource(false)
			schema, _, _, plan := policyAssignmentTestSetup(t, r)
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				requests++
				if req.Method != http.MethodGet {
					t.Errorf("unsafe mutation %s", req.Method)
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"data": current})
			}))
			defer server.Close()
			r.client = client.New("t", server.URL, "", time.Second)
			resp := resource.CreateResponse{State: tfsdk.State{Schema: schema}}
			r.Create(context.Background(), resource.CreateRequest{Plan: plan}, &resp)
			if !resp.Diagnostics.HasError() || requests != 1 {
				t.Fatalf("diagnostics=%s requests=%d", resp.Diagnostics, requests)
			}
		})
	}
}
func TestPolicyAssignmentDeleteSafety(t *testing.T) {
	for _, test := range []struct {
		name      string
		current   client.PolicyAssignmentState
		wantError bool
	}{
		{"externally moved", assignmentState([]string{"other"}, []string{}, false), false},
		{"inherited target", assignmentState([]string{"policy"}, []string{"policy"}, false), true},
		{"opted out", assignmentState([]string{"policy"}, []string{}, true), true},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := policyAssignmentTestResource(true)
			_, _, state, _ := policyAssignmentTestSetup(t, r)
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				requests++
				if req.Method != http.MethodGet {
					t.Errorf("unsafe mutation %s", req.Method)
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"data": test.current})
			}))
			defer server.Close()
			r.client = client.New("t", server.URL, "", time.Second)
			resp := resource.DeleteResponse{State: state}
			r.Delete(context.Background(), resource.DeleteRequest{State: state}, &resp)
			if resp.Diagnostics.HasError() != test.wantError || requests != 1 {
				t.Fatalf("diagnostics=%s requests=%d", resp.Diagnostics, requests)
			}
		})
	}
}
func TestPolicyAssignmentConflictPreservesState(t *testing.T) {
	for _, create := range []bool{false, true} {
		t.Run(map[bool]string{false: "delete", true: "create"}[create], func(t *testing.T) {
			r := policyAssignmentTestResource(false)
			schema, _, state, plan := policyAssignmentTestSetup(t, r)
			puts := 0
			current := assignmentState([]string{"policy"}, []string{}, false)
			if create {
				current.PolicyIDs = []string{}
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method == http.MethodPut {
					puts++
					http.Error(w, `{"error":{"message":"CAS conflict"}}`, http.StatusConflict)
					return
				}
				if strings.Contains(req.URL.Path, "/assignments/") {
					_ = json.NewEncoder(w).Encode(map[string]any{"data": current})
					return
				}
				_, _ = w.Write([]byte(`{"data":{"id":"policy","policy_type":"budget"}}`))
			}))
			defer server.Close()
			r.client = client.New("t", server.URL, "", time.Second)
			if create {
				resp := resource.CreateResponse{State: tfsdk.State{Schema: schema}}
				r.Create(context.Background(), resource.CreateRequest{Plan: plan}, &resp)
				if !resp.Diagnostics.HasError() || !resp.State.Raw.IsNull() {
					t.Fatalf("diagnostics=%s state=%v", resp.Diagnostics, resp.State.Raw)
				}
			} else {
				resp := resource.DeleteResponse{State: state}
				r.Delete(context.Background(), resource.DeleteRequest{State: state}, &resp)
				if !resp.Diagnostics.HasError() || !resp.State.Raw.Equal(state.Raw) {
					t.Fatalf("diagnostics=%s state=%v", resp.Diagnostics, resp.State.Raw)
				}
			}
			if puts != 1 {
				t.Fatalf("puts=%d", puts)
			}
		})
	}
}
func TestPolicyAssignmentMalformedStateAndErrorsDoNotDelete(t *testing.T) {
	for _, test := range []struct {
		name    string
		status  int
		payload string
		removed bool
	}{
		{"404", 404, `{}`, true}, {"403", 403, `{}`, false}, {"500", 500, `{}`, false}, {"500 mentioning 404", 500, `{"error":{"message":"previous (404)"}}`, false}, {"missing data", 200, `{}`, false}, {"incomplete CAS", 200, `{"data":{"policy_ids":[]}}`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := policyAssignmentTestResource(false)
			_, _, state, _ := policyAssignmentTestSetup(t, r)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(test.payload))
			}))
			defer server.Close()
			r.client = client.New("t", server.URL, "", time.Second)
			resp := resource.ReadResponse{State: state}
			r.Read(context.Background(), resource.ReadRequest{State: state}, &resp)
			if test.removed {
				if resp.Diagnostics.HasError() || !resp.State.Raw.IsNull() {
					t.Fatalf("diagnostics=%s", resp.Diagnostics)
				}
			} else if !resp.Diagnostics.HasError() || !resp.State.Raw.Equal(state.Raw) {
				t.Fatalf("diagnostics=%s", resp.Diagnostics)
			}
		})
	}
}
func TestPolicyAssignmentStrictImportAndReplacement(t *testing.T) {
	for _, r := range []*policyAssignmentResource{policyAssignmentTestResource(false), policyAssignmentTestResource(true)} {
		schema, model, _, _ := policyAssignmentTestSetup(t, r)
		for _, name := range []string{"workspace_id", "policy_type", "policy_id", r.subjectAttribute} {
			attr := schema.Attributes[name].(resourceschema.StringAttribute)
			_, replace := applyStringPlanModifiersForTest(attr.PlanModifiers, types.StringValue("new"), types.StringValue("new"), types.StringValue("old"))
			if !attr.Required || !replace {
				t.Fatalf("%s not immutable required", name)
			}
		}
		for _, id := range []string{"bad", "ws/budget/user/policy/extra", "/budget/user/policy", "ws/rate_limit/user/policy", "ws/budget/ user/policy"} {
			resp := resource.ImportStateResponse{State: tfsdk.State{Schema: schema}}
			r.ImportState(context.Background(), resource.ImportStateRequest{ID: id}, &resp)
			if !resp.Diagnostics.HasError() {
				t.Errorf("expected invalid import error for %q", id)
			}
		}
		if err := model.validate(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPolicyAssignmentFailedPostconditionsRetainIdentity(t *testing.T) {
	for _, create := range []bool{false, true} {
		t.Run(map[bool]string{false: "delete", true: "create"}[create], func(t *testing.T) {
			r := policyAssignmentTestResource(false)
			schema, _, state, plan := policyAssignmentTestSetup(t, r)
			current := assignmentState([]string{"policy"}, []string{}, false)
			if create {
				current.PolicyIDs = []string{}
			}
			puts := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method == http.MethodPut {
					puts++
					_, _ = w.Write([]byte(`{"data":true}`))
					return
				}
				if strings.Contains(req.URL.Path, "/assignments/") {
					_ = json.NewEncoder(w).Encode(map[string]any{"data": current})
					return
				}
				_, _ = w.Write([]byte(`{"data":{"id":"policy","policy_type":"budget"}}`))
			}))
			defer server.Close()
			r.client = client.New("t", server.URL, "", time.Second)
			if create {
				resp := resource.CreateResponse{State: tfsdk.State{Schema: schema}}
				r.Create(context.Background(), resource.CreateRequest{Plan: plan}, &resp)
				if !resp.Diagnostics.HasError() || resp.State.Raw.IsNull() {
					t.Fatalf("diagnostics=%s state=%v", resp.Diagnostics, resp.State.Raw)
				}
			} else {
				resp := resource.DeleteResponse{State: state}
				r.Delete(context.Background(), resource.DeleteRequest{State: state}, &resp)
				if !resp.Diagnostics.HasError() || !resp.State.Raw.Equal(state.Raw) {
					t.Fatalf("diagnostics=%s state=%v", resp.Diagnostics, resp.State.Raw)
				}
			}
			if puts != 1 {
				t.Fatalf("puts=%d", puts)
			}
		})
	}
}

func TestPolicyAssignmentUnsafePolicyNeverMutates(t *testing.T) {
	for _, operation := range []string{"create", "import", "delete"} {
		t.Run(operation, func(t *testing.T) {
			r := policyAssignmentTestResource(true)
			schema, _, state, plan := policyAssignmentTestSetup(t, r)
			current := assignmentState([]string{"policy"}, []string{}, false)
			if operation == "create" {
				current.PolicyIDs = []string{}
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method != http.MethodGet {
					t.Errorf("unsafe mutation %s", req.Method)
					w.WriteHeader(500)
					return
				}
				if strings.Contains(req.URL.Path, "/assignments/") {
					_ = json.NewEncoder(w).Encode(map[string]any{"data": current})
					return
				}
				_, _ = w.Write([]byte(`{"data":{"id":"policy","policy_type":"budget","reference":"legacy_mapping"}}`))
			}))
			defer server.Close()
			r.client = client.New("t", server.URL, "", time.Second)
			switch operation {
			case "create":
				resp := resource.CreateResponse{State: tfsdk.State{Schema: schema}}
				r.Create(context.Background(), resource.CreateRequest{Plan: plan}, &resp)
				if !resp.Diagnostics.HasError() {
					t.Fatal("unsafe policy allowed")
				}
			case "import":
				resp := resource.ImportStateResponse{State: tfsdk.State{Schema: schema}}
				r.ImportState(context.Background(), resource.ImportStateRequest{ID: "ws/budget/subject/policy"}, &resp)
				if !resp.Diagnostics.HasError() {
					t.Fatal("unsafe policy allowed")
				}
			case "delete":
				resp := resource.DeleteResponse{State: state}
				r.Delete(context.Background(), resource.DeleteRequest{State: state}, &resp)
				if !resp.Diagnostics.HasError() {
					t.Fatal("unsafe policy allowed")
				}
			}
		})
	}
}

func TestPolicyAssignmentValidateConfig(t *testing.T) {
	for _, r := range []*policyAssignmentResource{policyAssignmentTestResource(false), policyAssignmentTestResource(true)} {
		schema, _, state, _ := policyAssignmentTestSetup(t, r)
		config := tfsdk.Config{Schema: schema, Raw: state.Raw}
		var valid resource.ValidateConfigResponse
		r.ValidateConfig(context.Background(), resource.ValidateConfigRequest{Config: config}, &valid)
		if valid.Diagnostics.HasError() {
			t.Fatal(valid.Diagnostics)
		}
		invalid := state
		if d := invalid.SetAttribute(context.Background(), path.Root("policy_type"), types.StringValue("unknown")); d.HasError() {
			t.Fatal(d)
		}
		var bad resource.ValidateConfigResponse
		r.ValidateConfig(context.Background(), resource.ValidateConfigRequest{Config: tfsdk.Config{Schema: schema, Raw: invalid.Raw}}, &bad)
		if !bad.Diagnostics.HasError() {
			t.Fatal("unsupported policy type allowed")
		}
		if d := state.SetAttribute(context.Background(), path.Root(r.subjectAttribute), types.StringUnknown()); d.HasError() {
			t.Fatal(d)
		}
		var unknown resource.ValidateConfigResponse
		r.ValidateConfig(context.Background(), resource.ValidateConfigRequest{Config: tfsdk.Config{Schema: schema, Raw: state.Raw}}, &unknown)
		if unknown.Diagnostics.HasError() {
			t.Fatal(unknown.Diagnostics)
		}
	}
}
