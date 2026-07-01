package provider

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/cloudopsworks/terraform-provider-openrouter/internal/client"
)

var (
	_ resource.Resource                = &guardrailResource{}
	_ resource.ResourceWithImportState = &guardrailResource{}
)

type guardrailResource struct {
	client *client.Client
}

type guardrailResourceModel struct {
	ID                    types.String  `tfsdk:"id"`
	Name                  types.String  `tfsdk:"name"`
	WorkspaceID           types.String  `tfsdk:"workspace_id"`
	Description           types.String  `tfsdk:"description"`
	LimitUSD              types.Float64 `tfsdk:"limit_usd"`
	ResetInterval         types.String  `tfsdk:"reset_interval"`
	AllowedModels         types.Set     `tfsdk:"allowed_models"`
	AllowedProviders      types.Set     `tfsdk:"allowed_providers"`
	IgnoredModels         types.Set     `tfsdk:"ignored_models"`
	IgnoredProviders      types.Set     `tfsdk:"ignored_providers"`
	ContentFilterBuiltins types.List    `tfsdk:"content_filter_builtins"`
	ContentFilters        types.List    `tfsdk:"content_filters"`
	EnforceZDR            types.Bool    `tfsdk:"enforce_zdr"`
	EnforceZDRAnthropic   types.Bool    `tfsdk:"enforce_zdr_anthropic"`
	EnforceZDRGoogle      types.Bool    `tfsdk:"enforce_zdr_google"`
	EnforceZDROpenAI      types.Bool    `tfsdk:"enforce_zdr_openai"`
	EnforceZDROther       types.Bool    `tfsdk:"enforce_zdr_other"`
	CreatedAt             types.String  `tfsdk:"created_at"`
	UpdatedAt             types.String  `tfsdk:"updated_at"`
}

type contentFilterBuiltinModel struct {
	Slug      types.String `tfsdk:"slug"`
	Action    types.String `tfsdk:"action"`
	Label     types.String `tfsdk:"label"`
	ScanScope types.String `tfsdk:"scan_scope"`
}

type contentFilterModel struct {
	Pattern types.String `tfsdk:"pattern"`
	Action  types.String `tfsdk:"action"`
	Label   types.String `tfsdk:"label"`
}

var (
	contentFilterBuiltinAttrTypes = map[string]attr.Type{
		"slug":       types.StringType,
		"action":     types.StringType,
		"label":      types.StringType,
		"scan_scope": types.StringType,
	}
	contentFilterBuiltinObjectType = types.ObjectType{AttrTypes: contentFilterBuiltinAttrTypes}
	contentFilterAttrTypes         = map[string]attr.Type{
		"pattern": types.StringType,
		"action":  types.StringType,
		"label":   types.StringType,
	}
	contentFilterObjectType = types.ObjectType{AttrTypes: contentFilterAttrTypes}
)

func NewGuardrailResource() resource.Resource {
	return &guardrailResource{}
}

func (r *guardrailResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_guardrail"
}

func (r *guardrailResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	providerData, err := configureClient(req.ProviderData)
	if err != nil {
		resp.Diagnostics.AddError("Unable to configure OpenRouter guardrail resource", err.Error())
		return
	}
	r.client = providerData.client
}

