package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/cloudopsworks/terraform-provider-openrouter/internal/client"
)

var (
	_ resource.Resource                = &apiKeyResource{}
	_ resource.ResourceWithImportState = &apiKeyResource{}
)

type apiKeyResource struct {
	client *client.Client
}

type apiKeyResourceModel struct {
	ID                 types.String  `tfsdk:"id"`
	Name               types.String  `tfsdk:"name"`
	WorkspaceID        types.String  `tfsdk:"workspace_id"`
	Limit              types.Float64 `tfsdk:"limit"`
	LimitRemaining     types.Float64 `tfsdk:"limit_remaining"`
	LimitReset         types.String  `tfsdk:"limit_reset"`
	IncludeBYOKInLimit types.Bool    `tfsdk:"include_byok_in_limit"`
	Disabled           types.Bool    `tfsdk:"disabled"`
	ExpiresAt          types.String  `tfsdk:"expires_at"`
	CreatorUserID      types.String  `tfsdk:"creator_user_id"`
	Label              types.String  `tfsdk:"label"`
	Key                types.String  `tfsdk:"key"`
	Usage              types.Float64 `tfsdk:"usage"`
	UsageDaily         types.Float64 `tfsdk:"usage_daily"`
	UsageWeekly        types.Float64 `tfsdk:"usage_weekly"`
	UsageMonthly       types.Float64 `tfsdk:"usage_monthly"`
	BYOKUsage          types.Float64 `tfsdk:"byok_usage"`
	BYOKUsageDaily     types.Float64 `tfsdk:"byok_usage_daily"`
	BYOKUsageWeekly    types.Float64 `tfsdk:"byok_usage_weekly"`
	BYOKUsageMonthly   types.Float64 `tfsdk:"byok_usage_monthly"`
	CreatedAt          types.String  `tfsdk:"created_at"`
	UpdatedAt          types.String  `tfsdk:"updated_at"`
}

func NewAPIKeyResource() resource.Resource {
	return &apiKeyResource{}
}

func (r *apiKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_api_key"
}

func (r *apiKeyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	providerData, err := configureClient(req.ProviderData)
	if err != nil {
		resp.Diagnostics.AddError("Unable to configure OpenRouter API key resource", err.Error())
		return
	}
	r.client = providerData.client
}

func (r *apiKeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		MarkdownDescription: "Manage OpenRouter management API keys.",
		Attributes: map[string]resourceschema.Attribute{
			"id":                    resourceschema.StringAttribute{Computed: true, MarkdownDescription: "Stable API key hash."},
			"name":                  resourceschema.StringAttribute{Required: true, MarkdownDescription: "Name of the API key."},
			"workspace_id":          resourceschema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Optional workspace UUID to send during API key creation. When omitted, the provider records whatever workspace the API returns.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"limit":                 resourceschema.Float64Attribute{Optional: true, Computed: true, MarkdownDescription: "Spending limit in USD."},
			"limit_remaining":       resourceschema.Float64Attribute{Computed: true, MarkdownDescription: "Remaining limit in USD."},
			"limit_reset":           resourceschema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Reset interval: daily, weekly, monthly, or null for no reset."},
			"include_byok_in_limit": resourceschema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "Whether BYOK usage counts toward the limit."},
			"disabled":              resourceschema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "Whether the key is disabled."},
			"expires_at":            resourceschema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "UTC ISO 8601 expiration timestamp. Replacement when changed.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"creator_user_id":       resourceschema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Optional organization member creator identifier. Replacement when changed.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"label":                 resourceschema.StringAttribute{Computed: true, MarkdownDescription: "Server-generated label."},
			"key":                   resourceschema.StringAttribute{Computed: true, Sensitive: true, MarkdownDescription: "Actual API key string, only returned on create and stored in state when available."},
			"usage":                 resourceschema.Float64Attribute{Computed: true},
			"usage_daily":           resourceschema.Float64Attribute{Computed: true},
			"usage_weekly":          resourceschema.Float64Attribute{Computed: true},
			"usage_monthly":         resourceschema.Float64Attribute{Computed: true},
			"byok_usage":            resourceschema.Float64Attribute{Computed: true},
			"byok_usage_daily":      resourceschema.Float64Attribute{Computed: true},
			"byok_usage_weekly":     resourceschema.Float64Attribute{Computed: true},
			"byok_usage_monthly":    resourceschema.Float64Attribute{Computed: true},
			"created_at":            resourceschema.StringAttribute{Computed: true},
			"updated_at":            resourceschema.StringAttribute{Computed: true},
		},
	}
}

