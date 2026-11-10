package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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

func scimMappingFixture() client.SCIMGroupMapping {
	return client.SCIMGroupMapping{ID: "mapping", SCIMGroupID: "group", WorkspaceID: "workspace", Role: "member", OrganizationID: "org", CreatedAt: "created", UpdatedAt: "updated"}
}
func scimMappingTestSetup(t *testing.T) (*scimGroupMappingResource, resourceschema.Schema, tfsdk.State, tfsdk.Plan) {
	t.Helper()
	r := NewSCIMGroupMappingResource().(*scimGroupMappingResource)
	var schema resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &schema)
	state := tfsdk.State{Schema: schema.Schema}
	m := flattenSCIMMapping(scimMappingFixture(), types.BoolValue(true))
	if d := state.Set(context.Background(), m); d.HasError() {
		t.Fatal(d)
	}
	plan := tfsdk.Plan{Schema: schema.Schema, Raw: state.Raw}
	for _, name := range []string{"id", "organization_id", "created_at", "updated_at"} {
		if d := plan.SetAttribute(context.Background(), path.Root(name), types.StringUnknown()); d.HasError() {
			t.Fatal(d)
		}
	}
	return r, schema.Schema, state, plan
}
func TestSCIMMappingResourceCreateRefusesAdoption(t *testing.T) {
	r, schema, _, plan := scimMappingTestSetup(t)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		calls++
		if req.Method != "GET" {
			t.Errorf("unsafe mutation %s", req.Method)
		}
		json.NewEncoder(w).Encode(map[string]any{"data": []client.SCIMGroupMapping{scimMappingFixture()}, "total_count": 1})
	}))
	defer server.Close()
	r.client = client.New("k", server.URL, "", time.Second)
	resp := resource.CreateResponse{State: tfsdk.State{Schema: schema}}
	r.Create(context.Background(), resource.CreateRequest{Plan: plan}, &resp)
	if !resp.Diagnostics.HasError() || calls != 1 {
		t.Fatalf("calls=%d diagnostics=%s", calls, resp.Diagnostics)
	}
}
func TestSCIMMappingResourceDeletionOwnershipAndErrors(t *testing.T) {
	for _, test := range []struct {
		name        string
		changeGroup bool
		status      int
		wantErr     bool
	}{{"safe", false, 200, false}, {"changed-owner", true, 200, true}, {"missing", false, 404, false}, {"500-mentions-404", false, 500, true}} {
		t.Run(test.name, func(t *testing.T) {
			r, _, state, _ := scimMappingTestSetup(t)
			deleted := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method == "GET" {
					if test.status != 200 {
						w.WriteHeader(test.status)
						fmt.Fprint(w, `{"error":{"message":"upstream (404)"}}`)
						return
					}
					m := scimMappingFixture()
					if test.changeGroup {
						m.SCIMGroupID = "external"
					}
					json.NewEncoder(w).Encode(map[string]any{"data": m})
					return
				}
				deleted = true
				if req.Method != "DELETE" || req.URL.Query().Get("keep_members") != "true" {
					t.Errorf("unsafe removal %s %s", req.Method, req.URL)
				}
				fmt.Fprint(w, `{"deleted":true}`)
			}))
			defer server.Close()
			r.client = client.New("k", server.URL, "", time.Second)
			resp := resource.DeleteResponse{State: state}
			r.Delete(context.Background(), resource.DeleteRequest{State: state}, &resp)
			if resp.Diagnostics.HasError() != test.wantErr {
				t.Fatalf("diagnostics=%s", resp.Diagnostics)
			}
			if deleted != (test.name == "safe") {
				t.Fatalf("deleted=%v", deleted)
			}
		})
	}
}
func TestSCIMMappingResourceImportDefaultsToRetainMembers(t *testing.T) {
	r, schema, _, _ := scimMappingTestSetup(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": scimMappingFixture()})
	}))
	defer server.Close()
	r.client = client.New("k", server.URL, "", time.Second)
	resp := resource.ImportStateResponse{State: tfsdk.State{Schema: schema}}
	r.ImportState(context.Background(), resource.ImportStateRequest{ID: "mapping"}, &resp)
	var state scimGroupMappingModel
	resp.Diagnostics.Append(resp.State.Get(context.Background(), &state)...)
	if resp.Diagnostics.HasError() || !state.KeepMembers.ValueBool() {
		t.Fatalf("keep=%s diagnostics=%s", state.KeepMembers, resp.Diagnostics)
	}
}
func TestSCIMMappingResourceReadDoesNotRedirectIdentity(t *testing.T) {
	r, _, state, _ := scimMappingTestSetup(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		m := scimMappingFixture()
		m.WorkspaceID = "other"
		json.NewEncoder(w).Encode(map[string]any{"data": m})
	}))
	defer server.Close()
	r.client = client.New("k", server.URL, "", time.Second)
	resp := resource.ReadResponse{State: state}
	r.Read(context.Background(), resource.ReadRequest{State: state}, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("accepted identity redirection")
	}
	var saved scimGroupMappingModel
	resp.State.Get(context.Background(), &saved)
	if saved.WorkspaceID.ValueString() != "workspace" {
		t.Fatalf("state changed %s", saved.WorkspaceID)
	}
}
func TestSCIMMappingResourceRetainPolicyOnlyUpdateDoesNotPatch(t *testing.T) {
	r, _, state, plan := scimMappingTestSetup(t)
	plan.SetAttribute(context.Background(), path.Root("keep_members"), types.BoolValue(false))
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		calls++
		if req.Method != "GET" {
			t.Errorf("unexpected policy-only mutation %s", req.Method)
		}
		json.NewEncoder(w).Encode(map[string]any{"data": scimMappingFixture()})
	}))
	defer server.Close()
	r.client = client.New("k", server.URL, "", time.Second)
	resp := resource.UpdateResponse{State: state}
	r.Update(context.Background(), resource.UpdateRequest{State: state, Plan: plan}, &resp)
	if resp.Diagnostics.HasError() || calls != 1 {
		t.Fatalf("calls=%d diagnostics=%s", calls, resp.Diagnostics)
	}
	var updated scimGroupMappingModel
	resp.State.Get(context.Background(), &updated)
	if updated.KeepMembers.ValueBool() {
		t.Fatal("local deletion policy not updated")
	}
}
func TestSCIMMappingResourceReadNon404Propagates(t *testing.T) {
	r, _, state, _ := scimMappingTestSetup(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(500)
		fmt.Fprint(w, `{"error":{"message":"contains (404) text"}}`)
	}))
	defer server.Close()
	r.client = client.New("k", server.URL, "", time.Second)
	resp := resource.ReadResponse{State: state}
	r.Read(context.Background(), resource.ReadRequest{State: state}, &resp)
	if !resp.Diagnostics.HasError() || resp.State.Raw.IsNull() {
		t.Fatalf("diagnostics=%s state=%v", resp.Diagnostics, resp.State.Raw)
	}
	if !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), "(500)") {
		t.Fatal(resp.Diagnostics)
	}
}

