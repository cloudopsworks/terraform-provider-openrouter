package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/cloudopsworks/terraform-provider-openrouter/internal/client"
)

var (
	_ resource.Resource                = &workspaceResource{}
	_ resource.ResourceWithImportState = &workspaceResource{}
)

type workspaceResource struct {
	client *client.Client
}

type workspaceResourceModel struct {
	ID                              types.String  `tfsdk:"id"`
	Name                            types.String  `tfsdk:"name"`
	Slug                            types.String  `tfsdk:"slug"`
	Description                     types.String  `tfsdk:"description"`
	DefaultTextModel                types.String  `tfsdk:"default_text_model"`
	DefaultImageModel               types.String  `tfsdk:"default_image_model"`
	DefaultProviderSort             types.String  `tfsdk:"default_provider_sort"`
	IOLoggingSamplingRate           types.Float64 `tfsdk:"io_logging_sampling_rate"`
	IsDataDiscountLoggingEnabled    types.Bool    `tfsdk:"is_data_discount_logging_enabled"`
	IsObservabilityBroadcastEnabled types.Bool    `tfsdk:"is_observability_broadcast_enabled"`
	IsObservabilityIOLoggingEnabled types.Bool    `tfsdk:"is_observability_io_logging_enabled"`
	CreatedAt                       types.String  `tfsdk:"created_at"`
	CreatedBy                       types.String  `tfsdk:"created_by"`
	UpdatedAt                       types.String  `tfsdk:"updated_at"`
}

func NewWorkspaceResource() resource.Resource {
	return &workspaceResource{}
}

func (r *workspaceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workspace"
}

func (r *workspaceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	providerData, err := configureClient(req.ProviderData)
	if err != nil {
		resp.Diagnostics.AddError("Unable to configure OpenRouter workspace resource", err.Error())
		return
	}
	r.client = providerData.client
}

func (r *workspaceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		MarkdownDescription: "Manage OpenRouter workspaces.",
		Attributes: map[string]resourceschema.Attribute{
			"id":                                  resourceschema.StringAttribute{Computed: true, MarkdownDescription: "Stable workspace UUID."},
			"name":                                resourceschema.StringAttribute{Required: true},
			"slug":                                resourceschema.StringAttribute{Required: true, MarkdownDescription: "Workspace slug."},
			"description":                         resourceschema.StringAttribute{Optional: true, Computed: true},
			"default_text_model":                  resourceschema.StringAttribute{Optional: true, Computed: true},
			"default_image_model":                 resourceschema.StringAttribute{Optional: true, Computed: true},
			"default_provider_sort":               resourceschema.StringAttribute{Optional: true, Computed: true},
			"io_logging_sampling_rate":            resourceschema.Float64Attribute{Optional: true, Computed: true, MarkdownDescription: "I/O logging sampling rate. `io_logging_api_key_ids` is intentionally not modeled in v1 because the docs do not show it round-tripping in read responses."},
			"is_data_discount_logging_enabled":    resourceschema.BoolAttribute{Optional: true, Computed: true},
			"is_observability_broadcast_enabled":  resourceschema.BoolAttribute{Optional: true, Computed: true},
			"is_observability_io_logging_enabled": resourceschema.BoolAttribute{Optional: true, Computed: true},
			"created_at":                          resourceschema.StringAttribute{Computed: true},
			"created_by":                          resourceschema.StringAttribute{Computed: true},
			"updated_at":                          resourceschema.StringAttribute{Computed: true},
		},
	}
}

func (r *workspaceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan workspaceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	request := client.WorkspaceUpsertRequest{
		Name:                            stringValueOrNil(plan.Name),
		Slug:                            stringValueOrNil(plan.Slug),
		Description:                     stringValueOrNil(plan.Description),
		DefaultTextModel:                stringValueOrNil(plan.DefaultTextModel),
		DefaultImageModel:               stringValueOrNil(plan.DefaultImageModel),
		DefaultProviderSort:             stringValueOrNil(plan.DefaultProviderSort),
		IOLoggingSamplingRate:           float64ValueOrNil(plan.IOLoggingSamplingRate),
		IsDataDiscountLoggingEnabled:    boolValueOrNil(plan.IsDataDiscountLoggingEnabled),
		IsObservabilityBroadcastEnabled: boolValueOrNil(plan.IsObservabilityBroadcastEnabled),
		IsObservabilityIOLoggingEnabled: boolValueOrNil(plan.IsObservabilityIOLoggingEnabled),
	}

	created, err := r.client.CreateWorkspace(ctx, request)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create OpenRouter workspace", err.Error())
		return
	}
	state := flattenWorkspaceModel(*created)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *workspaceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state workspaceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	workspace, err := r.client.GetWorkspace(ctx, state.ID.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "(404)") {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read OpenRouter workspace", err.Error())
		return
	}

	updatedState := flattenWorkspaceModel(*workspace)
	resp.Diagnostics.Append(resp.State.Set(ctx, &updatedState)...)
}

