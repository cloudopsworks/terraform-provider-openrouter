resource "openrouter_workspace" "team" {
  name                             = "Engineering"
  slug                             = "engineering"
  description                      = "Engineering workspace"
  default_text_model               = "openai/gpt-4o"
  default_provider_sort            = "price"
  io_logging_sampling_rate         = 1
  is_data_discount_logging_enabled = true
}

resource "openrouter_api_key" "service" {
  name                  = "svc-engineering"
  workspace_id          = openrouter_workspace.team.id
  limit                 = 50
  limit_reset           = "monthly"
  include_byok_in_limit = true
}

resource "openrouter_guardrail" "team" {
  name              = "engineering"
  workspace_id      = openrouter_workspace.team.id
  description       = "Engineering guardrail"
  limit_usd         = 200
  reset_interval    = "monthly"
  allowed_providers = ["openai", "anthropic"]

  enforce_zdr_anthropic = true
  enforce_zdr_google    = false
  enforce_zdr_openai    = true
  enforce_zdr_other     = false

  content_filter_builtins = [
    {
      slug   = "email"
      action = "redact"
    },
    {
      slug       = "regex-prompt-injection"
      action     = "flag"
      scan_scope = "user_only"
    }
  ]

  content_filters = [
    {
      pattern = "SECRET_[A-Z0-9]+"
      action  = "redact"
      label   = "[SECRET]"
    }
  ]
}

# The API key ID is its non-secret hash, which is required for the assignment.
resource "openrouter_guardrail_key_assignment" "service" {
  guardrail_id = openrouter_guardrail.team.id
  key_hash     = openrouter_api_key.service.id
}

resource "openrouter_workspace_budget" "team_monthly" {
  workspace_id = openrouter_workspace.team.id
  interval     = "monthly"
  limit_usd    = 200
}

# Requires Terraform >= 1.11. The raw credential remains write-only.
resource "openrouter_byok_key" "openai" {
  provider_slug  = "openai"
  workspace_id   = openrouter_workspace.team.id
  key_wo         = var.openai_api_key
  key_wo_version = 1

  name           = "engineering-openai"
  allowed_models = ["openai/gpt-4o"]
}
