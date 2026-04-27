# openrouter_guardrails

Lists OpenRouter guardrails.

## Example Usage

```hcl
data "openrouter_guardrails" "all" {}
```

## Attribute Reference

- `total_count` - Number of returned guardrails.
- `items` - List of guardrails. Each item includes `id`, `name`, `description`, `limit_usd`, `reset_interval`, `allowed_models`, `allowed_providers`, `ignored_models`, `ignored_providers`, `enforce_zdr`, `created_at`, and `updated_at`.
