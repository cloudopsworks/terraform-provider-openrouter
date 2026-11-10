package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudopsworks/terraform-provider-openrouter/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                   = &policyAssignmentResource{}
	_ resource.ResourceWithImportState    = &policyAssignmentResource{}
	_ resource.ResourceWithValidateConfig = &policyAssignmentResource{}
)

type policyAssignmentResource struct {
	client           *client.Client
	subjectType      string
	subjectAttribute string
	typeSuffix       string
}

type policyAssignmentModel struct{ ID, WorkspaceID, PolicyType, SubjectID, PolicyID types.String }

func NewAPIKeyPolicyAssignmentResource() resource.Resource {
	return &policyAssignmentResource{subjectType: "api-keys", subjectAttribute: "key_hash", typeSuffix: "api_key_policy_assignment"}
}
func NewMemberPolicyAssignmentResource() resource.Resource {
	return &policyAssignmentResource{subjectType: "members", subjectAttribute: "member_id", typeSuffix: "member_policy_assignment"}
}
func (r *policyAssignmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + r.typeSuffix
}
func (r *policyAssignmentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	data, err := configureClient(req.ProviderData)
	if err != nil {
		resp.Diagnostics.AddError("Unable to configure OpenRouter policy assignment", err.Error())
		return
	}
	r.client = data.client
}
func (r *policyAssignmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	required := func(description string) resourceschema.StringAttribute {
		return resourceschema.StringAttribute{Required: true, MarkdownDescription: description, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}}
	}
	resp.Schema = resourceschema.Schema{MarkdownDescription: "Manage one direct policy assignment with optimistic concurrency. Existing bindings must be imported; inherited defaults and opt-outs are not managed.", Attributes: map[string]resourceschema.Attribute{
		"id":               resourceschema.StringAttribute{Computed: true, MarkdownDescription: "Composite workspace/type/subject/policy identity."},
		"workspace_id":     required("Workspace UUID. Replacement when changed."),
		"policy_type":      required("Policy type: budget, model_availability, or moderation. Replacement when changed."),
		"policy_id":        required("Ordinary workspace policy UUID. Replacement when changed."),
		r.subjectAttribute: required("Existing subject identifier. Replacement when changed."),
	}}
}

// ValidateConfig rejects malformed known identity fields before an API call;
// unknown references are validated after Terraform resolves them during apply.
func (r *policyAssignmentResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	for _, name := range []string{"workspace_id", "policy_type", r.subjectAttribute, "policy_id"} {
		var value types.String
		resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root(name), &value)...)
		if value.IsUnknown() || value.IsNull() {
			continue
		}
		text := value.ValueString()
		if text == "" || strings.TrimSpace(text) != text || strings.Contains(text, "/") {
			resp.Diagnostics.AddAttributeError(path.Root(name), "Invalid OpenRouter policy assignment identifier", "Identifiers must be nonempty and contain no slashes or surrounding whitespace.")
		}
		if name == "policy_type" && text != "budget" && text != "model_availability" && text != "moderation" {
			resp.Diagnostics.AddAttributeError(path.Root(name), "Unsupported OpenRouter policy type", "Supported direct policy types are budget, model_availability, and moderation.")
		}
	}
}

func policyAssignmentNotFound(err error) bool {
	return err != nil && strings.HasPrefix(err.Error(), "openrouter API error (404)")
}

type policyAssignmentAttributes interface {
	GetAttribute(context.Context, path.Path, any) diag.Diagnostics
}

