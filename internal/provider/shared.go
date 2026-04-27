package provider

import (
	"context"
	"fmt"
	"sort"
	"strings"

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

func parseCompositeImportID(id string) (string, string, error) {
	parts := strings.SplitN(id, "_", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("expected import format <workspace_id>_<name>")
	}
	return parts[0], parts[1], nil
}
