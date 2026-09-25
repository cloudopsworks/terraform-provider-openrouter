# openrouter_byok_key

Manages an OpenRouter bring-your-own-key (BYOK) provider credential without storing the raw provider credential in Terraform state.

## Requirements

The provider advertises the BYOK write-only schema globally, so every configuration requires Terraform or OpenTofu `>= 1.11`, even when it does not create a BYOK credential. `key_wo` is intentionally unavailable to state, plan output, and read operations. Do not use this resource when the raw credential must be recoverable from Terraform state.

## Example Usage

```hcl
variable "openai_api_key" {
  type      = string
  sensitive = true
}

resource "openrouter_byok_key" "openai" {
  provider_slug  = "openai"
  workspace_id   = openrouter_workspace.team.id
  key_wo         = var.openai_api_key
  key_wo_version = 1

  name           = "team-openai"
  declared_zdr   = true
  allowed_models = ["openai/gpt-4o"]
  disabled       = false
}
```

## Argument Reference

- `key_wo` - (Optional, Sensitive, Write-only) Raw provider credential. Required when creating a credential and whenever `key_wo_version` changes. It is never stored in state.
- `key_wo_version` - (Optional, Computed) Version marker for credential rotation. Change it and provide a new `key_wo` to send a rotation request. The initial/imported value is `0` when not configured.
- `provider_slug` - (Required, Forces replacement) Lowercase provider slug.
- `workspace_id` - (Optional, Computed, Forces replacement) Workspace UUID. If omitted, OpenRouter selects and the provider records the default workspace.
- `name` - (Optional) Credential name. Set `null` to clear it.
- `declared_zdr` - (Optional) Declared zero-data-retention setting. Set `null` to clear it.
- `allowed_models` - (Optional) Model allowlist set. `null` permits all models; a configured set is limited to 100 entries.
- `allowed_api_key_hashes` - (Optional) API key-hash allowlist set. `null` permits all keys. A configured set contains 1–100 lowercase SHA-256 hashes.
- `allowed_user_ids` - (Optional) User-ID allowlist set. `null` permits all users; a configured set is limited to 100 entries.
- `disabled` - (Optional, Computed) Whether the credential is disabled.
- `is_fallback` - (Optional, Computed) Whether OpenRouter can use this credential as a fallback. It cannot be combined with `is_byok_only`.
- `is_required` - (Optional, Computed) Whether the credential is required.
- `is_byok_only` - (Optional, Computed) Whether the credential is available only to BYOK requests.

Use `null`, not an empty set, to remove a nullable allowlist. The attributes are Terraform sets, so ordering does not affect plans.

## Attribute Reference

- `id` - Credential UUID.
- `label` - Server-generated label.
- `sort_order` - Server-assigned ordering value.
- `created_at` - Creation timestamp.

## Rotation and Drift

The provider sends `key_wo` only on creation or when `key_wo_version` changes. Updating other metadata does not resend the credential. Because the raw credential is write-only, Terraform cannot detect an out-of-band change to its value; use `key_wo_version` to declare intentional rotations.

## Deletion Lifecycle

OpenRouter [soft-deletes BYOK credentials](https://openrouter.ai/docs/api/api-reference/byok/delete-a-byok-provider-credential): encrypted credential material is wiped and the record is marked deleted/tombstoned. After a successful confirmed deletion, Terraform removes this resource from state. A DELETE `404` is treated as an idempotent successful deletion.

## Import

```sh
terraform import openrouter_byok_key.openai <credential_uuid>
```

Imports set `key_wo_version` to `0`; set a higher version together with `key_wo` for the first Terraform-managed rotation.
