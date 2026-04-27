package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/cloudopsworks/terraform-provider-openrouter/internal/client"
)

var _ datasource.DataSource = &organizationDataSource{}

type organizationDataSource struct{ client *client.Client }

type organizationDataSourceModel struct {
	TotalCount types.Int64 `tfsdk:"total_count"`
	Members    types.List  `tfsdk:"members"`
}

func NewOrganizationDataSource() datasource.DataSource { return &organizationDataSource{} }

func (d *organizationDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization"
}

func (d *organizationDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	providerData, err := configureClient(req.ProviderData)
	if err != nil {
		resp.Diagnostics.AddError("Unable to configure OpenRouter organization data source", err.Error())
		return
	}
	d.client = providerData.client
}

func (d *organizationDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasourceschema.Schema{MarkdownDescription: "Return organization membership information using OpenRouter's organization members endpoint.", Attributes: map[string]datasourceschema.Attribute{
		"total_count": datasourceschema.Int64Attribute{Computed: true},
		"members": datasourceschema.ListNestedAttribute{Computed: true, NestedObject: datasourceschema.NestedAttributeObject{Attributes: map[string]datasourceschema.Attribute{
			"id":         datasourceschema.StringAttribute{Computed: true},
			"first_name": datasourceschema.StringAttribute{Computed: true},
			"last_name":  datasourceschema.StringAttribute{Computed: true},
			"email":      datasourceschema.StringAttribute{Computed: true},
			"role":       datasourceschema.StringAttribute{Computed: true},
		}}},
	}}
}

func (d *organizationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	members, err := d.client.ListOrganizationMembers(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to list OpenRouter organization members", err.Error())
		return
	}
	itemType := map[string]attr.Type{"id": types.StringType, "first_name": types.StringType, "last_name": types.StringType, "email": types.StringType, "role": types.StringType}
	objects := make([]attr.Value, 0, len(members))
	for _, item := range members {
		object, diags := types.ObjectValue(itemType, map[string]attr.Value{
			"id":         types.StringValue(item.ID),
			"first_name": types.StringValue(item.FirstName),
			"last_name":  types.StringValue(item.LastName),
			"email":      types.StringValue(item.Email),
			"role":       types.StringValue(item.Role),
		})
		resp.Diagnostics.Append(diags...)
		objects = append(objects, object)
	}
	listValue, diags := types.ListValue(types.ObjectType{AttrTypes: itemType}, objects)
	resp.Diagnostics.Append(diags...)
	state := organizationDataSourceModel{TotalCount: types.Int64Value(int64(len(members))), Members: listValue}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
