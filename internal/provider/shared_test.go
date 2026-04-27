package provider

import "testing"

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
