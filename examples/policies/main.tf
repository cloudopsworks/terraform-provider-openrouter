terraform {
  required_version = ">= 1.11"
  required_providers {
    openrouter = {
      source = "cloudopsworks/openrouter"
    }
  }
}

provider "openrouter" {}

resource "openrouter_workspace" "team" {
  name = "Policy-managed engineering"
  slug = "policy-engineering"
}

resource "openrouter_api_key" "service" {
  name         = "policy-engineering-service"
  workspace_id = openrouter_workspace.team.id
}

resource "openrouter_budget_policy" "service" {
  name         = "Monthly service allowance"
  workspace_id = openrouter_workspace.team.id
  config = {
    limit_usd               = 100
    reset_interval          = "monthly"
    include_byok_in_budgets = false
  }
}

resource "openrouter_model_availability_policy" "approved" {
  name         = "Approved providers"
  workspace_id = openrouter_workspace.team.id
  config = {
    allowed_providers     = ["openai", "anthropic"]
    enforce_zdr_openai    = true
    enforce_zdr_anthropic = true
    enforce_zdr_xai       = true
  }
}

resource "openrouter_api_key_policy_assignment" "service_budget" {
  workspace_id = openrouter_workspace.team.id
  policy_type  = "budget"
  policy_id    = openrouter_budget_policy.service.id
  key_hash     = openrouter_api_key.service.id
}

resource "openrouter_api_key_policy_assignment" "approved_models" {
  workspace_id = openrouter_workspace.team.id
  policy_type  = "model_availability"
  policy_id    = openrouter_model_availability_policy.approved.id
  key_hash     = openrouter_api_key.service.id
}

resource "openrouter_member_policy_assignment" "approved_models" {
  for_each = var.member_ids

  workspace_id = openrouter_workspace.team.id
  policy_type  = "model_availability"
  policy_id    = openrouter_model_availability_policy.approved.id
  member_id    = each.value
}
