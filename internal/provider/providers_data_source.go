package provider

import (
	"context"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/cloudopsworks/terraform-provider-openrouter/internal/client"
)

var _ datasource.DataSource = &providersDataSource{}

type providersDataSource struct{ client *client.Client }

type providersDataSourceModel struct {
	TotalCount types.Int64 `tfsdk:"total_count"`
	Items      types.List  `tfsdk:"items"`
}

func NewProvidersDataSource() datasource.DataSource { return &providersDataSource{} }

func (d *providersDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_providers"
}

func (d *providersDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	providerData, err := configureClient(req.ProviderData)
	if err != nil {
		resp.Diagnostics.AddError("Unable to configure OpenRouter providers data source", err.Error())
		return
	}
	d.client = providerData.client
}

func (d *providersDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasourceschema.Schema{Attributes: map[string]datasourceschema.Attribute{
		"total_count": datasourceschema.Int64Attribute{Computed: true},
		"items": datasourceschema.ListNestedAttribute{Computed: true, NestedObject: datasourceschema.NestedAttributeObject{Attributes: map[string]datasourceschema.Attribute{
			"slug":                     datasourceschema.StringAttribute{Computed: true},
			"name":                     datasourceschema.StringAttribute{Computed: true},
			"status":                   datasourceschema.StringAttribute{Computed: true},
			"description":              datasourceschema.StringAttribute{Computed: true},
			"privacy_policy_url":       datasourceschema.StringAttribute{Computed: true},
			"terms_of_service_url":     datasourceschema.StringAttribute{Computed: true},
			"status_page_url":          datasourceschema.StringAttribute{Computed: true},
			"headquarters":             datasourceschema.StringAttribute{Computed: true},
			"datacenters":              datasourceschema.SetAttribute{Computed: true, ElementType: types.StringType},
			"moderated":                datasourceschema.BoolAttribute{Computed: true},
			"supports_tool_call":       datasourceschema.BoolAttribute{Computed: true},
			"supports_reasoning":       datasourceschema.BoolAttribute{Computed: true},
			"supports_multimodal":      datasourceschema.BoolAttribute{Computed: true},
			"supports_response_schema": datasourceschema.BoolAttribute{Computed: true},
		}}},
	}}
}

func (d *providersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	items, err := d.client.ListProviders(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to list OpenRouter providers", err.Error())
		return
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Slug < items[j].Slug })
	itemType := map[string]attr.Type{
		"slug":                     types.StringType,
		"name":                     types.StringType,
		"status":                   types.StringType,
		"description":              types.StringType,
		"privacy_policy_url":       types.StringType,
		"terms_of_service_url":     types.StringType,
		"status_page_url":          types.StringType,
		"headquarters":             types.StringType,
		"datacenters":              types.SetType{ElemType: types.StringType},
		"moderated":                types.BoolType,
		"supports_tool_call":       types.BoolType,
		"supports_reasoning":       types.BoolType,
		"supports_multimodal":      types.BoolType,
		"supports_response_schema": types.BoolType,
	}
	objects := make([]attr.Value, 0, len(items))
	for _, item := range items {
		datacenters, diags := setStringValueOrNull(ctx, item.Datacenters)
		resp.Diagnostics.Append(diags...)
		object, diags := types.ObjectValue(itemType, map[string]attr.Value{
			"slug":                     types.StringValue(item.Slug),
			"name":                     types.StringValue(item.Name),
			"status":                   stringPtrValue(item.Status),
			"description":              stringPtrValue(item.Description),
			"privacy_policy_url":       stringPtrValue(item.PrivacyPolicyURL),
			"terms_of_service_url":     stringPtrValue(item.TermsOfServiceURL),
			"status_page_url":          stringPtrValue(item.StatusPageURL),
			"headquarters":             stringPtrValue(item.Headquarters),
			"datacenters":              datacenters,
			"moderated":                boolPtrValue(item.Moderated),
			"supports_tool_call":       boolPtrValue(item.SupportsToolCall),
			"supports_reasoning":       boolPtrValue(item.SupportsReasoning),
			"supports_multimodal":      boolPtrValue(item.SupportsMultimodal),
			"supports_response_schema": boolPtrValue(item.SupportsResponseSchema),
		})
		resp.Diagnostics.Append(diags...)
		objects = append(objects, object)
	}
	listValue, diags := types.ListValue(types.ObjectType{AttrTypes: itemType}, objects)
	resp.Diagnostics.Append(diags...)
	state := providersDataSourceModel{TotalCount: types.Int64Value(int64(len(items))), Items: listValue}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
