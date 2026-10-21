# openrouter_byok_keys

Lists OpenRouter BYOK provider credentials without returning raw provider credentials.

## Example Usage

```hcl
data "openrouter_byok_keys" "openai" {
  workspace_id = openrouter_workspace.team.id
  provider_slug = "openai"
}
```

## Argument Reference

- `workspace_id` - (Optional) Workspace UUID filter.
- `provider_slug` - (Optional) Provider slug filter.

When `workspace_id` is omitted, OpenRouter lists credentials for the default workspace. It does not enumerate every workspace.

## Attribute Reference

- `total_count` - Number of returned credentials.
- `items` - Credentials with `id`, `provider_slug`, `workspace_id`, `name`, `label`, `declared_zdr`, `disabled`, `is_fallback`, `is_required`, `is_byok_only`, `allowed_models`, `allowed_api_key_hashes`, `allowed_user_ids`, `sort_order`, and `created_at`.

Raw provider credential values are never returned.
