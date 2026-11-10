package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	ds "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/cloudopsworks/terraform-provider-openrouter/internal/client"
)

var _ datasource.DataSourceWithValidateConfig = (*modelDataSource)(nil)

type modelDataSource struct{ client *client.Client }

func NewModelDataSource() datasource.DataSource { return &modelDataSource{} }
func (d *modelDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_model"
}
func (d *modelDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	pd, err := configureClient(req.ProviderData)
	if err != nil {
		resp.Diagnostics.AddError("Unable to configure OpenRouter model data source", err.Error())
		return
	}
	d.client = pd.client
}
func (d *modelDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := modelMetadataAttributes()
	attrs["slug"] = ds.StringAttribute{Required: true, MarkdownDescription: "Model lookup slug in author/model form, optionally including a variant suffix. Known aliases resolve to the returned id."}
	resp.Schema = ds.Schema{MarkdownDescription: "Read public metadata and base pricing for one model without inference. Availability and pricing are refresh-time snapshots, not cost enforcement guarantees. Conditional price overrides are not represented.", Attributes: attrs}
}
func (d *modelDataSource) ValidateConfig(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	var slug types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("slug"), &slug)...)
	if slug.IsNull() || slug.IsUnknown() {
		return
	}
	if _, _, err := client.SplitModelSlug(slug.ValueString()); err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("slug"), "Invalid model slug", err.Error())
	}
}
func (d *modelDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var slug types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("slug"), &slug)...)
	if resp.Diagnostics.HasError() {
		return
	}
	model, err := d.client.GetModel(ctx, slug.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read OpenRouter model", err.Error())
		return
	}
	values, diags := modelMetadataValues(ctx, *model)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.State.Raw = req.Config.Raw
	for k, v := range values {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(k), v)...)
	}
}

func modelMetadataAttributes() map[string]ds.Attribute {
	return map[string]ds.Attribute{
		"id":                   ds.StringAttribute{Computed: true, MarkdownDescription: "Resolved model ID; aliases may return a different ID from the requested slug."},
		"canonical_slug":       ds.StringAttribute{Computed: true},
		"name":                 ds.StringAttribute{Computed: true},
		"description":          ds.StringAttribute{Computed: true},
		"created":              ds.Int64Attribute{Computed: true, MarkdownDescription: "Model creation Unix timestamp."},
		"context_length":       ds.Int64Attribute{Computed: true},
		"expiration_date":      ds.StringAttribute{Computed: true},
		"knowledge_cutoff":     ds.StringAttribute{Computed: true},
		"supported_parameters": ds.SetAttribute{Computed: true, ElementType: types.StringType},
		"input_modalities":     ds.SetAttribute{Computed: true, ElementType: types.StringType},
		"output_modalities":    ds.SetAttribute{Computed: true, ElementType: types.StringType},
		"pricing":              ds.MapAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Base price decimal strings as returned by OpenRouter: prompt/completion/cache/reasoning/audio prices are USD per token, image/image_output are USD per image, request is USD per request and web_search is USD per search. Not USD per million tokens. Conditional overrides and discounts are not included."},
	}
}
func modelMetadataTypes() map[string]attr.Type {
	result := map[string]attr.Type{}
	for k, v := range modelMetadataAttributes() {
		result[k] = v.GetType()
	}
	return result
}
func modelMetadataValues(ctx context.Context, m client.Model) (map[string]attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics
	parameters, d := types.SetValueFrom(ctx, types.StringType, m.SupportedParameters)
	diags.Append(d...)
	input, d := types.SetValueFrom(ctx, types.StringType, m.Architecture.InputModalities)
	diags.Append(d...)
	output, d := types.SetValueFrom(ctx, types.StringType, m.Architecture.OutputModalities)
	diags.Append(d...)
	pricing, d := types.MapValueFrom(ctx, types.StringType, m.Pricing.BasePrices())
	diags.Append(d...)
	contextLength := types.Int64Null()
	if m.ContextLength != nil {
		contextLength = types.Int64Value(*m.ContextLength)
	}
	return map[string]attr.Value{
		"id":                   types.StringValue(m.ID),
		"canonical_slug":       types.StringValue(m.CanonicalSlug),
		"name":                 types.StringValue(m.Name),
		"description":          stringPtrValue(m.Description),
		"created":              types.Int64Value(*m.Created),
		"context_length":       contextLength,
		"expiration_date":      stringPtrValue(m.ExpirationDate),
		"knowledge_cutoff":     stringPtrValue(m.KnowledgeCutoff),
		"supported_parameters": parameters,
		"input_modalities":     input,
		"output_modalities":    output,
		"pricing":              pricing,
	}, diags
}
