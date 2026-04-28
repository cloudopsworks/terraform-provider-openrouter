package provider

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	providerschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/cloudopsworks/terraform-provider-openrouter/internal/client"
)

var _ provider.Provider = &openRouterProvider{}

type openRouterProvider struct {
	version string
}

type openRouterProviderModel struct {
	APIKey  types.String `tfsdk:"api_key"`
	BaseURL types.String `tfsdk:"base_url"`
}

type providerData struct {
	client *client.Client
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &openRouterProvider{version: version}
	}
}

func (p *openRouterProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "openrouter"
	resp.Version = p.version
}

func (p *openRouterProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = providerschema.Schema{
		MarkdownDescription: "OpenRouter management provider.",
		Attributes: map[string]providerschema.Attribute{
			"api_key": providerschema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "OpenRouter management API key. Can also be set with OPENROUTER_API_KEY.",
			},
			"base_url": providerschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Override for the OpenRouter management API base URL. Defaults to https://openrouter.ai/api/v1.",
			},
		},
	}
}

func resolveAPIKey(configValue types.String) (string, string) {
	apiKey := strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY"))
	if configValue.IsNull() || configValue.IsUnknown() {
		if apiKey == "" {
			return "", "none"
		}
		return apiKey, "environment"
	}

	configured := strings.TrimSpace(configValue.ValueString())
	if configured == "" {
		if apiKey == "" {
			return "", "none"
		}
		return apiKey, "environment"
	}

	return configured, "configuration"
}

func (p *openRouterProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config openRouterProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	traceCtx := tflog.SetField(ctx, "provider", "openrouter")
	traceCtx = tflog.SetField(traceCtx, "provider_version", p.version)

	apiKey, apiKeySource := resolveAPIKey(config.APIKey)
	baseURLConfigured := !config.BaseURL.IsNull() && !config.BaseURL.IsUnknown() && config.BaseURL.ValueString() != ""
	tflog.Trace(traceCtx, "configuring OpenRouter provider", map[string]interface{}{
		"api_key_source":      apiKeySource,
		"base_url_configured": baseURLConfigured,
	})
	if apiKey == "" {
		tflog.Trace(traceCtx, "OpenRouter provider configuration failed", map[string]interface{}{
			"reason": "missing_api_key",
		})
		resp.Diagnostics.AddError("Missing OpenRouter API key", "Set api_key in the provider configuration or OPENROUTER_API_KEY in the environment.")
		return
	}

	baseURL := client.DefaultBaseURL
	if baseURLConfigured {
		baseURL = config.BaseURL.ValueString()
	}

	userAgent := fmt.Sprintf("terraform-provider-openrouter/%s", p.version)
	cl := client.New(apiKey, baseURL, userAgent, 30*time.Second)

	tflog.Trace(traceCtx, "validating OpenRouter provider credentials", map[string]interface{}{
		"api_key_source": apiKeySource,
		"base_url":       baseURL,
	})
	currentKey, err := cl.GetCurrentKey(ctx)
	if err != nil {
		tflog.Trace(traceCtx, "OpenRouter provider credential validation failed", map[string]interface{}{
			"base_url": baseURL,
			"error":    err.Error(),
		})
		resp.Diagnostics.AddError("Unable to validate OpenRouter credentials", err.Error())
		return
	}
	if !currentKey.IsManagementKey {
		tflog.Trace(traceCtx, "OpenRouter provider credential validation failed", map[string]interface{}{
			"base_url":          baseURL,
			"is_management_key": currentKey.IsManagementKey,
			"key_hash":          currentKey.Hash,
		})
		resp.Diagnostics.AddError("OpenRouter management key required", "The configured API key is not a management key. The OpenRouter management endpoints used by this provider require a management key.")
		return
	}

	tflog.Trace(traceCtx, "OpenRouter provider configured successfully", map[string]interface{}{
		"base_url":          baseURL,
		"is_management_key": currentKey.IsManagementKey,
		"key_hash":          currentKey.Hash,
	})
	data := &providerData{client: cl}
	resp.DataSourceData = data
	resp.ResourceData = data
}

func (p *openRouterProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewAPIKeyResource,
		NewWorkspaceResource,
		NewGuardrailResource,
	}
}

func (p *openRouterProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewAPIKeysDataSource,
		NewWorkspacesDataSource,
		NewGuardrailsDataSource,
		NewOrganizationDataSource,
		NewProvidersDataSource,
	}
}
