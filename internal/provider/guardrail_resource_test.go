package provider

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	frameworkdatasource "github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/cloudopsworks/terraform-provider-openrouter/internal/client"
)

func TestGuardrailResourceWorkspaceIDSchemaSupportsAPIDefault(t *testing.T) {
	t.Parallel()

	var resp frameworkresource.SchemaResponse
	NewGuardrailResource().Schema(context.Background(), frameworkresource.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected schema diagnostics: %s", resp.Diagnostics)
	}

	attr, ok := resp.Schema.Attributes["workspace_id"].(resourceschema.StringAttribute)
	if !ok {
		t.Fatalf("workspace_id attribute type = %T, want schema.StringAttribute", resp.Schema.Attributes["workspace_id"])
	}
	if !attr.Optional {
		t.Fatal("workspace_id must remain Optional so users can explicitly target a workspace")
	}
	if !attr.Computed {
		t.Fatal("workspace_id must be Computed so API-defaulted workspace IDs can be recorded in state")
	}

	priorWorkspaceID := "f7574683-5741-5f2a-9704-aef2c3ff2a05"
	plannedWorkspaceID, requiresReplace := applyStringPlanModifiersForTest(
		attr.PlanModifiers,
		types.StringNull(),
		types.StringUnknown(),
		types.StringValue(priorWorkspaceID),
	)
	if requiresReplace {
		t.Fatal("workspace_id must not force replacement when an omitted value is already resolved in state")
	}
	if plannedWorkspaceID.IsUnknown() || plannedWorkspaceID.ValueString() != priorWorkspaceID {
		t.Fatalf("workspace_id planned value = %s, want prior state value %q", plannedWorkspaceID.String(), priorWorkspaceID)
	}

	newWorkspaceID := "a1114683-5741-5f2a-9704-aef2c3ff2a05"
	_, requiresReplace = applyStringPlanModifiersForTest(
		attr.PlanModifiers,
		types.StringValue(newWorkspaceID),
		types.StringValue(newWorkspaceID),
		types.StringValue(priorWorkspaceID),
	)
	if !requiresReplace {
		t.Fatal("workspace_id must continue to force replacement when an explicitly configured value changes")
	}
}

func TestGuardrailResourceOfficialAPISchema(t *testing.T) {
	t.Parallel()

	var resp frameworkresource.SchemaResponse
	NewGuardrailResource().Schema(context.Background(), frameworkresource.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected schema diagnostics: %s", resp.Diagnostics)
	}

	for _, name := range []string{"enforce_zdr_anthropic", "enforce_zdr_google", "enforce_zdr_openai", "enforce_zdr_other"} {
		attr, ok := resp.Schema.Attributes[name].(resourceschema.BoolAttribute)
		if !ok {
			t.Fatalf("%s attribute type = %T, want schema.BoolAttribute", name, resp.Schema.Attributes[name])
		}
		if !attr.Optional || !attr.Computed {
			t.Fatalf("%s must be Optional+Computed, got optional=%t computed=%t", name, attr.Optional, attr.Computed)
		}
	}

	builtins, ok := resp.Schema.Attributes["content_filter_builtins"].(resourceschema.ListNestedAttribute)
	if !ok {
		t.Fatalf("content_filter_builtins attribute type = %T, want schema.ListNestedAttribute", resp.Schema.Attributes["content_filter_builtins"])
	}
	if !builtins.Optional || !builtins.Computed {
		t.Fatalf("content_filter_builtins must be Optional+Computed, got optional=%t computed=%t", builtins.Optional, builtins.Computed)
	}
	builtinAttrs := builtins.NestedObject.Attributes
	if attr, ok := builtinAttrs["label"].(resourceschema.StringAttribute); !ok || !attr.Computed || attr.Optional || attr.Required {
		t.Fatalf("content_filter_builtins.label must be computed-only, got %#v", builtinAttrs["label"])
	}
	for _, name := range []string{"slug", "action"} {
		attr, ok := builtinAttrs[name].(resourceschema.StringAttribute)
		if !ok || !attr.Required {
			t.Fatalf("content_filter_builtins.%s must be required string, got %#v", name, builtinAttrs[name])
		}
	}

	custom, ok := resp.Schema.Attributes["content_filters"].(resourceschema.ListNestedAttribute)
	if !ok {
		t.Fatalf("content_filters attribute type = %T, want schema.ListNestedAttribute", resp.Schema.Attributes["content_filters"])
	}
	if !custom.Optional || !custom.Computed {
		t.Fatalf("content_filters must be Optional+Computed, got optional=%t computed=%t", custom.Optional, custom.Computed)
	}
	customAttrs := custom.NestedObject.Attributes
	if attr, ok := customAttrs["label"].(resourceschema.StringAttribute); !ok || !attr.Optional || !attr.Computed {
		t.Fatalf("content_filters.label must be Optional+Computed, got %#v", customAttrs["label"])
	}
}

