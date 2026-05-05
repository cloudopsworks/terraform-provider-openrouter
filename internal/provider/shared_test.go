package provider

import (
	"testing"

	"github.com/cloudopsworks/terraform-provider-openrouter/internal/client"
)

func TestParseCompositeImportID(t *testing.T) {
	workspaceID, name, err := parseCompositeImportID("550e8400-e29b-41d4-a716-446655440000_my_name")
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if workspaceID != "550e8400-e29b-41d4-a716-446655440000" {
		t.Fatalf("unexpected workspace ID: %s", workspaceID)
	}
	if name != "my_name" {
		t.Fatalf("unexpected name: %s", name)
	}
}

func TestParseCompositeImportIDInvalid(t *testing.T) {
	if _, _, err := parseCompositeImportID("missingseparator"); err == nil {
		t.Fatal("expected error for invalid import ID")
	}
}

func TestFindAPIKeyByCompositeImport(t *testing.T) {
	workspaceID := "550e8400-e29b-41d4-a716-446655440000"
	otherWorkspaceID := "550e8400-e29b-41d4-a716-446655440001"
	item, err := findAPIKeyByCompositeImport([]client.APIKey{
		{Name: "service", WorkspaceID: &otherWorkspaceID},
		{Name: "service", WorkspaceID: &workspaceID},
	}, workspaceID, "service")
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if item == nil || item.WorkspaceID == nil || *item.WorkspaceID != workspaceID {
		t.Fatalf("expected workspace match %q, got %#v", workspaceID, item)
	}
}

func TestFindWorkspaceByCompositeImport(t *testing.T) {
	workspace := &client.Workspace{
		ID:   "550e8400-e29b-41d4-a716-446655440000",
		Name: "platform",
	}
	if err := findWorkspaceByCompositeImport(workspace, workspace.ID, workspace.Name); err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
}

func TestFindGuardrailByCompositeImport(t *testing.T) {
	workspaceID := "550e8400-e29b-41d4-a716-446655440000"
	otherWorkspaceID := "550e8400-e29b-41d4-a716-446655440001"
	item, err := findGuardrailByCompositeImport([]client.Guardrail{
		{Name: "prod", WorkspaceID: &otherWorkspaceID},
		{Name: "prod", WorkspaceID: &workspaceID},
	}, workspaceID, "prod")
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if item == nil || item.WorkspaceID == nil || *item.WorkspaceID != workspaceID {
		t.Fatalf("expected workspace match %q, got %#v", workspaceID, item)
	}
}
