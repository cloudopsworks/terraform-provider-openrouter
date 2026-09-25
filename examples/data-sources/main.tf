data "openrouter_workspace" "engineering" {
  slug = "engineering"
}

data "openrouter_workspaces" "all" {}
data "openrouter_api_keys" "all" {}
data "openrouter_guardrails" "all" {}
data "openrouter_organization" "current" {}
data "openrouter_providers" "all" {}

# Omitting workspace_id lists the default workspace only; this filter is explicit.
data "openrouter_byok_keys" "engineering_openai" {
  workspace_id  = data.openrouter_workspace.engineering.id
  provider_slug = "openai"
}
