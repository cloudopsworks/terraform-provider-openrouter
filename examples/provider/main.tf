terraform {
  required_providers {
    openrouter = {
      source = "cloudopsworks/openrouter"
    }
  }
}

provider "openrouter" {
  api_key = var.openrouter_management_key
}
