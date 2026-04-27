# openrouter_workspaces

Lists OpenRouter workspaces.

## Example Usage

```hcl
data "openrouter_workspaces" "all" {}
```

## Attribute Reference

- `total_count` - Number of returned workspaces.
- `items` - List of workspaces. Each item includes `id`, `name`, `slug`, `description`, `default_text_model`, `default_image_model`, `default_provider_sort`, `io_logging_sampling_rate`, `is_data_discount_logging_enabled`, `is_observability_broadcast_enabled`, `is_observability_io_logging_enabled`, `created_at`, `created_by`, and `updated_at`.
