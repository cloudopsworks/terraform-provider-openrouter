# OpenRouter Provider

Use the OpenRouter provider to manage OpenRouter management-plane resources with Terraform or OpenTofu.

## Example Usage

```hcl
provider "openrouter" {
  api_key = var.openrouter_management_key
}
```

## Authentication

The provider requires an OpenRouter **management** key. It validates the key at provider configure time.

## Supported Resources

- `openrouter_api_key`
- `openrouter_workspace`
- `openrouter_guardrail`

## Supported Data Sources

- `openrouter_api_keys`
- `openrouter_workspaces`
- `openrouter_guardrails`
- `openrouter_organization`
- `openrouter_providers`
