package client

type CurrentKey struct {
	Hash               string   `json:"hash"`
	Name               string   `json:"name"`
	Label              string   `json:"label"`
	WorkspaceID        string   `json:"workspace_id"`
	IsManagementKey    bool     `json:"is_management_key"`
	IsProvisioningKey  bool     `json:"is_provisioning_key"`
	Disabled           bool     `json:"disabled"`
	Limit              *float64 `json:"limit"`
	LimitRemaining     *float64 `json:"limit_remaining"`
	LimitReset         *string  `json:"limit_reset"`
	IncludeBYOKInLimit bool     `json:"include_byok_in_limit"`
	Usage              float64  `json:"usage"`
	CreatedAt          string   `json:"created_at"`
	UpdatedAt          string   `json:"updated_at"`
	ExpiresAt          *string  `json:"expires_at"`
}

type APIKey struct {
	Hash               string   `json:"hash"`
	Name               string   `json:"name"`
	Label              string   `json:"label"`
	Disabled           bool     `json:"disabled"`
	Limit              *float64 `json:"limit"`
	LimitRemaining     *float64 `json:"limit_remaining"`
	LimitReset         *string  `json:"limit_reset"`
	IncludeBYOKInLimit bool     `json:"include_byok_in_limit"`
	Usage              float64  `json:"usage"`
	UsageDaily         float64  `json:"usage_daily"`
	UsageWeekly        float64  `json:"usage_weekly"`
	UsageMonthly       float64  `json:"usage_monthly"`
	BYOKUsage          float64  `json:"byok_usage"`
	BYOKUsageDaily     float64  `json:"byok_usage_daily"`
	BYOKUsageWeekly    float64  `json:"byok_usage_weekly"`
	BYOKUsageMonthly   float64  `json:"byok_usage_monthly"`
	CreatedAt          string   `json:"created_at"`
	UpdatedAt          string   `json:"updated_at"`
	ExpiresAt          *string  `json:"expires_at"`
	CreatorUserID      *string  `json:"creator_user_id"`
	WorkspaceID        *string  `json:"workspace_id"`
}

type APIKeyCreateRequest struct {
	Name               string   `json:"name"`
	Limit              *float64 `json:"limit,omitempty"`
	LimitReset         *string  `json:"limit_reset,omitempty"`
	IncludeBYOKInLimit *bool    `json:"include_byok_in_limit,omitempty"`
	ExpiresAt          *string  `json:"expires_at,omitempty"`
	CreatorUserID      *string  `json:"creator_user_id,omitempty"`
	WorkspaceID        *string  `json:"workspace_id,omitempty"`
}

type APIKeyUpdateRequest struct {
	Name               *string  `json:"name,omitempty"`
	Disabled           *bool    `json:"disabled,omitempty"`
	Limit              *float64 `json:"limit,omitempty"`
	LimitReset         *string  `json:"limit_reset,omitempty"`
	IncludeBYOKInLimit *bool    `json:"include_byok_in_limit,omitempty"`
}

type APIKeyCreateResponse struct {
	Data APIKey `json:"data"`
	Key  string `json:"key"`
}

type Workspace struct {
	ID                              string   `json:"id"`
	Name                            string   `json:"name"`
	Slug                            string   `json:"slug"`
	Description                     *string  `json:"description"`
	DefaultTextModel                *string  `json:"default_text_model"`
	DefaultImageModel               *string  `json:"default_image_model"`
	DefaultProviderSort             *string  `json:"default_provider_sort"`
	IOLoggingSamplingRate           *float64 `json:"io_logging_sampling_rate"`
	IsDataDiscountLoggingEnabled    bool     `json:"is_data_discount_logging_enabled"`
	IsObservabilityBroadcastEnabled bool     `json:"is_observability_broadcast_enabled"`
	IsObservabilityIOLoggingEnabled bool     `json:"is_observability_io_logging_enabled"`
	CreatedAt                       string   `json:"created_at"`
	CreatedBy                       *string  `json:"created_by"`
	UpdatedAt                       *string  `json:"updated_at"`
}

type WorkspaceUpsertRequest struct {
	Name                            *string  `json:"name,omitempty"`
	Slug                            *string  `json:"slug,omitempty"`
	Description                     *string  `json:"description,omitempty"`
	DefaultTextModel                *string  `json:"default_text_model,omitempty"`
	DefaultImageModel               *string  `json:"default_image_model,omitempty"`
	DefaultProviderSort             *string  `json:"default_provider_sort,omitempty"`
	IOLoggingSamplingRate           *float64 `json:"io_logging_sampling_rate,omitempty"`
	IsDataDiscountLoggingEnabled    *bool    `json:"is_data_discount_logging_enabled,omitempty"`
	IsObservabilityBroadcastEnabled *bool    `json:"is_observability_broadcast_enabled,omitempty"`
	IsObservabilityIOLoggingEnabled *bool    `json:"is_observability_io_logging_enabled,omitempty"`
}

type Guardrail struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Description      *string  `json:"description"`
	LimitUSD         *float64 `json:"limit_usd"`
	ResetInterval    *string  `json:"reset_interval"`
	AllowedModels    []string `json:"allowed_models"`
	AllowedProviders []string `json:"allowed_providers"`
	IgnoredModels    []string `json:"ignored_models"`
	IgnoredProviders []string `json:"ignored_providers"`
	EnforceZDR       *bool    `json:"enforce_zdr"`
	CreatedAt        string   `json:"created_at"`
	UpdatedAt        *string  `json:"updated_at"`
}

type GuardrailUpsertRequest struct {
	Name             *string   `json:"name,omitempty"`
	Description      *string   `json:"description,omitempty"`
	LimitUSD         *float64  `json:"limit_usd,omitempty"`
	ResetInterval    *string   `json:"reset_interval,omitempty"`
	AllowedModels    *[]string `json:"allowed_models,omitempty"`
	AllowedProviders *[]string `json:"allowed_providers,omitempty"`
	IgnoredModels    *[]string `json:"ignored_models,omitempty"`
	IgnoredProviders *[]string `json:"ignored_providers,omitempty"`
	EnforceZDR       *bool     `json:"enforce_zdr,omitempty"`
}

type OrganizationMember struct {
	ID        string `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Role      string `json:"role"`
}

type ProviderInfo struct {
	Slug                   string  `json:"slug"`
	Name                   string  `json:"name"`
	Status                 *string `json:"status"`
	Description            *string `json:"description"`
	Moderated              *bool   `json:"moderated"`
	SupportsToolCall       *bool   `json:"supports_tool_call"`
	SupportsReasoning      *bool   `json:"supports_reasoning"`
	SupportsMultimodal     *bool   `json:"supports_multimodal"`
	SupportsResponseSchema *bool   `json:"supports_response_schema"`
}
