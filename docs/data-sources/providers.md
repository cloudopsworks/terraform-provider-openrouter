# openrouter_providers

Lists OpenRouter providers.

## Example Usage

```hcl
data "openrouter_providers" "all" {}
```

## Attribute Reference

- `total_count` - Number of returned providers.
- `items` - List of providers. Each item includes `slug`, `name`, `privacy_policy_url`, `terms_of_service_url`, `status_page_url`, `headquarters`, `datacenters`, plus any currently exposed compatibility/status fields returned by the API.
