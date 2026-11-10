# openrouter_workspace_members

Reads all pages of membership for one workspace; never adds/removes people or changes roles.

```hcl
data "openrouter_workspace_members" "team" {
  workspace_id = var.workspace_id
}
```

- `workspace_id` — Required workspace identifier.
- `total_count` — Number of returned members.
- `items` — Memberships with `id`, `workspace_id`, `user_id`, `role`, and `created_at`. Roles are observations, not configurable grants.

The default workspace includes organization members implicitly. The documented response does not reliably distinguish implicit/default or SCIM-managed membership from manually managed membership. Consequently this provider intentionally does **not** expose membership mutation resources. Do not infer ownership from a slug, creator or membership ID. Removal can also be blocked by active API keys; the provider never deletes keys to work around this.

Membership user identifiers are stored in Terraform state; restrict state access. See the [workspace guide](https://openrouter.ai/docs/guides/features/workspaces/overview) and [ownership ADR](../adr/0003-identity-observability-and-read-only-apis.md).

Pagination is bounded at 100,000 reported entries; inconsistent totals, duplicate identities or incomplete pages return errors rather than silently truncating or treating malformed data as an empty list.