func (r *workspaceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan workspaceResourceModel
	var state workspaceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	request := client.WorkspaceUpsertRequest{
		Name:                            stringValueOrNil(plan.Name),
		Slug:                            stringValueOrNil(plan.Slug),
		Description:                     stringValueOrNil(plan.Description),
		DefaultTextModel:                stringValueOrNil(plan.DefaultTextModel),
		DefaultImageModel:               stringValueOrNil(plan.DefaultImageModel),
		DefaultProviderSort:             stringValueOrNil(plan.DefaultProviderSort),
		IOLoggingSamplingRate:           float64ValueOrNil(plan.IOLoggingSamplingRate),
		IsDataDiscountLoggingEnabled:    boolValueOrNil(plan.IsDataDiscountLoggingEnabled),
		IsObservabilityBroadcastEnabled: boolValueOrNil(plan.IsObservabilityBroadcastEnabled),
		IsObservabilityIOLoggingEnabled: boolValueOrNil(plan.IsObservabilityIOLoggingEnabled),
	}

	updated, err := r.client.UpdateWorkspace(ctx, state.ID.ValueString(), request)
	if err != nil {
		resp.Diagnostics.AddError("Unable to update OpenRouter workspace", err.Error())
		return
	}

	newState := flattenWorkspaceModel(*updated)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *workspaceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state workspaceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteWorkspace(ctx, state.ID.ValueString()); err != nil && !strings.Contains(err.Error(), "(404)") {
		resp.Diagnostics.AddError("Unable to delete OpenRouter workspace", err.Error())
	}
}

func (r *workspaceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importID := req.ID
	if workspaceID, slugOrName, err := parseCompositeImportID(req.ID); err == nil {
		importID = workspaceID
		workspace, getErr := r.client.GetWorkspace(ctx, workspaceID)
		if getErr != nil {
			resp.Diagnostics.AddError("Unable to import OpenRouter workspace", getErr.Error())
			return
		}
		if workspace.Slug != slugOrName && workspace.Name != slugOrName {
			resp.Diagnostics.AddError("Unable to import OpenRouter workspace", fmt.Sprintf("workspace %q did not match name/slug %q", workspaceID, slugOrName))
			return
		}
		state := flattenWorkspaceModel(*workspace)
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		return
	}

	workspace, err := r.client.GetWorkspace(ctx, importID)
	if err != nil {
		resp.Diagnostics.AddError("Unable to import OpenRouter workspace", err.Error())
		return
	}
	state := flattenWorkspaceModel(*workspace)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func flattenWorkspaceModel(in client.Workspace) workspaceResourceModel {
	return workspaceResourceModel{
		ID:                              types.StringValue(in.ID),
		Name:                            types.StringValue(in.Name),
		Slug:                            types.StringValue(in.Slug),
		Description:                     stringPtrValue(in.Description),
		DefaultTextModel:                stringPtrValue(in.DefaultTextModel),
		DefaultImageModel:               stringPtrValue(in.DefaultImageModel),
		DefaultProviderSort:             stringPtrValue(in.DefaultProviderSort),
		IOLoggingSamplingRate:           float64PtrValue(in.IOLoggingSamplingRate),
		IsDataDiscountLoggingEnabled:    types.BoolValue(in.IsDataDiscountLoggingEnabled),
		IsObservabilityBroadcastEnabled: types.BoolValue(in.IsObservabilityBroadcastEnabled),
		IsObservabilityIOLoggingEnabled: types.BoolValue(in.IsObservabilityIOLoggingEnabled),
		CreatedAt:                       types.StringValue(in.CreatedAt),
		CreatedBy:                       stringPtrValue(in.CreatedBy),
		UpdatedAt:                       stringPtrValue(in.UpdatedAt),
	}
}
