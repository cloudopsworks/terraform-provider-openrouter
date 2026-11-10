package provider

import (
	"context"

	"github.com/cloudopsworks/terraform-provider-openrouter/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type scimGroupsDataSource struct{ client *client.Client }
type scimGroupsDataSourceModel struct {
	DisplayName types.String `tfsdk:"display_name"`
	ExternalID  types.String `tfsdk:"external_id"`
	TotalCount  types.Int64  `tfsdk:"total_count"`
	Items       types.List   `tfsdk:"items"`
}

func NewSCIMGroupsDataSource() datasource.DataSource { return &scimGroupsDataSource{} }
func (d *scimGroupsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_scim_groups"
}
func (d *scimGroupsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	data, err := configureClient(req.ProviderData)
	if err != nil {
		resp.Diagnostics.AddError("Unable to configure SCIM groups data source", err.Error())
		return
	}
	d.client = data.client
}
func (d *scimGroupsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasourceschema.Schema{MarkdownDescription: "Read IdP-provisioned SCIM groups, including all pages. Filters are exact-match and may return multiple groups; use stable group IDs for mapping ownership. Requires Enterprise SCIM provisioning; this source does not trigger synchronization or provision identities.", Attributes: map[string]datasourceschema.Attribute{
		"display_name": datasourceschema.StringAttribute{Optional: true},
		"external_id":  datasourceschema.StringAttribute{Optional: true},
		"total_count":  datasourceschema.Int64Attribute{Computed: true},
		"items": datasourceschema.ListNestedAttribute{Computed: true, NestedObject: datasourceschema.NestedAttributeObject{Attributes: map[string]datasourceschema.Attribute{
			"id": datasourceschema.StringAttribute{Computed: true}, "organization_id": datasourceschema.StringAttribute{Computed: true}, "external_id": datasourceschema.StringAttribute{Computed: true}, "display_name": datasourceschema.StringAttribute{Computed: true}, "created_at": datasourceschema.StringAttribute{Computed: true}, "updated_at": datasourceschema.StringAttribute{Computed: true},
		}}},
	}}
}
func (d *scimGroupsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state scimGroupsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	groups, err := d.client.ListSCIMGroups(ctx, state.DisplayName.ValueString(), state.ExternalID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to list SCIM groups", err.Error())
		return
	}
	attrs := map[string]attr.Type{"id": types.StringType, "organization_id": types.StringType, "external_id": types.StringType, "display_name": types.StringType, "created_at": types.StringType, "updated_at": types.StringType}
	values := make([]attr.Value, 0, len(groups))
	for _, g := range groups {
		object, diags := types.ObjectValue(attrs, map[string]attr.Value{"id": types.StringValue(g.ID), "organization_id": types.StringValue(g.OrganizationID), "external_id": stringPtrValue(g.ExternalID), "display_name": types.StringValue(g.DisplayName), "created_at": types.StringValue(g.CreatedAt), "updated_at": types.StringValue(g.UpdatedAt)})
		resp.Diagnostics.Append(diags...)
		values = append(values, object)
	}
	items, diagnostics := types.ListValue(types.ObjectType{AttrTypes: attrs}, values)
	resp.Diagnostics.Append(diagnostics...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Items = items
	state.TotalCount = types.Int64Value(int64(len(groups)))
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
