terraform {
  required_providers {
    openrouter = {
      source = "cloudopsworks/openrouter"
    }
  }
}

provider "openrouter" {
  azure_key_vault = {
    vault_url   = var.azure_key_vault_url
    secret_name = var.openrouter_secret_name
    version     = var.openrouter_secret_version
    json_key    = "api_key"
  }
}

variable "azure_key_vault_url" {
  type        = string
  description = "Azure Key Vault URL."
}

variable "openrouter_secret_name" {
  type        = string
  description = "Azure Key Vault secret name containing the OpenRouter key."
}

variable "openrouter_secret_version" {
  type        = string
  description = "Optional Azure Key Vault secret version."
  default     = ""
}
