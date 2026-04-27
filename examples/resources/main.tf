resource "openrouter_workspace" "team" {
  name                            = "Engineering"
  slug                            = "engineering"
  description                     = "Engineering workspace"
  default_text_model              = "openai/gpt-4o"
  default_provider_sort           = "price"
  io_logging_sampling_rate        = 1
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
  description       = "Engineering guardrail"
  limit_usd         = 200
  reset_interval    = "monthly"
  allowed_providers = ["openai", "anthropic"]
  enforce_zdr       = true
}
