# openrouter_budget_policy

Manages a native, non-default budget policy in one workspace.

## Example Usage

```hcl
resource "openrouter_budget_policy" "service" {
  name         = "Monthly service allowance"
  workspace_id = openrouter_workspace.team.id
  config = {
    limit_usd               = 100
    reset_interval          = "monthly"
    include_byok_in_budgets = false
  }
}
```

## Argument Reference

- `name` - (Required) Policy name, 1–255 characters.
- `workspace_id` - (Required, Forces replacement) Workspace UUID.
- `config` - (Required) Typed budget configuration:
  - `limit_usd` - (Optional) Positive USD limit, or null for no limit.
  - `reset_interval` - (Optional) `daily`, `weekly`, `monthly`, or null for no reset.
  - `include_byok_in_budgets` - (Required) Whether BYOK inference spend counts toward the limit.

The provider sends both nullable budget fields together. Removing a configured value sends JSON null rather than silently retaining the previous config.

## Attribute Reference

- `id` - Policy UUID returned by OpenRouter.

## Ownership and Lifecycle

A policy budget applies independently to each assigned member/API key. It is not a shared workspace spending pool; use `openrouter_workspace_budget` for aggregate spend.

Configuration updates replace the policy config and omit the bulk assignment arrays, preserving externally managed member and API-key assignments. Use `openrouter_api_key_policy_assignment` or `openrouter_member_policy_assignment` for individual direct bindings.

This resource rejects legacy-mapped policies, account-owned policies, workspace defaults, and copies replacing an existing legacy default. Those objects can have copy-on-edit or undeletable lifecycles. Do not manage the same effective controls through both a legacy guardrail and a policy resource. See the [migration guide](https://openrouter.ai/docs/guides/features/guardrails/policies-migration).

## Import

```sh
terraform import openrouter_budget_policy.service '<workspace_uuid>/<policy_uuid>'
```

Import adopts only an eligible native, non-default policy. Refresh removes a resource from state when OpenRouter returns HTTP 404; other API errors are surfaced.
