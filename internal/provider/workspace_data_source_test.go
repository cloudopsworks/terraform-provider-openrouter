package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/cloudopsworks/terraform-provider-openrouter/internal/client"
)

func TestResolveWorkspaceLookup(t *testing.T) {
	tests := []struct {
		name      string
		id        types.String
		slug      types.String
		workspace types.String
		wantKind  string
		wantValue string
		wantErr   bool
	}{
		{
			name:      "uses id",
			id:        types.StringValue("550e8400-e29b-41d4-a716-446655440000"),
			slug:      types.StringNull(),
			workspace: types.StringNull(),
			wantKind:  "id",
			wantValue: "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name:      "uses slug",
			id:        types.StringNull(),
			slug:      types.StringValue("production"),
			workspace: types.StringNull(),
			wantKind:  "slug",
			wantValue: "production",
		},
		{
			name:      "uses name",
			id:        types.StringNull(),
			slug:      types.StringNull(),
			workspace: types.StringValue("Production"),
			wantKind:  "name",
			wantValue: "Production",
		},
		{
			name:      "rejects none",
			id:        types.StringNull(),
			slug:      types.StringNull(),
			workspace: types.StringNull(),
			wantErr:   true,
		},
		{
			name:      "rejects multiple",
			id:        types.StringValue("550e8400-e29b-41d4-a716-446655440000"),
			slug:      types.StringValue("production"),
			workspace: types.StringNull(),
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotKind, gotValue, err := resolveWorkspaceLookup(tt.id, tt.slug, tt.workspace)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotKind != tt.wantKind {
				t.Fatalf("kind = %q, want %q", gotKind, tt.wantKind)
			}
			if gotValue != tt.wantValue {
				t.Fatalf("value = %q, want %q", gotValue, tt.wantValue)
			}
		})
	}
}

func TestFindWorkspaceByName(t *testing.T) {
	items := []client.Workspace{
		{ID: "1", Name: "Production", Slug: "production"},
		{ID: "2", Name: "Staging", Slug: "staging"},
	}

	item, err := findWorkspaceByName(items, "Production")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item.ID != "1" {
		t.Fatalf("id = %q, want 1", item.ID)
	}
}

func TestFindWorkspaceByNameMissing(t *testing.T) {
	_, err := findWorkspaceByName([]client.Workspace{{ID: "1", Name: "Production"}}, "Staging")
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestFindWorkspaceByNameDuplicate(t *testing.T) {
	_, err := findWorkspaceByName([]client.Workspace{
		{ID: "1", Name: "Production"},
		{ID: "2", Name: "Production"},
	}, "Production")
	if err == nil {
		t.Fatal("expected error for duplicate name")
	}
}
