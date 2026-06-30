package provider

import (
	"context"
	"testing"

	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/cloudopsworks/terraform-provider-openrouter/internal/client"
)

func TestGuardrailResourceWorkspaceIDSchemaSupportsAPIDefault(t *testing.T) {
	t.Parallel()

	var resp frameworkresource.SchemaResponse
	NewGuardrailResource().Schema(context.Background(), frameworkresource.SchemaRequest{}, &resp)
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

	priorWorkspaceID := "f7574683-5741-5f2a-9704-aef2c3ff2a05"
	plannedWorkspaceID, requiresReplace := applyStringPlanModifiersForTest(
		attr.PlanModifiers,
		types.StringNull(),
		types.StringUnknown(),
		types.StringValue(priorWorkspaceID),
	)
	if requiresReplace {
		t.Fatal("workspace_id must not force replacement when an omitted value is already resolved in state")
	}
	if plannedWorkspaceID.IsUnknown() || plannedWorkspaceID.ValueString() != priorWorkspaceID {
		t.Fatalf("workspace_id planned value = %s, want prior state value %q", plannedWorkspaceID.String(), priorWorkspaceID)
	}

	newWorkspaceID := "a1114683-5741-5f2a-9704-aef2c3ff2a05"
	_, requiresReplace = applyStringPlanModifiersForTest(
		attr.PlanModifiers,
		types.StringValue(newWorkspaceID),
		types.StringValue(newWorkspaceID),
		types.StringValue(priorWorkspaceID),
	)
	if !requiresReplace {
		t.Fatal("workspace_id must continue to force replacement when an explicitly configured value changes")
	}
}

func TestFlattenGuardrailModelStoresReturnedWorkspaceID(t *testing.T) {
	t.Parallel()

	workspaceID := "f7574683-5741-5f2a-9704-aef2c3ff2a05"
	model, diags := flattenGuardrailModel(context.Background(), client.Guardrail{
		ID:          "guardrail-id",
		Name:        "test-guardrail",
		WorkspaceID: &workspaceID,
	})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %s", diags)
	}

	if model.WorkspaceID.IsNull() || model.WorkspaceID.IsUnknown() {
		t.Fatal("workspace_id should be known when the API returns a default workspace ID")
	}
	if got := model.WorkspaceID.ValueString(); got != workspaceID {
		t.Fatalf("workspace_id = %q, want %q", got, workspaceID)
	}
}