func (r *policyAssignmentResource) readModel(ctx context.Context, source policyAssignmentAttributes) (policyAssignmentModel, diag.Diagnostics) {
	var model policyAssignmentModel
	var diags diag.Diagnostics
	for _, item := range []struct {
		name  string
		value *types.String
	}{{"id", &model.ID}, {"workspace_id", &model.WorkspaceID}, {"policy_type", &model.PolicyType}, {r.subjectAttribute, &model.SubjectID}, {"policy_id", &model.PolicyID}} {
		diags.Append(source.GetAttribute(ctx, path.Root(item.name), item.value)...)
	}
	return model, diags
}
func (r *policyAssignmentResource) writeModel(ctx context.Context, target interface {
	Set(context.Context, any) diag.Diagnostics
}, model policyAssignmentModel) diag.Diagnostics {
	values := map[string]attr.Value{"id": model.ID, "workspace_id": model.WorkspaceID, "policy_type": model.PolicyType, r.subjectAttribute: model.SubjectID, "policy_id": model.PolicyID}
	attributeTypes := make(map[string]attr.Type, len(values))
	for name := range values {
		attributeTypes[name] = types.StringType
	}
	object, diags := types.ObjectValue(attributeTypes, values)
	if diags.HasError() {
		return diags
	}
	diags.Append(target.Set(ctx, object)...)
	return diags
}
func (m policyAssignmentModel) validate() error {
	for _, v := range []types.String{m.WorkspaceID, m.PolicyType, m.SubjectID, m.PolicyID} {
		if v.IsNull() || v.IsUnknown() || strings.TrimSpace(v.ValueString()) != v.ValueString() || v.ValueString() == "" || strings.Contains(v.ValueString(), "/") {
			return fmt.Errorf("workspace, policy type, subject and policy IDs must be known, nonempty identifiers without slashes or surrounding whitespace")
		}
	}
	switch m.PolicyType.ValueString() {
	case "budget", "model_availability", "moderation":
	default:
		return fmt.Errorf("policy_type must be budget, model_availability, or moderation")
	}
	return nil
}
func (m policyAssignmentModel) identity() string {
	return strings.Join([]string{m.WorkspaceID.ValueString(), m.PolicyType.ValueString(), m.SubjectID.ValueString(), m.PolicyID.ValueString()}, "/")
}
func (r *policyAssignmentResource) assignment(ctx context.Context, m policyAssignmentModel) (*client.PolicyAssignmentState, error) {
	if err := m.validate(); err != nil {
		return nil, err
	}
	return r.client.GetPolicyAssignment(ctx, r.subjectType, m.SubjectID.ValueString(), m.WorkspaceID.ValueString(), m.PolicyType.ValueString())
}
func (r *policyAssignmentResource) change(ctx context.Context, m policyAssignmentModel, current client.PolicyAssignmentState, assigned bool) error {
	return r.client.ChangePolicyAssignment(ctx, r.subjectType, m.SubjectID.ValueString(), client.ChangePolicyAssignmentRequest{WorkspaceID: m.WorkspaceID.ValueString(), PolicyType: m.PolicyType.ValueString(), PolicyID: m.PolicyID.ValueString(), Assigned: assigned, ExpectedAssignment: current})
}
func policyAssignmentContains(ids []string, id string) bool {
	for _, candidate := range ids {
		if candidate == id {
			return true
		}
	}
	return false
}
func (r *policyAssignmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	model, diags := r.readModel(ctx, req.Plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	current, err := r.assignment(ctx, model)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read OpenRouter policy assignment before creation", err.Error())
		return
	}
	if len(current.PolicyIDs) != 0 || current.OptedOut || policyAssignmentContains(current.WorkspaceDefaultPolicyIDs, model.PolicyID.ValueString()) {
		resp.Diagnostics.AddError("Existing OpenRouter policy assignment", "Import existing direct bindings instead. This resource does not replace direct policies or manage inherited defaults or opt-outs.")
		return
	}
	if err := r.client.ValidateDirectAssignmentPolicy(ctx, model.WorkspaceID.ValueString(), model.PolicyType.ValueString(), model.PolicyID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Unsupported OpenRouter policy assignment", err.Error())
		return
	}
	if err := r.change(ctx, model, *current, true); err != nil {
		resp.Diagnostics.AddError("Unable to assign OpenRouter policy", err.Error())
		return
	}
	model.ID = types.StringValue(model.identity())
	resp.Diagnostics.Append(r.writeModel(ctx, &resp.State, model)...)
	if resp.Diagnostics.HasError() {
		return
	}
	verified, err := r.assignment(ctx, model)
	if err != nil {
		resp.Diagnostics.AddError("Unable to verify OpenRouter policy assignment", err.Error())
		return
	}
	if !policyAssignmentContains(verified.PolicyIDs, model.PolicyID.ValueString()) || verified.OptedOut != current.OptedOut {
		resp.Diagnostics.AddError("Unable to verify OpenRouter policy assignment", "OpenRouter did not report the requested direct assignment with the original opt-out state. The binding remains tracked for recovery.")
	}
}
func (r *policyAssignmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	model, diags := r.readModel(ctx, req.State)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	current, err := r.assignment(ctx, model)
	if err != nil {
		if policyAssignmentNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read OpenRouter policy assignment", err.Error())
		return
	}
	if !policyAssignmentContains(current.PolicyIDs, model.PolicyID.ValueString()) {
		resp.State.RemoveResource(ctx)
		return
	}
	model.ID = types.StringValue(model.identity())
	resp.Diagnostics.Append(r.writeModel(ctx, &resp.State, model)...)
}
func (r *policyAssignmentResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("OpenRouter policy assignment replacement required", "All assignment identity fields require replacement; in-place updates are not supported.")
}
func (r *policyAssignmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	model, diags := r.readModel(ctx, req.State)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	current, err := r.assignment(ctx, model)
	if err != nil {
		if policyAssignmentNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Unable to read OpenRouter policy assignment before removal", err.Error())
		return
	}
	if !policyAssignmentContains(current.PolicyIDs, model.PolicyID.ValueString()) {
		return
	}
	if current.OptedOut || policyAssignmentContains(current.WorkspaceDefaultPolicyIDs, model.PolicyID.ValueString()) {
		resp.Diagnostics.AddError("Unsafe OpenRouter policy assignment removal", "The policy is inherited or the subject is opted out. Refusing to change inherited-default or opt-out behavior.")
		return
	}
	if err := r.client.ValidateDirectAssignmentPolicy(ctx, model.WorkspaceID.ValueString(), model.PolicyType.ValueString(), model.PolicyID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Unsupported OpenRouter policy assignment removal", err.Error())
		return
	}
	if err := r.change(ctx, model, *current, false); err != nil {
		resp.Diagnostics.AddError("Unable to unassign OpenRouter policy", err.Error())
		return
	}
	verified, err := r.assignment(ctx, model)
	if err != nil {
		if policyAssignmentNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Unable to verify OpenRouter policy assignment removal", err.Error())
		return
	}
	if policyAssignmentContains(verified.PolicyIDs, model.PolicyID.ValueString()) || verified.OptedOut != current.OptedOut {
		resp.Diagnostics.AddError("Unable to verify OpenRouter policy assignment removal", "OpenRouter still reports the direct policy or changed the opt-out state.")
	}
}
func (r *policyAssignmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 4 {
		resp.Diagnostics.AddError("Invalid OpenRouter policy assignment import", "Expected <workspace_id>/<policy_type>/<subject_id>/<policy_id>.")
		return
	}
	model := policyAssignmentModel{WorkspaceID: types.StringValue(parts[0]), PolicyType: types.StringValue(parts[1]), SubjectID: types.StringValue(parts[2]), PolicyID: types.StringValue(parts[3])}
	current, err := r.assignment(ctx, model)
	if err != nil {
		resp.Diagnostics.AddError("Unable to import OpenRouter policy assignment", err.Error())
		return
	}
	if !policyAssignmentContains(current.PolicyIDs, model.PolicyID.ValueString()) {
		resp.Diagnostics.AddError("Unable to import OpenRouter policy assignment", "The requested policy is not directly assigned to this subject.")
		return
	}
	if current.OptedOut || policyAssignmentContains(current.WorkspaceDefaultPolicyIDs, model.PolicyID.ValueString()) {
		resp.Diagnostics.AddError("Unsupported OpenRouter policy assignment import", "Inherited-default bindings and opt-outs are not supported.")
		return
	}
	if err := r.client.ValidateDirectAssignmentPolicy(ctx, model.WorkspaceID.ValueString(), model.PolicyType.ValueString(), model.PolicyID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Unsupported OpenRouter policy assignment import", err.Error())
		return
	}
	model.ID = types.StringValue(model.identity())
	resp.Diagnostics.Append(r.writeModel(ctx, &resp.State, model)...)
}
