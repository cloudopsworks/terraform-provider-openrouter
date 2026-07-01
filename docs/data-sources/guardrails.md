# openrouter_guardrails

Lists OpenRouter guardrails.

## Example Usage

```hcl
data "openrouter_guardrails" "all" {}

data "openrouter_guardrails" "workspace" {
  workspace_id = openrouter_workspace.team.id
}
```

## Attribute Reference

- `workspace_id` - (Optional) Workspace UUID filter. When omitted, OpenRouter returns guardrails from the default workspace.
- `total_count` - Number of returned guardrails.
- `items` - List of guardrails. Each item includes `id`, `name`, `workspace_id`, `description`, `limit_usd`, `reset_interval`, `allowed_models`, `allowed_providers`, `ignored_models`, `ignored_providers`, `content_filter_builtins`, `content_filters`, `enforce_zdr`, `enforce_zdr_anthropic`, `enforce_zdr_google`, `enforce_zdr_openai`, `enforce_zdr_other`, `created_at`, and `updated_at`.

Nested `content_filter_builtins` entries include `slug`, `action`, `label`, and `scan_scope`.
Nested `content_filters` entries include `pattern`, `action`, and `label`.
