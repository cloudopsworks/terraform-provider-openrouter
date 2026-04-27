# openrouter_workspace

Manages an OpenRouter workspace.

## Example Usage

```hcl
resource "openrouter_workspace" "team" {
  name                            = "Platform"
  slug                            = "platform"
  description                     = "Platform workspace"
  default_text_model              = "openai/gpt-4o"
  default_provider_sort           = "price"
  io_logging_sampling_rate        = 1
  is_data_discount_logging_enabled = true
}
```

## Import

```sh
terraform import openrouter_workspace.team <workspace_id>_<slug>
```

You can also import by canonical workspace ID or slug.
