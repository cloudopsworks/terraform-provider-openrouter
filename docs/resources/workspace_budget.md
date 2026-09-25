# openrouter_workspace_budget

Manages one OpenRouter budget interval for a workspace.

## Example Usage

```hcl
resource "openrouter_workspace_budget" "monthly" {
  workspace_id = openrouter_workspace.team.id
  interval     = "monthly"
  limit_usd    = 250
}
```

## Argument Reference

- `workspace_id` - (Required, Forces replacement) Workspace reference. OpenRouter accepts either a workspace UUID or slug. A UUID returned by `openrouter_workspace.id` is recommended for stability.
- `interval` - (Required, Forces replacement) Budget interval: `daily`, `weekly`, `monthly`, or `lifetime`.
- `limit_usd` - (Required) Positive USD limit.

## Attribute Reference

- `id` - Composite budget identity in `<workspace_ref>/<interval>` format. The provider preserves the configured workspace reference (UUID or slug).
- `created_at` - Creation timestamp returned by OpenRouter.
- `updated_at` - Last update timestamp returned by OpenRouter.

## Interval Ordering and Ownership

OpenRouter enforces interval precedence: configure `lifetime`, then `monthly`, then `weekly`, then `daily`. When introducing multiple budgets, apply them in that order (separate applies are the clearest option). Manage each workspace/interval pair with exactly one resource; creation refuses to overwrite an existing interval and directs you to import it.

`workspace_id` is part of this resource's immutable identity. A slug is accepted and remains in the composite ID, but a canonical UUID is recommended because a slug can be renamed or otherwise become less stable.

## Import

```sh
terraform import openrouter_workspace_budget.monthly <workspace_ref>/<interval>
```

For example, use `<workspace_uuid>/monthly` (recommended) or `<workspace_slug>/monthly` for a monthly budget.
