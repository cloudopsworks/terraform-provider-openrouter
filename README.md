# Terraform Provider OpenRouter

Terraform/OpenTofu provider for the OpenRouter management API.

## Features

### Resources

- `openrouter_api_key`
- `openrouter_workspace`
- `openrouter_guardrail`

### Data sources

- `openrouter_workspace`
- `openrouter_api_keys`
- `openrouter_workspaces`
- `openrouter_guardrails`
- `openrouter_organization`
- `openrouter_providers`

## Requirements

- Go `1.24+`
- Terraform `>= 1.7`
- OpenTofu `>= 1.7`
- OpenRouter management API key

## Installation

### Terraform

```hcl
terraform {
  required_providers {
    openrouter = {
      source  = "cloudopsworks/openrouter"
      version = "~> 0.1"
    }
  }
}
```

### OpenTofu

After the provider is published to the OpenTofu Registry, use the same namespace/type pair:

```hcl
terraform {
  required_providers {
    openrouter = {
      source  = "cloudopsworks/openrouter"
      version = "~> 0.1"
    }
  }
}
```

## Provider configuration

```hcl
provider "openrouter" {
  api_key = var.openrouter_management_key
}
```

You can also set `OPENROUTER_API_KEY` instead of `api_key`.

Optional arguments:

- `api_key` - OpenRouter management API key.
- `base_url` - Override the management API base URL. Defaults to `https://openrouter.ai/api/v1`.

## Example usage

See:

- `examples/provider/`
- `examples/resources/`
- `examples/data-sources/`
- `docs/`

## Known limitations in v1

- OpenRouter returns the actual API key secret only on create. The provider exposes it as `key` and marks it sensitive, but it can still exist in state.
- `openrouter_workspace` intentionally does not model `io_logging_api_key_ids` because the documented write shape is not exposed as a stable read field.
- `openrouter_organization` is implemented from the documented organization-members endpoint.
- Guardrails are managed by guardrail ID/name because current public guardrail CRUD responses do not expose a stable workspace field.

## Development

```sh
make fmt
make lint
make test
make build
```

For acceptance tests:

```sh
make testacc
```

## Registry publishing

This repository includes HashiCorp scaffolding-style release packaging for Terraform Registry/OpenTofu Registry publication:

- `.goreleaser.yml`
- `.github/workflows/release.yml`
- `terraform-registry-manifest.json`

Release details and the remaining registry-side manual steps are documented in [`RELEASING.md`](./RELEASING.md).

## Blueprint integration

This repository remains aligned with the CloudOpsWorks repository-management blueprint, while provider packaging and release assets now follow Terraform provider registry conventions.
