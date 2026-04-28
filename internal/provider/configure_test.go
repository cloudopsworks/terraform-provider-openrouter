package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

func TestDataSourceConfigureIgnoresNilProviderData(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*datasource.ConfigureResponse)
	}{
		{
			name: "api keys",
			configure: func(resp *datasource.ConfigureResponse) {
				ds := &apiKeysDataSource{}
				ds.Configure(context.Background(), datasource.ConfigureRequest{}, resp)
			},
		},
		{
			name: "guardrails",
			configure: func(resp *datasource.ConfigureResponse) {
				ds := &guardrailsDataSource{}
				ds.Configure(context.Background(), datasource.ConfigureRequest{}, resp)
			},
		},
		{
			name: "organization",
			configure: func(resp *datasource.ConfigureResponse) {
				ds := &organizationDataSource{}
				ds.Configure(context.Background(), datasource.ConfigureRequest{}, resp)
			},
		},
		{
			name: "providers",
			configure: func(resp *datasource.ConfigureResponse) {
				ds := &providersDataSource{}
				ds.Configure(context.Background(), datasource.ConfigureRequest{}, resp)
			},
		},
		{
			name: "workspace",
			configure: func(resp *datasource.ConfigureResponse) {
				ds := &workspaceDataSource{}
				ds.Configure(context.Background(), datasource.ConfigureRequest{}, resp)
			},
		},
		{
			name: "workspaces",
			configure: func(resp *datasource.ConfigureResponse) {
				ds := &workspacesDataSource{}
				ds.Configure(context.Background(), datasource.ConfigureRequest{}, resp)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var resp datasource.ConfigureResponse
			tt.configure(&resp)

			if resp.Diagnostics.HasError() {
				t.Fatalf("expected no diagnostics, got: %v", resp.Diagnostics)
			}
		})
	}
}

func TestResourceConfigureIgnoresNilProviderData(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*resource.ConfigureResponse)
	}{
		{
			name: "api key",
			configure: func(resp *resource.ConfigureResponse) {
				r := &apiKeyResource{}
				r.Configure(context.Background(), resource.ConfigureRequest{}, resp)
			},
		},
		{
			name: "guardrail",
			configure: func(resp *resource.ConfigureResponse) {
				r := &guardrailResource{}
				r.Configure(context.Background(), resource.ConfigureRequest{}, resp)
			},
		},
		{
			name: "workspace",
			configure: func(resp *resource.ConfigureResponse) {
				r := &workspaceResource{}
				r.Configure(context.Background(), resource.ConfigureRequest{}, resp)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var resp resource.ConfigureResponse
			tt.configure(&resp)

			if resp.Diagnostics.HasError() {
				t.Fatalf("expected no diagnostics, got: %v", resp.Diagnostics)
			}
		})
	}
}

func TestConfigureClientRejectsMissingClient(t *testing.T) {
	t.Parallel()

	if _, err := configureClient((*providerData)(nil)); err == nil {
		t.Fatal("expected error for nil provider data")
	}

	if _, err := configureClient(&providerData{}); err == nil {
		t.Fatal("expected error for provider data without client")
	}
}