func TestGuardrailsDataSourceOfficialAPISchema(t *testing.T) {
	t.Parallel()

	var resp frameworkdatasource.SchemaResponse
	NewGuardrailsDataSource().Schema(context.Background(), frameworkdatasource.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected schema diagnostics: %s", resp.Diagnostics)
	}

	items, ok := resp.Schema.Attributes["items"].(datasourceschema.ListNestedAttribute)
	if !ok {
		t.Fatalf("items attribute type = %T, want datasource schema.ListNestedAttribute", resp.Schema.Attributes["items"])
	}
	itemAttrs := items.NestedObject.Attributes
	for _, name := range []string{"enforce_zdr_anthropic", "enforce_zdr_google", "enforce_zdr_openai", "enforce_zdr_other"} {
		attr, ok := itemAttrs[name].(datasourceschema.BoolAttribute)
		if !ok || !attr.Computed {
			t.Fatalf("items.%s must be computed bool, got %#v", name, itemAttrs[name])
		}
	}
	for _, name := range []string{"content_filter_builtins", "content_filters"} {
		attr, ok := itemAttrs[name].(datasourceschema.ListNestedAttribute)
		if !ok || !attr.Computed {
			t.Fatalf("items.%s must be computed nested list, got %#v", name, itemAttrs[name])
		}
	}
}

func TestFlattenGuardrailModelStoresReturnedWorkspaceID(t *testing.T) {
	t.Parallel()

	workspaceID := "f7574683-5741-5f2a-9704-aef2c3ff2a05"
	model, diags := flattenGuardrailModel(context.Background(), client.Guardrail{
		ID:          "guardrail-id",
		Name:        "test-guardrail",
		WorkspaceID: &workspaceID,
	})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %s", diags)
	}

	if model.WorkspaceID.IsNull() || model.WorkspaceID.IsUnknown() {
		t.Fatal("workspace_id should be known when the API returns a default workspace ID")
	}
	if got := model.WorkspaceID.ValueString(); got != workspaceID {
		t.Fatalf("workspace_id = %q, want %q", got, workspaceID)
	}
}

func TestFlattenGuardrailModelStoresOfficialGuardrailFields(t *testing.T) {
	t.Parallel()

	workspaceID := "f7574683-5741-5f2a-9704-aef2c3ff2a05"
	description := "production guardrail"
	resetInterval := "monthly"
	updatedAt := "2026-06-30T00:00:00Z"
	builtinLabel := "[EMAIL]"
	scanScope := "user_only"
	customLabel := "[API_KEY]"
	trueValue := true
	falseValue := false

	model, diags := flattenGuardrailModel(context.Background(), client.Guardrail{
		ID:               "guardrail-id",
		Name:             "test-guardrail",
		WorkspaceID:      &workspaceID,
		Description:      &description,
		ResetInterval:    &resetInterval,
		AllowedProviders: []string{"openai"},
		ContentFilterBuiltins: []client.ContentFilterBuiltinEntry{
			{Slug: "email", Action: "redact", Label: &builtinLabel},
			{Slug: "regex-prompt-injection", Action: "flag", ScanScope: &scanScope},
		},
		ContentFilters: []client.ContentFilterEntry{
			{Pattern: "SECRET_[A-Z0-9]+", Action: "redact", Label: &customLabel},
		},
		EnforceZDRAnthropic: &trueValue,
		EnforceZDRGoogle:    &falseValue,
		EnforceZDROpenAI:    &trueValue,
		EnforceZDROther:     &falseValue,
		CreatedAt:           "2026-06-30T00:00:00Z",
		UpdatedAt:           &updatedAt,
	})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %s", diags)
	}

	if got := model.EnforceZDRAnthropic.ValueBool(); got != true {
		t.Fatalf("enforce_zdr_anthropic = %t, want true", got)
	}
	if got := model.EnforceZDRGoogle.ValueBool(); got != false {
		t.Fatalf("enforce_zdr_google = %t, want false", got)
	}
	if model.ContentFilterBuiltins.IsNull() || len(model.ContentFilterBuiltins.Elements()) != 2 {
		t.Fatalf("content_filter_builtins = %s, want two entries", model.ContentFilterBuiltins.String())
	}
	var builtins []contentFilterBuiltinModel
	diags = model.ContentFilterBuiltins.ElementsAs(context.Background(), &builtins, false)
	if diags.HasError() {
		t.Fatalf("decode builtin filters: %s", diags)
	}
	if got := builtins[0].Label.ValueString(); got != builtinLabel {
		t.Fatalf("first builtin label = %q, want %q", got, builtinLabel)
	}
	if got := builtins[1].ScanScope.ValueString(); got != scanScope {
		t.Fatalf("second builtin scan_scope = %q, want %q", got, scanScope)
	}

	var customFilters []contentFilterModel
	diags = model.ContentFilters.ElementsAs(context.Background(), &customFilters, false)
	if diags.HasError() {
		t.Fatalf("decode custom filters: %s", diags)
	}
	if len(customFilters) != 1 || customFilters[0].Pattern.ValueString() != "SECRET_[A-Z0-9]+" || customFilters[0].Label.ValueString() != customLabel {
		t.Fatalf("content_filters = %#v, want configured custom regex filter", customFilters)
	}
}

