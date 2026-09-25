# OpenRouter Provider

Use the OpenRouter provider to manage OpenRouter management-plane resources with Terraform or OpenTofu.

## Requirements

- Terraform `>= 1.11` or OpenTofu `>= 1.11`
- Go `>= 1.26.3` for development

The provider advertises the write-only `openrouter_byok_key.key_wo` schema globally. Terraform and OpenTofu must support write-only arguments even when a configuration does not create a BYOK credential.

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

- `api_key` - (Optional, Sensitive) OpenRouter management API key. Can also be supplied with `OPENROUTER_API_KEY` or a supported cloud secret source.
- `base_url` - (Optional) Override for the OpenRouter management API base URL. Defaults to `https://openrouter.ai/api/v1`.
- `aws_secrets_manager` - (Optional) Read provider settings from AWS Secrets Manager using the AWS default credential chain, with optional STS AssumeRole support.
- `gcp_secret_manager` - (Optional) Read provider settings from Google Secret Manager using Application Default Credentials, with optional service-account impersonation.
- `azure_key_vault` - (Optional) Read the management API key from Azure Key Vault using `DefaultAzureCredential`.

## Authentication

The provider requires an OpenRouter **management** key and validates that requirement during provider configuration. Authentication is resolved in this order:

1. Explicit provider arguments (`api_key` and `base_url`).
2. Direct environment values (`OPENROUTER_API_KEY` and `OPENROUTER_BASE_URL`).
3. Exactly one cloud secret source when no direct key is configured.

Direct provider and environment values override fields returned by a secret. When a direct key is available, configured cloud sources are ignored completely. If no direct key exists, configure exactly one cloud source; missing, ambiguous, inaccessible, malformed, or empty sources fail provider configuration.

### Environment-only initialization

```hcl
provider "openrouter" {}
```

```sh
export OPENROUTER_API_KEY="sk-or-..."
export OPENROUTER_BASE_URL="https://openrouter.ai/api/v1" # optional
```

### AWS Secrets Manager

The AWS source uses the AWS SDK default credential chain and can optionally assume
`role_arn` before reading the secret.

```hcl
provider "openrouter" {
  aws_secrets_manager = {
    region            = "us-east-1"
    secret_id         = "prod/openrouter/management"
    version_stage     = "AWSCURRENT"
    role_arn          = "arn:aws:iam::123456789012:role/openrouter-secret-reader"
    role_session_name = "terraform-provider-openrouter"
    json_key          = "api_key"
  }
}
```

When no direct key or HCL cloud block is configured, AWS source discovery uses:

| Purpose | Environment variables |
| --- | --- |
| Secret ID | `OPENROUTER_AWS_SECRETS_MANAGER_SECRET_ID`, `OPENROUTER_AWS_SECRET_ID` |
| Region | `OPENROUTER_AWS_SECRETS_MANAGER_REGION`, `AWS_REGION`, `AWS_DEFAULT_REGION` |
| Version | `OPENROUTER_AWS_SECRETS_MANAGER_VERSION_ID`, `OPENROUTER_AWS_SECRETS_MANAGER_VERSION_STAGE` |
| JSON key | `OPENROUTER_AWS_SECRETS_MANAGER_JSON_KEY` |
| Assume role | `OPENROUTER_AWS_SECRETS_MANAGER_ROLE_ARN`, `OPENROUTER_AWS_ROLE_ARN` |

### GCP Secret Manager

```hcl
provider "openrouter" {
  gcp_secret_manager = {
    project_id                  = "platform-prod"
    secret_id                   = "openrouter-management-key"
    version                     = "latest"
    impersonate_service_account = "secret-reader@platform-prod.iam.gserviceaccount.com"
    json_key                    = "api_key"
  }
}
```

GCP source discovery uses `OPENROUTER_GCP_SECRET_MANAGER_PROJECT_ID` (or
`GOOGLE_CLOUD_PROJECT` / `GCLOUD_PROJECT`) and
`OPENROUTER_GCP_SECRET_MANAGER_SECRET_ID` (or `OPENROUTER_GCP_SECRET_ID`).
Optional source settings are read from the corresponding
`OPENROUTER_GCP_SECRET_MANAGER_*` environment variables, including comma-separated
`DELEGATES` and `SCOPES`.

### Azure Key Vault

```hcl
provider "openrouter" {
  azure_key_vault = {
    vault_url   = "https://platform-prod.vault.azure.net/"
    secret_name = "openrouter-management-key"
    json_key    = "api_key"
  }
}
```

Azure authentication uses the standard Azure SDK `DefaultAzureCredential` chain.

### Secret payload formats

Secret values may be plaintext or JSON. Plaintext values are treated as the
management API key:

```text
sk-or-...
```

JSON values may include `api_key` (or `openrouter_api_key`) and optional
`base_url` fields:

```json
{
  "api_key": "sk-or-...",
  "base_url": "https://openrouter.ai/api/v1"
}
```

Set `json_key` for a nested JSON path such as `openrouter.api_key`.

## Supported Resources

- `openrouter_api_key`
- `openrouter_workspace`
- `openrouter_guardrail`
- `openrouter_guardrail_key_assignment`
- `openrouter_workspace_budget`
- `openrouter_byok_key`

## Supported Data Sources

- `openrouter_workspace`
- `openrouter_api_keys`
- `openrouter_workspaces`
- `openrouter_guardrails`
- `openrouter_organization`
- `openrouter_providers`
- `openrouter_byok_keys`

## Management notes

- Use `openrouter_guardrail_key_assignment` to associate a key hash with a guardrail; one Terraform resource should own each key assignment.
- Use `openrouter_workspace_budget` for a workspace interval budget. Its identity preserves the configured `<workspace_ref>/<interval>`; OpenRouter accepts a UUID or slug, though a UUID is recommended for stability.
- `openrouter_byok_key` uses a provider-wide write-only schema, so Terraform/OpenTofu `>= 1.11` is required; raw provider credentials are never returned in state.

See [resources](resources/) and [data sources](data-sources/) for feature-specific lifecycle and import guidance.