func TestSCIMMappingResourceDriftRemoval(t *testing.T) {
	r, _, state, _ := scimMappingTestSetup(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(404)
		fmt.Fprint(w, `{"error":{"message":"mapping not found"}}`)
	}))
	defer server.Close()
	r.client = client.New("k", server.URL, "", time.Second)
	resp := resource.ReadResponse{State: state}
	r.Read(context.Background(), resource.ReadRequest{State: state}, &resp)
	if resp.Diagnostics.HasError() || !resp.State.Raw.IsNull() {
		t.Fatalf("diagnostics=%s state=%v", resp.Diagnostics, resp.State.Raw)
	}
}
func TestSCIMMappingResourceRejectsRedirectedUpdateResponse(t *testing.T) {
	r, _, state, plan := scimMappingTestSetup(t)
	plan.SetAttribute(context.Background(), path.Root("role"), types.StringValue("admin"))
	patches := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		m := scimMappingFixture()
		if req.Method == "PATCH" {
			patches++
			m.Role = "admin"
			m.WorkspaceID = "other"
		}
		json.NewEncoder(w).Encode(map[string]any{"data": m})
	}))
	defer server.Close()
	r.client = client.New("k", server.URL, "", time.Second)
	resp := resource.UpdateResponse{State: state}
	r.Update(context.Background(), resource.UpdateRequest{State: state, Plan: plan}, &resp)
	var saved scimGroupMappingModel
	resp.State.Get(context.Background(), &saved)
	if !resp.Diagnostics.HasError() || patches != 1 || saved.WorkspaceID.ValueString() != "workspace" {
		t.Fatalf("patches=%d workspace=%s diagnostics=%s", patches, saved.WorkspaceID, resp.Diagnostics)
	}
}

func TestSCIMMappingConfigRejectsInvalidKnownValues(t *testing.T) {
	for _, test := range []struct{ attribute, value string }{{"role", "viewer"}, {"scim_group_id", " group"}, {"workspace_id", "workspace/other"}} {
		t.Run(test.attribute, func(t *testing.T) {
			r, schema, state, _ := scimMappingTestSetup(t)
			state.SetAttribute(context.Background(), path.Root(test.attribute), types.StringValue(test.value))
			resp := resource.ValidateConfigResponse{}
			r.ValidateConfig(context.Background(), resource.ValidateConfigRequest{Config: tfsdk.Config{Schema: schema, Raw: state.Raw}}, &resp)
			if !resp.Diagnostics.HasError() {
				t.Fatal("accepted invalid known configuration")
			}
		})
	}
}