func TestFlattenGuardrailModelPreservesLegacyEnforceZDRWhenAPIOnlyReturnsProviderFlags(t *testing.T) {
	t.Parallel()

	trueValue := true
	model, diags := flattenGuardrailModel(context.Background(), client.Guardrail{
		ID:                  "guardrail-id",
		Name:                "test-guardrail",
		EnforceZDR:          nil,
		EnforceZDRAnthropic: &trueValue,
	}, guardrailResourceModel{EnforceZDR: types.BoolValue(true)})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %s", diags)
	}
	if model.EnforceZDR.IsNull() || model.EnforceZDR.IsUnknown() || !model.EnforceZDR.ValueBool() {
		t.Fatalf("legacy enforce_zdr = %s, want preserved true", model.EnforceZDR.String())
	}
}

func TestFlattenGuardrailModelPreservesExplicitEmptyFilterLists(t *testing.T) {
	t.Parallel()

	model, diags := flattenGuardrailModel(context.Background(), client.Guardrail{
		ID:   "guardrail-id",
		Name: "test-guardrail",
	}, guardrailResourceModel{
		ContentFilterBuiltins: emptyContentFilterBuiltinsList(),
		ContentFilters:        emptyContentFiltersList(),
	})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %s", diags)
	}
	if model.ContentFilterBuiltins.IsNull() || len(model.ContentFilterBuiltins.Elements()) != 0 {
		t.Fatalf("content_filter_builtins = %s, want preserved empty list", model.ContentFilterBuiltins.String())
	}
	if model.ContentFilters.IsNull() || len(model.ContentFilters.Elements()) != 0 {
		t.Fatalf("content_filters = %s, want preserved empty list", model.ContentFilters.String())
	}
}

func TestBuildGuardrailUpsertRequestOfficialFields(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scanScope := "user_only"
	plan := guardrailResourceModel{
		Name:             types.StringValue("prod"),
		WorkspaceID:      types.StringValue("ws-1"),
		AllowedProviders: mustStringSet(t, "anthropic", "openai"),
	}
	config := guardrailResourceModel{
		ContentFilterBuiltins: mustList(t, contentFilterBuiltinObjectType,
			mustObject(t, contentFilterBuiltinAttrTypes, map[string]attr.Value{
				"slug":       types.StringValue("regex-prompt-injection"),
				"action":     types.StringValue("flag"),
				"label":      types.StringValue("[IGNORED]"),
				"scan_scope": types.StringValue(scanScope),
			}),
		),
		ContentFilters: mustList(t, contentFilterObjectType,
			mustObject(t, contentFilterAttrTypes, map[string]attr.Value{
				"pattern": types.StringValue("SECRET_[A-Z0-9]+"),
				"action":  types.StringValue("redact"),
				"label":   types.StringValue("[SECRET]"),
			}),
		),
		EnforceZDRAnthropic: types.BoolValue(true),
		EnforceZDRGoogle:    types.BoolValue(false),
		EnforceZDROpenAI:    types.BoolValue(true),
		EnforceZDROther:     types.BoolValue(false),
	}

	request, diags := buildGuardrailUpsertRequest(ctx, plan, config, true)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %s", diags)
	}
	if request.WorkspaceID == nil || *request.WorkspaceID != "ws-1" {
		t.Fatalf("workspace_id = %#v, want ws-1", request.WorkspaceID)
	}
	if request.ContentFilterBuiltins == nil || len(*request.ContentFilterBuiltins) != 1 {
		t.Fatalf("content_filter_builtins = %#v, want one entry", request.ContentFilterBuiltins)
	}
	if got := (*request.ContentFilterBuiltins)[0]; got.Slug != "regex-prompt-injection" || got.Action != "flag" || got.ScanScope == nil || *got.ScanScope != scanScope {
		t.Fatalf("builtin request entry = %#v, want slug/action/scan_scope", got)
	}
	if request.ContentFilters == nil || len(*request.ContentFilters) != 1 {
		t.Fatalf("content_filters = %#v, want one entry", request.ContentFilters)
	}
	if got := (*request.ContentFilters)[0]; got.Pattern != "SECRET_[A-Z0-9]+" || got.Action != "redact" || got.Label == nil || *got.Label != "[SECRET]" {
		t.Fatalf("custom filter request entry = %#v, want pattern/action/label", got)
	}
	if request.EnforceZDRAnthropic == nil || !*request.EnforceZDRAnthropic || request.EnforceZDRGoogle == nil || *request.EnforceZDRGoogle {
		t.Fatalf("per-provider ZDR request values not preserved: %#v", request)
	}

	payload, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal request payload: %v", err)
	}
	builtinPayload := decoded["content_filter_builtins"].([]any)[0].(map[string]any)
	if _, ok := builtinPayload["label"]; ok {
		t.Fatalf("builtin request payload included deprecated label: %s", payload)
	}
}

