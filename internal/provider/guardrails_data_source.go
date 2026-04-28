package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/cloudopsworks/terraform-provider-openrouter/internal/client"
)

var _ datasource.DataSource = &guardrailsDataSource{}

type guardrailsDataSource struct{ client *client.Client }

type guardrailsDataSourceModel struct {
	TotalCount types.Int64 `tfsdk:"total_count"`
	Items      types.List  `tfsdk:"items"`
}

func NewGuardrailsDataSource() datasource.DataSource { return &guardrailsDataSource{} }

func (d *guardrailsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_guardrails"
}

func (d *guardrailsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	providerData, err := configureClient(req.ProviderData)
	if err != nil {
		resp.Diagnostics.AddError("Unable to configure OpenRouter guardrails data source", err.Error())
		return
	}
	d.client = providerData.client
}

func (d *guardrailsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasourceschema.Schema{Attributes: map[string]datasourceschema.Attribute{
		"total_count": datasourceschema.Int64Attribute{Computed: true},
		"items": datasourceschema.ListNestedAttribute{Computed: true, NestedObject: datasourceschema.NestedAttributeObject{Attributes: map[string]datasourceschema.Attribute{
			"id":                datasourceschema.StringAttribute{Computed: true},
			"name":              datasourceschema.StringAttribute{Computed: true},
			"description":       datasourceschema.StringAttribute{Computed: true},
			"limit_usd":         datasourceschema.Float64Attribute{Computed: true},
			"reset_interval":    datasourceschema.StringAttribute{Computed: true},
			"allowed_models":    datasourceschema.SetAttribute{Computed: true, ElementType: types.StringType},
			"allowed_providers": datasourceschema.SetAttribute{Computed: true, ElementType: types.StringType},
			"ignored_models":    datasourceschema.SetAttribute{Computed: true, ElementType: types.StringType},
			"ignored_providers": datasourceschema.SetAttribute{Computed: true, ElementType: types.StringType},
			"enforce_zdr":       datasourceschema.BoolAttribute{Computed: true},
			"created_at":        datasourceschema.StringAttribute{Computed: true},
			"updated_at":        datasourceschema.StringAttribute{Computed: true},
		}}},
	}}
}

func (d *guardrailsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	items, err := d.client.ListGuardrails(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to list OpenRouter guardrails", err.Error())
		return
	}
	itemType := map[string]attr.Type{
		"id":                types.StringType,
		"name":              types.StringType,
		"description":       types.StringType,
		"limit_usd":         types.Float64Type,
		"reset_interval":    types.StringType,
		"allowed_models":    types.SetType{ElemType: types.StringType},
		"allowed_providers": types.SetType{ElemType: types.StringType},
		"ignored_models":    types.SetType{ElemType: types.StringType},
		"ignored_providers": types.SetType{ElemType: types.StringType},
		"enforce_zdr":       types.BoolType,
		"created_at":        types.StringType,
		"updated_at":        types.StringType,
	}
	objects := make([]attr.Value, 0, len(items))
	for _, item := range items {
		allowedModels, diags := setStringValueOrNull(ctx, item.AllowedModels)
		resp.Diagnostics.Append(diags...)
		allowedProviders, diags := setStringValueOrNull(ctx, item.AllowedProviders)
		resp.Diagnostics.Append(diags...)
		ignoredModels, diags := setStringValueOrNull(ctx, item.IgnoredModels)
		resp.Diagnostics.Append(diags...)
		ignoredProviders, diags := setStringValueOrNull(ctx, item.IgnoredProviders)
		resp.Diagnostics.Append(diags...)
		object, diags := types.ObjectValue(itemType, map[string]attr.Value{
			"id":                types.StringValue(item.ID),
			"name":              types.StringValue(item.Name),
			"description":       stringPtrValue(item.Description),
			"limit_usd":         float64PtrValue(item.LimitUSD),
			"reset_interval":    stringPtrValue(item.ResetInterval),
			"allowed_models":    allowedModels,
			"allowed_providers": allowedProviders,
			"ignored_models":    ignoredModels,
			"ignored_providers": ignoredProviders,
			"enforce_zdr":       boolPtrValue(item.EnforceZDR),
			"created_at":        types.StringValue(item.CreatedAt),
			"updated_at":        stringPtrValue(item.UpdatedAt),
		})
		resp.Diagnostics.Append(diags...)
		objects = append(objects, object)
	}
	listValue, diags := types.ListValue(types.ObjectType{AttrTypes: itemType}, objects)
	resp.Diagnostics.Append(diags...)
	state := guardrailsDataSourceModel{TotalCount: types.Int64Value(int64(len(items))), Items: listValue}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
