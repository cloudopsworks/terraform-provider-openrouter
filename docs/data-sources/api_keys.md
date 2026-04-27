# openrouter_api_keys

Lists OpenRouter API keys.

## Example Usage

```hcl
data "openrouter_api_keys" "workspace" {
  workspace_id = openrouter_workspace.team.id
}
```

## Argument Reference

- `workspace_id` - (Optional) Filter results to a single workspace UUID.
- `include_disabled` - (Optional) Include disabled API keys.

## Attribute Reference

- `total_count` - Number of returned API keys.
- `items` - List of API keys. Each item includes `id`, `name`, `workspace_id`, `label`, `disabled`, `limit`, `limit_remaining`, `limit_reset`, `include_byok_in_limit`, `expires_at`, `creator_user_id`, `created_at`, and `updated_at`.
