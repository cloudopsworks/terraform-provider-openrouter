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
	_ resource.Resource                = &guardrailKeyAssignmentResource{}
	_ resource.ResourceWithImportState = &guardrailKeyAssignmentResource{}
)

type guardrailKeyAssignmentResource struct {
	client *client.Client
}

type guardrailKeyAssignmentResourceModel struct {
	ID          types.String `tfsdk:"id"`
	GuardrailID types.String `tfsdk:"guardrail_id"`
	KeyHash     types.String `tfsdk:"key_hash"`
}

func NewGuardrailKeyAssignmentResource() resource.Resource {
	return &guardrailKeyAssignmentResource{}
}

func (r *guardrailKeyAssignmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_guardrail_key_assignment"
}

func (r *guardrailKeyAssignmentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	providerData, err := configureClient(req.ProviderData)
	if err != nil {
		resp.Diagnostics.AddError("Unable to configure OpenRouter guardrail key assignment resource", err.Error())
		return
	}
	r.client = providerData.client
}

func (r *guardrailKeyAssignmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		MarkdownDescription: "Assign one OpenRouter API key to one guardrail. A key can be assigned to at most one guardrail.",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{Computed: true, MarkdownDescription: "Composite guardrail/key assignment identity."},
			"guardrail_id": resourceschema.StringAttribute{
				Required: true,
			},
			"key_hash": resourceschema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
		},
	}
}

func (r *guardrailKeyAssignmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan guardrailKeyAssignmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := validateGuardrailKeyAssignmentInputs(plan.GuardrailID.ValueString(), plan.KeyHash.ValueString()); err != nil {
		resp.Diagnostics.AddError("Invalid OpenRouter guardrail key assignment", err.Error())
		return
	}
	if err := r.preflightCreate(ctx, plan.GuardrailID.ValueString(), plan.KeyHash.ValueString()); err != nil {
		resp.Diagnostics.AddError("Unable to safely assign OpenRouter API key to guardrail", err.Error())
		return
	}
	if err := r.client.AssignGuardrailKeys(ctx, plan.GuardrailID.ValueString(), []string{plan.KeyHash.ValueString()}); err != nil {
		resp.Diagnostics.AddError("Unable to assign OpenRouter API key to guardrail", err.Error())
		return
	}
	plan.ID = types.StringValue(guardrailKeyAssignmentID(plan.GuardrailID.ValueString(), plan.KeyHash.ValueString()))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.verifyAssignment(ctx, plan.GuardrailID.ValueString(), plan.KeyHash.ValueString()); err != nil {
		// Do not remove here: assigning this key may have displaced a previous
		// guardrail. A failed verification must never leave the key unguarded.
		resp.Diagnostics.AddError("Unable to verify OpenRouter guardrail key assignment", err.Error())
	}
}

func (r *guardrailKeyAssignmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state guardrailKeyAssignmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	assigned, err := r.client.ListGuardrailKeyAssignments(ctx, state.GuardrailID.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "(404)") {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read OpenRouter guardrail key assignment", err.Error())
		return
	}
	if !containsKeyHash(assigned, state.KeyHash.ValueString()) {
		resp.State.RemoveResource(ctx)
		return
	}
	state.ID = types.StringValue(guardrailKeyAssignmentID(state.GuardrailID.ValueString(), state.KeyHash.ValueString()))
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *guardrailKeyAssignmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state guardrailKeyAssignmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Assignment is exclusive. Re-read ownership before removing so Terraform never
	// unassigns a key that another guardrail claimed after this resource was applied.
	assigned, err := r.client.ListGuardrailKeyAssignments(ctx, state.GuardrailID.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "(404)") {
			return
		}
		resp.Diagnostics.AddError("Unable to read OpenRouter guardrail key assignment before removal", err.Error())
		return
	}
	if !containsKeyHash(assigned, state.KeyHash.ValueString()) {
		return
	}
	if err := r.client.RemoveGuardrailKeys(ctx, state.GuardrailID.ValueString(), []string{state.KeyHash.ValueString()}); err != nil {
		if !strings.Contains(err.Error(), "(404)") {
			resp.Diagnostics.AddError("Unable to remove OpenRouter API key from guardrail", err.Error())
		}
		return
	}
	remaining, err := r.client.ListGuardrailKeyAssignments(ctx, state.GuardrailID.ValueString())
	if err != nil {
		if !strings.Contains(err.Error(), "(404)") {
			resp.Diagnostics.AddError("Unable to verify OpenRouter API key removal from guardrail", err.Error())
		}
		return
	}
	if containsKeyHash(remaining, state.KeyHash.ValueString()) {
		resp.Diagnostics.AddError("Unable to verify OpenRouter API key removal from guardrail", "OpenRouter still reports the key assigned to this guardrail after removal.")
	}
}

