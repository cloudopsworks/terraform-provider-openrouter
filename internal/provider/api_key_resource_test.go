package provider

import (
	"context"
	"testing"

	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"

	"github.com/cloudopsworks/terraform-provider-openrouter/internal/client"
)

func TestAPIKeyResourceWorkspaceIDSchemaSupportsAPIDefault(t *testing.T) {
	t.Parallel()

	var resp frameworkresource.SchemaResponse
	NewAPIKeyResource().Schema(context.Background(), frameworkresource.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected schema diagnostics: %s", resp.Diagnostics)
	}

	attr, ok := resp.Schema.Attributes["workspace_id"].(resourceschema.StringAttribute)
	if !ok {
		t.Fatalf("workspace_id attribute type = %T, want schema.StringAttribute", resp.Schema.Attributes["workspace_id"])
	}
	if !attr.Optional {
		t.Fatal("workspace_id must remain Optional so users can explicitly target a workspace")
	}
	if !attr.Computed {
		t.Fatal("workspace_id must be Computed so API-defaulted workspace IDs can be recorded in state")
	}

	modifiersByDescription := make(map[string]bool, len(attr.PlanModifiers))
	for _, modifier := range attr.PlanModifiers {
		modifiersByDescription[modifier.Description(context.Background())] = true
	}

	if !modifiersByDescription["If the value of this attribute changes, Terraform will destroy and recreate the resource."] {
		t.Fatal("workspace_id must continue to force replacement when an explicitly configured value changes")
	}
	if !modifiersByDescription["Once set, the value of this attribute in state will not change."] {
		t.Fatal("workspace_id must use prior state for unknown planned values to avoid spurious replans")
	}
}

func TestFlattenAPIKeyModelStoresReturnedWorkspaceID(t *testing.T) {
	t.Parallel()

	workspaceID := "f7574683-5741-5f2a-9704-aef2c3ff2a05"
	model := flattenAPIKeyModel(client.APIKey{
		Hash:        "key-hash",
		Name:        "test-key",
		WorkspaceID: &workspaceID,
	})

	if model.WorkspaceID.IsNull() || model.WorkspaceID.IsUnknown() {
		t.Fatal("workspace_id should be known when the API returns a default workspace ID")
	}
	if got := model.WorkspaceID.ValueString(); got != workspaceID {
		t.Fatalf("workspace_id = %q, want %q", got, workspaceID)
	}
}
