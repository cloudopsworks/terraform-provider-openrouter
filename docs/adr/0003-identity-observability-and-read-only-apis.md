# ADR 0003: Identity, observability, and read-only API boundaries

- Status: Accepted implementation boundaries; integration owner verifies implementation.
- Evidence retrieved: 2026-11-09.
- Official specification: <https://openrouter.ai/openapi.json>, `info.version: 1.0.0`.
- Local research snapshot: `/tmp/openrouter-openapi-expanded-20261109.json` (not a committed artifact).
- Snapshot SHA-256: `71a832c2ed8d30ca3791e5677bb9f07156053454a7c07b122405c110ba8e6f16`.

The live specification is mutable: its version alone does not pin these contracts. This ADR records research and accepted implementation boundaries, not proof of implemented behavior. No authenticated calls or mutations were made for this research.

## Context and scope

Add declarative management only where the API exposes a maintainable lifecycle. Management keys are account-wide, not workspace-scoped. Organization management keys require organization-admin creation. Administrative read success does not establish permission for mutation or entitlement to other features. See [management key documentation](https://openrouter.ai/docs/guides/overview/auth/management-api-keys).

## 1. SCIM mappings, not directory CRUD

Manage a mapping between an existing SCIM group and workspace. Do not create IdP groups, users, provisioning directories, provisioning credentials, or SSO connections. These management routes are distinct from the IdP's SCIM 2.0 connector URL. SCIM mappings require Enterprise, active SSO, provisioning, and pushed groups. See the [SCIM guide](https://openrouter.ai/docs/guides/features/scim-mappings).

- `POST /scim/group-mappings` creates a group/workspace/role association; `GET`, `PATCH`, and `DELETE /scim/group-mappings/{id}` complete its lifecycle.
- Update changes only `role`; changes to either identity require replacement. Import uses the mapping UUID.
- Expose an explicit keep-members-on-destroy policy. DELETE requires `keep_members`; omission is an error, not a safe implicit default. Keeping members transfers their access to manual management. Other overlapping mappings may retain access regardless.
- Paginate group/mapping discovery. Name lookup must fail on ambiguity, not choose an arbitrary match.
- Do not trigger `POST /scim/sync-jobs` during read, plan, or capability validation. Sync is an action, not directory CRUD.

Evidence: [create mapping](https://openrouter.ai/docs/api/api-reference/scim/create-a-scim-group-mapping), [update mapping](https://openrouter.ai/docs/api/api-reference/scim/update-a-scim-group-mapping), [delete mapping](https://openrouter.ai/docs/api/api-reference/scim/delete-a-scim-group-mapping).

## 2. Workspace memberships are read-only until ownership is identifiable

Implement only a paginated `workspace_members` data source in this increment. Defer the managed manual-membership resource and its mutation clients. The API exposes add/remove routes, but that alone does not provide a safely distinguishable ownership lifecycle. `Workspace` has no documented `is_default` or `default_workspace_id`, and `WorkspaceMember` has no SCIM-provenance or implicit-membership discriminator. Do not infer these properties from names, slugs, membership IDs, or creators. No alternative authoritative discriminator was established by this research.

Reconsider a non-authoritative single workspace/user association resource only when an authoritative default-workspace and SCIM/manual-ownership contract or an equivalently safe explicit workflow is established. It must not reconcile all members or remove unrelated access. Role would be computed because add inherits organization role. Import and destroy must not adopt implicit default-workspace membership or fight IdP-managed access. Never delete users' API keys to unblock removal. Server rejection is a useful error, not a substitute for a documented safe ownership contract.

Evidence: [list](https://openrouter.ai/docs/api/api-reference/workspaces/list-workspace-members), [add](https://openrouter.ai/docs/api/api-reference/workspaces/bulk-add-members-to-a-workspace), [remove](https://openrouter.ai/docs/api/api-reference/workspaces/bulk-remove-members-from-a-workspace), [workspace guide](https://openrouter.ai/docs/guides/features/workspaces/overview).

## 3. Observability is manageable with write-only configuration

Do not defer the entire capability. A resource can own typed outer metadata and explicitly submitted configuration updates without claiming full secret drift detection. The API provides POST/list at `/observability/destinations` and GET/PATCH/DELETE by UUID. Deletion is soft. Type and workspace are absent from PATCH and therefore require replacement. The create limit is five destinations per type; avoid automatic create-before-destroy guarantees.

### Proposed initial Terraform schema

| Attribute | Contract |
| --- | --- |
| `id` | Computed UUID, import identifier. |
| `type` | Required string, replacement on change; validate documented stable types. |
| `workspace_id` | Required UUID, replacement on change; require explicit scope rather than implicit default workspace. |
| `name` | Required string. API reads can return null; never fabricate a remote name during import. Configuration must supply a name before apply. |
| `enabled` | Optional boolean, provider default `false`; always explicitly send on create. |
| `privacy_mode` | Optional boolean, provider default `true`; explicitly send on create. |
| `sampling_rate` | Optional number, provider default `1`, range 0.0001 through 1. |
| `regions` | Optional set of canonical strings, provider default `["global"]`; nonempty, values `global`, `europe`, `us`. Reject the `eu` alias in configuration to avoid normalization inconsistency. |
| `api_key_hashes` | Optional set of strings; null means all keys, non-null must contain at least one element. Clearing must send JSON null. |
| `broadcast_generation_cost`, `broadcast_generation_identity`, `broadcast_generation_request_context` | Optional booleans, default false; send explicitly on create. |
| `config_wo` | Optional sensitive write-only string containing a JSON object. Read only from Terraform configuration, never plan/state. Required at creation by operation-aware validation. |
| `config_wo_revision` | Optional integer, default `0`; nonnegative. Creation requires a positive revision and `config_wo`. Changing revision to a positive value requires `config_wo` and triggers its submission. |
| `created_at`, `updated_at` | Computed strings. |

These defaults are provider safety choices, not upstream defaults: OpenRouter defaults `enabled=true` and `privacy_mode=false`. Applying a configuration with omitted booleans to an imported destination may deliberately plan to disable it or tighten privacy; import itself must not mutate it. Document the plan behavior.

Initial scope may omit writable `filter_rules` and the beta `broadcast_generation_cost_allocation` option; omission must preserve imported remote settings, not clear them. Do not imply ownership of omitted fields. If filter management is added, model its object/groups structure and implement explicit JSON-null clearing rather than relying on `omitempty`.

### Why a JSON string, not map(string)

Configuration is provider-specific and server-validated. Request schemas allow arbitrary object properties. Concrete response variants include string credentials, nested `headers` objects, booleans, and S3 authentication-mode-dependent fields; a string map loses valid types. A write-only JSON-object string supports these variants without persisting credentials or prematurely inventing a cross-provider schema. Validate JSON object shape without echoing values in errors; exclude top-level null and reject null-valued configuration fields in the initial subset rather than pretending they are deletion requests.

The current stable types are `arize`, `braintrust`, `clickhouse`, `datadog`, `grafana`, `langfuse`, `langsmith`, `newrelic`, `opik`, `otel-collector`, `posthog`, `ramp`, `s3`, `sentry`, `snowflake`, `weave`, and `webhook`. Unknown future types require a deliberate compatibility review.

### Write, read, and ownership semantics

1. Create submits the configured object and explicit safe metadata, records only the revision and metadata, then reads metadata. Configuration must be present at creation even though schema-level Optional permits import and metadata-only management.
2. Update submits configuration only when the revision changes. An unchanged revision means configuration is ignored even if its text changes. Metadata-only updates must not resend secrets. Revision is a user intent marker, not an API version, secret hash, or proof of remote configuration equality.
3. PATCH configuration is a merge: omitted fields preserve their values and masked values are ignored. Sending `{}` is not a reset. `config: null` is not allowed by the object request schema. Neither nested-key deletion nor replacement of the whole object is documented. Removing a JSON key from configuration does not revoke it remotely. To guarantee removal, replace the destination or obtain an independently documented deletion contract.
4. Reads must discard the entire returned `config`, whether masked or plaintext; do not store it, serialize it as computed JSON, echo it in diagnostics, or use it as a source for a later PATCH. No pattern-based mask detection is necessary. Do not persist secret hashes as revision substitutes.
5. Import GETs the UUID and records metadata with revision `0`, without configuration. Metadata-only management remains possible; setting positive revision plus `config_wo` explicitly opts into a configuration write. Out-of-band credential drift cannot be detected.
6. Reject the exact Langfuse credential placeholder literals shown in the API examples at their documented credential fields. There is no universal documented masking syntax: arbitrary URLs, names or nested header strings must not be rejected merely for containing ellipses or asterisks. This narrow validation cannot identify every masked value. The API may ignore masked writes; a successful PATCH is not proof of a credential rotation. Never recycle returned configuration; document the limitation.
7. New S3 IAM-role destinations return an `externalId` inside configuration. This subset intentionally does not export configuration, so it cannot complete a Terraform-managed IAM trust-policy handoff. Defer that specific bootstrap workflow until a narrowly typed computed `external_id` output and its lifecycle are designed; do not claim universal end-to-end destination onboarding.
8. Data sources should expose only explicitly selected metadata, never raw configuration. HTTP logging and API error handling must not disclose request/response configuration or Authorization headers.

Evidence: [create API](https://openrouter.ai/docs/api/api-reference/observability/create-an-observability-destination), [update API](https://openrouter.ai/docs/api/api-reference/observability/update-an-observability-destination), [delete API](https://openrouter.ai/docs/api/api-reference/observability/delete-an-observability-destination), and the fingerprinted OpenAPI schemas `CreateObservabilityDestinationRequest`, `UpdateObservabilityDestinationRequest`, `ObservabilityDestination`, and its discriminated variants.

### Terraform compatibility and tests

Write-only attributes require Terraform 1.11 or later. They are unavailable from plan/state, cannot independently produce a diff, and must not be Computed. HashiCorp recommends pairing a write-only value with a version/trigger attribute. See [Framework write-only arguments](https://developer.hashicorp.com/terraform/plugin/framework/resources/write-only-arguments). The integration owner must verify actual repository framework/runtime compatibility; this research did not select or upgrade dependencies. Do not silently fall back to a persisted sensitive configuration attribute.

Required tests: secret canaries absent from plan/state/diagnostics; masked and unmasked reads both discarded; unchanged revision excludes config; changed revision requires and submits config; metadata-only import/update; explicit false serialization; null allowlist clearing versus omission; rejection of null/non-object config; replacement on type/workspace; 404 removal from state; errors other than 404 preserved; soft-delete success. Mock tests must not assert whole-object replacement or unsupported field deletion. Live acceptance remains opt-in and must avoid enabling trace forwarding unless explicitly authorized.

## 4. Model and usage data are read-only snapshots

Use data sources, not resources, for model discovery and activity. Support deterministic ordering/identity, pagination, nullable metadata, precise string pricing, and unknown response fields. A model lookup is not a generation request and must never run inference as validation. Distinguish public catalog from account-effective availability; authentication and regional routing may change results.

Activity is limited to the last 30 completed UTC days. Prefer an explicit date, document empty results and aging out, and support only documented filters. Account-wide results are not workspace-only unless filtered; never silently discard workspace boundaries. Store metadata/aggregates only, not prompt/completion content. Avoid assertions about fixed model counts or prices.

Evidence: [model catalog](https://openrouter.ai/docs/api/api-reference/models/list-all-models-and-their-properties), [single model](https://openrouter.ai/docs/api/api-reference/models/get-a-model-by-its-slug), [effective models](https://openrouter.ai/docs/api/api-reference/models/list-models-filtered-by-user-provider-preferences-privacy-settings-and-guardrails), [activity](https://openrouter.ai/docs/api/api-reference/analytics/get-user-activity-grouped-by-endpoint).

## Verification boundary

Capability checks are GET-only and scoped where possible. A 404 from SCIM group discovery must not be reclassified as proof that the feature does not exist or that all management access is invalid. Distinguish account entitlement/route availability from invalid credentials and from transport failures. Future mutation tests require explicit integration-test scope; documentation retrieval and existing read-only checks do not authorize provisioning, membership changes, trace forwarding, purchases, or inference.
