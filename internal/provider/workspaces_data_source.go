package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/cloudopsworks/terraform-provider-openrouter/internal/client"
)

var _ datasource.DataSource = &workspacesDataSource{}

type workspacesDataSource struct{ client *client.Client }

type workspacesDataSourceModel struct {
	TotalCount types.Int64 `tfsdk:"total_count"`
	Items      types.List  `tfsdk:"items"`
}

func NewWorkspacesDataSource() datasource.DataSource { return &workspacesDataSource{} }

func (d *workspacesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workspaces"
}

func (d *workspacesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	providerData, err := configureClient(req.ProviderData)
	if err != nil {
		resp.Diagnostics.AddError("Unable to configure OpenRouter workspaces data source", err.Error())
		return
	}
	d.client = providerData.client
}

func (d *workspacesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasourceschema.Schema{Attributes: map[string]datasourceschema.Attribute{
		"total_count": datasourceschema.Int64Attribute{Computed: true},
		"items": datasourceschema.ListNestedAttribute{Computed: true, NestedObject: datasourceschema.NestedAttributeObject{Attributes: map[string]datasourceschema.Attribute{
			"id":                                  datasourceschema.StringAttribute{Computed: true},
			"name":                                datasourceschema.StringAttribute{Computed: true},
			"slug":                                datasourceschema.StringAttribute{Computed: true},
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
		}}},
	}}
}

func (d *workspacesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	items, err := d.client.ListWorkspaces(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to list OpenRouter workspaces", err.Error())
		return
	}
	itemType := map[string]attr.Type{
		"id":                                  types.StringType,
		"name":                                types.StringType,
		"slug":                                types.StringType,
		"description":                         types.StringType,
		"default_text_model":                  types.StringType,
		"default_image_model":                 types.StringType,
		"default_provider_sort":               types.StringType,
		"io_logging_sampling_rate":            types.Float64Type,
		"is_data_discount_logging_enabled":    types.BoolType,
		"is_observability_broadcast_enabled":  types.BoolType,
		"is_observability_io_logging_enabled": types.BoolType,
		"created_at":                          types.StringType,
		"created_by":                          types.StringType,
		"updated_at":                          types.StringType,
	}
	objects := make([]attr.Value, 0, len(items))
	for _, item := range items {
		object, diags := types.ObjectValue(itemType, map[string]attr.Value{
			"id":                                  types.StringValue(item.ID),
			"name":                                types.StringValue(item.Name),
			"slug":                                types.StringValue(item.Slug),
			"description":                         stringPtrValue(item.Description),
			"default_text_model":                  stringPtrValue(item.DefaultTextModel),
			"default_image_model":                 stringPtrValue(item.DefaultImageModel),
			"default_provider_sort":               stringPtrValue(item.DefaultProviderSort),
			"io_logging_sampling_rate":            float64PtrValue(item.IOLoggingSamplingRate),
			"is_data_discount_logging_enabled":    types.BoolValue(item.IsDataDiscountLoggingEnabled),
			"is_observability_broadcast_enabled":  types.BoolValue(item.IsObservabilityBroadcastEnabled),
			"is_observability_io_logging_enabled": types.BoolValue(item.IsObservabilityIOLoggingEnabled),
			"created_at":                          types.StringValue(item.CreatedAt),
			"created_by":                          stringPtrValue(item.CreatedBy),
			"updated_at":                          stringPtrValue(item.UpdatedAt),
		})
		resp.Diagnostics.Append(diags...)
		objects = append(objects, object)
	}
	listValue, diags := types.ListValue(types.ObjectType{AttrTypes: itemType}, objects)
	resp.Diagnostics.Append(diags...)
	state := workspacesDataSourceModel{TotalCount: types.Int64Value(int64(len(items))), Items: listValue}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
