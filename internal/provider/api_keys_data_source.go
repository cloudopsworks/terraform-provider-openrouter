package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/cloudopsworks/terraform-provider-openrouter/internal/client"
)

var _ datasource.DataSource = &apiKeysDataSource{}

type apiKeysDataSource struct {
	client *client.Client
}

type apiKeysDataSourceModel struct {
	WorkspaceID     types.String `tfsdk:"workspace_id"`
	IncludeDisabled types.Bool   `tfsdk:"include_disabled"`
	TotalCount      types.Int64  `tfsdk:"total_count"`
	Items           types.List   `tfsdk:"items"`
}

func NewAPIKeysDataSource() datasource.DataSource {
	return &apiKeysDataSource{}
}

func (d *apiKeysDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_api_keys"
}

func (d *apiKeysDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	providerData, err := configureClient(req.ProviderData)
	if err != nil {
		resp.Diagnostics.AddError("Unable to configure OpenRouter API keys data source", err.Error())
		return
	}
	d.client = providerData.client
}

func (d *apiKeysDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasourceschema.Schema{
		MarkdownDescription: "List OpenRouter API keys.",
		Attributes: map[string]datasourceschema.Attribute{
			"workspace_id":     datasourceschema.StringAttribute{Optional: true},
			"include_disabled": datasourceschema.BoolAttribute{Optional: true},
			"total_count":      datasourceschema.Int64Attribute{Computed: true},
			"items": datasourceschema.ListNestedAttribute{
				Computed: true,
				NestedObject: datasourceschema.NestedAttributeObject{Attributes: map[string]datasourceschema.Attribute{
					"id":                    datasourceschema.StringAttribute{Computed: true},
					"name":                  datasourceschema.StringAttribute{Computed: true},
					"workspace_id":          datasourceschema.StringAttribute{Computed: true},
					"label":                 datasourceschema.StringAttribute{Computed: true},
					"disabled":              datasourceschema.BoolAttribute{Computed: true},
					"limit":                 datasourceschema.Float64Attribute{Computed: true},
					"limit_remaining":       datasourceschema.Float64Attribute{Computed: true},
					"limit_reset":           datasourceschema.StringAttribute{Computed: true},
					"include_byok_in_limit": datasourceschema.BoolAttribute{Computed: true},
					"expires_at":            datasourceschema.StringAttribute{Computed: true},
					"creator_user_id":       datasourceschema.StringAttribute{Computed: true},
					"created_at":            datasourceschema.StringAttribute{Computed: true},
					"updated_at":            datasourceschema.StringAttribute{Computed: true},
				}},
			},
		},
	}
}

func (d *apiKeysDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config apiKeysDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	workspaceID := stringValueOrNil(config.WorkspaceID)
	includeDisabled := false
	if !config.IncludeDisabled.IsNull() && !config.IncludeDisabled.IsUnknown() {
		includeDisabled = config.IncludeDisabled.ValueBool()
	}

	items, err := d.client.ListAPIKeys(ctx, workspaceID, includeDisabled)
	if err != nil {
		resp.Diagnostics.AddError("Unable to list OpenRouter API keys", err.Error())
		return
	}

	itemType := map[string]attr.Type{
		"id":                    types.StringType,
		"name":                  types.StringType,
		"workspace_id":          types.StringType,
		"label":                 types.StringType,
		"disabled":              types.BoolType,
		"limit":                 types.Float64Type,
		"limit_remaining":       types.Float64Type,
		"limit_reset":           types.StringType,
		"include_byok_in_limit": types.BoolType,
		"expires_at":            types.StringType,
		"creator_user_id":       types.StringType,
		"created_at":            types.StringType,
		"updated_at":            types.StringType,
	}
	objects := make([]attr.Value, 0, len(items))
	for _, item := range items {
		object, diags := types.ObjectValue(itemType, map[string]attr.Value{
			"id":                    types.StringValue(item.Hash),
			"name":                  types.StringValue(item.Name),
			"workspace_id":          stringPtrValue(item.WorkspaceID),
			"label":                 types.StringValue(item.Label),
			"disabled":              types.BoolValue(item.Disabled),
			"limit":                 float64PtrValue(item.Limit),
			"limit_remaining":       float64PtrValue(item.LimitRemaining),
			"limit_reset":           stringPtrValue(item.LimitReset),
			"include_byok_in_limit": types.BoolValue(item.IncludeBYOKInLimit),
			"expires_at":            stringPtrValue(item.ExpiresAt),
			"creator_user_id":       stringPtrValue(item.CreatorUserID),
			"created_at":            types.StringValue(item.CreatedAt),
			"updated_at":            types.StringValue(item.UpdatedAt),
		})
		resp.Diagnostics.Append(diags...)
		objects = append(objects, object)
	}
	listValue, diags := types.ListValue(types.ObjectType{AttrTypes: itemType}, objects)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state := apiKeysDataSourceModel{
		WorkspaceID:     config.WorkspaceID,
		IncludeDisabled: config.IncludeDisabled,
		TotalCount:      types.Int64Value(int64(len(items))),
		Items:           listValue,
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
