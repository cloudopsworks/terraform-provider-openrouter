package provider

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/cloudopsworks/terraform-provider-openrouter/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func configureClient(data any) (*providerData, error) {
	providerData, ok := data.(*providerData)
	if !ok || providerData == nil || providerData.client == nil {
		return nil, fmt.Errorf("provider not configured")
	}
	return providerData, nil
}

func stringPtrValue(v *string) types.String {
	if v == nil {
		return types.StringNull()
	}
	return types.StringValue(*v)
}

func float64PtrValue(v *float64) types.Float64 {
	if v == nil {
		return types.Float64Null()
	}
	return types.Float64Value(*v)
}

func boolPtrValue(v *bool) types.Bool {
	if v == nil {
		return types.BoolNull()
	}
	return types.BoolValue(*v)
}

func stringValueOrNil(v types.String) *string {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	value := v.ValueString()
	return &value
}

func float64ValueOrNil(v types.Float64) *float64 {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	value := v.ValueFloat64()
	return &value
}

func boolValueOrNil(v types.Bool) *bool {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	value := v.ValueBool()
	return &value
}

func setStringValue(ctx context.Context, values []string) (types.Set, diag.Diagnostics) {
	sorted := append([]string(nil), values...)
	sort.Strings(sorted)
	return types.SetValueFrom(ctx, types.StringType, sorted)
}

func setStringValueOrNull(ctx context.Context, values []string) (types.Set, diag.Diagnostics) {
	if len(values) == 0 {
		return types.SetNull(types.StringType), nil
	}
	return setStringValue(ctx, values)
}

func setToStringSlice(ctx context.Context, value types.Set) ([]string, diag.Diagnostics) {
	if value.IsNull() || value.IsUnknown() {
		return nil, nil
	}
	var values []string
	diags := value.ElementsAs(ctx, &values, false)
	sort.Strings(values)
	return values, diags
}

func setInt64Value(ctx context.Context, values []int64) (types.Set, diag.Diagnostics) {
	sorted := append([]int64(nil), values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	return types.SetValueFrom(ctx, types.Int64Type, sorted)
}

func setInt64ValueOrNull(ctx context.Context, values []int64) (types.Set, diag.Diagnostics) {
	if len(values) == 0 {
		return types.SetNull(types.Int64Type), nil
	}
	return setInt64Value(ctx, values)
}

func setToInt64Slice(ctx context.Context, value types.Set) ([]int64, diag.Diagnostics) {
	if value.IsNull() || value.IsUnknown() {
		return nil, nil
	}
	var values []int64
	diags := value.ElementsAs(ctx, &values, false)
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	return values, diags
}

func stringSliceOrNil(values []string) *[]string {
	if len(values) == 0 {
		return nil
	}
	copied := append([]string(nil), values...)
	return &copied
}
func int64SliceOrNil(values []int64) *[]int64 {
	if len(values) == 0 {
		return nil
	}
	copied := append([]int64(nil), values...)
	return &copied
}

func parseCompositeImportID(id string) (string, string, error) {
	parts := strings.SplitN(id, "_", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("expected import format <workspace_id>_<name>")
	}
	return parts[0], parts[1], nil
}

func findAPIKeyByCompositeImport(items []client.APIKey, workspaceID, name string) (*client.APIKey, error) {
	matches := make([]client.APIKey, 0, 1)
	for _, item := range items {
		if item.Name != name || item.WorkspaceID == nil || *item.WorkspaceID != workspaceID {
			continue
		}
		matches = append(matches, item)
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("no API key named %q found in workspace %q", name, workspaceID)
	case 1:
		return &matches[0], nil
	default:
		return nil, fmt.Errorf("multiple API keys named %q found in workspace %q", name, workspaceID)
	}
}

func findWorkspaceByCompositeImport(workspace *client.Workspace, workspaceID, name string) error {
	if workspace == nil {
		return fmt.Errorf("workspace %q not found", workspaceID)
	}
	if workspace.ID != workspaceID {
		return fmt.Errorf("workspace %q did not match requested workspace ID", workspaceID)
	}
	if workspace.Name != name {
		return fmt.Errorf("workspace %q did not match name %q", workspaceID, name)
	}
	return nil
}

func findGuardrailByCompositeImport(items []client.Guardrail, workspaceID, name string) (*client.Guardrail, error) {
	matches := make([]client.Guardrail, 0, 1)
	for _, item := range items {
		if item.Name != name || item.WorkspaceID == nil || *item.WorkspaceID != workspaceID {
			continue
		}
		matches = append(matches, item)
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("no guardrail named %q found in workspace %q", name, workspaceID)
	case 1:
		return &matches[0], nil
	default:
		return nil, fmt.Errorf("multiple guardrails named %q found in workspace %q", name, workspaceID)
	}
}
