# Releasing the OpenRouter provider

This repository is prepared for Terraform Registry and OpenTofu Registry publication using the same GitHub release assets.

## One-time setup

1. Generate a GPG key dedicated to provider releases.
2. Export the private key in ASCII armor and add these GitHub repository secrets:
   - `GPG_PRIVATE_KEY`
   - `PASSPHRASE`
3. Export the public key and register it in:
   - Terraform Registry -> User Settings -> Signing Keys
   - OpenTofu Registry by submitting a **Provider Signing Key** issue in `opentofu/registry`
4. Ensure the repository is public and the repository name remains `terraform-provider-openrouter`.

## Release flow

1. Update documentation and code.
2. Run local verification:
   - `make fmt`
   - `make lint`
   - `make test`
   - optional: `make release-snapshot` (requires local GoReleaser)
3. Create and push a semantic version tag such as `v0.1.0`.
4. GitHub Actions runs `.github/workflows/release.yml` and publishes:
   - per-platform provider zip archives
   - `terraform-provider-openrouter_<version>_SHA256SUMS`
   - `terraform-provider-openrouter_<version>_SHA256SUMS.sig`
   - `terraform-provider-openrouter_<version>_manifest.json`

## Publish to the registries

### Terraform Registry

1. Sign in to <https://registry.terraform.io/> with a GitHub account that can administer `cloudopsworks/terraform-provider-openrouter`.
2. Publish the provider from the registry UI.
3. Confirm the registry webhook is installed and healthy.

Reference: <https://developer.hashicorp.com/terraform/registry/providers/publishing>

### OpenTofu Registry

1. Submit a **new provider** issue in <https://github.com/opentofu/registry> for `cloudopsworks/terraform-provider-openrouter`.
2. Submit or update the **provider signing key** issue for the same namespace/provider.
3. Wait for the registry automation/indexing to ingest the provider release.

Reference: <https://github.com/opentofu/registry>

## Source addresses

- Terraform: `cloudopsworks/openrouter` (default host `registry.terraform.io`)
- OpenTofu: `cloudopsworks/openrouter` (default host `registry.opentofu.org`)
