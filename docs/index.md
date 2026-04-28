# OpenRouter Provider

Use the OpenRouter provider to manage OpenRouter management-plane resources with Terraform or OpenTofu.

## Example Usage

```hcl
terraform {
  required_providers {
    openrouter = {
      source  = "cloudopsworks/openrouter"
      version = "~> 0.1"
    }
  }
}

provider "openrouter" {
  api_key = var.openrouter_management_key
}
```

## Argument Reference

- `api_key` - (Optional, Sensitive) OpenRouter management API key. Can also be supplied with `OPENROUTER_API_KEY`.
- `base_url` - (Optional) Override for the OpenRouter management API base URL. Defaults to `https://openrouter.ai/api/v1`.

## Authentication

The provider requires an OpenRouter **management** key and validates that requirement during provider configuration.

## Supported Resources

- `openrouter_api_key`
- `openrouter_workspace`
- `openrouter_guardrail`

## Supported Data Sources

- `openrouter_workspace`
- `openrouter_api_keys`
- `openrouter_workspaces`
- `openrouter_guardrails`
- `openrouter_organization`
- `openrouter_providers`
