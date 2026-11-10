package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudopsworks/terraform-provider-openrouter/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.ResourceWithImportState = &scimGroupMappingResource{}
var _ resource.ResourceWithValidateConfig = &scimGroupMappingResource{}

type scimGroupMappingResource struct{ client *client.Client }
type scimGroupMappingModel struct {
	ID             types.String `tfsdk:"id"`
	SCIMGroupID    types.String `tfsdk:"scim_group_id"`
	WorkspaceID    types.String `tfsdk:"workspace_id"`
	Role           types.String `tfsdk:"role"`
	KeepMembers    types.Bool   `tfsdk:"keep_members"`
	OrganizationID types.String `tfsdk:"organization_id"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
}

func NewSCIMGroupMappingResource() resource.Resource { return &scimGroupMappingResource{} }
func (r *scimGroupMappingResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_scim_group_mapping"
}
func (r *scimGroupMappingResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	data, err := configureClient(req.ProviderData)
	if err != nil {
		resp.Diagnostics.AddError("Unable to configure SCIM mapping", err.Error())
		return
	}
	r.client = data.client
}
func (r *scimGroupMappingResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	immutable := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = resourceschema.Schema{MarkdownDescription: "Manage a SCIM group-to-workspace role mapping, not IdP users/groups. Enterprise SCIM provisioning is required. Existing bindings must be imported. Deletion preserves workspace memberships by default; concurrent external mapping writes are not protected by API compare-and-swap.", Attributes: map[string]resourceschema.Attribute{
		"id":              resourceschema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		"scim_group_id":   resourceschema.StringAttribute{Required: true, PlanModifiers: immutable},
		"workspace_id":    resourceschema.StringAttribute{Required: true, PlanModifiers: immutable},
		"role":            resourceschema.StringAttribute{Required: true, MarkdownDescription: "Mapping role: admin or member. Updating it changes effective access granted by this mapping."},
		"keep_members":    resourceschema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Explicit deletion policy sent to OpenRouter. Defaults to true: leave memberships as manually managed when deleting the mapping. Setting false removes access granted solely by this mapping; do not overlap that access with a manual-membership owner."},
		"organization_id": resourceschema.StringAttribute{Computed: true},
		"created_at":      resourceschema.StringAttribute{Computed: true},
		"updated_at":      resourceschema.StringAttribute{Computed: true},
	}}
}
func identityConfigValue(v types.String) bool {
	return !v.IsNull() && !v.IsUnknown() && v.ValueString() != "" && strings.TrimSpace(v.ValueString()) == v.ValueString() && !strings.ContainsAny(v.ValueString(), "/\\?#")
}
func (r *scimGroupMappingResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	for _, name := range []string{"scim_group_id", "workspace_id", "role"} {
		var v types.String
		resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root(name), &v)...)
		if v.IsUnknown() || v.IsNull() {
			continue
		}
		if !identityConfigValue(v) {
			resp.Diagnostics.AddAttributeError(path.Root(name), "Invalid SCIM mapping value", "Values must be nonempty without slashes or surrounding whitespace.")
		}
		if name == "role" && v.ValueString() != "admin" && v.ValueString() != "member" {
			resp.Diagnostics.AddAttributeError(path.Root(name), "Invalid SCIM mapping role", "Role must be admin or member.")
		}
	}
}
func (m scimGroupMappingModel) validate() error {
	if !identityConfigValue(m.SCIMGroupID) || !identityConfigValue(m.WorkspaceID) || (m.Role.ValueString() != "admin" && m.Role.ValueString() != "member") || m.KeepMembers.IsNull() || m.KeepMembers.IsUnknown() {
		return fmt.Errorf("SCIM group, workspace, role and keep_members must be known valid values")
	}
	return nil
}
func scimNotFound(err error) bool {
	return err != nil && strings.HasPrefix(err.Error(), "openrouter API error (404)")
}
func (m scimGroupMappingModel) matches(remote *client.SCIMGroupMapping) bool {
	return remote.ID == m.ID.ValueString() && remote.SCIMGroupID == m.SCIMGroupID.ValueString() && remote.WorkspaceID == m.WorkspaceID.ValueString() && (m.OrganizationID.IsNull() || m.OrganizationID.IsUnknown() || remote.OrganizationID == m.OrganizationID.ValueString())
}
func flattenSCIMMapping(m client.SCIMGroupMapping, keep types.Bool) scimGroupMappingModel {
	return scimGroupMappingModel{
		ID:             types.StringValue(m.ID),
		SCIMGroupID:    types.StringValue(m.SCIMGroupID),
		WorkspaceID:    types.StringValue(m.WorkspaceID),
		Role:           types.StringValue(m.Role),
		KeepMembers:    keep,
		OrganizationID: types.StringValue(m.OrganizationID),
		CreatedAt:      types.StringValue(m.CreatedAt),
		UpdatedAt:      types.StringValue(m.UpdatedAt),
	}
}
func (r *scimGroupMappingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan scimGroupMappingModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := plan.validate(); err != nil {
		resp.Diagnostics.AddError("Invalid SCIM mapping", err.Error())
		return
	}
	mappings, err := r.client.ListSCIMGroupMappings(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to check existing SCIM mappings", err.Error())
		return
	}
	for _, m := range mappings {
		if m.SCIMGroupID == plan.SCIMGroupID.ValueString() && m.WorkspaceID == plan.WorkspaceID.ValueString() {
			resp.Diagnostics.AddError("SCIM mapping already exists", "Import the existing mapping instead of adopting or reapplying its access during creation.")
			return
		}
	}
	created, err := r.client.CreateSCIMGroupMapping(ctx, client.SCIMGroupMappingCreateRequest{SCIMGroupID: plan.SCIMGroupID.ValueString(), WorkspaceID: plan.WorkspaceID.ValueString(), Role: plan.Role.ValueString()})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create SCIM mapping", err.Error())
		return
	}
	state := flattenSCIMMapping(*created, plan.KeepMembers)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
func (r *scimGroupMappingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state scimGroupMappingModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	remote, err := r.client.GetSCIMGroupMapping(ctx, state.ID.ValueString())
	if err != nil {
		if scimNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read SCIM mapping", err.Error())
		return
	}
	if !state.matches(remote) {
		resp.Diagnostics.AddError("SCIM mapping identity changed", "Refusing to adopt a mapping with a different group, workspace or organization.")
		return
	}
	updated := flattenSCIMMapping(*remote, state.KeepMembers)
	resp.Diagnostics.Append(resp.State.Set(ctx, &updated)...)
}
func (r *scimGroupMappingResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state scimGroupMappingModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := plan.validate(); err != nil {
		resp.Diagnostics.AddError("Invalid SCIM mapping", err.Error())
		return
	}
	current, err := r.client.GetSCIMGroupMapping(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read SCIM mapping before update", err.Error())
		return
	}
	if !state.matches(current) || !plan.SCIMGroupID.Equal(state.SCIMGroupID) || !plan.WorkspaceID.Equal(state.WorkspaceID) {
		resp.Diagnostics.AddError("SCIM mapping identity changed", "Group and workspace changes require replacement; refusing an unrelated mapping mutation.")
		return
	}
	updated := current
	if current.Role != plan.Role.ValueString() {
		updated, err = r.client.UpdateSCIMGroupMapping(ctx, state.ID.ValueString(), plan.Role.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Unable to update SCIM mapping", err.Error())
			return
		}
		if !state.matches(updated) {
			resp.Diagnostics.AddError("SCIM mapping identity changed", "The update response changed group, workspace or organization. Existing identity remains tracked; reconcile before retrying.")
			return
		}
	}
	result := flattenSCIMMapping(*updated, plan.KeepMembers)
	resp.Diagnostics.Append(resp.State.Set(ctx, &result)...)
}
func (r *scimGroupMappingResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state scimGroupMappingModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := state.validate(); err != nil {
		resp.Diagnostics.AddError("Invalid SCIM mapping deletion policy", err.Error())
		return
	}
	current, err := r.client.GetSCIMGroupMapping(ctx, state.ID.ValueString())
	if err != nil {
		if scimNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Unable to read SCIM mapping before deletion", err.Error())
		return
	}
	if !state.matches(current) {
		resp.Diagnostics.AddError("SCIM mapping identity changed", "Refusing to delete a mapping with different ownership.")
		return
	}
	if err := r.client.DeleteSCIMGroupMapping(ctx, state.ID.ValueString(), state.KeepMembers.ValueBool()); err != nil && !scimNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete SCIM mapping", err.Error())
	}
}
func (r *scimGroupMappingResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	remote, err := r.client.GetSCIMGroupMapping(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Unable to import SCIM mapping", err.Error())
		return
	}
	state := flattenSCIMMapping(*remote, types.BoolValue(true))
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
