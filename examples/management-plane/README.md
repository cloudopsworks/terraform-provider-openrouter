# Identity, observability and read-only discovery

Supply an existing workspace UUID, a model slug and an explicit completed UTC usage date. Authenticate using `OPENROUTER_API_KEY`. Protect Terraform state: membership identifiers and usage aggregates are not secrets, but can be sensitive organizational information.

The default configuration performs only data-source reads. It does not create directory identities, add/remove workspace members, trigger SCIM sync, perform inference or enable logging. SCIM lookup is opt-in because a management key does not establish Enterprise provisioning access.

Setting nonempty `scim_group_ids` creates mappings and can grant access. Existing mappings should be imported first. `keep_members = true` means mapping destruction retains manual workspace access; it does not revoke that access.

`create_destination = true` opts into creation of a disabled Langfuse destination. Supply provider-specific JSON via the sensitive ephemeral `destination_config_json` input. Change the revision to submit a later configuration merge. Returned configuration is never stored; removing a JSON key does not remove it remotely. Enabling trace forwarding requires a separate deliberate change and any applicable workspace settings. No external sink is provisioned.

Model prices and activity are snapshots, not billing controls or service guarantees. The catalog reads one explicit page; activity errors if it exceeds the configured state bound rather than truncating results. Old dates age out of API retention.

See [ADR-0003](../../docs/adr/0003-identity-observability-and-read-only-apis.md) for managed membership deferral and observability limitations. Do not run this example against existing access controls merely to test the provider; local fake-API protocol tests cover lifecycle mutations without touching a real organization.
