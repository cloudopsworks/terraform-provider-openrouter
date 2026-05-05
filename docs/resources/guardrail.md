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
  enforce_zdr       = true
}
```

## Argument Reference

- `name` - (Required) Guardrail name.
- `workspace_id` - (Optional, Computed, Forces replacement) Workspace UUID for the guardrail. Defaults to the default workspace when omitted by the API.
- `description` - (Optional) Guardrail description.
- `limit_usd` - (Optional) USD limit.
- `reset_interval` - (Optional) Reset interval.
- `allowed_models` - (Optional) Explicitly allowed model slugs.
- `allowed_providers` - (Optional) Explicitly allowed provider slugs.
- `ignored_models` - (Optional) Ignored model slugs.
- `ignored_providers` - (Optional) Ignored provider slugs.
- `enforce_zdr` - (Optional) Whether zero-data-retention enforcement is enabled.

## Attribute Reference

In addition to the arguments above, the resource exports:

- `id` - Guardrail identifier.
- `created_at` - Creation timestamp.
- `updated_at` - Last update timestamp.

## Import

```sh
terraform import openrouter_guardrail.prod <workspace_id>_<name>
```

The provider requires the composite import format `<workspace_id>_<name>`.
