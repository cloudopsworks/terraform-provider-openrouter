# Flagship example: govern OpenRouter for multiple teams as code.
#
# This is the differentiator over key-only providers: instead of issuing bare
# API keys, you define a governed workspace per team, attach a guardrail
# (spend cap, allowed providers, zero-data-retention), and issue a
# spend-limited key scoped to that workspace — all reproducible across
# environments and reviewable in a pull request.

terraform {
  required_providers {
    openrouter = {
      source  = "cloudopsworks/openrouter"
      version = "~> 0.1"
    }
  }
}

provider "openrouter" {
  # Set OPENROUTER_API_KEY in the environment, or pass api_key here.
  api_key = var.openrouter_management_key
}

# One governed unit per product team. Add a team by adding a map entry —
# no new resource blocks required.
resource "openrouter_workspace" "team" {
  for_each = var.teams

  name                             = each.value.name
  slug                             = each.key
  description                      = "${each.value.name} workspace (managed by Terraform)"
  default_text_model               = each.value.default_text_model
  default_provider_sort            = "price"
  io_logging_sampling_rate         = 1
  is_data_discount_logging_enabled = true
}

resource "openrouter_guardrail" "team" {
  for_each = var.teams

  name              = each.key
  workspace_id      = openrouter_workspace.team[each.key].id
  description       = "${each.value.name} guardrail (managed by Terraform)"
  limit_usd         = each.value.monthly_limit_usd
  reset_interval    = "monthly"
  allowed_providers = each.value.allowed_providers
  enforce_zdr       = true
}

resource "openrouter_api_key" "team" {
  for_each = var.teams

  name                  = "${each.key}-prod"
  workspace_id          = openrouter_workspace.team[each.key].id
  limit                 = each.value.monthly_limit_usd
  limit_reset           = "monthly"
  include_byok_in_limit = true

  # Ensure the guardrail exists before the key is issued.
  depends_on = [openrouter_guardrail.team]
}
