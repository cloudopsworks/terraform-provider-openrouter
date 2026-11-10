# openrouter_scim_groups

Reads existing IdP-provisioned SCIM groups, following API pagination. It never provisions groups or triggers a sync. Requires the account's SCIM/SSO feature to be available.

```hcl
data "openrouter_scim_groups" "engineering" {
  display_name = "Engineering"
}
```

- `display_name`, `external_id` — Optional lookup filters. Group names are not guaranteed to be unique; resolve a unique group before using its UUID in a mapping.
- `total_count` — Number of returned matching groups.
- `items` — Groups with `id`, `organization_id`, nullable `external_id`, `display_name`, `created_at`, and `updated_at`.

Group metadata is stored in Terraform state. Protect state access accordingly. An authorization/feature error is not reported as an empty group list. See [SCIM mappings](https://openrouter.ai/docs/guides/features/scim-mappings).

Pagination is bounded at 100,000 reported entries; inconsistent totals, duplicate identities or incomplete pages return errors rather than silently truncating or treating malformed data as an empty list.
