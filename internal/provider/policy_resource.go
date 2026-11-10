package provider

import (
	"context"
	"errors"
	"strings"

	"github.com/cloudopsworks/terraform-provider-openrouter/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rs "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                   = &policyResource{}
	_ resource.ResourceWithImportState    = &policyResource{}
	_ resource.ResourceWithValidateConfig = &policyResource{}
)

type policyResource struct {
	client *client.Client
	kind   string
}
type policyResourceModel struct {
	ID          types.String `tfsdk:"id"`
	WorkspaceID types.String `tfsdk:"workspace_id"`
	Name        types.String `tfsdk:"name"`
	Config      types.Object `tfsdk:"config"`
}

func NewBudgetPolicyResource() resource.Resource { return &policyResource{kind: "budget"} }
func NewModelAvailabilityPolicyResource() resource.Resource {
	return &policyResource{kind: "model_availability"}
}
func (r *policyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + r.kind + "_policy"
}
func (r *policyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	configured, err := configureClient(req.ProviderData)
	if err != nil {
		resp.Diagnostics.AddError("Unable to configure policy resource", err.Error())
		return
	}
	r.client = configured.client
}
func (r *policyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = rs.Schema{MarkdownDescription: "Manage an independent workspace-scoped OpenRouter policy. Defaults, legacy-mapped policies and account policies are excluded. Assignment lists are not managed by this resource.", Attributes: map[string]rs.Attribute{
		"id":           rs.StringAttribute{Computed: true, MarkdownDescription: "Policy UUID."},
		"workspace_id": rs.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, MarkdownDescription: "Workspace UUID. Replacement when changed."},
		"name":         rs.StringAttribute{Required: true},
		"config":       rs.SingleNestedAttribute{Required: true, Attributes: policyConfigAttributes(r.kind), MarkdownDescription: "Complete policy config. Removed optional fields are sent as null to clear them."},
	}}
}
func (r *policyResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var model policyResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(validatePolicyConfig(ctx, r.kind, model)...)
}
func (r *policyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan policyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(validatePolicyConfig(ctx, r.kind, plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	config, diags := policyAuthoringConfig(ctx, r.kind, plan.Config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	var created *client.Policy
	var err error
	switch value := config.(type) {
	case client.BudgetPolicyConfig:
		created, err = r.client.CreateBudgetPolicy(ctx, plan.WorkspaceID.ValueString(), plan.Name.ValueString(), value)
	case client.ModelAvailabilityPolicyConfig:
		created, err = r.client.CreateModelAvailabilityPolicy(ctx, plan.WorkspaceID.ValueString(), plan.Name.ValueString(), value)
	}
	if created != nil && created.ID != "" {
		plan.ID = types.StringValue(created.ID)
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to create policy", err.Error())
		return
	}
	refreshed, err := r.client.GetPolicy(ctx, r.kind, plan.WorkspaceID.ValueString(), created.ID)
	if err == nil {
		err = refreshed.EnsureManageable()
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to verify created policy", err.Error())
		return
	}
	resp.Diagnostics.Append(r.setPolicyState(ctx, refreshed, &plan)...)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	}
}
func (r *policyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state policyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	policy, err := r.client.GetPolicy(ctx, r.kind, state.WorkspaceID.ValueString(), state.ID.ValueString())
	if err != nil {
		if isPolicyNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read policy", err.Error())
		return
	}
	if err = policy.EnsureManageable(); err != nil {
		resp.Diagnostics.AddError("Unsupported policy ownership", err.Error())
		return
	}
	resp.Diagnostics.Append(r.setPolicyState(ctx, policy, &state)...)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	}
}
func (r *policyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state policyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(validatePolicyConfig(ctx, r.kind, plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !plan.WorkspaceID.Equal(state.WorkspaceID) {
		resp.Diagnostics.AddError("Policy workspace changed", "workspace_id changes require replacement")
		return
	}
	prior, err := r.client.GetPolicy(ctx, r.kind, state.WorkspaceID.ValueString(), state.ID.ValueString())
	if err == nil {
		err = prior.EnsureManageable()
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to safely update policy", err.Error())
		return
	}
	config, diags := policyAuthoringConfig(ctx, r.kind, plan.Config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	var updated *client.Policy
	switch value := config.(type) {
	case client.BudgetPolicyConfig:
		updated, err = r.client.UpdateBudgetPolicy(ctx, state.WorkspaceID.ValueString(), state.ID.ValueString(), plan.Name.ValueString(), value)
	case client.ModelAvailabilityPolicyConfig:
		updated, err = r.client.UpdateModelAvailabilityPolicy(ctx, state.WorkspaceID.ValueString(), state.ID.ValueString(), plan.Name.ValueString(), value)
	}
	if err != nil {
		var identityChanged *client.PolicyIdentityChangedError
		if errors.As(err, &identityChanged) {
			// A legacy conversion can race the preflight and create a new policy.
			// Retain the validated write identity and requested state for recovery;
			// never retry, compensate, or pretend the operation succeeded.
			plan.ID = types.StringValue(identityChanged.ReturnedID)
			resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
			resp.Diagnostics.AddError("Policy identity changed during update", err.Error()+". The returned policy identity has been retained in state for recovery; inspect the original and returned policies before continuing.")
			return
		}
		resp.Diagnostics.AddError("Unable to update policy", err.Error())
		return
	}
	plan.ID = types.StringValue(updated.ID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	refreshed, err := r.client.GetPolicy(ctx, r.kind, state.WorkspaceID.ValueString(), updated.ID)
	if err == nil {
		err = refreshed.EnsureManageable()
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to verify updated policy", err.Error())
		return
	}
	resp.Diagnostics.Append(r.setPolicyState(ctx, refreshed, &plan)...)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	}
}
func (r *policyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state policyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	policy, err := r.client.GetPolicy(ctx, r.kind, state.WorkspaceID.ValueString(), state.ID.ValueString())
	if err != nil {
		if isPolicyNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Unable to read policy before delete", err.Error())
		return
	}
	if err = policy.EnsureManageable(); err != nil {
		resp.Diagnostics.AddError("Unable to safely delete policy", err.Error())
		return
	}
	if err = r.client.DeletePolicy(ctx, r.kind, state.WorkspaceID.ValueString(), state.ID.ValueString()); err != nil && !isPolicyNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete policy", err.Error())
		return
	}
	_, err = r.client.GetPolicy(ctx, r.kind, state.WorkspaceID.ValueString(), state.ID.ValueString())
	if err == nil {
		resp.Diagnostics.AddError("Unable to verify policy deletion", "policy remains present after delete")
	} else if !isPolicyNotFound(err) {
		resp.Diagnostics.AddError("Unable to verify policy deletion", err.Error())
	}
}
func (r *policyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	workspace, id, ok := strings.Cut(req.ID, "/")
	if !ok || strings.TrimSpace(workspace) == "" || strings.TrimSpace(id) == "" || strings.Contains(id, "/") {
		resp.Diagnostics.AddError("Invalid policy import identity", "expected <workspace_id>/<policy_id>")
		return
	}
	policy, err := r.client.GetPolicy(ctx, r.kind, workspace, id)
	if err == nil {
		err = policy.EnsureManageable()
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to import policy", err.Error())
		return
	}
	state := policyResourceModel{ID: types.StringValue(id), WorkspaceID: types.StringValue(workspace)}
	resp.Diagnostics.Append(r.setPolicyState(ctx, policy, &state)...)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	}
}

// Only a transport status prefix is authoritative; error messages may mention other status codes.
func isPolicyNotFound(err error) bool {
	return err != nil && strings.HasPrefix(err.Error(), "openrouter API error (404):")
}