func TestBuildGuardrailUpsertRequestOmitsNullFilterListsAndClearsExplicitEmptyLists(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	plan := guardrailResourceModel{Name: types.StringValue("prod")}

	request, diags := buildGuardrailUpsertRequest(ctx, plan, guardrailResourceModel{
		ContentFilterBuiltins: types.ListNull(contentFilterBuiltinObjectType),
		ContentFilters:        types.ListNull(contentFilterObjectType),
	}, false)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %s", diags)
	}
	if request.ContentFilterBuiltins != nil || request.ContentFilters != nil {
		t.Fatalf("null configured filters must be omitted, got %#v", request)
	}

	request, diags = buildGuardrailUpsertRequest(ctx, plan, guardrailResourceModel{
		ContentFilterBuiltins: emptyContentFilterBuiltinsList(),
		ContentFilters:        emptyContentFiltersList(),
	}, false)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %s", diags)
	}
	if request.ContentFilterBuiltins == nil || *request.ContentFilterBuiltins != nil {
		t.Fatalf("empty builtin filters must be represented as pointer to nil slice for JSON null, got %#v", request.ContentFilterBuiltins)
	}
	if request.ContentFilters == nil || *request.ContentFilters != nil {
		t.Fatalf("empty custom filters must be represented as pointer to nil slice for JSON null, got %#v", request.ContentFilters)
	}
	payload, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("marshal clear request: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal clear request: %v", err)
	}
	if value, ok := decoded["content_filter_builtins"]; !ok || value != nil {
		t.Fatalf("content_filter_builtins payload = %#v, want explicit null in %s", decoded["content_filter_builtins"], payload)
	}
	if value, ok := decoded["content_filters"]; !ok || value != nil {
		t.Fatalf("content_filters payload = %#v, want explicit null in %s", decoded["content_filters"], payload)
	}
}

func TestBuildGuardrailUpsertRequestRejectsContradictoryMixedZDR(t *testing.T) {
	t.Parallel()

	_, diags := buildGuardrailUpsertRequest(context.Background(),
		guardrailResourceModel{Name: types.StringValue("prod")},
		guardrailResourceModel{
			EnforceZDR:          types.BoolValue(true),
			EnforceZDRAnthropic: types.BoolValue(false),
		},
		false,
	)
	if !diags.HasError() {
		t.Fatal("expected contradictory ZDR configuration to return diagnostics")
	}

	_, diags = buildGuardrailUpsertRequest(context.Background(),
		guardrailResourceModel{Name: types.StringValue("prod")},
		guardrailResourceModel{
			EnforceZDR:          types.BoolValue(true),
			EnforceZDRAnthropic: types.BoolValue(true),
		},
		false,
	)
	if diags.HasError() {
		t.Fatalf("consistent mixed ZDR configuration should be allowed: %s", diags)
	}
}

func mustObject(t *testing.T, attrTypes map[string]attr.Type, values map[string]attr.Value) attr.Value {
	t.Helper()
	object, diags := types.ObjectValue(attrTypes, values)
	if diags.HasError() {
		t.Fatalf("build object value: %s", diags)
	}
	return object
}

func mustList(t *testing.T, elemType attr.Type, values ...attr.Value) types.List {
	t.Helper()
	list, diags := types.ListValue(elemType, values)
	if diags.HasError() {
		t.Fatalf("build list value: %s", diags)
	}
	return list
}

func mustStringSet(t *testing.T, values ...string) types.Set {
	t.Helper()
	set, diags := types.SetValueFrom(context.Background(), types.StringType, values)
	if diags.HasError() {
		t.Fatalf("build string set: %s", diags)
	}
	return set
}