func (r *apiKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan apiKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	request := client.APIKeyCreateRequest{
		Name:               plan.Name.ValueString(),
		WorkspaceID:        stringValueOrNil(plan.WorkspaceID),
		Limit:              float64ValueOrNil(plan.Limit),
		LimitReset:         stringValueOrNil(plan.LimitReset),
		IncludeBYOKInLimit: boolValueOrNil(plan.IncludeBYOKInLimit),
		ExpiresAt:          stringValueOrNil(plan.ExpiresAt),
		CreatorUserID:      stringValueOrNil(plan.CreatorUserID),
	}

	created, err := r.client.CreateAPIKey(ctx, request)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create OpenRouter API key", err.Error())
		return
	}

	state := flattenAPIKeyModel(created.Data)
	state.Key = types.StringValue(created.Key)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *apiKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state apiKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	key, err := r.client.GetAPIKey(ctx, state.ID.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "(404)") {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read OpenRouter API key", err.Error())
		return
	}

	updatedState := flattenAPIKeyModel(*key)
	updatedState.Key = state.Key
	resp.Diagnostics.Append(resp.State.Set(ctx, &updatedState)...)
}

func (r *apiKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan apiKeyResourceModel
	var state apiKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	request := client.APIKeyUpdateRequest{
		Name:               stringValueOrNil(plan.Name),
		Disabled:           boolValueOrNil(plan.Disabled),
		Limit:              float64ValueOrNil(plan.Limit),
		LimitReset:         stringValueOrNil(plan.LimitReset),
		IncludeBYOKInLimit: boolValueOrNil(plan.IncludeBYOKInLimit),
	}

	updated, err := r.client.UpdateAPIKey(ctx, state.ID.ValueString(), request)
	if err != nil {
		resp.Diagnostics.AddError("Unable to update OpenRouter API key", err.Error())
		return
	}

	newState := flattenAPIKeyModel(*updated)
	newState.Key = state.Key
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *apiKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state apiKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteAPIKey(ctx, state.ID.ValueString()); err != nil && !strings.Contains(err.Error(), "(404)") {
		resp.Diagnostics.AddError("Unable to delete OpenRouter API key", err.Error())
	}
}

func (r *apiKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if workspaceID, name, err := parseCompositeImportID(req.ID); err == nil {
		keys, listErr := r.client.ListAPIKeys(ctx, &workspaceID, true)
		if listErr != nil {
			resp.Diagnostics.AddError("Unable to import OpenRouter API key", listErr.Error())
			return
		}
		matches := make([]client.APIKey, 0)
		for _, item := range keys {
			if item.Name == name {
				matches = append(matches, item)
			}
		}
		if len(matches) == 0 {
			resp.Diagnostics.AddError("Unable to import OpenRouter API key", fmt.Sprintf("no API key named %q found in workspace %q", name, workspaceID))
			return
		}
		if len(matches) > 1 {
			resp.Diagnostics.AddError("Unable to import OpenRouter API key", fmt.Sprintf("multiple API keys named %q found in workspace %q", name, workspaceID))
			return
		}
		state := flattenAPIKeyModel(matches[0])
		state.Key = types.StringNull()
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		return
	}

	key, err := r.client.GetAPIKey(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Unable to import OpenRouter API key", err.Error())
		return
	}
	state := flattenAPIKeyModel(*key)
	state.Key = types.StringNull()
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func flattenAPIKeyModel(in client.APIKey) apiKeyResourceModel {
	model := apiKeyResourceModel{
		ID:                 types.StringValue(in.Hash),
		Name:               types.StringValue(in.Name),
		WorkspaceID:        stringPtrValue(in.WorkspaceID),
		Limit:              float64PtrValue(in.Limit),
		LimitRemaining:     float64PtrValue(in.LimitRemaining),
		LimitReset:         stringPtrValue(in.LimitReset),
		IncludeBYOKInLimit: types.BoolValue(in.IncludeBYOKInLimit),
		Disabled:           types.BoolValue(in.Disabled),
		ExpiresAt:          stringPtrValue(in.ExpiresAt),
		CreatorUserID:      stringPtrValue(in.CreatorUserID),
		Label:              types.StringValue(in.Label),
		Usage:              types.Float64Value(in.Usage),
		UsageDaily:         types.Float64Value(in.UsageDaily),
		UsageWeekly:        types.Float64Value(in.UsageWeekly),
		UsageMonthly:       types.Float64Value(in.UsageMonthly),
		BYOKUsage:          types.Float64Value(in.BYOKUsage),
		BYOKUsageDaily:     types.Float64Value(in.BYOKUsageDaily),
		BYOKUsageWeekly:    types.Float64Value(in.BYOKUsageWeekly),
		BYOKUsageMonthly:   types.Float64Value(in.BYOKUsageMonthly),
		CreatedAt:          types.StringValue(in.CreatedAt),
		UpdatedAt:          types.StringValue(in.UpdatedAt),
		Key:                types.StringNull(),
	}
	if model.WorkspaceID.IsNull() {
		model.WorkspaceID = types.StringNull()
	}
	return model
}
