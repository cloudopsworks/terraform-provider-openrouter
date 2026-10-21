package provider

import (
	"context"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/cloudopsworks/terraform-provider-openrouter/internal/client"
)

var (
	_ resource.Resource                = &byokKeyResource{}
	_ resource.ResourceWithImportState = &byokKeyResource{}
)

type byokKeyResource struct{ client *client.Client }

type byokKeyResourceModel struct {
	ID                  types.String `tfsdk:"id"`
	KeyWO               types.String `tfsdk:"key_wo"`
	KeyWOVersion        types.Int64  `tfsdk:"key_wo_version"`
	ProviderSlug        types.String `tfsdk:"provider_slug"`
	WorkspaceID         types.String `tfsdk:"workspace_id"`
	Name                types.String `tfsdk:"name"`
	DeclaredZDR         types.Bool   `tfsdk:"declared_zdr"`
	AllowedModels       types.Set    `tfsdk:"allowed_models"`
	AllowedAPIKeyHashes types.Set    `tfsdk:"allowed_api_key_hashes"`
	AllowedUserIDs      types.Set    `tfsdk:"allowed_user_ids"`
	Disabled            types.Bool   `tfsdk:"disabled"`
	IsFallback          types.Bool   `tfsdk:"is_fallback"`
	IsRequired          types.Bool   `tfsdk:"is_required"`
	IsBYOKOnly          types.Bool   `tfsdk:"is_byok_only"`
	Label               types.String `tfsdk:"label"`
	SortOrder           types.Int64  `tfsdk:"sort_order"`
	CreatedAt           types.String `tfsdk:"created_at"`
}

func NewBYOKKeyResource() resource.Resource { return &byokKeyResource{} }
func (r *byokKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_byok_key"
}
func (r *byokKeyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	data, err := configureClient(req.ProviderData)
	if err != nil {
		resp.Diagnostics.AddError("Unable to configure OpenRouter BYOK key resource", err.Error())
		return
	}
	r.client = data.client
}
func (r *byokKeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{MarkdownDescription: "Manage a write-only OpenRouter BYOK provider credential.", Attributes: map[string]resourceschema.Attribute{
		"id":                     resourceschema.StringAttribute{Computed: true},
		"key_wo":                 resourceschema.StringAttribute{Optional: true, WriteOnly: true, Sensitive: true, MarkdownDescription: "Raw provider credential. Write-only and never stored in Terraform state."},
		"key_wo_version":         resourceschema.Int64Attribute{Optional: true, Computed: true, MarkdownDescription: "Increment to rotate key_wo. The raw credential is sent only when this version changes.", PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}},
		"provider_slug":          resourceschema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
		"workspace_id":           resourceschema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown(), stringplanmodifier.RequiresReplace()}},
		"name":                   resourceschema.StringAttribute{Optional: true},
		"declared_zdr":           resourceschema.BoolAttribute{Optional: true},
		"allowed_models":         byokStringSetAttribute("Optional model allowlist; null permits every model."),
		"allowed_api_key_hashes": byokStringSetAttribute("Optional API key hash allowlist; null permits every key."),
		"allowed_user_ids":       byokStringSetAttribute("Optional user ID allowlist; null permits every user."),
		"disabled":               resourceschema.BoolAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}},
		"is_fallback":            resourceschema.BoolAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}},
		"is_required":            resourceschema.BoolAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}},
		"is_byok_only":           resourceschema.BoolAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}},
		"label":                  resourceschema.StringAttribute{Computed: true}, "sort_order": resourceschema.Int64Attribute{Computed: true}, "created_at": resourceschema.StringAttribute{Computed: true},
	}}
}
func byokStringSetAttribute(description string) resourceschema.SetAttribute {
	return resourceschema.SetAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: description, PlanModifiers: []planmodifier.Set{setplanmodifier.UseStateForUnknown()}}
}