func (r *guardrailResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		MarkdownDescription: "Manage OpenRouter guardrails.",
		Attributes: map[string]resourceschema.Attribute{
			"id":   resourceschema.StringAttribute{Computed: true},
			"name": resourceschema.StringAttribute{Required: true},
			"workspace_id": resourceschema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Workspace UUID for the guardrail. If omitted, OpenRouter assigns the guardrail to the default workspace. Replacement when changed.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description":       resourceschema.StringAttribute{Optional: true, Computed: true},
			"limit_usd":         resourceschema.Float64Attribute{Optional: true, Computed: true},
			"reset_interval":    resourceschema.StringAttribute{Optional: true, Computed: true},
			"allowed_models":    resourceschema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType},
			"allowed_providers": resourceschema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType},
			"ignored_models":    resourceschema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType},
			"ignored_providers": resourceschema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType},
			"content_filter_builtins": resourceschema.ListNestedAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Builtin content filters applied to requests. Known slugs include `email`, `phone`, `ssn`, `credit-card`, `ip-address`, `person-name`, `address`, and `regex-prompt-injection`. OpenRouter may add more slugs.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
				NestedObject: resourceschema.NestedAttributeObject{Attributes: map[string]resourceschema.Attribute{
					"slug": resourceschema.StringAttribute{
						Required:            true,
						MarkdownDescription: "Builtin filter slug. Known values include `email`, `phone`, `ssn`, `credit-card`, `ip-address`, `person-name`, `address`, and `regex-prompt-injection`; unknown future OpenRouter values are accepted.",
					},
					"action": resourceschema.StringAttribute{
						Required:            true,
						MarkdownDescription: "Action to take when the builtin filter triggers. Known values are `redact`, `block`, and `flag`; unknown future OpenRouter values are accepted.",
					},
					"label": resourceschema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Read-only, system-assigned label returned by OpenRouter. This value is not configurable.",
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
						},
					},
					"scan_scope": resourceschema.StringAttribute{
						Optional:            true,
						Computed:            true,
						MarkdownDescription: "Prompt-injection scan scope. Known values are `user_only` and `all_messages`; unknown future OpenRouter values are accepted.",
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
						},
					},
				}},
			},
			"content_filters": resourceschema.ListNestedAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Custom regex content filters applied to request messages.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
				NestedObject: resourceschema.NestedAttributeObject{Attributes: map[string]resourceschema.Attribute{
					"pattern": resourceschema.StringAttribute{
						Required:            true,
						MarkdownDescription: "Regex pattern matched against request content.",
					},
					"action": resourceschema.StringAttribute{
						Required:            true,
						MarkdownDescription: "Action to take when the pattern matches. Known values are `redact` and `block`; unknown future OpenRouter values are accepted.",
					},
					"label": resourceschema.StringAttribute{
						Optional:            true,
						Computed:            true,
						MarkdownDescription: "Optional label used in redaction placeholders or block/error messages.",
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
						},
					},
				}},
			},
			"enforce_zdr": resourceschema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Deprecated aggregate zero-data-retention enforcement flag. Prefer per-provider fields.",
			},
			"enforce_zdr_anthropic": guardrailZDRBoolAttribute("Whether to enforce zero data retention for Anthropic models. Falls back to `enforce_zdr` when omitted."),
			"enforce_zdr_google":    guardrailZDRBoolAttribute("Whether to enforce zero data retention for Google models. Falls back to `enforce_zdr` when omitted."),
			"enforce_zdr_openai":    guardrailZDRBoolAttribute("Whether to enforce zero data retention for OpenAI models. Falls back to `enforce_zdr` when omitted."),
			"enforce_zdr_other":     guardrailZDRBoolAttribute("Whether to enforce zero data retention for models from providers other than Anthropic, Google, or OpenAI. Falls back to `enforce_zdr` when omitted."),
			"created_at":            resourceschema.StringAttribute{Computed: true},
			"updated_at":            resourceschema.StringAttribute{Computed: true},
		},
	}
}

func (r *guardrailResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan guardrailResourceModel
	var config guardrailResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	request, diags := buildGuardrailUpsertRequest(ctx, plan, config, true)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.client.CreateGuardrail(ctx, request)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create OpenRouter guardrail", err.Error())
		return
	}

	state, diags := flattenGuardrailModel(ctx, *created, plan)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *guardrailResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state guardrailResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	guardrail, err := r.client.GetGuardrail(ctx, state.ID.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "(404)") {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read OpenRouter guardrail", err.Error())
		return
	}

	updatedState, diags := flattenGuardrailModel(ctx, *guardrail, state)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &updatedState)...)
}

