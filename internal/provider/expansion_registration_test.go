package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

func TestExpansionRegisteredWithSafeSchemas(t *testing.T) {
	server := providerserver.NewProtocol6(New("test")())()
	response, err := server.GetProviderSchema(context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for _, diagnostic := range response.Diagnostics {
		if diagnostic.Severity == tfprotov6.DiagnosticSeverityError {
			t.Fatalf("provider schema: %s: %s", diagnostic.Summary, diagnostic.Detail)
		}
	}
	for _, name := range []string{"openrouter_scim_group_mapping", "openrouter_observability_destination"} {
		if response.ResourceSchemas[name] == nil {
			t.Errorf("missing resource schema %s", name)
		}
	}
	for _, name := range []string{"openrouter_scim_groups", "openrouter_workspace_members", "openrouter_model", "openrouter_models", "openrouter_activity"} {
		if response.DataSourceSchemas[name] == nil {
			t.Errorf("missing data source schema %s", name)
		}
	}
	if response.ResourceSchemas["openrouter_workspace_member"] != nil {
		t.Error("workspace membership mutation must remain deferred until implicit/SCIM ownership is distinguishable")
	}
	if schema := response.ResourceSchemas["openrouter_observability_destination"]; schema != nil {
		found := false
		for _, attribute := range schema.Block.Attributes {
			if attribute.Name == "config_wo" {
				found = true
				if !attribute.WriteOnly || !attribute.Sensitive {
					t.Error("destination configuration must remain sensitive and write-only")
				}
			}
			if attribute.Name == "config" {
				t.Error("returned destination config must not enter ordinary state")
			}
		}
		if !found {
			t.Error("missing write-only destination configuration")
		}
	}
}
