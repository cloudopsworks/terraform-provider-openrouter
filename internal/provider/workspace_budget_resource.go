package provider

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/cloudopsworks/terraform-provider-openrouter/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rs "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type workspaceBudgetResource struct {
	client *client.Client
}

type workspaceBudgetModel struct {
	ID          types.String  `tfsdk:"id"`
	WorkspaceID types.String  `tfsdk:"workspace_id"`
	Interval    types.String  `tfsdk:"interval"`
	LimitUSD    types.Float64 `tfsdk:"limit_usd"`
	CreatedAt   types.String  `tfsdk:"created_at"`
	UpdatedAt   types.String  `tfsdk:"updated_at"`
}

func NewWorkspaceBudgetResource() resource.Resource {
	return &workspaceBudgetResource{}
}

func (r *workspaceBudgetResource) Metadata(_ context.Context, request resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = request.ProviderTypeName + "_workspace_budget"
}

func (r *workspaceBudgetResource) Configure(_ context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
	if request.ProviderData == nil {
		return
	}
	configured, err := configureClient(request.ProviderData)
	if err != nil {
		response.Diagnostics.AddError("Unable to configure workspace budget", err.Error())
		return
	}
	r.client = configured.client
}

func (r *workspaceBudgetResource) Schema(_ context.Context, _ resource.SchemaRequest, response *resource.SchemaResponse) {
	response.Schema = rs.Schema{Attributes: map[string]rs.Attribute{
		"id":           rs.StringAttribute{Computed: true},
		"workspace_id": rs.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
		"interval":     rs.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
		"limit_usd":    rs.Float64Attribute{Required: true},
		"created_at":   rs.StringAttribute{Computed: true},
		"updated_at":   rs.StringAttribute{Computed: true},
	}}
}

func budgetID(workspace, interval string) string {
	return workspace + "/" + interval
}

func flattenBudget(budget *client.WorkspaceBudget, configured workspaceBudgetModel) workspaceBudgetModel {
	return workspaceBudgetModel{
		ID:          types.StringValue(budgetID(configured.WorkspaceID.ValueString(), configured.Interval.ValueString())),
		WorkspaceID: configured.WorkspaceID,
		Interval:    configured.Interval,
		LimitUSD:    types.Float64Value(budget.LimitUSD),
		CreatedAt:   types.StringValue(budget.CreatedAt),
		UpdatedAt:   types.StringValue(budget.UpdatedAt),
	}
}

func stateForBudgetFailure(configured workspaceBudgetModel) workspaceBudgetModel {
	return workspaceBudgetModel{
		ID:          types.StringValue(budgetID(configured.WorkspaceID.ValueString(), configured.Interval.ValueString())),
		WorkspaceID: configured.WorkspaceID,
		Interval:    configured.Interval,
		LimitUSD:    configured.LimitUSD,
		CreatedAt:   types.StringNull(),
		UpdatedAt:   types.StringNull(),
	}
}

func validBudget(model workspaceBudgetModel) error {
	if strings.TrimSpace(model.WorkspaceID.ValueString()) == "" {
		return fmt.Errorf("workspace_id must not be empty")
	}
	limit := model.LimitUSD.ValueFloat64()
	if limit <= 0 || math.IsNaN(limit) || math.IsInf(limit, 0) {
		return fmt.Errorf("limit_usd must be positive")
	}
	if !workspaceBudgetInterval(model.Interval.ValueString()) {
		return fmt.Errorf("interval must be daily, weekly, monthly, or lifetime")
	}
	return nil
}

func workspaceBudgetInterval(interval string) bool {
	switch interval {
	case "daily", "weekly", "monthly", "lifetime":
		return true
	default:
		return false
	}
}

