package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"

	"github.com/cloudopsworks/terraform-provider-openrouter/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	frameworkprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

type identityProtocolProvider struct{ openRouterProvider }

func (*identityProtocolProvider) Resources(context.Context) []func() frameworkresource.Resource {
	return []func() frameworkresource.Resource{NewSCIMGroupMappingResource}
}
func (*identityProtocolProvider) DataSources(context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{NewSCIMGroupsDataSource, NewWorkspaceMembersDataSource}
}

var _ frameworkprovider.Provider = &identityProtocolProvider{}

func identityProtocolFactories() map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){"openrouter": providerserver.NewProtocol6WithError(&identityProtocolProvider{openRouterProvider: openRouterProvider{version: "test"}})}
}
func identityTerraform(t *testing.T) {
	t.Helper()
	if os.Getenv("TF_ACC_TERRAFORM_PATH") == "" {
		binary, err := exec.LookPath("terraform")
		if err != nil {
			t.Skip("Terraform executable is required for local protocol tests")
		}
		t.Setenv("TF_ACC_TERRAFORM_PATH", binary)
	}
}
func TestSCIMMappingResourceProtocol(t *testing.T) {
	identityTerraform(t)
	var mu sync.Mutex
	current := map[string]client.SCIMGroupMapping{}
	created, patched := 0, 0
	var deletes []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("Authorization") != "Bearer local-management-key" {
			t.Errorf("wrong authorization")
			http.Error(w, "unauthorized", 401)
			return
		}
		if r.URL.Path == "/key" {
			fmt.Fprint(w, `{"data":{"label":"local-key","is_management_key":true}}`)
			return
		}
		if r.URL.Path == "/scim/group-mappings" {
			if r.Method == "GET" {
				items := make([]client.SCIMGroupMapping, 0, len(current))
				for _, m := range current {
					items = append(items, m)
				}
				json.NewEncoder(w).Encode(map[string]any{"data": items, "total_count": len(items)})
				return
			}
			if r.Method == "POST" {
				var body client.SCIMGroupMappingCreateRequest
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				created++
				id := fmt.Sprintf("mapping-%d", created)
				m := scimMappingFixture()
				m.ID = id
				m.SCIMGroupID = body.SCIMGroupID
				m.WorkspaceID = body.WorkspaceID
				m.Role = body.Role
				current[id] = m
				json.NewEncoder(w).Encode(map[string]any{"data": m})
				return
			}
		}
		if strings.HasPrefix(r.URL.Path, "/scim/group-mappings/") {
			id := strings.TrimPrefix(r.URL.Path, "/scim/group-mappings/")
			m, exists := current[id]
			if !exists {
				w.WriteHeader(404)
				fmt.Fprint(w, `{"error":{"message":"not found"}}`)
				return
			}
			switch r.Method {
			case "GET":
				json.NewEncoder(w).Encode(map[string]any{"data": m})
				return
			case "PATCH":
				var body map[string]string
				json.NewDecoder(r.Body).Decode(&body)
				if len(body) != 1 {
					t.Errorf("unexpected patch fields %v", body)
				}
				patched++
				m.Role = body["role"]
				m.UpdatedAt = "changed"
				current[id] = m
				json.NewEncoder(w).Encode(map[string]any{"data": m})
				return
			case "DELETE":
				keep := r.URL.Query().Get("keep_members")
				if keep != "true" && keep != "false" {
					t.Errorf("missing deletion policy")
				}
				deletes = append(deletes, keep)
				delete(current, id)
				fmt.Fprint(w, `{"deleted":true}`)
				return
			}
		}
		t.Errorf("unexpected %s %s", r.Method, r.URL)
		http.Error(w, "not found", 404)
	}))
	defer server.Close()
	config := func(group, role, keep string) string {
		return fmt.Sprintf(`provider "openrouter" {
 api_key = "local-management-key"
 base_url = %q
}
resource "openrouter_scim_group_mapping" "test" {
 scim_group_id = %q
 workspace_id = "workspace"
 role = %q
 %s
}`, server.URL, group, role, keep)
	}
	address := "openrouter_scim_group_mapping.test"
	resource.UnitTest(t, resource.TestCase{ProtoV6ProviderFactories: identityProtocolFactories(), TerraformVersionChecks: []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_11_0)}, CheckDestroy: func(_ *terraform.State) error {
		mu.Lock()
		defer mu.Unlock()
		if len(current) != 0 {
			return fmt.Errorf("mapping remained")
		}
		return nil
	}, Steps: []resource.TestStep{
		{Config: config("group", "member", ""), Check: resource.ComposeTestCheckFunc(resource.TestCheckResourceAttr(address, "id", "mapping-1"), resource.TestCheckResourceAttr(address, "keep_members", "true"))},
		{Config: config("group", "member", ""), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		{ResourceName: address, ImportState: true, ImportStateVerify: true},
		{Config: config("group", "admin", "")},
		{Config: config("group", "admin", "keep_members = false"), Check: resource.TestCheckResourceAttr(address, "keep_members", "false")},
		{Config: config("new-group", "member", ""), Check: resource.TestCheckResourceAttr(address, "id", "mapping-2")},
		{Config: config("new-group", "member", ""), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
	}})
	mu.Lock()
	defer mu.Unlock()
	if created != 2 || patched != 1 || len(deletes) != 2 || deletes[0] != "false" || deletes[1] != "true" {
		t.Fatalf("created=%d patched=%d deleted=%v", created, patched, deletes)
	}
}
func TestIdentityDataSourcesProtocol(t *testing.T) {
	identityTerraform(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("read-only mutation %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/key":
			fmt.Fprint(w, `{"data":{"label":"local-key","is_management_key":true}}`)
		case "/scim/groups":
			if r.URL.Query().Get("display_name") != "Engineering" {
				t.Errorf("missing exact filter")
			}
			json.NewEncoder(w).Encode(map[string]any{"data": []client.SCIMGroup{{ID: "group", OrganizationID: "org", DisplayName: "Engineering", CreatedAt: "now", UpdatedAt: "now"}}, "total_count": 1})
		case "/workspaces/workspace/members":
			json.NewEncoder(w).Encode(map[string]any{"data": []client.WorkspaceMember{{ID: "membership", WorkspaceID: "workspace", UserID: "user", Role: "restricted_viewer", CreatedAt: "now"}}, "total_count": 1})
		default:
			t.Errorf("unexpected %s", r.URL)
			http.Error(w, "not found", 404)
		}
	}))
	defer server.Close()
	config := fmt.Sprintf(`provider "openrouter" {
 api_key = "local-management-key"
 base_url = %q
}
data "openrouter_scim_groups" "test" { display_name = "Engineering" }
data "openrouter_workspace_members" "test" { workspace_id = "workspace" }
`, server.URL)
	resource.UnitTest(t, resource.TestCase{ProtoV6ProviderFactories: identityProtocolFactories(), TerraformVersionChecks: []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_11_0)}, Steps: []resource.TestStep{{Config: config, Check: resource.ComposeTestCheckFunc(resource.TestCheckResourceAttr("data.openrouter_scim_groups.test", "total_count", "1"), resource.TestCheckResourceAttr("data.openrouter_scim_groups.test", "items.0.id", "group"), resource.TestCheckNoResourceAttr("data.openrouter_scim_groups.test", "items.0.external_id"), resource.TestCheckResourceAttr("data.openrouter_workspace_members.test", "total_count", "1"), resource.TestCheckResourceAttr("data.openrouter_workspace_members.test", "items.0.role", "restricted_viewer"))}}})
}
