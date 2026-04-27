# openrouter_api_key

Manages an OpenRouter API key.

## Example Usage

```hcl
resource "openrouter_api_key" "service" {
  name                  = "service-key"
  workspace_id          = openrouter_workspace.team.id
  limit                 = 100
  limit_reset           = "monthly"
  include_byok_in_limit = true
}
```

## Argument Reference

- `name` - (Required) Name of the API key.
- `workspace_id` - (Required, Forces replacement) Workspace UUID used for deterministic key management.
- `limit` - (Optional) Spending limit in USD.
- `limit_reset` - (Optional) Reset interval: `daily`, `weekly`, `monthly`, or unset for no reset.
- `include_byok_in_limit` - (Optional) Whether BYOK usage counts toward the limit.
- `disabled` - (Optional) Whether the key is disabled.
- `expires_at` - (Optional, Forces replacement) UTC ISO-8601 expiration timestamp.
- `creator_user_id` - (Optional, Forces replacement) Optional organization member creator identifier.

## Attribute Reference

In addition to the arguments above, the resource exports:

- `id` - Stable API key hash.
- `label` - Server-generated label.
- `key` - Sensitive API key secret. Only returned on create.
- `limit_remaining` - Remaining limit in USD.
- `usage`, `usage_daily`, `usage_weekly`, `usage_monthly` - Usage totals.
- `byok_usage`, `byok_usage_daily`, `byok_usage_weekly`, `byok_usage_monthly` - BYOK usage totals.
- `created_at` - Creation timestamp.
- `updated_at` - Last update timestamp.

## Import

```sh
terraform import openrouter_api_key.service <workspace_id>_<name>
```

You can also import by canonical API key hash.
