# Terraform Provider OpenRouter

[![Latest Release](https://img.shields.io/github/release/cloudopsworks/terraform-provider-openrouter.svg?style=for-the-badge)](https://github.com/cloudopsworks/terraform-provider-openrouter/releases/latest) [![Terraform Registry](https://img.shields.io/badge/registry-cloudopsworks%2Fopenrouter-844FBA?style=for-the-badge&logo=terraform)](https://registry.terraform.io/providers/cloudopsworks/openrouter/latest) [![OpenTofu](https://img.shields.io/badge/OpenTofu-compatible-FFDA18?style=for-the-badge&logo=opentofu&logoColor=black)](https://opentofu.org) [![Go Report Card](https://goreportcard.com/badge/github.com/cloudopsworks/terraform-provider-openrouter?style=for-the-badge)](https://goreportcard.com/report/github.com/cloudopsworks/terraform-provider-openrouter) [![Last Updated](https://img.shields.io/github/last-commit/cloudopsworks/terraform-provider-openrouter.svg?style=for-the-badge)](https://github.com/cloudopsworks/terraform-provider-openrouter/commits) [![Stars](https://img.shields.io/github/stars/cloudopsworks/terraform-provider-openrouter.svg?style=for-the-badge)](https://github.com/cloudopsworks/terraform-provider-openrouter/stargazers)

Govern your entire OpenRouter organization as code — workspaces, guardrails, spend limits, and API keys. A Terraform/OpenTofu provider for the OpenRouter management API, not just an API-key wrapper.

---

OpenRouter now routes 20+ trillion tokens per week and has become shared infrastructure for most AI teams. Once it is a shared dependency, you need to manage it like one: scoped keys, spend caps, guardrails, and organization membership — versioned, reviewed in pull requests, and reproducible across environments.

This is the only Terraform/OpenTofu provider that manages OpenRouter as a governed platform rather than a bare key vault.

### Why this provider

| Capability | `cloudopsworks/openrouter` | Key-only providers |
|---|:--:|:--:|
| API keys (CRUD, spend / time limits) | ✅ | ✅ |
| Workspaces | ✅ | ❌ |
| Guardrails | ✅ | ❌ |
| Organization / members | ✅ | ❌ |
| Providers data source | ✅ | ❌ |
| OpenTofu support | ✅ | rarely |

### Resources

- `openrouter_api_key` — keys with spend and time limits
- `openrouter_workspace`
- `openrouter_guardrail`

### Data sources

- `openrouter_workspace`
- `openrouter_workspaces`
- `openrouter_api_keys`
- `openrouter_guardrails`
- `openrouter_organization`
- `openrouter_providers`

## Requirements

- Go `1.25+`
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

## Govern a team's access (not just a bare key)

```hcl
# Scope a workspace, attach a guardrail, then issue a spend-capped key.
resource "openrouter_workspace" "search_team" {
  name = "search-team"
}

resource "openrouter_guardrail" "no_pii" {
  name = "no-pii"
  # ...guardrail configuration...
}

resource "openrouter_api_key" "search_team_prod" {
  name  = "search-team-prod"
  limit = 200 # USD spend cap
  # scope to the workspace / guardrail above
}
```

> Align argument names with the per-resource pages under `docs/` — the snippet above shows the governance pattern, not a verified schema.

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

## Example usage

See:

- `examples/provider/`
- `examples/resources/`
- `examples/data-sources/`
- `docs/`

## Known limitations in v1

- OpenRouter returns the actual API key secret only on create. The provider exposes it as `key` and marks it sensitive, but it can still exist in state.
- `openrouter_organization` is implemented from the documented organization-members endpoint.

## Registry publishing

This repository includes HashiCorp scaffolding-style release packaging for Terraform Registry/OpenTofu Registry publication:

- `.goreleaser.yml`
- `.github/workflows/release.yml`
- `terraform-registry-manifest.json`

Release details and the remaining registry-side manual steps are documented in [`RELEASING.md`](./RELEASING.md).

## Blueprint integration

This repository remains aligned with the CloudOpsWorks repository-management blueprint, while provider packaging and release assets now follow Terraform provider registry conventions.
