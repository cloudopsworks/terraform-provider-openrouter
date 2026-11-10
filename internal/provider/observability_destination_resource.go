package provider

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/cloudopsworks/terraform-provider-openrouter/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	schema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/float64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                   = &observabilityDestinationResource{}
	_ resource.ResourceWithImportState    = &observabilityDestinationResource{}
	_ resource.ResourceWithValidateConfig = &observabilityDestinationResource{}
)

type observabilityDestinationResource struct{ client *client.Client }
type observabilityDestinationModel struct {
	ID                                types.String  `tfsdk:"id"`
	WorkspaceID                       types.String  `tfsdk:"workspace_id"`
	Type                              types.String  `tfsdk:"type"`
	Name                              types.String  `tfsdk:"name"`
	Enabled                           types.Bool    `tfsdk:"enabled"`
	PrivacyMode                       types.Bool    `tfsdk:"privacy_mode"`
	SamplingRate                      types.Float64 `tfsdk:"sampling_rate"`
	Regions                           types.Set     `tfsdk:"regions"`
	APIKeyHashes                      types.Set     `tfsdk:"api_key_hashes"`
	BroadcastGenerationCost           types.Bool    `tfsdk:"broadcast_generation_cost"`
	BroadcastGenerationIdentity       types.Bool    `tfsdk:"broadcast_generation_identity"`
	BroadcastGenerationRequestContext types.Bool    `tfsdk:"broadcast_generation_request_context"`
	ConfigWO                          types.String  `tfsdk:"config_wo"`
	ConfigWORevision                  types.Int64   `tfsdk:"config_wo_revision"`
	CreatedAt                         types.String  `tfsdk:"created_at"`
	UpdatedAt                         types.String  `tfsdk:"updated_at"`
}

func NewObservabilityDestinationResource() resource.Resource {
	return &observabilityDestinationResource{}
}
func (r *observabilityDestinationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_observability_destination"
}
func (r *observabilityDestinationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	data, err := configureClient(req.ProviderData)
	if err != nil {
		resp.Diagnostics.AddError("Unable to configure observability destination", err.Error())
		return
	}
	r.client = data.client
}
func (r *observabilityDestinationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	boolAttr := func(value bool) schema.BoolAttribute {
		return schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(value)}
	}
	regions, _ := types.SetValueFrom(context.Background(), types.StringType, []string{"global"})
	immutable := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{MarkdownDescription: "Manage a destination with typed metadata and revision-triggered write-only configuration. Returned provider configuration is never stored; omitted configuration fields are preserved remotely. Filter rules and beta cost-allocation settings are not owned.", Attributes: map[string]schema.Attribute{
		"id":           schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		"workspace_id": schema.StringAttribute{Required: true, PlanModifiers: immutable},
		"type":         schema.StringAttribute{Required: true, PlanModifiers: immutable},
		"name":         schema.StringAttribute{Required: true},
		"enabled":      boolAttr(false), "privacy_mode": boolAttr(true),
		"sampling_rate":             schema.Float64Attribute{Optional: true, Computed: true, Default: float64default.StaticFloat64(1)},
		"regions":                   schema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType, Default: setdefault.StaticValue(regions)},
		"api_key_hashes":            schema.SetAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: "Null forwards all API keys. Removing a configured set sends JSON null; an explicit set must be nonempty."},
		"broadcast_generation_cost": boolAttr(false), "broadcast_generation_identity": boolAttr(false), "broadcast_generation_request_context": boolAttr(false),
		"config_wo":          schema.StringAttribute{Optional: true, Sensitive: true, WriteOnly: true, MarkdownDescription: "Write-only provider-specific JSON object. Required for creation and changed revisions. PATCH merges fields; omission does not delete remote keys."},
		"config_wo_revision": schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(0), MarkdownDescription: "Positive revision required at creation. Change to another positive revision with config_wo to submit a configuration merge. Imported revision is zero."},
		"created_at":         schema.StringAttribute{Computed: true}, "updated_at": schema.StringAttribute{Computed: true},
	}}
}
func (r *observabilityDestinationResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config observabilityDestinationModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(validateObservabilityConfig(ctx, config)...)
}

var observabilityTypes = map[string]bool{"arize": true, "braintrust": true, "clickhouse": true, "datadog": true, "grafana": true, "langfuse": true, "langsmith": true, "newrelic": true, "opik": true, "otel-collector": true, "posthog": true, "ramp": true, "s3": true, "sentry": true, "snowflake": true, "weave": true, "webhook": true}

