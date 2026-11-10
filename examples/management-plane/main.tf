terraform {
  required_version = ">= 1.11"
  required_providers {
    openrouter = {
      source = "cloudopsworks/openrouter"
    }
  }
}

# Supply OPENROUTER_API_KEY through the environment, not a checked-in value.
provider "openrouter" {}

data "openrouter_model" "selected" {
  slug = var.model_slug
}

data "openrouter_models" "page" {
  limit  = 10
  offset = 0
}

data "openrouter_activity" "daily" {
  date               = var.usage_date
  workspace_id       = var.workspace_id
  group_by_workspace = true
  max_items          = 1000
}

data "openrouter_workspace_members" "team" {
  workspace_id = var.workspace_id
}

# SCIM requires Enterprise/SSO provisioning; lookup is disabled by default.
data "openrouter_scim_groups" "available" {
  count = var.lookup_scim_groups ? 1 : 0
}

# Empty by default. Mapping creation can change existing group members' access.
resource "openrouter_scim_group_mapping" "team" {
  for_each      = var.scim_group_ids
  scim_group_id = each.value
  workspace_id  = var.workspace_id
  role          = "member"
  keep_members  = true
}

# Creation is opt-in and disabled; no real trace-forwarding is enabled here.
resource "openrouter_observability_destination" "traces" {
  count              = var.create_destination ? 1 : 0
  workspace_id       = var.workspace_id
  type               = "langfuse"
  name               = "team traces"
  enabled            = false
  privacy_mode       = true
  config_wo          = var.destination_config_json
  config_wo_revision = var.destination_config_revision
}
