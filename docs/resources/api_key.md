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

## Import

```sh
terraform import openrouter_api_key.service <workspace_id>_<name>
```

You can also import by canonical API key hash.
