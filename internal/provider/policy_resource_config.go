package provider

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/cloudopsworks/terraform-provider-openrouter/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	rs "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

type budgetPolicyConfigModel struct {
	LimitUSD             types.Float64 `tfsdk:"limit_usd"`
	ResetInterval        types.String  `tfsdk:"reset_interval"`
	IncludeBYOKInBudgets types.Bool    `tfsdk:"include_byok_in_budgets"`
}
type modelAvailabilityPolicyConfigModel struct {
	AllowSafetyRetentionGoogle     types.Bool `tfsdk:"allow_safety_retention_google"`
	AllowSafetyRetentionOther      types.Bool `tfsdk:"allow_safety_retention_other"`
	AllowedDataRegions             types.Set  `tfsdk:"allowed_data_regions"`
	AllowedModels                  types.Set  `tfsdk:"allowed_models"`
	AllowedProviders               types.Set  `tfsdk:"allowed_providers"`
	EnableFreeModelPublication     types.Bool `tfsdk:"enable_free_model_publication"`
	EnableFreeModelTraining        types.Bool `tfsdk:"enable_free_model_training"`
	EnablePaidModelTraining        types.Bool `tfsdk:"enable_paid_model_training"`
	EnforceFirstPartyHarnessModels types.Bool `tfsdk:"enforce_first_party_harness_models"`
	EnforceZDR                     types.Bool `tfsdk:"enforce_zdr"`
	EnforceZDRAnthropic            types.Bool `tfsdk:"enforce_zdr_anthropic"`
	EnforceZDRGoogle               types.Bool `tfsdk:"enforce_zdr_google"`
	EnforceZDROpenAI               types.Bool `tfsdk:"enforce_zdr_openai"`
	EnforceZDROther                types.Bool `tfsdk:"enforce_zdr_other"`
	EnforceZDRXAI                  types.Bool `tfsdk:"enforce_zdr_xai"`
	IgnoredModels                  types.Set  `tfsdk:"ignored_models"`
	IgnoredProviders               types.Set  `tfsdk:"ignored_providers"`
	PlatformBannedAuthors          types.Set  `tfsdk:"platform_banned_authors"`
	PlatformBannedModels           types.Set  `tfsdk:"platform_banned_models"`
	PlatformBannedProviders        types.Set  `tfsdk:"platform_banned_providers"`
	RequireEUDataRegion            types.Bool `tfsdk:"require_eu_data_region"`
}

func policyConfigAttributes(kind string) map[string]rs.Attribute {
	if kind == "budget" {
		return map[string]rs.Attribute{
			"limit_usd":               rs.Float64Attribute{Optional: true, MarkdownDescription: "Positive per-subject spending limit in USD, or null for no limit."},
			"reset_interval":          rs.StringAttribute{Optional: true, MarkdownDescription: "daily, weekly, monthly, or null for a lifetime limit. Both nullable fields are always sent."},
			"include_byok_in_budgets": rs.BoolAttribute{Required: true, MarkdownDescription: "Whether BYOK spend counts toward each subject's budget."},
		}
	}
	return map[string]rs.Attribute{
		"allow_safety_retention_google":      rs.BoolAttribute{Optional: true},
		"allow_safety_retention_other":       rs.BoolAttribute{Optional: true},
		"allowed_data_regions":               rs.SetAttribute{Optional: true, ElementType: types.StringType},
		"allowed_models":                     rs.SetAttribute{Optional: true, ElementType: types.StringType},
		"allowed_providers":                  rs.SetAttribute{Optional: true, ElementType: types.StringType},
		"enable_free_model_publication":      rs.BoolAttribute{Optional: true},
		"enable_free_model_training":         rs.BoolAttribute{Optional: true},
		"enable_paid_model_training":         rs.BoolAttribute{Optional: true},
		"enforce_first_party_harness_models": rs.BoolAttribute{Optional: true},
		"enforce_zdr":                        rs.BoolAttribute{Optional: true},
		"enforce_zdr_anthropic":              rs.BoolAttribute{Optional: true},
		"enforce_zdr_google":                 rs.BoolAttribute{Optional: true},
		"enforce_zdr_openai":                 rs.BoolAttribute{Optional: true},
		"enforce_zdr_other":                  rs.BoolAttribute{Optional: true},
		"enforce_zdr_xai":                    rs.BoolAttribute{Optional: true},
		"ignored_models":                     rs.SetAttribute{Optional: true, ElementType: types.StringType},
		"ignored_providers":                  rs.SetAttribute{Optional: true, ElementType: types.StringType},
		"platform_banned_authors":            rs.SetAttribute{Optional: true, ElementType: types.StringType},
		"platform_banned_models":             rs.SetAttribute{Optional: true, ElementType: types.StringType},
		"platform_banned_providers":          rs.SetAttribute{Optional: true, ElementType: types.StringType},
		"require_eu_data_region":             rs.BoolAttribute{Optional: true},
	}
}
func policyConfigType(ctx context.Context, kind string) types.ObjectType {
	attributes := make(map[string]attr.Type)
	for name, value := range policyConfigAttributes(kind) {
		attributes[name] = value.GetType()
	}
	return types.ObjectType{AttrTypes: attributes}
}

