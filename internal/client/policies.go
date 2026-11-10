package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// BudgetPolicyConfig is a complete authoring config. Nil pointers deliberately
// serialize as null: PUT replaces the config rather than patching it.
type BudgetPolicyConfig struct {
	LimitUSD             *float64 `json:"limit_usd"`
	ResetInterval        *string  `json:"reset_interval"`
	IncludeBYOKInBudgets bool     `json:"include_byok_in_budgets"`
}

// ModelAvailabilityPolicyConfig contains the complete typed authoring surface.
type ModelAvailabilityPolicyConfig struct {
	AllowSafetyRetentionGoogle     *bool    `json:"allow_safety_retention_google"`
	AllowSafetyRetentionOther      *bool    `json:"allow_safety_retention_other"`
	AllowedDataRegions             []string `json:"allowed_data_regions"`
	AllowedModels                  []string `json:"allowed_models"`
	AllowedProviders               []string `json:"allowed_providers"`
	EnableFreeModelPublication     *bool    `json:"enable_free_model_publication"`
	EnableFreeModelTraining        *bool    `json:"enable_free_model_training"`
	EnablePaidModelTraining        *bool    `json:"enable_paid_model_training"`
	EnforceFirstPartyHarnessModels *bool    `json:"enforce_first_party_harness_models"`
	EnforceZDR                     *bool    `json:"enforce_zdr"`
	EnforceZDRAnthropic            *bool    `json:"enforce_zdr_anthropic"`
	EnforceZDRGoogle               *bool    `json:"enforce_zdr_google"`
	EnforceZDROpenAI               *bool    `json:"enforce_zdr_openai"`
	EnforceZDROther                *bool    `json:"enforce_zdr_other"`
	EnforceZDRXAI                  *bool    `json:"enforce_zdr_xai"`
	IgnoredModels                  []string `json:"ignored_models"`
	IgnoredProviders               []string `json:"ignored_providers"`
	PlatformBannedAuthors          []string `json:"platform_banned_authors"`
	PlatformBannedModels           []string `json:"platform_banned_models"`
	PlatformBannedProviders        []string `json:"platform_banned_providers"`
	RequireEUDataRegion            *bool    `json:"require_eu_data_region"`
}

// Policy is the common policy envelope; callers decode Config into the matching
// typed config. Assignment metadata is intentionally not writable here.
type Policy struct {
	ID                    string          `json:"id"`
	Name                  string          `json:"name"`
	PolicyType            string          `json:"policy_type"`
	Config                json.RawMessage `json:"config"`
	Reference             *string         `json:"reference"`
	IsWorkspaceDefault    bool            `json:"is_workspace_default"`
	ReplacesLegacyDefault bool            `json:"replaces_legacy_default"`
	IsAccountPolicy       bool            `json:"is_account_policy"`
}

// UnmarshalJSON preserves the distinction between omitted ownership flags and
// invalid explicit nulls before encoding/json can coerce null into false.
func (p *Policy) UnmarshalJSON(data []byte) error {
	type wirePolicy Policy
	var wire wirePolicy
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for _, field := range []string{"is_workspace_default", "is_account_policy", "replaces_legacy_default"} {
		if value, present := fields[field]; present && strings.TrimSpace(string(value)) == "null" {
			return fmt.Errorf("openrouter policy response %s must be a boolean when present", field)
		}
	}
	*p = Policy(wire)
	return nil
}

func policyPath(kind string) (string, error) {
	switch kind {
	case "budget":
		return "/guardrails/policies/budget", nil
	case "model_availability":
		return "/guardrails/policies/model-availability", nil
	}
	return "", fmt.Errorf("unsupported policy type %q", kind)
}

// PolicyIdentityChangedError reports a structurally valid policy response whose
// identity changed during a write. The returned policy can be retained for
// recovery, but the operation must still be reported as a failure.
type PolicyIdentityChangedError struct {
	RequestedID string
	ReturnedID  string
}

func (e *PolicyIdentityChangedError) Error() string {
	return fmt.Sprintf("openrouter policy response identity %q differs from requested %q", e.ReturnedID, e.RequestedID)
}

