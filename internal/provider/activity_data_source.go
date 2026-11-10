package provider

import (
	"context"
	"math/big"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	ds "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/cloudopsworks/terraform-provider-openrouter/internal/client"
)

var _ datasource.DataSourceWithValidateConfig = (*activityDataSource)(nil)

type activityDataSource struct{ client *client.Client }

func NewActivityDataSource() datasource.DataSource { return &activityDataSource{} }
func (d *activityDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_activity"
}
func (d *activityDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	pd, err := configureClient(req.ProviderData)
	if err != nil {
		resp.Diagnostics.AddError("Unable to configure OpenRouter activity data source", err.Error())
		return
	}
	d.client = pd.client
}
func (d *activityDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = ds.Schema{MarkdownDescription: "Read an explicit UTC day's endpoint usage snapshot; requires a management key. OpenRouter retains the last 30 completed UTC days. Reads age out and can change; do not use this data source as a billing enforcement control. No inference or prompt/completion contents are read.", Attributes: map[string]ds.Attribute{
		"date":         ds.StringAttribute{Required: true, MarkdownDescription: "Single completed UTC date in YYYY-MM-DD format. The API enforces its rolling retention window."},
		"workspace_id": ds.StringAttribute{Optional: true, MarkdownDescription: "Only activity attributed to this workspace. Historical pre-resolution records remain in the account default workspace."}, "api_key_hash": ds.StringAttribute{Optional: true, MarkdownDescription: "Filter by key hash, not a secret API key."}, "user_id": ds.StringAttribute{Optional: true},
		"group_by_workspace": ds.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "Split rows by workspace and include workspace_id; default false."},
		"max_items":          ds.Int64Attribute{Optional: true, Computed: true, MarkdownDescription: "Provider-side state bound, 1–10000; default 1000. Exceeding it returns an error, never silently truncates. The activity API does not provide pagination."},
		"total_count":        ds.Int64Attribute{Computed: true, MarkdownDescription: "Number of rows for the requested snapshot; not an account-wide usage total."},
		"items":              ds.ListNestedAttribute{Computed: true, NestedObject: ds.NestedAttributeObject{Attributes: activityItemAttributes()}},
	}}
}
func activityItemAttributes() map[string]ds.Attribute {
	result := map[string]ds.Attribute{}
	for _, name := range []string{"date", "model", "model_permaslug", "endpoint_id", "provider_name", "workspace_id"} {
		result[name] = ds.StringAttribute{Computed: true}
	}
	for _, name := range []string{"requests", "prompt_tokens", "completion_tokens", "reasoning_tokens", "cached_tokens"} {
		result[name] = ds.Int64Attribute{Computed: true}
	}
	result["usage"] = ds.NumberAttribute{Computed: true, MarkdownDescription: "OpenRouter credits spent, USD."}
	result["byok_usage_inference"] = ds.NumberAttribute{Computed: true, MarkdownDescription: "External BYOK inference spend, USD."}
	return result
}
func (d *activityDataSource) ValidateConfig(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	var date types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("date"), &date)...)
	if !date.IsNull() && !date.IsUnknown() {
		if err := client.ValidateActivityDate(date.ValueString()); err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("date"), "Invalid activity date", err.Error())
		}
	}
	for _, name := range []string{"workspace_id", "api_key_hash", "user_id"} {
		var value types.String
		resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root(name), &value)...)
		if !value.IsNull() && !value.IsUnknown() && strings.TrimSpace(value.ValueString()) == "" {
			resp.Diagnostics.AddAttributeError(path.Root(name), "Invalid activity filter", "An explicitly configured filter must not be empty.")
		}
	}
	var max types.Int64
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("max_items"), &max)...)
	if !max.IsNull() && !max.IsUnknown() && (max.ValueInt64() < 1 || max.ValueInt64() > 10000) {
		resp.Diagnostics.AddAttributeError(path.Root("max_items"), "Invalid activity state bound", "max_items must be 1..10000.")
	}
}
func (d *activityDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var date, workspace, key, user types.String
	var group types.Bool
	var max types.Int64
	for name, target := range map[string]*types.String{"date": &date, "workspace_id": &workspace, "api_key_hash": &key, "user_id": &user} {
		resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root(name), target)...)
	}
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("group_by_workspace"), &group)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("max_items"), &max)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if group.IsNull() {
		group = types.BoolValue(false)
	}
	if max.IsNull() {
		max = types.Int64Value(1000)
	}
	if group.IsUnknown() || max.IsUnknown() || max.ValueInt64() < 1 || max.ValueInt64() > 10000 {
		resp.Diagnostics.AddError("Invalid activity request", "group_by_workspace must be known and max_items must be 1..10000.")
		return
	}
	items, err := d.client.GetActivity(ctx, client.ActivityOptions{Date: date.ValueString(), WorkspaceID: workspace.ValueString(), APIKeyHash: key.ValueString(), UserID: user.ValueString(), GroupByWorkspace: group.ValueBool(), MaxItems: int(max.ValueInt64())})
	if err != nil {
		resp.Diagnostics.AddError("Unable to read OpenRouter activity", err.Error())
		return
	}
	itemTypes := map[string]attr.Type{}
	for k, v := range activityItemAttributes() {
		itemTypes[k] = v.GetType()
	}
	objects := make([]attr.Value, 0, len(items))
	for _, item := range items {
		usage, _, err := big.ParseFloat(item.Usage.String(), 10, 512, big.ToNearestEven)
		if err != nil {
			resp.Diagnostics.AddError("Invalid activity spend", "Unable to decode usage as a Terraform number.")
			return
		}
		byok, _, err := big.ParseFloat(item.BYOKUsageInference.String(), 10, 512, big.ToNearestEven)
		if err != nil {
			resp.Diagnostics.AddError("Invalid activity spend", "Unable to decode BYOK usage as a Terraform number.")
			return
		}
		values := map[string]attr.Value{
			"date":                 types.StringValue(item.Date),
			"model":                types.StringValue(item.Model),
			"model_permaslug":      types.StringValue(item.ModelPermaslug),
			"endpoint_id":          types.StringValue(item.EndpointID),
			"provider_name":        types.StringValue(item.ProviderName),
			"workspace_id":         stringPtrValue(item.WorkspaceID),
			"usage":                types.NumberValue(usage),
			"byok_usage_inference": types.NumberValue(byok),
			"requests":             types.Int64Value(*item.Requests),
			"prompt_tokens":        types.Int64Value(*item.PromptTokens),
			"completion_tokens":    types.Int64Value(*item.CompletionTokens),
			"reasoning_tokens":     types.Int64Value(*item.ReasoningTokens),
			"cached_tokens":        types.Int64Value(*item.CachedTokens),
		}
		object, diags := types.ObjectValue(itemTypes, values)
		resp.Diagnostics.Append(diags...)
		objects = append(objects, object)
	}
	list, diags := types.ListValue(types.ObjectType{AttrTypes: itemTypes}, objects)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.State.Raw = req.Config.Raw
	for k, v := range map[string]attr.Value{"group_by_workspace": group, "max_items": max, "total_count": types.Int64Value(int64(len(items))), "items": list} {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(k), v)...)
	}
}
