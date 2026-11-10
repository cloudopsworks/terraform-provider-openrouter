# openrouter_scim_group_mapping

Manages the association of an existing IdP-provisioned SCIM group with a workspace. This does not create directory users/groups, configure SSO, or run a SCIM sync job. Enterprise/SSO/provisioning access is required; a management key alone does not establish feature entitlement.

```hcl
resource "openrouter_scim_group_mapping" "engineering" {
  scim_group_id = var.scim_group_id
  workspace_id = var.workspace_id
  role         = "member"
  keep_members = true
}
```

## Arguments and attributes

- `scim_group_id`, `workspace_id` — Required stable UUIDs; changing either replaces the mapping.
- `role` — Required `admin` or `member`; the API supports updating this field.
- `keep_members` — Optional, defaults to `true`. Explicitly controls deletion: `true` preserves affected workspace memberships as manual access. `false` permits removal of mapping-derived access, except access retained through other mappings. This is a provider safety default; the API requires a choice and has no implicit default. Retaining memberships means destroying a mapping does **not** revoke that retained access.
- `id`, `organization_id`, `created_at`, `updated_at` — Computed remote mapping identity and metadata.

Do not manage the same access through a second mapping resource or a competing IdP/manual controller. A group can map to several workspaces; overlapping mappings can affect the effective role. Existing mappings must be imported rather than silently adopted by create. Malformed successful creates may report a candidate mapping UUID for inspection; verify group/workspace ownership before importing it. A candidate diagnostic never adopts the object or retries creation. Entirely malformed responses and transport failures can remain ambiguous, so inspect remote mappings before retrying. Membership changes can be asynchronous; the resource represents mapping configuration, not a promise that a sync has completed.

## Import

```sh
terraform import openrouter_scim_group_mapping.engineering <mapping_uuid>
```

Imported mappings default to `keep_members = true`; review any subsequent change to the deletion policy. Imports verify the remote object. Unknown or malformed identities and non-404 API errors are not treated as deletion.

See the [SCIM guide](https://openrouter.ai/docs/guides/features/scim-mappings), [mapping deletion contract](https://openrouter.ai/docs/api/api-reference/scim/delete-a-scim-group-mapping), and [ownership ADR](../adr/0003-identity-observability-and-read-only-apis.md).
