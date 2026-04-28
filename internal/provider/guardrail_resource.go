package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
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
	ID               types.String  `tfsdk:"id"`
	Name             types.String  `tfsdk:"name"`
	Description      types.String  `tfsdk:"description"`
	LimitUSD         types.Float64 `tfsdk:"limit_usd"`
	ResetInterval    types.String  `tfsdk:"reset_interval"`
	AllowedModels    types.Set     `tfsdk:"allowed_models"`
	AllowedProviders types.Set     `tfsdk:"allowed_providers"`
	IgnoredModels    types.Set     `tfsdk:"ignored_models"`
	IgnoredProviders types.Set     `tfsdk:"ignored_providers"`
	EnforceZDR       types.Bool    `tfsdk:"enforce_zdr"`
	CreatedAt        types.String  `tfsdk:"created_at"`
	UpdatedAt        types.String  `tfsdk:"updated_at"`
}

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
		MarkdownDescription: "Manage OpenRouter guardrails. OpenRouter's current API does not expose a workspace field on guardrail CRUD responses, so this v1 resource manages guardrails by guardrail ID and name only.",
		Attributes: map[string]resourceschema.Attribute{
			"id":                resourceschema.StringAttribute{Computed: true},
			"name":              resourceschema.StringAttribute{Required: true},
			"description":       resourceschema.StringAttribute{Optional: true, Computed: true},
			"limit_usd":         resourceschema.Float64Attribute{Optional: true, Computed: true},
			"reset_interval":    resourceschema.StringAttribute{Optional: true, Computed: true},
			"allowed_models":    resourceschema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType},
			"allowed_providers": resourceschema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType},
			"ignored_models":    resourceschema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType},
			"ignored_providers": resourceschema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType},
			"enforce_zdr":       resourceschema.BoolAttribute{Optional: true, Computed: true},
			"created_at":        resourceschema.StringAttribute{Computed: true},
			"updated_at":        resourceschema.StringAttribute{Computed: true},
		},
	}
}

func (r *guardrailResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan guardrailResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	allowedModels, diags := setToStringSlice(ctx, plan.AllowedModels)
	resp.Diagnostics.Append(diags...)
	allowedProviders, diags := setToStringSlice(ctx, plan.AllowedProviders)
	resp.Diagnostics.Append(diags...)
	ignoredModels, diags := setToStringSlice(ctx, plan.IgnoredModels)
	resp.Diagnostics.Append(diags...)
	ignoredProviders, diags := setToStringSlice(ctx, plan.IgnoredProviders)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	request := client.GuardrailUpsertRequest{
		Name:             stringValueOrNil(plan.Name),
		Description:      stringValueOrNil(plan.Description),
		LimitUSD:         float64ValueOrNil(plan.LimitUSD),
		ResetInterval:    stringValueOrNil(plan.ResetInterval),
		AllowedModels:    &allowedModels,
		AllowedProviders: &allowedProviders,
		IgnoredModels:    &ignoredModels,
		IgnoredProviders: &ignoredProviders,
		EnforceZDR:       boolValueOrNil(plan.EnforceZDR),
	}

	created, err := r.client.CreateGuardrail(ctx, request)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create OpenRouter guardrail", err.Error())
		return
	}

	state, diags := flattenGuardrailModel(ctx, *created)
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

	updatedState, diags := flattenGuardrailModel(ctx, *guardrail)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &updatedState)...)
}

func (r *guardrailResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan guardrailResourceModel
	var state guardrailResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	allowedModels, diags := setToStringSlice(ctx, plan.AllowedModels)
	resp.Diagnostics.Append(diags...)
	allowedProviders, diags := setToStringSlice(ctx, plan.AllowedProviders)
	resp.Diagnostics.Append(diags...)
	ignoredModels, diags := setToStringSlice(ctx, plan.IgnoredModels)
	resp.Diagnostics.Append(diags...)
	ignoredProviders, diags := setToStringSlice(ctx, plan.IgnoredProviders)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	request := client.GuardrailUpsertRequest{
		Name:             stringValueOrNil(plan.Name),
		Description:      stringValueOrNil(plan.Description),
		LimitUSD:         float64ValueOrNil(plan.LimitUSD),
		ResetInterval:    stringValueOrNil(plan.ResetInterval),
		AllowedModels:    &allowedModels,
		AllowedProviders: &allowedProviders,
		IgnoredModels:    &ignoredModels,
		IgnoredProviders: &ignoredProviders,
		EnforceZDR:       boolValueOrNil(plan.EnforceZDR),
	}

	updated, err := r.client.UpdateGuardrail(ctx, state.ID.ValueString(), request)
	if err != nil {
		resp.Diagnostics.AddError("Unable to update OpenRouter guardrail", err.Error())
		return
	}

	newState, diags := flattenGuardrailModel(ctx, *updated)
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
	if _, name, err := parseCompositeImportID(req.ID); err == nil {
		guardrails, listErr := r.client.ListGuardrails(ctx)
		if listErr != nil {
			resp.Diagnostics.AddError("Unable to import OpenRouter guardrail", listErr.Error())
			return
		}
		matches := make([]client.Guardrail, 0)
		for _, item := range guardrails {
			if item.Name == name {
				matches = append(matches, item)
			}
		}
		if len(matches) == 0 {
			resp.Diagnostics.AddError("Unable to import OpenRouter guardrail", fmt.Sprintf("no guardrail named %q found", name))
			return
		}
		if len(matches) > 1 {
			resp.Diagnostics.AddError("Unable to import OpenRouter guardrail", fmt.Sprintf("multiple guardrails named %q found; import by canonical ID instead", name))
			return
		}
		state, diags := flattenGuardrailModel(ctx, matches[0])
		resp.Diagnostics.Append(diags...)
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		return
	}

	guardrail, err := r.client.GetGuardrail(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Unable to import OpenRouter guardrail", err.Error())
		return
	}
	state, diags := flattenGuardrailModel(ctx, *guardrail)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func flattenGuardrailModel(ctx context.Context, in client.Guardrail) (guardrailResourceModel, diag.Diagnostics) {
	allowedModels, diags := setStringValueOrNull(ctx, in.AllowedModels)
	allowedProviders, moreDiags := setStringValueOrNull(ctx, in.AllowedProviders)
	diags.Append(moreDiags...)
	ignoredModels, moreDiags := setStringValueOrNull(ctx, in.IgnoredModels)
	diags.Append(moreDiags...)
	ignoredProviders, moreDiags := setStringValueOrNull(ctx, in.IgnoredProviders)
	diags.Append(moreDiags...)

	return guardrailResourceModel{
		ID:               types.StringValue(in.ID),
		Name:             types.StringValue(in.Name),
		Description:      stringPtrValue(in.Description),
		LimitUSD:         float64PtrValue(in.LimitUSD),
		ResetInterval:    stringPtrValue(in.ResetInterval),
		AllowedModels:    allowedModels,
		AllowedProviders: allowedProviders,
		IgnoredModels:    ignoredModels,
		IgnoredProviders: ignoredProviders,
		EnforceZDR:       boolPtrValue(in.EnforceZDR),
		CreatedAt:        types.StringValue(in.CreatedAt),
		UpdatedAt:        stringPtrValue(in.UpdatedAt),
	}, diags
}
