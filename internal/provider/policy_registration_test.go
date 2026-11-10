package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

func TestPolicyResourcesRegisteredWithValidSchemas(t *testing.T) {
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
	for _, name := range []string{
		"openrouter_budget_policy",
		"openrouter_model_availability_policy",
		"openrouter_api_key_policy_assignment",
		"openrouter_member_policy_assignment",
	} {
		if response.ResourceSchemas[name] == nil {
			t.Errorf("missing resource schema %s", name)
		}
	}
}