func policyAuthoringConfig(ctx context.Context, kind string, object types.Object) (any, diag.Diagnostics) {
	var diags diag.Diagnostics
	if kind == "budget" {
		var model budgetPolicyConfigModel
		diags.Append(object.As(ctx, &model, basetypes.ObjectAsOptions{})...)
		if diags.HasError() {
			return nil, diags
		}
		return client.BudgetPolicyConfig{LimitUSD: float64ValueOrNil(model.LimitUSD), ResetInterval: stringValueOrNil(model.ResetInterval), IncludeBYOKInBudgets: model.IncludeBYOKInBudgets.ValueBool()}, diags
	}
	var model modelAvailabilityPolicyConfigModel
	diags.Append(object.As(ctx, &model, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return nil, diags
	}
	config := client.ModelAvailabilityPolicyConfig{
		AllowSafetyRetentionGoogle:     boolValueOrNil(model.AllowSafetyRetentionGoogle),
		AllowSafetyRetentionOther:      boolValueOrNil(model.AllowSafetyRetentionOther),
		EnableFreeModelPublication:     boolValueOrNil(model.EnableFreeModelPublication),
		EnableFreeModelTraining:        boolValueOrNil(model.EnableFreeModelTraining),
		EnablePaidModelTraining:        boolValueOrNil(model.EnablePaidModelTraining),
		EnforceFirstPartyHarnessModels: boolValueOrNil(model.EnforceFirstPartyHarnessModels),
		EnforceZDR:                     boolValueOrNil(model.EnforceZDR),
		EnforceZDRAnthropic:            boolValueOrNil(model.EnforceZDRAnthropic),
		EnforceZDRGoogle:               boolValueOrNil(model.EnforceZDRGoogle),
		EnforceZDROpenAI:               boolValueOrNil(model.EnforceZDROpenAI),
		EnforceZDROther:                boolValueOrNil(model.EnforceZDROther),
		EnforceZDRXAI:                  boolValueOrNil(model.EnforceZDRXAI),
		RequireEUDataRegion:            boolValueOrNil(model.RequireEUDataRegion),
	}
	config.AllowedDataRegions, diags = policyConfigSet(ctx, model.AllowedDataRegions, diags)
	config.AllowedModels, diags = policyConfigSet(ctx, model.AllowedModels, diags)
	config.AllowedProviders, diags = policyConfigSet(ctx, model.AllowedProviders, diags)
	config.IgnoredModels, diags = policyConfigSet(ctx, model.IgnoredModels, diags)
	config.IgnoredProviders, diags = policyConfigSet(ctx, model.IgnoredProviders, diags)
	config.PlatformBannedAuthors, diags = policyConfigSet(ctx, model.PlatformBannedAuthors, diags)
	config.PlatformBannedModels, diags = policyConfigSet(ctx, model.PlatformBannedModels, diags)
	config.PlatformBannedProviders, diags = policyConfigSet(ctx, model.PlatformBannedProviders, diags)
	return config, diags
}
func policyConfigSet(ctx context.Context, value types.Set, diags diag.Diagnostics) ([]string, diag.Diagnostics) {
	values, d := setToStringSlice(ctx, value)
	diags.Append(d...)
	if !value.IsNull() && !value.IsUnknown() && values == nil {
		values = []string{}
	}
	return values, diags
}
func policyConfigState(ctx context.Context, kind string, raw json.RawMessage) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if kind == "budget" {
		var config client.BudgetPolicyConfig
		if err := json.Unmarshal(raw, &config); err != nil {
			diags.AddError("Invalid budget policy config", err.Error())
			return types.ObjectNull(policyConfigType(ctx, kind).AttrTypes), diags
		}
		model := budgetPolicyConfigModel{LimitUSD: float64PtrValue(config.LimitUSD), ResetInterval: stringPtrValue(config.ResetInterval), IncludeBYOKInBudgets: types.BoolValue(config.IncludeBYOKInBudgets)}
		return types.ObjectValueFrom(ctx, policyConfigType(ctx, kind).AttrTypes, model)
	}
	var config client.ModelAvailabilityPolicyConfig
	if err := json.Unmarshal(raw, &config); err != nil {
		diags.AddError("Invalid model availability policy config", err.Error())
		return types.ObjectNull(policyConfigType(ctx, kind).AttrTypes), diags
	}
	model := modelAvailabilityPolicyConfigModel{
		AllowSafetyRetentionGoogle:     boolPtrValue(config.AllowSafetyRetentionGoogle),
		AllowSafetyRetentionOther:      boolPtrValue(config.AllowSafetyRetentionOther),
		EnableFreeModelPublication:     boolPtrValue(config.EnableFreeModelPublication),
		EnableFreeModelTraining:        boolPtrValue(config.EnableFreeModelTraining),
		EnablePaidModelTraining:        boolPtrValue(config.EnablePaidModelTraining),
		EnforceFirstPartyHarnessModels: boolPtrValue(config.EnforceFirstPartyHarnessModels),
		EnforceZDR:                     boolPtrValue(config.EnforceZDR),
		EnforceZDRAnthropic:            boolPtrValue(config.EnforceZDRAnthropic),
		EnforceZDRGoogle:               boolPtrValue(config.EnforceZDRGoogle),
		EnforceZDROpenAI:               boolPtrValue(config.EnforceZDROpenAI),
		EnforceZDROther:                boolPtrValue(config.EnforceZDROther),
		EnforceZDRXAI:                  boolPtrValue(config.EnforceZDRXAI),
		RequireEUDataRegion:            boolPtrValue(config.RequireEUDataRegion),
	}
	model.AllowedDataRegions = policyConfigSetState(ctx, config.AllowedDataRegions, &diags)
	model.AllowedModels = policyConfigSetState(ctx, config.AllowedModels, &diags)
	model.AllowedProviders = policyConfigSetState(ctx, config.AllowedProviders, &diags)
	model.IgnoredModels = policyConfigSetState(ctx, config.IgnoredModels, &diags)
	model.IgnoredProviders = policyConfigSetState(ctx, config.IgnoredProviders, &diags)
	model.PlatformBannedAuthors = policyConfigSetState(ctx, config.PlatformBannedAuthors, &diags)
	model.PlatformBannedModels = policyConfigSetState(ctx, config.PlatformBannedModels, &diags)
	model.PlatformBannedProviders = policyConfigSetState(ctx, config.PlatformBannedProviders, &diags)
	object, d := types.ObjectValueFrom(ctx, policyConfigType(ctx, kind).AttrTypes, model)
	diags.Append(d...)
	return object, diags
}
func policyConfigSetState(ctx context.Context, values []string, diags *diag.Diagnostics) types.Set {
	if values == nil {
		return types.SetNull(types.StringType)
	}
	value, d := types.SetValueFrom(ctx, types.StringType, values)
	diags.Append(d...)
	return value
}
func validatePolicyConfig(ctx context.Context, kind string, model policyResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	if !model.Name.IsUnknown() && (utf8.RuneCountInString(model.Name.ValueString()) < 1 || utf8.RuneCountInString(model.Name.ValueString()) > 255) {
		diags.AddError("Invalid policy name", "name must contain 1 to 255 characters")
	}
	if !model.WorkspaceID.IsUnknown() && strings.TrimSpace(model.WorkspaceID.ValueString()) == "" {
		diags.AddError("Invalid policy workspace", "workspace_id must not be empty")
	}
	if model.Config.IsNull() || model.Config.IsUnknown() {
		return diags
	}
	if kind == "budget" {
		var config budgetPolicyConfigModel
		diags.Append(model.Config.As(ctx, &config, basetypes.ObjectAsOptions{UnhandledUnknownAsEmpty: true})...)
		if !config.LimitUSD.IsNull() && !config.LimitUSD.IsUnknown() {
			value := config.LimitUSD.ValueFloat64()
			if value <= 0 || math.IsInf(value, 0) || math.IsNaN(value) {
				diags.AddError("Invalid policy limit", "limit_usd must be positive or null")
			}
		}
		if !config.ResetInterval.IsNull() && !config.ResetInterval.IsUnknown() && config.ResetInterval.ValueString() == "" {
			diags.AddError("Invalid policy interval", "reset_interval must not be empty; use null for a lifetime limit")
		}
	} else {
		var config modelAvailabilityPolicyConfigModel
		diags.Append(model.Config.As(ctx, &config, basetypes.ObjectAsOptions{UnhandledUnknownAsEmpty: true})...)
		if !config.AllowedDataRegions.IsNull() && !config.AllowedDataRegions.IsUnknown() && len(config.AllowedDataRegions.Elements()) == 0 {
			diags.AddError("Invalid data regions", "allowed_data_regions must contain at least one region or be null")
		}
	}
	return diags
}

func (r *policyResource) setPolicyState(ctx context.Context, policy *client.Policy, model *policyResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	config, d := policyConfigState(ctx, r.kind, policy.Config)
	diags.Append(d...)
	if !diags.HasError() {
		model.ID = types.StringValue(policy.ID)
		model.Name = types.StringValue(policy.Name)
		model.Config = config
	}
	return diags
}