func (r *guardrailResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan guardrailResourceModel
	var config guardrailResourceModel
	var state guardrailResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	request, diags := buildGuardrailUpsertRequest(ctx, plan, config, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	updated, err := r.client.UpdateGuardrail(ctx, state.ID.ValueString(), request)
	if err != nil {
		resp.Diagnostics.AddError("Unable to update OpenRouter guardrail", err.Error())
		return
	}

	newState, diags := flattenGuardrailModel(ctx, *updated, guardrailUpdateFlattenPrior(plan, state))
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *guardrailResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state guardrailResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteGuardrail(ctx, state.ID.ValueString()); err != nil && !strings.Contains(err.Error(), "(404)") {
		resp.Diagnostics.AddError("Unable to delete OpenRouter guardrail", err.Error())
	}
}

func (r *guardrailResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	workspaceID, name, err := parseCompositeImportID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Unable to import OpenRouter guardrail", err.Error())
		return
	}
	guardrails, listErr := r.client.ListGuardrails(ctx, &workspaceID)
	if listErr != nil {
		resp.Diagnostics.AddError("Unable to import OpenRouter guardrail", listErr.Error())
		return
	}
	guardrail, err := findGuardrailByCompositeImport(guardrails, workspaceID, name)
	if err != nil {
		resp.Diagnostics.AddError("Unable to import OpenRouter guardrail", err.Error())
		return
	}
	state, diags := flattenGuardrailModel(ctx, *guardrail)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func flattenGuardrailModel(ctx context.Context, in client.Guardrail, prior ...guardrailResourceModel) (guardrailResourceModel, diag.Diagnostics) {
	allowedModels, diags := setStringValueOrNull(ctx, in.AllowedModels)
	allowedProviders, moreDiags := setStringValueOrNull(ctx, in.AllowedProviders)
	diags.Append(moreDiags...)
	ignoredModels, moreDiags := setStringValueOrNull(ctx, in.IgnoredModels)
	diags.Append(moreDiags...)
	ignoredProviders, moreDiags := setStringValueOrNull(ctx, in.IgnoredProviders)
	diags.Append(moreDiags...)
	contentFilterBuiltins, moreDiags := contentFilterBuiltinsValue(ctx, in.ContentFilterBuiltins)
	diags.Append(moreDiags...)
	contentFilters, moreDiags := contentFiltersValue(ctx, in.ContentFilters)
	diags.Append(moreDiags...)
	if contentFilterBuiltins.IsNull() && len(prior) > 0 && isKnownEmptyList(prior[0].ContentFilterBuiltins) {
		contentFilterBuiltins = emptyContentFilterBuiltinsList()
	}
	if contentFilters.IsNull() && len(prior) > 0 && isKnownEmptyList(prior[0].ContentFilters) {
		contentFilters = emptyContentFiltersList()
	}

	return guardrailResourceModel{
		ID:                    types.StringValue(in.ID),
		Name:                  types.StringValue(in.Name),
		WorkspaceID:           stringPtrValue(in.WorkspaceID),
		Description:           stringPtrValue(in.Description),
		LimitUSD:              float64PtrValue(in.LimitUSD),
		ResetInterval:         stringPtrValue(in.ResetInterval),
		AllowedModels:         allowedModels,
		AllowedProviders:      allowedProviders,
		IgnoredModels:         ignoredModels,
		IgnoredProviders:      ignoredProviders,
		ContentFilterBuiltins: contentFilterBuiltins,
		ContentFilters:        contentFilters,
		EnforceZDR:            legacyEnforceZDRValue(in, prior...),
		EnforceZDRAnthropic:   boolPtrValue(in.EnforceZDRAnthropic),
		EnforceZDRGoogle:      boolPtrValue(in.EnforceZDRGoogle),
		EnforceZDROpenAI:      boolPtrValue(in.EnforceZDROpenAI),
		EnforceZDROther:       boolPtrValue(in.EnforceZDROther),
		CreatedAt:             types.StringValue(in.CreatedAt),
		UpdatedAt:             stringPtrValue(in.UpdatedAt),
	}, diags
}

func guardrailZDRBoolAttribute(description string) resourceschema.BoolAttribute {
	return resourceschema.BoolAttribute{
		Optional:            true,
		Computed:            true,
		MarkdownDescription: description,
		PlanModifiers: []planmodifier.Bool{
			boolplanmodifier.UseStateForUnknown(),
		},
	}
}

func guardrailUpdateFlattenPrior(plan, state guardrailResourceModel) guardrailResourceModel {
	prior := plan
	if prior.EnforceZDR.IsUnknown() {
		prior.EnforceZDR = state.EnforceZDR
	}
	if prior.ContentFilterBuiltins.IsUnknown() {
		prior.ContentFilterBuiltins = state.ContentFilterBuiltins
	}
	if prior.ContentFilters.IsUnknown() {
		prior.ContentFilters = state.ContentFilters
	}
	return prior
}

func buildGuardrailUpsertRequest(ctx context.Context, plan, config guardrailResourceModel, includeWorkspaceID bool) (client.GuardrailUpsertRequest, diag.Diagnostics) {
	var diags diag.Diagnostics

	allowedModels, moreDiags := setToStringSlice(ctx, plan.AllowedModels)
	diags.Append(moreDiags...)
	allowedProviders, moreDiags := setToStringSlice(ctx, plan.AllowedProviders)
	diags.Append(moreDiags...)
	ignoredModels, moreDiags := setToStringSlice(ctx, plan.IgnoredModels)
	diags.Append(moreDiags...)
	ignoredProviders, moreDiags := setToStringSlice(ctx, plan.IgnoredProviders)
	diags.Append(moreDiags...)

	contentFilterBuiltins, moreDiags := contentFilterBuiltinsRequest(ctx, config.ContentFilterBuiltins)
	diags.Append(moreDiags...)
	contentFilters, moreDiags := contentFiltersRequest(ctx, config.ContentFilters)
	diags.Append(moreDiags...)
	diags.Append(validateGuardrailZDRConfig(config)...)
	if diags.HasError() {
		return client.GuardrailUpsertRequest{}, diags
	}

	request := client.GuardrailUpsertRequest{
		Name:                  stringValueOrNil(plan.Name),
		Description:           stringValueOrNil(plan.Description),
		LimitUSD:              float64ValueOrNil(plan.LimitUSD),
		ResetInterval:         stringValueOrNil(plan.ResetInterval),
		AllowedModels:         stringSliceOrNil(allowedModels),
		AllowedProviders:      stringSliceOrNil(allowedProviders),
		IgnoredModels:         stringSliceOrNil(ignoredModels),
		IgnoredProviders:      stringSliceOrNil(ignoredProviders),
		ContentFilterBuiltins: contentFilterBuiltins,
		ContentFilters:        contentFilters,
		EnforceZDR:            boolValueOrNil(config.EnforceZDR),
		EnforceZDRAnthropic:   boolValueOrNil(config.EnforceZDRAnthropic),
		EnforceZDRGoogle:      boolValueOrNil(config.EnforceZDRGoogle),
		EnforceZDROpenAI:      boolValueOrNil(config.EnforceZDROpenAI),
		EnforceZDROther:       boolValueOrNil(config.EnforceZDROther),
	}
	if includeWorkspaceID {
		request.WorkspaceID = stringValueOrNil(plan.WorkspaceID)
	}
	return request, diags
}

func validateGuardrailZDRConfig(config guardrailResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	legacy := boolValueOrNil(config.EnforceZDR)
	if legacy == nil {
		return diags
	}
	for _, field := range []struct {
		name  string
		value types.Bool
	}{
		{name: "enforce_zdr_anthropic", value: config.EnforceZDRAnthropic},
		{name: "enforce_zdr_google", value: config.EnforceZDRGoogle},
		{name: "enforce_zdr_openai", value: config.EnforceZDROpenAI},
		{name: "enforce_zdr_other", value: config.EnforceZDROther},
	} {
		providerValue := boolValueOrNil(field.value)
		if providerValue == nil || *providerValue == *legacy {
			continue
		}
		diags.AddError(
			"Contradictory zero-data-retention configuration",
			"`enforce_zdr` is deprecated and cannot conflict with `"+field.name+"`. Remove `enforce_zdr` and use per-provider ZDR fields, or set both fields to the same value.",
		)
	}
	return diags
}

func contentFilterBuiltinsRequest(ctx context.Context, value types.List) (*[]client.ContentFilterBuiltinEntryInput, diag.Diagnostics) {
	if value.IsNull() || value.IsUnknown() {
		return nil, nil
	}
	if len(value.Elements()) == 0 {
		var clear []client.ContentFilterBuiltinEntryInput
		return &clear, nil
	}
	var models []contentFilterBuiltinModel
	diags := value.ElementsAs(ctx, &models, false)
	if diags.HasError() {
		return nil, diags
	}
	entries := make([]client.ContentFilterBuiltinEntryInput, 0, len(models))
	for _, item := range models {
		if item.Slug.IsNull() || item.Slug.IsUnknown() || item.Action.IsNull() || item.Action.IsUnknown() {
			diags.AddError("Invalid builtin content filter", "Each builtin content filter must have known `slug` and `action` values.")
			continue
		}
		entries = append(entries, client.ContentFilterBuiltinEntryInput{
			Slug:      item.Slug.ValueString(),
			Action:    item.Action.ValueString(),
			ScanScope: stringValueOrNil(item.ScanScope),
		})
	}
	return &entries, diags
}

func contentFiltersRequest(ctx context.Context, value types.List) (*[]client.ContentFilterEntry, diag.Diagnostics) {
	if value.IsNull() || value.IsUnknown() {
		return nil, nil
	}
	if len(value.Elements()) == 0 {
		var clear []client.ContentFilterEntry
		return &clear, nil
	}
	var models []contentFilterModel
	diags := value.ElementsAs(ctx, &models, false)
	if diags.HasError() {
		return nil, diags
	}
	entries := make([]client.ContentFilterEntry, 0, len(models))
	for _, item := range models {
		if item.Pattern.IsNull() || item.Pattern.IsUnknown() || item.Action.IsNull() || item.Action.IsUnknown() {
			diags.AddError("Invalid custom content filter", "Each custom content filter must have known `pattern` and `action` values.")
			continue
		}
		entries = append(entries, client.ContentFilterEntry{
			Pattern: item.Pattern.ValueString(),
			Action:  item.Action.ValueString(),
			Label:   stringValueOrNil(item.Label),
		})
	}
	return &entries, diags
}

func contentFilterBuiltinsValue(ctx context.Context, values []client.ContentFilterBuiltinEntry) (types.List, diag.Diagnostics) {
	if len(values) == 0 {
		return types.ListNull(contentFilterBuiltinObjectType), nil
	}
	objects := make([]attr.Value, 0, len(values))
	var diags diag.Diagnostics
	for _, value := range values {
		object, moreDiags := types.ObjectValue(contentFilterBuiltinAttrTypes, map[string]attr.Value{
			"slug":       types.StringValue(value.Slug),
			"action":     types.StringValue(value.Action),
			"label":      stringPtrValue(value.Label),
			"scan_scope": stringPtrValue(value.ScanScope),
		})
		diags.Append(moreDiags...)
		objects = append(objects, object)
	}
	if diags.HasError() {
		return types.ListNull(contentFilterBuiltinObjectType), diags
	}
	return types.ListValue(contentFilterBuiltinObjectType, objects)
}

func contentFiltersValue(ctx context.Context, values []client.ContentFilterEntry) (types.List, diag.Diagnostics) {
	if len(values) == 0 {
		return types.ListNull(contentFilterObjectType), nil
	}
	objects := make([]attr.Value, 0, len(values))
	var diags diag.Diagnostics
	for _, value := range values {
		object, moreDiags := types.ObjectValue(contentFilterAttrTypes, map[string]attr.Value{
			"pattern": types.StringValue(value.Pattern),
			"action":  types.StringValue(value.Action),
			"label":   stringPtrValue(value.Label),
		})
		diags.Append(moreDiags...)
		objects = append(objects, object)
	}
	if diags.HasError() {
		return types.ListNull(contentFilterObjectType), diags
	}
	return types.ListValue(contentFilterObjectType, objects)
}

func emptyContentFilterBuiltinsList() types.List {
	return types.ListValueMust(contentFilterBuiltinObjectType, []attr.Value{})
}

func emptyContentFiltersList() types.List {
	return types.ListValueMust(contentFilterObjectType, []attr.Value{})
}

func isKnownEmptyList(value types.List) bool {
	return !value.IsNull() && !value.IsUnknown() && len(value.Elements()) == 0
}

func legacyEnforceZDRValue(in client.Guardrail, prior ...guardrailResourceModel) types.Bool {
	if in.EnforceZDR != nil {
		return boolPtrValue(in.EnforceZDR)
	}
	if !hasPerProviderZDR(in) || len(prior) == 0 || prior[0].EnforceZDR.IsNull() || prior[0].EnforceZDR.IsUnknown() {
		return types.BoolNull()
	}
	return prior[0].EnforceZDR
}

func hasPerProviderZDR(in client.Guardrail) bool {
	return in.EnforceZDRAnthropic != nil || in.EnforceZDRGoogle != nil || in.EnforceZDROpenAI != nil || in.EnforceZDROther != nil
}
