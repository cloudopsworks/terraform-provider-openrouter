# openrouter_organization

Returns organization membership information using the documented organization members endpoint.

## Example Usage

```hcl
data "openrouter_organization" "current" {}
```

## Attribute Reference

- `total_count` - Number of returned members.
- `members` - List of organization members. Each item includes `id`, `first_name`, `last_name`, `email`, and `role`.
