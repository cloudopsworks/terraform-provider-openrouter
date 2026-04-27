# openrouter_guardrail

Manages an OpenRouter guardrail.

## Example Usage

```hcl
resource "openrouter_guardrail" "prod" {
  name              = "production"
  description       = "Production restrictions"
  limit_usd         = 250
  reset_interval    = "monthly"
  allowed_providers = ["openai", "anthropic"]
  enforce_zdr       = true
}
```

## Import

```sh
terraform import openrouter_guardrail.prod <guardrail_id>
```
