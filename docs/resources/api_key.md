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
- `workspace_id` - (Optional, Computed, Forces replacement) Workspace UUID to send during key creation. If omitted, the provider records the workspace returned by OpenRouter.
- `limit` - (Optional) Spending limit in USD.
- `limit_reset` - (Optional) Reset interval: `daily`, `weekly`, `monthly`, or unset for no reset.
- `include_byok_in_limit` - (Optional) Whether BYOK usage counts toward the limit.
- `disabled` - (Optional) Whether the key is disabled.
- `expires_at` - (Optional, Forces replacement) UTC ISO-8601 expiration timestamp.
- `creator_user_id` - (Optional, Computed, Forces replacement) Optional organization member creator identifier. If omitted, the provider records the creator returned by OpenRouter.

### Disabled key creation

OpenRouter does not accept `disabled` during key creation. For `disabled = true`,
the provider creates the key, disables it with a separate update, then reads its
actual status. This is **not atomic**: the key is initially enabled until the
update succeeds. Use `limit = 0` when a new key must not have a positive spending
allowance during that window. A zero limit is not a substitute for disabling the key.

If the disable or confirmation request fails, Terraform reports an error and
retains the created key's ID, one-time secret, and last API-returned status in
state so it remains tracked for recovery or destruction. Do not assume the key is
disabled after a failed apply.

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

The provider requires the composite import format `<workspace_id>_<name>`.
