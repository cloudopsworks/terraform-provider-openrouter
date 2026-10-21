package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/cloudopsworks/terraform-provider-openrouter/internal/client"
)

var _ datasource.DataSource = &byokKeysDataSource{}

type byokKeysDataSource struct{ client *client.Client }
type byokKeysDataSourceModel struct {
	WorkspaceID  types.String `tfsdk:"workspace_id"`
	ProviderSlug types.String `tfsdk:"provider_slug"`
	TotalCount   types.Int64  `tfsdk:"total_count"`
	Items        types.List   `tfsdk:"items"`
}

func NewBYOKKeysDataSource() datasource.DataSource { return &byokKeysDataSource{} }
func (d *byokKeysDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_byok_keys"
}
func (d *byokKeysDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	data, err := configureClient(req.ProviderData)
	if err != nil {
		resp.Diagnostics.AddError("Unable to configure OpenRouter BYOK keys data source", err.Error())
		return
	}
	d.client = data.client
}
func (d *byokKeysDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasourceschema.Schema{MarkdownDescription: "List OpenRouter BYOK provider credentials. Raw provider keys are never returned.", Attributes: map[string]datasourceschema.Attribute{
		"workspace_id": datasourceschema.StringAttribute{Optional: true}, "provider_slug": datasourceschema.StringAttribute{Optional: true}, "total_count": datasourceschema.Int64Attribute{Computed: true},
		"items": datasourceschema.ListNestedAttribute{Computed: true, NestedObject: datasourceschema.NestedAttributeObject{Attributes: map[string]datasourceschema.Attribute{
			"id": datasourceschema.StringAttribute{Computed: true}, "provider_slug": datasourceschema.StringAttribute{Computed: true}, "workspace_id": datasourceschema.StringAttribute{Computed: true}, "name": datasourceschema.StringAttribute{Computed: true}, "label": datasourceschema.StringAttribute{Computed: true}, "declared_zdr": datasourceschema.BoolAttribute{Computed: true}, "disabled": datasourceschema.BoolAttribute{Computed: true}, "is_fallback": datasourceschema.BoolAttribute{Computed: true}, "is_required": datasourceschema.BoolAttribute{Computed: true}, "is_byok_only": datasourceschema.BoolAttribute{Computed: true}, "allowed_models": datasourceschema.ListAttribute{Computed: true, ElementType: types.StringType}, "allowed_api_key_hashes": datasourceschema.ListAttribute{Computed: true, ElementType: types.StringType}, "allowed_user_ids": datasourceschema.ListAttribute{Computed: true, ElementType: types.StringType}, "sort_order": datasourceschema.Int64Attribute{Computed: true}, "created_at": datasourceschema.StringAttribute{Computed: true},
		}}},
	}}
}
func (d *byokKeysDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config byokKeysDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	items, err := d.client.ListBYOKCredentials(ctx, stringValueOrNil(config.WorkspaceID), stringValueOrNil(config.ProviderSlug))
	if err != nil {
		resp.Diagnostics.AddError("Unable to list OpenRouter BYOK credentials", err.Error())
		return
	}
	itemType := byokDataSourceItemTypes()
	objects := make([]attr.Value, 0, len(items))
	for _, item := range items {
		models, ds := types.ListValueFrom(ctx, types.StringType, item.AllowedModels)
		resp.Diagnostics.Append(ds...)
		hashes, ds := types.ListValueFrom(ctx, types.StringType, item.AllowedAPIKeyHashes)
		resp.Diagnostics.Append(ds...)
		users, ds := types.ListValueFrom(ctx, types.StringType, item.AllowedUserIDs)
		resp.Diagnostics.Append(ds...)
		object, ds := types.ObjectValue(itemType, map[string]attr.Value{"id": types.StringValue(item.ID), "provider_slug": types.StringValue(item.Provider), "workspace_id": types.StringValue(item.WorkspaceID), "name": stringPtrValue(item.Name), "label": types.StringValue(item.Label), "declared_zdr": boolPtrValue(item.DeclaredZDR), "disabled": types.BoolValue(item.Disabled), "is_fallback": types.BoolValue(item.IsFallback), "is_required": types.BoolValue(item.IsRequired), "is_byok_only": types.BoolValue(item.IsBYOKOnly), "allowed_models": models, "allowed_api_key_hashes": hashes, "allowed_user_ids": users, "sort_order": types.Int64Value(int64(item.SortOrder)), "created_at": types.StringValue(item.CreatedAt)})
		resp.Diagnostics.Append(ds...)
		objects = append(objects, object)
	}
	list, ds := types.ListValue(types.ObjectType{AttrTypes: itemType}, objects)
	resp.Diagnostics.Append(ds...)
	if resp.Diagnostics.HasError() {
		return
	}
	state := byokKeysDataSourceModel{WorkspaceID: config.WorkspaceID, ProviderSlug: config.ProviderSlug, TotalCount: types.Int64Value(int64(len(items))), Items: list}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
func byokDataSourceItemTypes() map[string]attr.Type {
	return map[string]attr.Type{"id": types.StringType, "provider_slug": types.StringType, "workspace_id": types.StringType, "name": types.StringType, "label": types.StringType, "declared_zdr": types.BoolType, "disabled": types.BoolType, "is_fallback": types.BoolType, "is_required": types.BoolType, "is_byok_only": types.BoolType, "allowed_models": types.ListType{ElemType: types.StringType}, "allowed_api_key_hashes": types.ListType{ElemType: types.StringType}, "allowed_user_ids": types.ListType{ElemType: types.StringType}, "sort_order": types.Int64Type, "created_at": types.StringType}
}
