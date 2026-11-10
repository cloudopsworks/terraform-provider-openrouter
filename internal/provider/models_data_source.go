package provider

import (
	"context"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	ds "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/cloudopsworks/terraform-provider-openrouter/internal/client"
)

var _ datasource.DataSourceWithValidateConfig = (*modelsDataSource)(nil)

type modelsDataSource struct{ client *client.Client }

func NewModelsDataSource() datasource.DataSource { return &modelsDataSource{} }
func (d *modelsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_models"
}
func (d *modelsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	pd, err := configureClient(req.ProviderData)
	if err != nil {
		resp.Diagnostics.AddError("Unable to configure OpenRouter models data source", err.Error())
		return
	}
	d.client = pd.client
}
func (d *modelsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = ds.Schema{MarkdownDescription: "Read one bounded model catalog page, sorted by returned ID for stable state. Pagination is explicit: offset/limit select one server-side page; total_count and has_more indicate further pages. API ordering and catalog membership can change between refreshes. Defaults to text-output models unless output_modalities is set; use all to include all modalities.", Attributes: map[string]ds.Attribute{
		"offset": ds.Int64Attribute{Optional: true, Computed: true, MarkdownDescription: "Records to skip; default 0."}, "limit": ds.Int64Attribute{Optional: true, Computed: true, MarkdownDescription: "Page size 1–1000; default 100. Never automatically fetches all models."},
		"search": ds.StringAttribute{Optional: true, MarkdownDescription: "Free-text model name or slug search sent as q."}, "input_modalities": ds.StringAttribute{Optional: true, MarkdownDescription: "Comma-separated input modality filter."}, "output_modalities": ds.StringAttribute{Optional: true, MarkdownDescription: "Comma-separated output modalities, or all; API default is text."},
		"total_count": ds.Int64Attribute{Computed: true, MarkdownDescription: "Total catalog matches, not the number in this page."}, "has_more": ds.BoolAttribute{Computed: true},
		"items": ds.ListNestedAttribute{Computed: true, NestedObject: ds.NestedAttributeObject{Attributes: modelMetadataAttributes()}},
	}}
}
func (d *modelsDataSource) ValidateConfig(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	for _, name := range []string{"offset", "limit"} {
		var n types.Int64
		resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root(name), &n)...)
		if n.IsNull() || n.IsUnknown() {
			continue
		}
		if (name == "offset" && n.ValueInt64() < 0) || (name == "limit" && (n.ValueInt64() < 1 || n.ValueInt64() > 1000)) {
			resp.Diagnostics.AddAttributeError(path.Root(name), "Invalid model catalog page", "offset must be nonnegative and limit must be 1..1000.")
		}
	}
}
func (d *modelsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var offset, limit types.Int64
	var search, input, output types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("offset"), &offset)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("limit"), &limit)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("search"), &search)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("input_modalities"), &input)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("output_modalities"), &output)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if offset.IsNull() {
		offset = types.Int64Value(0)
	}
	if limit.IsNull() {
		limit = types.Int64Value(100)
	}
	// Validate in int64 space before converting to the platform-sized int.
	if offset.IsUnknown() || limit.IsUnknown() || offset.ValueInt64() < 0 || limit.ValueInt64() < 1 || limit.ValueInt64() > 1000 || int64(int(offset.ValueInt64())) != offset.ValueInt64() {
		resp.Diagnostics.AddError("Invalid model catalog page", "offset must be nonnegative and limit must be 1..1000.")
		return
	}
	page, err := d.client.GetModelsPage(ctx, client.ModelsPageOptions{Offset: int(offset.ValueInt64()), Limit: int(limit.ValueInt64()), Search: search.ValueString(), InputModalities: input.ValueString(), OutputModalities: output.ValueString()})
	if err != nil {
		resp.Diagnostics.AddError("Unable to read OpenRouter model catalog", err.Error())
		return
	}
	sort.Slice(page.Data, func(i, j int) bool { return page.Data[i].ID < page.Data[j].ID })
	objects := make([]attr.Value, 0, len(page.Data))
	itemTypes := modelMetadataTypes()
	for _, model := range page.Data {
		values, diags := modelMetadataValues(ctx, model)
		resp.Diagnostics.Append(diags...)
		object, diags := types.ObjectValue(itemTypes, values)
		resp.Diagnostics.Append(diags...)
		objects = append(objects, object)
	}
	items, diags := types.ListValue(types.ObjectType{AttrTypes: itemTypes}, objects)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.State.Raw = req.Config.Raw
	for k, v := range map[string]attr.Value{"offset": offset, "limit": limit, "total_count": types.Int64Value(*page.TotalCount), "has_more": types.BoolValue(page.Links.Next != nil && *page.Links.Next != ""), "items": items} {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(k), v)...)
	}
}