func validateObservabilityConfig(ctx context.Context, m observabilityDestinationModel) diag.Diagnostics {
	var d diag.Diagnostics
	if !m.WorkspaceID.IsUnknown() && !m.WorkspaceID.IsNull() && !byokUUIDPattern.MatchString(m.WorkspaceID.ValueString()) {
		d.AddAttributeError(path.Root("workspace_id"), "Invalid workspace", "workspace_id must be a UUID.")
	}
	if !m.Type.IsUnknown() && !m.Type.IsNull() && !observabilityTypes[m.Type.ValueString()] {
		d.AddAttributeError(path.Root("type"), "Unsupported destination type", "Use a documented stable observability destination type.")
	}
	if !m.Name.IsUnknown() && !m.Name.IsNull() && strings.TrimSpace(m.Name.ValueString()) == "" {
		d.AddAttributeError(path.Root("name"), "Invalid destination name", "name must not be empty.")
	}
	if !m.SamplingRate.IsUnknown() && !m.SamplingRate.IsNull() && (m.SamplingRate.ValueFloat64() < 0.0001 || m.SamplingRate.ValueFloat64() > 1) {
		d.AddAttributeError(path.Root("sampling_rate"), "Invalid sampling rate", "sampling_rate must be between 0.0001 and 1.")
	}
	if !m.ConfigWORevision.IsUnknown() && !m.ConfigWORevision.IsNull() && m.ConfigWORevision.ValueInt64() < 0 {
		d.AddAttributeError(path.Root("config_wo_revision"), "Invalid configuration revision", "config_wo_revision must not be negative.")
	}
	for _, entry := range []struct {
		name  string
		value types.Set
	}{{"regions", m.Regions}, {"api_key_hashes", m.APIKeyHashes}} {
		if entry.value.IsUnknown() || entry.value.IsNull() {
			continue
		}
		var values []types.String
		d.Append(entry.value.ElementsAs(ctx, &values, false)...)
		if len(values) == 0 {
			d.AddAttributeError(path.Root(entry.name), "Empty destination set", "A configured set must not be empty.")
		}
		for _, v := range values {
			if v.IsUnknown() {
				continue
			}
			if entry.name == "regions" {
				if v.IsNull() || (v.ValueString() != "global" && v.ValueString() != "europe" && v.ValueString() != "us") {
					d.AddAttributeError(path.Root(entry.name), "Invalid data region", "Use canonical regions global, europe or us.")
				}
				continue
			}
			if v.IsNull() || !byokHashPattern.MatchString(v.ValueString()) {
				d.AddAttributeError(path.Root(entry.name), "Invalid API key hash", "Each key hash must be a lowercase SHA-256 hash.")
			}
		}
	}
	if m.ConfigWO.IsNull() || m.ConfigWO.IsUnknown() {
		return d
	}
	var object map[string]any
	if err := json.Unmarshal([]byte(m.ConfigWO.ValueString()), &object); err != nil || object == nil {
		d.AddAttributeError(path.Root("config_wo"), "Invalid destination configuration", "config_wo must contain a JSON object.")
		return d
	}
	if !validObservabilityValues(object) {
		d.AddAttributeError(path.Root("config_wo"), "Invalid destination configuration", "Configuration must not contain nulls; field deletion is not supported.")
	}
	// The API documents these Langfuse examples, not a universal mask syntax.
	// Do not classify arbitrary header/URL strings by ellipses or asterisks.
	if m.Type.ValueString() == "langfuse" && (object["publicKey"] == "pk-l...EfGh" || object["secretKey"] == "sk-l...AbCd") {
		d.AddAttributeError(path.Root("config_wo"), "Masked destination credential", "Supply an actual credential, not a documented Langfuse response placeholder.")
	}
	if m.Type.ValueString() == "s3" && object["authMode"] == "iam_role" {
		d.AddAttributeError(path.Root("config_wo"), "Unsupported S3 onboarding", "S3 IAM-role onboarding requires an external-ID handoff that this write-only resource does not expose.")
	}
	return d
}
func validObservabilityValues(value any) bool {
	switch v := value.(type) {
	case nil:
		return false
	case map[string]any:
		for _, item := range v {
			if !validObservabilityValues(item) {
				return false
			}
		}
	case []any:
		for _, item := range v {
			if !validObservabilityValues(item) {
				return false
			}
		}
	}
	return true
}
func observabilityWrite(ctx context.Context, m observabilityDestinationModel) (client.ObservabilityDestinationWrite, diag.Diagnostics) {
	out := client.ObservabilityDestinationWrite{
		WorkspaceID:                       m.WorkspaceID.ValueString(),
		Type:                              m.Type.ValueString(),
		Name:                              m.Name.ValueString(),
		Enabled:                           m.Enabled.ValueBool(),
		PrivacyMode:                       m.PrivacyMode.ValueBool(),
		SamplingRate:                      m.SamplingRate.ValueFloat64(),
		BroadcastGenerationCost:           m.BroadcastGenerationCost.ValueBool(),
		BroadcastGenerationIdentity:       m.BroadcastGenerationIdentity.ValueBool(),
		BroadcastGenerationRequestContext: m.BroadcastGenerationRequestContext.ValueBool(),
	}
	var d diag.Diagnostics
	d.Append(m.Regions.ElementsAs(ctx, &out.Regions, false)...)
	if !m.APIKeyHashes.IsNull() {
		d.Append(m.APIKeyHashes.ElementsAs(ctx, &out.APIKeyHashes, false)...)
	}
	return out, d
}
func (r *observabilityDestinationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan, config observabilityDestinationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(validateObservabilityConfig(ctx, config)...)
	if config.ConfigWO.IsNull() || config.ConfigWO.IsUnknown() || plan.ConfigWORevision.ValueInt64() < 1 {
		resp.Diagnostics.AddError("Missing destination configuration", "Creation requires config_wo and a positive config_wo_revision.")
	}
	if resp.Diagnostics.HasError() {
		return
	}
	body, d := observabilityWrite(ctx, plan)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	body.Config = json.RawMessage(config.ConfigWO.ValueString())
	got, err := r.client.CreateObservabilityDestination(ctx, body)
	if err != nil {
		if got != nil && byokUUIDPattern.MatchString(got.ID) && got.WorkspaceID == plan.WorkspaceID.ValueString() && got.Type == plan.Type.ValueString() {
			// The write succeeded but metadata was malformed. Retain only the
			// trusted identity so the created object stays tracked for reconciliation.
			recovery := observabilityDestinationModel{
				ID: types.StringValue(got.ID), WorkspaceID: plan.WorkspaceID, Type: plan.Type,
				Name: types.StringNull(), Enabled: types.BoolNull(), PrivacyMode: types.BoolNull(),
				SamplingRate: types.Float64Null(), Regions: types.SetNull(types.StringType), APIKeyHashes: types.SetNull(types.StringType),
				BroadcastGenerationCost: types.BoolNull(), BroadcastGenerationIdentity: types.BoolNull(), BroadcastGenerationRequestContext: types.BoolNull(),
				ConfigWO: types.StringNull(), ConfigWORevision: plan.ConfigWORevision,
				CreatedAt: types.StringNull(), UpdatedAt: types.StringNull(),
			}
			resp.Diagnostics.Append(resp.State.Set(ctx, &recovery)...)
		} else if got != nil {
			resp.Diagnostics.AddError("Untrusted destination identity", "Creation returned an invalid identity. Manual reconciliation may be required; no returned identity was adopted.")
		}
		resp.Diagnostics.AddError("Unable to create observability destination", err.Error())
		return
	}
	if !byokUUIDPattern.MatchString(got.ID) || got.WorkspaceID != plan.WorkspaceID.ValueString() || got.Type != plan.Type.ValueString() {
		resp.Diagnostics.AddError("Invalid destination identity", "Creation returned an unexpected destination identity. Manual reconciliation may be required; no returned identity was adopted.")
		return
	}
	state, d := flattenObservability(ctx, *got, plan.ConfigWORevision)
	resp.Diagnostics.Append(d...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
func (r *observabilityDestinationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state observabilityDestinationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.client.GetObservabilityDestination(ctx, state.ID.ValueString())
	if err != nil {
		if observabilityNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read observability destination", err.Error())
		return
	}
	if !observabilityIdentity(*got, state) {
		resp.Diagnostics.AddError("Invalid destination identity", "Read returned unexpected workspace or destination type.")
		return
	}
	next, d := flattenObservability(ctx, *got, state.ConfigWORevision)
	resp.Diagnostics.Append(d...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &next)...)
}
func (r *observabilityDestinationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, config, state observabilityDestinationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(validateObservabilityConfig(ctx, config)...)
	configChanged := !plan.ConfigWORevision.Equal(state.ConfigWORevision)
	if configChanged && (plan.ConfigWORevision.ValueInt64() < 1 || config.ConfigWO.IsUnknown() || config.ConfigWO.IsNull()) {
		resp.Diagnostics.AddError("Missing destination configuration", "A changed revision must be positive and include config_wo.")
	}
	if resp.Diagnostics.HasError() {
		return
	}
	current, err := r.client.GetObservabilityDestination(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to verify observability destination", err.Error())
		return
	}
	if !observabilityIdentity(*current, state) {
		resp.Diagnostics.AddError("Invalid destination identity", "Refusing to update unexpected workspace or destination type.")
		return
	}
	body, d := observabilityWrite(ctx, plan)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	if configChanged {
		body.Config = json.RawMessage(config.ConfigWO.ValueString())
	}
	got, err := r.client.UpdateObservabilityDestination(ctx, state.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Unable to update observability destination", err.Error())
		return
	}
	if !observabilityIdentity(*got, state) {
		resp.Diagnostics.AddError("Invalid destination identity", "Update returned unexpected workspace or destination type.")
		return
	}
	next, d := flattenObservability(ctx, *got, plan.ConfigWORevision)
	resp.Diagnostics.Append(d...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &next)...)
}
func (r *observabilityDestinationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state observabilityDestinationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.client.GetObservabilityDestination(ctx, state.ID.ValueString())
	if err != nil {
		if observabilityNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Unable to verify observability destination", err.Error())
		return
	}
	if !observabilityIdentity(*got, state) {
		resp.Diagnostics.AddError("Invalid destination identity", "Refusing to delete unexpected workspace or destination type.")
		return
	}
	if err := r.client.DeleteObservabilityDestination(ctx, state.ID.ValueString()); err != nil && !observabilityNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete observability destination", err.Error())
	}
}
func (r *observabilityDestinationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if !byokUUIDPattern.MatchString(req.ID) {
		resp.Diagnostics.AddError("Invalid destination import", "Expected a destination UUID.")
		return
	}
	got, err := r.client.GetObservabilityDestination(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Unable to import observability destination", err.Error())
		return
	}
	state, d := flattenObservability(ctx, *got, types.Int64Value(0))
	resp.Diagnostics.Append(d...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
func observabilityIdentity(got client.ObservabilityDestination, state observabilityDestinationModel) bool {
	return got.ID == state.ID.ValueString() && got.WorkspaceID == state.WorkspaceID.ValueString() && got.Type == state.Type.ValueString()
}
func observabilityNotFound(err error) bool {
	return strings.HasPrefix(err.Error(), "openrouter API error (404)")
}
func flattenObservability(ctx context.Context, in client.ObservabilityDestination, revision types.Int64) (observabilityDestinationModel, diag.Diagnostics) {
	var d diag.Diagnostics
	regions, ds := types.SetValueFrom(ctx, types.StringType, in.Regions)
	d.Append(ds...)
	hashes, ds := types.SetValueFrom(ctx, types.StringType, in.APIKeyHashes)
	d.Append(ds...)
	return observabilityDestinationModel{
		ID:                                types.StringValue(in.ID),
		WorkspaceID:                       types.StringValue(in.WorkspaceID),
		Type:                              types.StringValue(in.Type),
		Name:                              stringPtrValue(in.Name),
		Enabled:                           types.BoolValue(in.Enabled),
		PrivacyMode:                       types.BoolValue(in.PrivacyMode),
		SamplingRate:                      types.Float64Value(in.SamplingRate),
		Regions:                           regions,
		APIKeyHashes:                      hashes,
		BroadcastGenerationCost:           types.BoolValue(in.BroadcastGenerationCost),
		BroadcastGenerationIdentity:       types.BoolValue(in.BroadcastGenerationIdentity),
		BroadcastGenerationRequestContext: types.BoolValue(in.BroadcastGenerationRequestContext),
		ConfigWO:                          types.StringNull(),
		ConfigWORevision:                  revision,
		CreatedAt:                         types.StringValue(in.CreatedAt),
		UpdatedAt:                         types.StringValue(in.UpdatedAt),
	}, d
}
