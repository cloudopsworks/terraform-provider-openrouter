package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/cloudopsworks/terraform-provider-openrouter/internal/secrets"
)

func TestProviderSettingsFromModelWithEnvPrecedence(t *testing.T) {
	settings := providerSettingsFromModelWithEnv(openRouterProviderModel{
		APIKey:  types.StringValue(" config-key "),
		BaseURL: types.StringNull(),
	}, mapEnv(map[string]string{
		"OPENROUTER_API_KEY":  "env-key",
		"OPENROUTER_BASE_URL": "https://env.example/v1",
	}))

	if settings.AdminAPIKey != "config-key" || settings.BaseURL != "https://env.example/v1" {
		t.Fatalf("settings = %#v", settings)
	}
}

func TestSourceConfigFromProviderModelEnvDirectSuppressesSecretDiscovery(t *testing.T) {
	cfg, diags := sourceConfigFromProviderModelWithEnv(context.Background(), openRouterProviderModel{}, mapEnv(map[string]string{
		"OPENROUTER_API_KEY":                       " env-key ",
		"OPENROUTER_AWS_SECRETS_MANAGER_SECRET_ID": "prod/openrouter",
	}))
	if diags.HasError() {
		t.Fatalf("diagnostics: %v", diags)
	}
	if cfg.Direct.AdminAPIKey != "env-key" || cfg.AWS != nil || cfg.GCP != nil || cfg.Azure != nil {
		t.Fatalf("unexpected source config: %#v", cfg)
	}
}

func TestSourceConfigFromProviderModelDirectSuppressesExplicitSecretValidation(t *testing.T) {
	awsObject, diags := types.ObjectValue(awsObjectTypesForTest(), map[string]attr.Value{
		"region":            types.StringValue("us-east-1"),
		"secret_id":         types.StringValue("prod/openrouter"),
		"version_id":        types.StringNull(),
		"version_stage":     types.StringNull(),
		"json_key":          types.StringNull(),
		"role_arn":          types.StringNull(),
		"role_session_name": types.StringValue("invalid-without-role"),
		"external_id":       types.StringNull(),
		"duration_seconds":  types.Int64Null(),
	})
	if diags.HasError() {
		t.Fatalf("object diagnostics: %v", diags)
	}

	cfg, diags := sourceConfigFromProviderModel(context.Background(), openRouterProviderModel{
		APIKey:            types.StringValue("direct-key"),
		AWSSecretsManager: awsObject,
		GCPSecretManager:  types.ObjectNull(gcpObjectTypesForTest()),
		AzureKeyVault:     types.ObjectNull(azureObjectTypesForTest()),
	})
	if diags.HasError() {
		t.Fatalf("direct configuration should suppress cloud validation: %v", diags)
	}
	if cfg.Direct.AdminAPIKey != "direct-key" || cfg.AWS != nil {
		t.Fatalf("unexpected source config: %#v", cfg)
	}
}

func TestSourceConfigFromProviderModelEnvDiscoversAWS(t *testing.T) {
	cfg, diags := sourceConfigFromProviderModelWithEnv(context.Background(), openRouterProviderModel{}, mapEnv(map[string]string{
		"OPENROUTER_AWS_SECRETS_MANAGER_REGION":    " us-east-1 ",
		"OPENROUTER_AWS_SECRETS_MANAGER_SECRET_ID": " prod/openrouter ",
		"OPENROUTER_AWS_SECRETS_MANAGER_JSON_KEY":  " api_key ",
	}))
	if diags.HasError() {
		t.Fatalf("diagnostics: %v", diags)
	}
	if cfg.AWS == nil || cfg.AWS.Region != "us-east-1" || cfg.AWS.SecretID != "prod/openrouter" || cfg.AWS.JSONKey != "api_key" {
		t.Fatalf("AWS config = %#v", cfg.AWS)
	}
}

func TestSourceConfigFromProviderModelEnvDiscoversGCP(t *testing.T) {
	cfg, diags := sourceConfigFromProviderModelWithEnv(context.Background(), openRouterProviderModel{}, mapEnv(map[string]string{
		"OPENROUTER_GCP_SECRET_MANAGER_PROJECT_ID": " platform-prod ",
		"OPENROUTER_GCP_SECRET_MANAGER_SECRET_ID":  " openrouter-key ",
		"OPENROUTER_GCP_SECRET_MANAGER_VERSION":    " latest ",
		"OPENROUTER_GCP_SECRET_MANAGER_JSON_KEY":   " api_key ",
	}))
	if diags.HasError() {
		t.Fatalf("diagnostics: %v", diags)
	}
	if cfg.GCP == nil || cfg.GCP.ProjectID != "platform-prod" || cfg.GCP.SecretID != "openrouter-key" || cfg.GCP.Version != "latest" || cfg.GCP.JSONKey != "api_key" {
		t.Fatalf("GCP config = %#v", cfg.GCP)
	}
}

func TestCloudSourceResolutionRequiresExactlyOneSource(t *testing.T) {
	if _, _, err := secrets.NewResolver(secrets.SourceConfig{}); err == nil {
		t.Fatal("expected missing source error")
	}
	if _, _, err := secrets.NewResolver(secrets.SourceConfig{
		AWS: &secrets.AWSConfig{SecretID: "aws"},
		GCP: &secrets.GCPConfig{ProjectID: "project", SecretID: "gcp"},
	}); err == nil {
		t.Fatal("expected multiple source error")
	}
	resolver, source, err := secrets.NewResolver(secrets.SourceConfig{
		Direct: secrets.Settings{AdminAPIKey: "direct"},
		AWS:    &secrets.AWSConfig{SecretID: "ignored"},
	})
	if err != nil || source != "configuration" {
		t.Fatalf("direct source = %q, err=%v", source, err)
	}
	settings, err := resolver.Resolve(context.Background())
	if err != nil || settings.AdminAPIKey != "direct" {
		t.Fatalf("direct settings = %#v, err=%v", settings, err)
	}
}

func mapEnv(values map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
	}
}

func awsObjectTypesForTest() map[string]attr.Type {
	return map[string]attr.Type{
		"region":            types.StringType,
		"secret_id":         types.StringType,
		"version_id":        types.StringType,
		"version_stage":     types.StringType,
		"json_key":          types.StringType,
		"role_arn":          types.StringType,
		"role_session_name": types.StringType,
		"external_id":       types.StringType,
		"duration_seconds":  types.Int64Type,
	}
}

func gcpObjectTypesForTest() map[string]attr.Type {
	return map[string]attr.Type{
		"project_id":                  types.StringType,
		"secret_id":                   types.StringType,
		"version":                     types.StringType,
		"json_key":                    types.StringType,
		"impersonate_service_account": types.StringType,
		"delegates":                   types.ListType{ElemType: types.StringType},
		"scopes":                      types.ListType{ElemType: types.StringType},
	}
}

func azureObjectTypesForTest() map[string]attr.Type {
	return map[string]attr.Type{
		"vault_url":   types.StringType,
		"secret_name": types.StringType,
		"version":     types.StringType,
		"json_key":    types.StringType,
	}
}
