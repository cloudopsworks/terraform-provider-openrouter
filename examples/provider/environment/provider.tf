terraform {
  required_providers {
    openrouter = {
      source = "cloudopsworks/openrouter"
    }
  }
}

# Initialize from environment variables:
#   OPENROUTER_API_KEY - required
#   OPENROUTER_BASE_URL - optional
provider "openrouter" {}
