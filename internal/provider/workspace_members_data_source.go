package provider

import (
	"context"

	"github.com/cloudopsworks/terraform-provider-openrouter/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type workspaceMembersDataSource struct{ client *client.Client }
type workspaceMembersDataSourceModel struct {
	WorkspaceID types.String `tfsdk:"workspace_id"`
	TotalCount  types.Int64  `tfsdk:"total_count"`
	Items       types.List   `tfsdk:"items"`
}

func NewWorkspaceMembersDataSource() datasource.DataSource { return &workspaceMembersDataSource{} }
func (d *workspaceMembersDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workspace_members"
}
func (d *workspaceMembersDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	data, err := configureClient(req.ProviderData)
	if err != nil {
		resp.Diagnostics.AddError("Unable to configure workspace members data source", err.Error())
		return
	}
	d.client = data.client
}
func (d *workspaceMembersDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasourceschema.Schema{MarkdownDescription: "Read all workspace membership pages. Supply the stable workspace UUID, not a slug. Default-workspace results include implicit organization membership; the API does not expose membership provenance. This source never adds/removes members or changes inherited roles.", Attributes: map[string]datasourceschema.Attribute{
		"workspace_id": datasourceschema.StringAttribute{Required: true}, "total_count": datasourceschema.Int64Attribute{Computed: true},
		"items": datasourceschema.ListNestedAttribute{Computed: true, NestedObject: datasourceschema.NestedAttributeObject{Attributes: map[string]datasourceschema.Attribute{
			"id": datasourceschema.StringAttribute{Computed: true}, "workspace_id": datasourceschema.StringAttribute{Computed: true}, "user_id": datasourceschema.StringAttribute{Computed: true}, "role": datasourceschema.StringAttribute{Computed: true}, "created_at": datasourceschema.StringAttribute{Computed: true},
		}}},
	}}
}
func (d *workspaceMembersDataSource) ValidateConfig(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	var v types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("workspace_id"), &v)...)
	if v.IsNull() || v.IsUnknown() {
		return
	}
	if !identityConfigValue(v) {
		resp.Diagnostics.AddAttributeError(path.Root("workspace_id"), "Invalid workspace identifier", "Supply a nonempty workspace UUID without slashes or surrounding whitespace.")
	}
}
func (d *workspaceMembersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state workspaceMembersDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	members, err := d.client.ListWorkspaceMembers(ctx, state.WorkspaceID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to list workspace members", err.Error())
		return
	}
	attrs := map[string]attr.Type{"id": types.StringType, "workspace_id": types.StringType, "user_id": types.StringType, "role": types.StringType, "created_at": types.StringType}
	values := make([]attr.Value, 0, len(members))
	for _, m := range members {
		object, diags := types.ObjectValue(attrs, map[string]attr.Value{"id": types.StringValue(m.ID), "workspace_id": types.StringValue(m.WorkspaceID), "user_id": types.StringValue(m.UserID), "role": types.StringValue(m.Role), "created_at": types.StringValue(m.CreatedAt)})
		resp.Diagnostics.Append(diags...)
		values = append(values, object)
	}
	items, diags := types.ListValue(types.ObjectType{AttrTypes: attrs}, values)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Items = items
	state.TotalCount = types.Int64Value(int64(len(members)))
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
