# openrouter_providers

Lists OpenRouter providers.

## Example Usage

```hcl
data "openrouter_providers" "all" {}
```

## Attribute Reference

- `total_count` - Number of returned providers.
- `items` - List of providers. Each item includes `slug`, `name`, `status`, `description`, `moderated`, `supports_tool_call`, `supports_reasoning`, `supports_multimodal`, and `supports_response_schema`.
