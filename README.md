# Terraform Provider OpenRouter

Terraform/OpenTofu provider for the OpenRouter management API.

## Features

- Resources
  - `openrouter_api_key`
  - `openrouter_workspace`
  - `openrouter_guardrail`
- Data sources
  - `openrouter_api_keys`
  - `openrouter_workspaces`
  - `openrouter_guardrails`
  - `openrouter_organization`
  - `openrouter_providers`

## Requirements

- Terraform `>= 1.7`
- OpenTofu `>= 1.7`
- OpenRouter **management** API key

## Provider configuration

```hcl
terraform {
  required_providers {
    openrouter = {
      source  = "cloudopsworks/openrouter"
      version = "0.1.0"
    }
  }
}

provider "openrouter" {
  api_key = var.openrouter_management_key
}
```

You can also set `OPENROUTER_API_KEY` instead of `api_key`.

## Known limitations in v1

- OpenRouter returns the actual API key secret only on create. The provider exposes it as `key` and marks it sensitive, but with Terraform/OpenTofu 1.7 it can still live in state.
- `openrouter_workspace` intentionally does **not** model `io_logging_api_key_ids` because the official docs show it in update requests but not as a stable round-trippable read field.
- `openrouter_organization` is implemented from the documented organization-members endpoint.
- Guardrails are managed by guardrail ID/name because current official guardrail CRUD docs do not expose a workspace field.

## Import

- `openrouter_api_key`: canonical hash, or `<workspace_id>_<name>`
- `openrouter_workspace`: canonical id/slug, or `<workspace_id>_<slug-or-name>`
- `openrouter_guardrail`: canonical id, or best-effort `<ignored>_<name>` when the name is unique

## Development

```sh
gofmt -w .
go test ./...
```


## Blueprint integration

This repository is aligned to the CloudOpsWorks `go-app-template` blueprint for CI, PR checks, release automation, scanning, and repository management. Deployment-specific sample configuration from the app template has been removed intentionally; release-management workflows remain enabled through the shared blueprint.