func (r *byokKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan, config byokKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(validateBYOKModel(ctx, plan, config, true)...)
	if resp.Diagnostics.HasError() {
		return
	}
	key := config.KeyWO.ValueString()
	created, err := r.client.CreateBYOKCredential(ctx, client.BYOKCreateRequest{Key: key, Provider: plan.ProviderSlug.ValueString(), WorkspaceID: stringValueOrNil(plan.WorkspaceID), Name: byokOptionalString(config.Name), DeclaredZDR: byokOptionalBool(config.DeclaredZDR), AllowedModels: byokOptionalStringSlice(ctx, config.AllowedModels), AllowedAPIKeyHashes: byokOptionalStringSlice(ctx, config.AllowedAPIKeyHashes), AllowedUserIDs: byokOptionalStringSlice(ctx, config.AllowedUserIDs), Disabled: boolValueOrNil(plan.Disabled), IsFallback: boolValueOrNil(plan.IsFallback), IsRequired: boolValueOrNil(plan.IsRequired), IsBYOKOnly: boolValueOrNil(plan.IsBYOKOnly)})
	if err == nil {
		if created == nil || !byokUUIDPattern.MatchString(created.ID) {
			resp.Diagnostics.AddError("Unable to create OpenRouter BYOK credential", "create response did not contain a valid credential UUID")
			return
		}
		state, diags := flattenBYOKKeyModel(ctx, *created, plan)
		resp.Diagnostics.Append(diags...)
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		return
	}
	// A malformed 2xx response can mean the server created the credential but
	// omitted metadata. Never flatten that partial object into state.
	if created != nil && byokUUIDPattern.MatchString(created.ID) {
		if recovered, getErr := r.client.GetBYOKCredential(ctx, created.ID); getErr == nil && recovered.ID == created.ID {
			state, diags := flattenBYOKKeyModel(ctx, *recovered, plan)
			resp.Diagnostics.Append(diags...)
			resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
			return
		}
		// Preserve only the syntactically valid identity returned by the create
		// response. The diagnostic remains, and a later refresh can recover it.
		state := byokIdentityState(created.ID, plan.KeyWOVersion)
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	}
	resp.Diagnostics.AddError("Unable to create OpenRouter BYOK credential", err.Error())
}
func (r *byokKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state byokKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.client.GetBYOKCredential(ctx, state.ID.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "(404)") {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read OpenRouter BYOK credential", err.Error())
		return
	}
	if got.ID != state.ID.ValueString() {
		resp.Diagnostics.AddError("Unable to read OpenRouter BYOK credential", "read response credential ID does not match state")
		return
	}
	updated, diags := flattenBYOKKeyModel(ctx, *got, state)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &updated)...)
}
func (r *byokKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, config, state byokKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(validateBYOKModel(ctx, plan, config, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	update := client.BYOKUpdateRequest{}
	if !plan.Name.Equal(state.Name) {
		update.Name = byokOptionalString(config.Name)
	}
	if !plan.DeclaredZDR.Equal(state.DeclaredZDR) {
		update.DeclaredZDR = byokOptionalBool(config.DeclaredZDR)
	}
	if !plan.AllowedModels.Equal(state.AllowedModels) {
		update.AllowedModels = byokOptionalStringSlice(ctx, config.AllowedModels)
	}
	if !plan.AllowedAPIKeyHashes.Equal(state.AllowedAPIKeyHashes) {
		update.AllowedAPIKeyHashes = byokOptionalStringSlice(ctx, config.AllowedAPIKeyHashes)
	}
	if !plan.AllowedUserIDs.Equal(state.AllowedUserIDs) {
		update.AllowedUserIDs = byokOptionalStringSlice(ctx, config.AllowedUserIDs)
	}
	if !plan.Disabled.Equal(state.Disabled) {
		update.Disabled = boolValueOrNil(plan.Disabled)
	}
	if !plan.IsFallback.Equal(state.IsFallback) {
		update.IsFallback = boolValueOrNil(plan.IsFallback)
	}
	if !plan.IsRequired.Equal(state.IsRequired) {
		update.IsRequired = boolValueOrNil(plan.IsRequired)
	}
	if !plan.IsBYOKOnly.Equal(state.IsBYOKOnly) {
		update.IsBYOKOnly = boolValueOrNil(plan.IsBYOKOnly)
	}
	if !plan.KeyWOVersion.Equal(state.KeyWOVersion) {
		if config.KeyWO.IsNull() || config.KeyWO.IsUnknown() || strings.TrimSpace(config.KeyWO.ValueString()) == "" {
			resp.Diagnostics.AddAttributeError(path.Root("key_wo"), "Missing BYOK credential key", "key_wo must be set whenever key_wo_version changes.")
			return
		}
		key := config.KeyWO.ValueString()
		update.Key = &key
	}
	updated, err := r.client.UpdateBYOKCredential(ctx, state.ID.ValueString(), update)
	if err == nil && updated != nil && updated.ID == state.ID.ValueString() {
		newState, diags := flattenBYOKKeyModel(ctx, *updated, plan)
		resp.Diagnostics.Append(diags...)
		resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
		return
	}
	// An ordinary PATCH failure is not recoverable: a GET would mask the failed operation.
	if updated == nil && err != nil {
		resp.Diagnostics.AddError("Unable to update OpenRouter BYOK credential", err.Error())
		return
	}
	// Do not let a decoded malformed or mismatched response alter identity.
	if recovered, getErr := r.client.GetBYOKCredential(ctx, state.ID.ValueString()); getErr == nil && recovered.ID == state.ID.ValueString() {
		newState, diags := flattenBYOKKeyModel(ctx, *recovered, plan)
		resp.Diagnostics.Append(diags...)
		resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
		return
	} else if getErr != nil {
		resp.Diagnostics.AddError("Unable to refresh OpenRouter BYOK credential after update", getErr.Error())
		return
	}
	resp.Diagnostics.AddError("Unable to refresh OpenRouter BYOK credential after update", "read response credential ID does not match state")
}
func (r *byokKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state byokKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteBYOKCredential(ctx, state.ID.ValueString()); err != nil {
		if strings.Contains(err.Error(), "(404)") {
			return
		}
		resp.Diagnostics.AddError("Unable to delete OpenRouter BYOK credential", err.Error())
		return
	}
}
func byokIdentityState(id string, version types.Int64) byokKeyResourceModel {
	return byokKeyResourceModel{
		ID: types.StringValue(id), KeyWO: types.StringNull(), KeyWOVersion: version,
		Name: types.StringNull(), DeclaredZDR: types.BoolNull(),
		AllowedModels: types.SetNull(types.StringType), AllowedAPIKeyHashes: types.SetNull(types.StringType), AllowedUserIDs: types.SetNull(types.StringType),
	}
}
func (r *byokKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if !byokUUIDPattern.MatchString(req.ID) {
		resp.Diagnostics.AddError("Unable to import OpenRouter BYOK credential", "expected a credential UUID")
		return
	}
	state := byokIdentityState(req.ID, types.Int64Value(0))
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func flattenBYOKKeyModel(ctx context.Context, in client.BYOKCredential, previous byokKeyResourceModel) (byokKeyResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	models, d := types.SetValueFrom(ctx, types.StringType, in.AllowedModels)
	diags.Append(d...)
	hashes, d := types.SetValueFrom(ctx, types.StringType, in.AllowedAPIKeyHashes)
	diags.Append(d...)
	users, d := types.SetValueFrom(ctx, types.StringType, in.AllowedUserIDs)
	diags.Append(d...)
	state := byokKeyResourceModel{ID: types.StringValue(in.ID), KeyWO: types.StringNull(), KeyWOVersion: previous.KeyWOVersion, ProviderSlug: types.StringValue(in.Provider), WorkspaceID: types.StringValue(in.WorkspaceID), Name: stringPtrValue(in.Name), DeclaredZDR: boolPtrValue(in.DeclaredZDR), AllowedModels: models, AllowedAPIKeyHashes: hashes, AllowedUserIDs: users, Disabled: types.BoolValue(in.Disabled), IsFallback: types.BoolValue(in.IsFallback), IsRequired: types.BoolValue(in.IsRequired), IsBYOKOnly: types.BoolValue(in.IsBYOKOnly), Label: types.StringValue(in.Label), SortOrder: types.Int64Value(int64(in.SortOrder)), CreatedAt: types.StringValue(in.CreatedAt)}
	if state.KeyWOVersion.IsNull() || state.KeyWOVersion.IsUnknown() {
		state.KeyWOVersion = types.Int64Value(0)
	}
	return state, diags
}
func byokOptionalBool(value types.Bool) *client.BYOKOptionalBool {
	if value.IsNull() || value.IsUnknown() {
		return client.BYOKNullBool()
	}
	return client.BYOKBool(value.ValueBool())
}

func byokOptionalString(value types.String) *client.BYOKOptionalString {
	if value.IsNull() || value.IsUnknown() {
		return client.BYOKNullString()
	}
	return client.BYOKString(value.ValueString())
}
func byokOptionalStringSlice(ctx context.Context, value types.Set) *client.BYOKOptionalStringSlice {
	if value.IsNull() || value.IsUnknown() {
		return client.BYOKNullStringSlice()
	}
	var values []string
	if value.ElementsAs(ctx, &values, false).HasError() {
		return nil
	}
	return client.BYOKStringSlice(values)
}
func validateBYOKModel(ctx context.Context, plan, config byokKeyResourceModel, creating bool) diag.Diagnostics {
	var diags diag.Diagnostics
	if creating && (config.KeyWO.IsNull() || config.KeyWO.IsUnknown() || strings.TrimSpace(config.KeyWO.ValueString()) == "") {
		diags.AddAttributeError(path.Root("key_wo"), "Missing BYOK credential key", "key_wo must be set when creating a BYOK credential.")
	}
	if !plan.ProviderSlug.IsUnknown() && !byokProviderPattern.MatchString(plan.ProviderSlug.ValueString()) {
		diags.AddAttributeError(path.Root("provider_slug"), "Invalid provider", "provider must be a lowercase provider slug.")
	}
	if !plan.WorkspaceID.IsNull() && !plan.WorkspaceID.IsUnknown() && !byokUUIDPattern.MatchString(plan.WorkspaceID.ValueString()) {
		diags.AddAttributeError(path.Root("workspace_id"), "Invalid workspace ID", "workspace_id must be a UUID when provided.")
	}
	if !plan.KeyWOVersion.IsNull() && !plan.KeyWOVersion.IsUnknown() && plan.KeyWOVersion.ValueInt64() < 0 {
		diags.AddAttributeError(path.Root("key_wo_version"), "Invalid BYOK key version", "key_wo_version must not be negative.")
	}
	if !config.Name.IsNull() && !config.Name.IsUnknown() && len(config.Name.ValueString()) > 255 {
		diags.AddAttributeError(path.Root("name"), "Invalid BYOK credential name", "name must not exceed 255 characters.")
	}
	if !plan.IsFallback.IsUnknown() && !plan.IsBYOKOnly.IsUnknown() && plan.IsFallback.ValueBool() && plan.IsBYOKOnly.ValueBool() {
		diags.AddAttributeError(path.Root("is_byok_only"), "Invalid BYOK mode", "is_byok_only cannot be combined with is_fallback.")
	}
	validateBYOKLists(ctx, config, &diags)
	return diags
}

var (
	byokHashPattern     = regexp.MustCompile(`^[a-f0-9]{64}$`)
	byokProviderPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9/-]*$`)
	byokUUIDPattern     = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

func validateBYOKLists(ctx context.Context, m byokKeyResourceModel, diags *diag.Diagnostics) {
	for _, entry := range []struct {
		name  string
		value types.Set
		empty bool
		hash  bool
	}{{"allowed_models", m.AllowedModels, true, false}, {"allowed_api_key_hashes", m.AllowedAPIKeyHashes, false, true}, {"allowed_user_ids", m.AllowedUserIDs, true, false}} {
		if entry.value.IsNull() || entry.value.IsUnknown() {
			continue
		}
		var values []string
		d := entry.value.ElementsAs(ctx, &values, false)
		diags.Append(d...)
		if len(values) > 100 || (!entry.empty && len(values) == 0) {
			diags.AddAttributeError(path.Root(entry.name), "Invalid BYOK allowlist", "The allowlist must contain the permitted number of values.")
		}
		for _, v := range values {
			if strings.TrimSpace(v) == "" {
				diags.AddAttributeError(path.Root(entry.name), "Invalid BYOK allowlist", "Allowlist entries must not be empty.")
			}
			if entry.hash && !byokHashPattern.MatchString(v) {
				diags.AddAttributeError(path.Root(entry.name), "Invalid API key hash", "allowed_api_key_hashes entries must be lowercase SHA-256 hashes.")
			}
		}
	}
}
