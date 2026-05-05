# openrouter_workspace

Manages an OpenRouter workspace.

## Example Usage

```hcl
resource "openrouter_workspace" "team" {
  name                             = "Platform"
  slug                             = "platform"
  description                      = "Platform workspace"
  default_text_model               = "openai/gpt-4o"
  default_provider_sort            = "price"
  io_logging_sampling_rate         = 1
  is_data_discount_logging_enabled = true
}
```

## Argument Reference

- `name` - (Required) Workspace name.
- `slug` - (Required) Workspace slug.
- `description` - (Optional) Workspace description.
- `default_text_model` - (Optional) Default text model.
- `default_image_model` - (Optional) Default image model.
- `default_provider_sort` - (Optional) Default provider sort strategy.
- `io_logging_api_key_ids` - (Optional) API key IDs used to filter I/O logging.
- `io_logging_sampling_rate` - (Optional) I/O logging sampling rate.
- `is_data_discount_logging_enabled` - (Optional) Enable data discount logging.
- `is_observability_broadcast_enabled` - (Optional) Enable observability broadcast.
- `is_observability_io_logging_enabled` - (Optional) Enable observability I/O logging.

## Attribute Reference

In addition to the arguments above, the resource exports:

- `id` - Stable workspace UUID.
- `created_at` - Creation timestamp.
- `created_by` - Creator identifier when available.
- `updated_at` - Last update timestamp.

## Import

```sh
terraform import openrouter_workspace.team <workspace_id>_<name>
```

The provider requires the composite import format `<workspace_id>_<name>`.
