# openrouter_guardrail

Manages an OpenRouter guardrail.

## Example Usage

```hcl
resource "openrouter_guardrail" "prod" {
  name              = "production"
  workspace_id      = openrouter_workspace.team.id
  description       = "Production restrictions"
  limit_usd         = 250
  reset_interval    = "monthly"
  allowed_providers = ["openai", "anthropic"]

  enforce_zdr_anthropic = true
  enforce_zdr_google    = false
  enforce_zdr_openai    = true
  enforce_zdr_other     = false

  content_filter_builtins = [
    {
      slug   = "email"
      action = "redact"
    },
    {
      slug       = "regex-prompt-injection"
      action     = "flag"
      scan_scope = "user_only"
    }
  ]

  content_filters = [
    {
      pattern = "SECRET_[A-Z0-9]+"
      action  = "redact"
      label   = "[SECRET]"
    }
  ]
}
```

## Argument Reference

- `name` - (Required) Guardrail name.
- `workspace_id` - (Optional, Computed, Forces replacement) Workspace UUID for the guardrail. If omitted, the provider records the workspace returned by OpenRouter.
- `description` - (Optional) Guardrail description.
- `limit_usd` - (Optional) USD limit.
- `reset_interval` - (Optional) Reset interval.
- `allowed_models` - (Optional) Explicitly allowed model slugs.
- `allowed_providers` - (Optional) Explicitly allowed provider slugs.
- `ignored_models` - (Optional) Ignored model slugs.
- `ignored_providers` - (Optional) Ignored provider slugs.
- `content_filter_builtins` - (Optional, Computed) Builtin content filters applied to requests. Explicitly setting an empty list clears builtin filters. Each object supports:
  - `slug` - (Required) Builtin filter identifier. Known values include `email`, `phone`, `ssn`, `credit-card`, `ip-address`, `person-name`, `address`, and `regex-prompt-injection`; future OpenRouter values are accepted.
  - `action` - (Required) Action when the builtin filter triggers. Known values are `redact`, `block`, and `flag`; future OpenRouter values are accepted. OpenRouter supports `flag` only for `regex-prompt-injection`.
  - `scan_scope` - (Optional, Computed) Prompt-injection scan scope. Known values are `user_only` and `all_messages`; future OpenRouter values are accepted.
  - `label` - (Computed) OpenRouter-assigned label. This value is read-only and cannot be configured.
- `content_filters` - (Optional, Computed) Custom regex content filters applied to request messages. Explicitly setting an empty list clears custom filters. Each object supports:
  - `pattern` - (Required) Regex pattern matched against request content.
  - `action` - (Required) Action when the pattern matches. Known values are `redact` and `block`; future OpenRouter values are accepted.
  - `label` - (Optional, Computed) Optional label used in redaction placeholders or block/error messages.
- `enforce_zdr` - (Optional, Computed, Deprecated) Deprecated aggregate zero-data-retention enforcement flag. Prefer the per-provider fields below. If used together with per-provider fields, values must not conflict.
- `enforce_zdr_anthropic` - (Optional, Computed) Whether to enforce zero data retention for Anthropic models. Falls back to `enforce_zdr` when omitted.
- `enforce_zdr_google` - (Optional, Computed) Whether to enforce zero data retention for Google models. Falls back to `enforce_zdr` when omitted.
- `enforce_zdr_openai` - (Optional, Computed) Whether to enforce zero data retention for OpenAI models. Falls back to `enforce_zdr` when omitted.
- `enforce_zdr_other` - (Optional, Computed) Whether to enforce zero data retention for models from other providers. Falls back to `enforce_zdr` when omitted.

## Attribute Reference

In addition to the arguments above, the resource exports:

- `id` - Guardrail identifier.
- `created_at` - Creation timestamp.
- `updated_at` - Last update timestamp.

## Notes

- Omit `content_filter_builtins` or `content_filters` to leave the OpenRouter-side value unmanaged/computed.
- Set `content_filter_builtins = []` or `content_filters = []` to clear the corresponding filters.
- Builtin filter `label` is returned by OpenRouter and is intentionally not configurable.

## Import

```sh
terraform import openrouter_guardrail.prod <workspace_id>_<name>
```

The provider requires the composite import format `<workspace_id>_<name>`.
