# openrouter_workspace

Resolves a single OpenRouter workspace by `id`, `slug`, or exact `name`.

## Example Usage

```hcl
data "openrouter_workspace" "by_slug" {
  slug = "engineering"
}

data "openrouter_workspace" "by_name" {
  name = "Engineering"
}
```

## Argument Reference

- `id` - (Optional, Computed) Workspace UUID to look up.
- `slug` - (Optional, Computed) Workspace slug to look up.
- `name` - (Optional, Computed) Exact workspace name to look up.

Set exactly one of `id`, `slug`, or `name`.

When `slug` or `id` is set, the provider uses the single-workspace API.
When `name` is set, the provider lists workspaces and searches for an exact name match.

## Attribute Reference

In addition to the arguments above, the data source exports:

- `description`
- `default_text_model`
- `default_image_model`
- `default_provider_sort`
- `io_logging_sampling_rate`
- `is_data_discount_logging_enabled`
- `is_observability_broadcast_enabled`
- `is_observability_io_logging_enabled`
- `created_at`
- `created_by`
- `updated_at`
