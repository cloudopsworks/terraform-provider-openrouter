package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/cloudopsworks/terraform-provider-openrouter/internal/client"
)

var _ datasource.DataSource = &workspaceDataSource{}

type workspaceDataSource struct{ client *client.Client }

type workspaceDataSourceModel struct {
	ID                              types.String  `tfsdk:"id"`
	Name                            types.String  `tfsdk:"name"`
	Slug                            types.String  `tfsdk:"slug"`
	Description                     types.String  `tfsdk:"description"`
	DefaultTextModel                types.String  `tfsdk:"default_text_model"`
	DefaultImageModel               types.String  `tfsdk:"default_image_model"`
	DefaultProviderSort             types.String  `tfsdk:"default_provider_sort"`
	IOLoggingSamplingRate           types.Float64 `tfsdk:"io_logging_sampling_rate"`
	IsDataDiscountLoggingEnabled    types.Bool    `tfsdk:"is_data_discount_logging_enabled"`
	IsObservabilityBroadcastEnabled types.Bool    `tfsdk:"is_observability_broadcast_enabled"`
	IsObservabilityIOLoggingEnabled types.Bool    `tfsdk:"is_observability_io_logging_enabled"`
	CreatedAt                       types.String  `tfsdk:"created_at"`
	CreatedBy                       types.String  `tfsdk:"created_by"`
	UpdatedAt                       types.String  `tfsdk:"updated_at"`
}

func NewWorkspaceDataSource() datasource.DataSource { return &workspaceDataSource{} }

func (d *workspaceDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workspace"
}

func (d *workspaceDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	providerData, err := configureClient(req.ProviderData)
	if err != nil {
		resp.Diagnostics.AddError("Unable to configure OpenRouter workspace data source", err.Error())
		return
	}
	d.client = providerData.client
}

func (d *workspaceDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasourceschema.Schema{
		MarkdownDescription: "Resolve a single OpenRouter workspace by `id`, `slug`, or exact `name`. Slug/ID use the single-workspace API; exact name matching falls back to listing workspaces.",
		Attributes: map[string]datasourceschema.Attribute{
			"id":                                  datasourceschema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Workspace UUID lookup key."},
			"name":                                datasourceschema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Exact workspace name lookup key when `id` or `slug` is not used."},
			"slug":                                datasourceschema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Workspace slug lookup key."},
			"description":                         datasourceschema.StringAttribute{Computed: true},
			"default_text_model":                  datasourceschema.StringAttribute{Computed: true},
			"default_image_model":                 datasourceschema.StringAttribute{Computed: true},
			"default_provider_sort":               datasourceschema.StringAttribute{Computed: true},
			"io_logging_sampling_rate":            datasourceschema.Float64Attribute{Computed: true},
			"is_data_discount_logging_enabled":    datasourceschema.BoolAttribute{Computed: true},
			"is_observability_broadcast_enabled":  datasourceschema.BoolAttribute{Computed: true},
			"is_observability_io_logging_enabled": datasourceschema.BoolAttribute{Computed: true},
			"created_at":                          datasourceschema.StringAttribute{Computed: true},
			"created_by":                          datasourceschema.StringAttribute{Computed: true},
			"updated_at":                          datasourceschema.StringAttribute{Computed: true},
		},
	}
}

func (d *workspaceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config workspaceDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	lookupKind, lookupValue, err := resolveWorkspaceLookup(config.ID, config.Slug, config.Name)
	if err != nil {
		resp.Diagnostics.AddError("Invalid OpenRouter workspace lookup", err.Error())
		return
	}

	var workspace *client.Workspace
	switch lookupKind {
	case "id", "slug":
		workspace, err = d.client.GetWorkspace(ctx, lookupValue)
		if err != nil {
			resp.Diagnostics.AddError("Unable to read OpenRouter workspace", err.Error())
			return
		}
	case "name":
		items, listErr := d.client.ListWorkspaces(ctx)
		if listErr != nil {
			resp.Diagnostics.AddError("Unable to list OpenRouter workspaces", listErr.Error())
			return
		}
		workspace, err = findWorkspaceByName(items, lookupValue)
		if err != nil {
			resp.Diagnostics.AddError("Unable to resolve OpenRouter workspace by name", err.Error())
			return
		}
	}

	state := flattenWorkspaceDataSourceModel(*workspace)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func resolveWorkspaceLookup(id, slug, name types.String) (string, string, error) {
	values := []struct {
		kind  string
		value types.String
	}{
		{kind: "id", value: id},
		{kind: "slug", value: slug},
		{kind: "name", value: name},
	}

	var selectedKind string
	var selectedValue string
	for _, item := range values {
		if item.value.IsNull() || item.value.IsUnknown() {
			continue
		}
		value := item.value.ValueString()
		if value == "" {
			continue
		}
		if selectedKind != "" {
			return "", "", fmt.Errorf("set exactly one of id, slug, or name")
		}
		selectedKind = item.kind
		selectedValue = value
	}

	if selectedKind == "" {
		return "", "", fmt.Errorf("one of id, slug, or name must be set")
	}
	return selectedKind, selectedValue, nil
}

func findWorkspaceByName(items []client.Workspace, name string) (*client.Workspace, error) {
	matches := make([]client.Workspace, 0, 1)
	for _, item := range items {
		if item.Name == name {
			matches = append(matches, item)
		}
	}

	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("no workspace named %q found", name)
	case 1:
		return &matches[0], nil
	default:
		return nil, fmt.Errorf("multiple workspaces named %q found; use slug or id instead", name)
	}
}

func flattenWorkspaceDataSourceModel(in client.Workspace) workspaceDataSourceModel {
	return workspaceDataSourceModel{
		ID:                              types.StringValue(in.ID),
		Name:                            types.StringValue(in.Name),
		Slug:                            types.StringValue(in.Slug),
		Description:                     stringPtrValue(in.Description),
		DefaultTextModel:                stringPtrValue(in.DefaultTextModel),
		DefaultImageModel:               stringPtrValue(in.DefaultImageModel),
		DefaultProviderSort:             stringPtrValue(in.DefaultProviderSort),
		IOLoggingSamplingRate:           float64PtrValue(in.IOLoggingSamplingRate),
		IsDataDiscountLoggingEnabled:    types.BoolValue(in.IsDataDiscountLoggingEnabled),
		IsObservabilityBroadcastEnabled: types.BoolValue(in.IsObservabilityBroadcastEnabled),
		IsObservabilityIOLoggingEnabled: types.BoolValue(in.IsObservabilityIOLoggingEnabled),
		CreatedAt:                       types.StringValue(in.CreatedAt),
		CreatedBy:                       stringPtrValue(in.CreatedBy),
		UpdatedAt:                       stringPtrValue(in.UpdatedAt),
	}
}