func (r *workspaceBudgetResource) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	var plan workspaceBudgetModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	if response.Diagnostics.HasError() {
		return
	}
	if err := validBudget(plan); err != nil {
		response.Diagnostics.AddError("Invalid workspace budget", err.Error())
		return
	}

	budgets, err := r.client.ListWorkspaceBudgets(ctx, plan.WorkspaceID.ValueString())
	if err != nil {
		response.Diagnostics.AddError("Unable to create workspace budget", err.Error())
		return
	}
	for _, budget := range budgets {
		if budget.ResetInterval == plan.Interval.ValueString() {
			response.Diagnostics.AddError("Workspace budget already exists", fmt.Sprintf("a %s budget already exists for workspace %q; import %q instead", plan.Interval.ValueString(), plan.WorkspaceID.ValueString(), budgetID(plan.WorkspaceID.ValueString(), plan.Interval.ValueString())))
			return
		}
	}

	budget, err := r.client.PutWorkspaceBudget(ctx, plan.WorkspaceID.ValueString(), plan.Interval.ValueString(), plan.LimitUSD.ValueFloat64())
	if err != nil {
		if budget != nil {
			response.Diagnostics.Append(response.State.Set(ctx, stateForBudgetFailure(plan))...)
		}
		response.Diagnostics.AddError("Unable to create workspace budget", err.Error())
		return
	}
	response.Diagnostics.Append(response.State.Set(ctx, flattenBudget(budget, plan))...)
}

func (r *workspaceBudgetResource) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	var state workspaceBudgetModel
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	if response.Diagnostics.HasError() {
		return
	}

	budgets, err := r.client.ListWorkspaceBudgets(ctx, state.WorkspaceID.ValueString())
	if err != nil {
		if isNotFound(err) {
			response.State.RemoveResource(ctx)
			return
		}
		response.Diagnostics.AddError("Unable to read workspace budget", err.Error())
		return
	}
	for i := range budgets {
		if budgets[i].ResetInterval == state.Interval.ValueString() {
			response.Diagnostics.Append(response.State.Set(ctx, flattenBudget(&budgets[i], state))...)
			return
		}
	}
	response.State.RemoveResource(ctx)
}

func (r *workspaceBudgetResource) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	var plan workspaceBudgetModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	if response.Diagnostics.HasError() {
		return
	}
	if err := validBudget(plan); err != nil {
		response.Diagnostics.AddError("Invalid workspace budget", err.Error())
		return
	}

	budget, err := r.client.PutWorkspaceBudget(ctx, plan.WorkspaceID.ValueString(), plan.Interval.ValueString(), plan.LimitUSD.ValueFloat64())
	if err != nil {
		if budget != nil {
			response.Diagnostics.Append(response.State.Set(ctx, stateForBudgetFailure(plan))...)
		}
		response.Diagnostics.AddError("Unable to update workspace budget", err.Error())
		return
	}
	response.Diagnostics.Append(response.State.Set(ctx, flattenBudget(budget, plan))...)
}

func (r *workspaceBudgetResource) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	var state workspaceBudgetModel
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	if response.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteWorkspaceBudget(ctx, state.WorkspaceID.ValueString(), state.Interval.ValueString()); err != nil && !isNotFound(err) {
		response.Diagnostics.AddError("Unable to delete workspace budget", err.Error())
		return
	}

	budgets, err := r.client.ListWorkspaceBudgets(ctx, state.WorkspaceID.ValueString())
	if err != nil {
		if isNotFound(err) {
			return
		}
		response.Diagnostics.AddError("Unable to verify workspace budget deletion", err.Error())
		return
	}
	for _, budget := range budgets {
		if budget.ResetInterval == state.Interval.ValueString() {
			response.Diagnostics.AddError("Unable to verify workspace budget deletion", "budget remains present after delete")
			return
		}
	}
}

func (r *workspaceBudgetResource) ImportState(ctx context.Context, request resource.ImportStateRequest, response *resource.ImportStateResponse) {
	workspace, interval, ok := strings.Cut(request.ID, "/")
	if !ok || strings.TrimSpace(workspace) == "" || strings.TrimSpace(interval) == "" || strings.Contains(interval, "/") || !workspaceBudgetInterval(interval) {
		response.Diagnostics.AddError("Unable to import workspace budget", "expected <workspace_id>/<interval>, where interval is daily, weekly, monthly, or lifetime")
		return
	}
	state := workspaceBudgetModel{
		ID:          types.StringValue(budgetID(workspace, interval)),
		WorkspaceID: types.StringValue(workspace),
		Interval:    types.StringValue(interval),
	}
	response.Diagnostics.Append(response.State.Set(ctx, &state)...)
}

func isNotFound(err error) bool {
	return strings.Contains(err.Error(), "(404)")
}