func (p Policy) validate(kind, id string) error {
	var config map[string]json.RawMessage
	if err := json.Unmarshal(p.Config, &config); err != nil || config == nil {
		return fmt.Errorf("openrouter %s policy response config must be an object", kind)
	}
	if strings.TrimSpace(p.ID) == "" || strings.TrimSpace(p.Name) == "" || p.PolicyType != kind {
		return fmt.Errorf("openrouter %s policy response missing required fields or has wrong policy_type", kind)
	}
	if kind == "budget" {
		// encoding/json maps null into a non-pointer bool as false. Omission has
		// a documented false default, but an explicit null is not a valid bool.
		if value, present := config["include_byok_in_budgets"]; present && strings.TrimSpace(string(value)) == "null" {
			return fmt.Errorf("invalid budget policy config: include_byok_in_budgets must be a boolean when present")
		}
		var typed BudgetPolicyConfig
		if err := json.Unmarshal(p.Config, &typed); err != nil {
			return fmt.Errorf("invalid budget policy config: %w", err)
		}
	} else {
		var typed ModelAvailabilityPolicyConfig
		if err := json.Unmarshal(p.Config, &typed); err != nil {
			return fmt.Errorf("invalid model availability policy config: %w", err)
		}
		// Nullable lists are valid, but null entries are not strings. Decoding
		// directly into []string would silently turn each null entry into "".
		for _, field := range []string{"allowed_data_regions", "allowed_models", "allowed_providers", "ignored_models", "ignored_providers", "platform_banned_authors", "platform_banned_models", "platform_banned_providers"} {
			if value, present := config[field]; present {
				var entries []*string
				if err := json.Unmarshal(value, &entries); err != nil {
					return fmt.Errorf("invalid model availability policy config %s: %w", field, err)
				}
				for _, entry := range entries {
					if entry == nil {
						return fmt.Errorf("invalid model availability policy config: %s entries must be strings, not null", field)
					}
				}
			}
		}
	}
	if id != "" && p.ID != id {
		return &PolicyIdentityChangedError{RequestedID: id, ReturnedID: p.ID}
	}
	return nil
}

// EnsureManageable refuses read-through/migrated policies whose updates can copy
// rather than mutate the identity, and defaults with a different deletion model.
func (p Policy) EnsureManageable() error {
	if p.Reference != nil || p.IsWorkspaceDefault || p.ReplacesLegacyDefault || p.IsAccountPolicy {
		return fmt.Errorf("policy %q is legacy-mapped, account-owned, or a workspace default; manage it outside this resource", p.ID)
	}
	return nil
}

func (c *Client) GetPolicy(ctx context.Context, kind, workspace, id string) (*Policy, error) {
	path, err := policyPath(kind)
	if err != nil {
		return nil, err
	}
	var response singleResponse[Policy]
	if err = c.do(ctx, http.MethodGet, path+"/"+url.PathEscape(id), url.Values{"workspace_id": {workspace}}, nil, &response); err != nil {
		return nil, err
	}
	if err = response.Data.validate(kind, id); err != nil {
		return nil, err
	}
	return &response.Data, nil
}

type policyWriteRequest struct {
	Name        string `json:"name"`
	WorkspaceID string `json:"workspace_id"`
	PolicyType  string `json:"policy_type,omitempty"`
	Config      any    `json:"config"`
}

func (c *Client) writePolicy(ctx context.Context, kind, workspace, id, name string, config any) (*Policy, error) {
	path, err := policyPath(kind)
	if err != nil {
		return nil, err
	}
	method := http.MethodPost
	body := policyWriteRequest{Name: name, WorkspaceID: workspace, Config: config, PolicyType: kind}
	if id != "" {
		method = http.MethodPut
		path += "/" + url.PathEscape(id)
		body.PolicyType = ""
	}
	var response singleResponse[Policy]
	if err = c.do(ctx, method, path, nil, body, &response); err != nil {
		return nil, err
	}
	if err = response.Data.validate(kind, id); err != nil {
		var identityChanged *PolicyIdentityChangedError
		if errors.As(err, &identityChanged) {
			return &response.Data, err
		}
		return nil, err
	}
	return &response.Data, nil
}

func (c *Client) CreateBudgetPolicy(ctx context.Context, workspace, name string, config BudgetPolicyConfig) (*Policy, error) {
	return c.writePolicy(ctx, "budget", workspace, "", name, config)
}
func (c *Client) UpdateBudgetPolicy(ctx context.Context, workspace, id, name string, config BudgetPolicyConfig) (*Policy, error) {
	return c.writePolicy(ctx, "budget", workspace, id, name, config)
}
func (c *Client) CreateModelAvailabilityPolicy(ctx context.Context, workspace, name string, config ModelAvailabilityPolicyConfig) (*Policy, error) {
	return c.writePolicy(ctx, "model_availability", workspace, "", name, config)
}
func (c *Client) UpdateModelAvailabilityPolicy(ctx context.Context, workspace, id, name string, config ModelAvailabilityPolicyConfig) (*Policy, error) {
	return c.writePolicy(ctx, "model_availability", workspace, id, name, config)
}
func (c *Client) DeletePolicy(ctx context.Context, kind, workspace, id string) error {
	path, err := policyPath(kind)
	if err != nil {
		return err
	}
	return c.do(ctx, http.MethodDelete, path+"/"+url.PathEscape(id), url.Values{"workspace_id": {workspace}}, nil, nil)
}