func (r *guardrailKeyAssignmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	guardrailID, keyHash, err := parseGuardrailKeyAssignmentImportID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Unable to import OpenRouter guardrail key assignment", err.Error())
		return
	}
	state := guardrailKeyAssignmentResourceModel{
		ID:          types.StringValue(guardrailKeyAssignmentID(guardrailID, keyHash)),
		GuardrailID: types.StringValue(guardrailID),
		KeyHash:     types.StringValue(keyHash),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func validateGuardrailKeyAssignmentInputs(guardrailID, keyHash string) error {
	if strings.TrimSpace(guardrailID) == "" {
		return fmt.Errorf("guardrail_id must not be empty or whitespace")
	}
	if strings.TrimSpace(keyHash) == "" {
		return fmt.Errorf("key_hash must not be empty or whitespace")
	}
	return nil
}

func guardrailKeyAssignmentID(guardrailID, keyHash string) string {
	return guardrailID + "/" + keyHash
}

func parseGuardrailKeyAssignmentImportID(id string) (string, string, error) {
	guardrailID, keyHash, ok := strings.Cut(id, "/")
	guardrailID = strings.TrimSpace(guardrailID)
	keyHash = strings.TrimSpace(keyHash)
	if !ok || guardrailID == "" || keyHash == "" {
		return "", "", fmt.Errorf("expected import format <guardrail_id>/<key_hash>")
	}
	return guardrailID, keyHash, nil
}

func containsKeyHash(keyHashes []string, want string) bool {
	for _, hash := range keyHashes {
		if hash == want {
			return true
		}
	}
	return false
}

func (r *guardrailKeyAssignmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan guardrailKeyAssignmentResourceModel
	var state guardrailKeyAssignmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := validateGuardrailKeyAssignmentInputs(plan.GuardrailID.ValueString(), plan.KeyHash.ValueString()); err != nil {
		resp.Diagnostics.AddError("Invalid OpenRouter guardrail key assignment", err.Error())
		return
	}

	// Assigning the target first preserves protection throughout a guardrail move.
	if err := r.preflightUpdate(ctx, state.GuardrailID.ValueString(), plan.GuardrailID.ValueString(), plan.KeyHash.ValueString()); err != nil {
		resp.Diagnostics.AddError("Unable to safely assign OpenRouter API key to guardrail", err.Error())
		return
	}
	if err := r.client.AssignGuardrailKeys(ctx, plan.GuardrailID.ValueString(), []string{plan.KeyHash.ValueString()}); err != nil {
		resp.Diagnostics.AddError("Unable to assign OpenRouter API key to guardrail", err.Error())
		return
	}
	plan.ID = types.StringValue(guardrailKeyAssignmentID(plan.GuardrailID.ValueString(), plan.KeyHash.ValueString()))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.verifyAssignment(ctx, plan.GuardrailID.ValueString(), plan.KeyHash.ValueString()); err != nil {
		// Do not restore or remove: either operation could race an external
		// reassignment and steal or unguard the key.
		resp.Diagnostics.AddError("Unable to verify OpenRouter guardrail key assignment", err.Error())
	}
}

func (r *guardrailKeyAssignmentResource) preflightCreate(ctx context.Context, guardrailID, keyHash string) error {
	bindings, err := r.client.ListGuardrailKeyAssignmentBindings(ctx)
	if err != nil {
		return fmt.Errorf("list existing key assignments: %w", err)
	}
	for _, binding := range bindings {
		if binding.KeyHash == keyHash {
			return fmt.Errorf("key %q is already assigned to guardrail %q; import or manage the existing assignment instead", keyHash, binding.GuardrailID)
		}
	}
	return nil
}

func (r *guardrailKeyAssignmentResource) preflightUpdate(ctx context.Context, priorGuardrailID, desiredGuardrailID, keyHash string) error {
	bindings, err := r.client.ListGuardrailKeyAssignmentBindings(ctx)
	if err != nil {
		return fmt.Errorf("list existing key assignments: %w", err)
	}
	for _, binding := range bindings {
		if binding.KeyHash == keyHash && binding.GuardrailID != priorGuardrailID && binding.GuardrailID != desiredGuardrailID {
			return fmt.Errorf("key %q is now assigned to guardrail %q, not this resource's prior guardrail %q", keyHash, binding.GuardrailID, priorGuardrailID)
		}
	}
	return nil
}

func (r *guardrailKeyAssignmentResource) verifyAssignment(ctx context.Context, guardrailID, keyHash string) error {
	assigned, err := r.client.ListGuardrailKeyAssignments(ctx, guardrailID)
	if err != nil {
		return err
	}
	if !containsKeyHash(assigned, keyHash) {
		return fmt.Errorf("assignment was not returned by OpenRouter")
	}
	return nil
}
