data "openrouter_workspace" "engineering" {
  slug = "engineering"
}

data "openrouter_workspaces" "all" {}
data "openrouter_api_keys" "all" {}
data "openrouter_guardrails" "all" {}
data "openrouter_organization" "current" {}
data "openrouter_providers" "all" {}
