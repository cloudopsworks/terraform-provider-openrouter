terraform {
  required_providers {
    openrouter = {
      source = "cloudopsworks/openrouter"
    }
  }
}

provider "openrouter" {
  aws_secrets_manager = {
    region        = var.aws_region
    secret_id     = var.openrouter_secret_id
    version_stage = "AWSCURRENT"
    json_key      = "api_key"
  }
}

variable "aws_region" {
  type        = string
  description = "AWS region containing the OpenRouter management secret."
}

variable "openrouter_secret_id" {
  type        = string
  description = "AWS Secrets Manager secret ID or ARN."
}
