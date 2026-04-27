# openrouter_api_keys

Lists OpenRouter API keys.

## Example Usage

```hcl
data "openrouter_api_keys" "workspace" {
  workspace_id = openrouter_workspace.team.id
}
```
