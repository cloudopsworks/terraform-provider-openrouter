# Example: Govern OpenRouter for multiple teams

This is the flagship example for `cloudopsworks/openrouter`. It shows the
capability that key-only OpenRouter providers don't have: managing OpenRouter as
a **governed platform**, not just issuing API keys.

For each team in `var.teams` it provisions, as code:

- an `openrouter_workspace` — an isolated unit with a default model,
- an `openrouter_guardrail` — a monthly USD cap, allowed providers, and
  zero-data-retention enforcement, scoped to that workspace,
- an `openrouter_api_key` — a spend-limited, monthly-reset key scoped to the
  workspace.

Adding a team is one map entry — no new resource blocks.

## Usage

```sh
export OPENROUTER_API_KEY="your-management-key"

terraform init
terraform plan
terraform apply
```

To govern your own teams, override `teams`:

```hcl
teams = {
  payments = {
    name               = "Payments Team"
    default_text_model = "openai/gpt-4o"
    monthly_limit_usd  = 300
    allowed_providers  = ["openai", "anthropic"]
  }
}
```

## Notes

- The API key secret is only returned by OpenRouter on create; it is exported as
  the sensitive `api_keys` output.
- Argument names here match the provider docs under `../../docs/resources/`.
