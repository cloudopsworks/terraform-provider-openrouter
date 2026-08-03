terraform {
  required_providers {
    openrouter = {
      source = "cloudopsworks/openrouter"
    }
  }
}

provider "openrouter" {
  gcp_secret_manager = {
    project_id = var.gcp_project_id
    secret_id  = var.openrouter_secret_id
    version    = "latest"
    json_key   = "api_key"
  }
}

variable "gcp_project_id" {
  type        = string
  description = "GCP project ID that owns the Secret Manager secret."
}

variable "openrouter_secret_id" {
  type        = string
  description = "GCP Secret Manager secret ID containing the OpenRouter key."
}
