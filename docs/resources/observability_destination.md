# openrouter_observability_destination

Manages an OpenRouter observability destination using typed metadata and write-only provider-specific configuration. Requires Terraform/OpenTofu `>= 1.11`. It does not create the external observability service or enable workspace-wide logging/broadcast settings.

```hcl
variable "destination_config_json" {
  type      = string
  sensitive = true
  ephemeral = true
}

resource "openrouter_observability_destination" "traces" {
  workspace_id       = var.workspace_id
  type               = "langfuse"
  name               = "team traces"
  enabled            = false
  privacy_mode       = true
  config_wo          = var.destination_config_json
  config_wo_revision = 1
}
```

## Arguments and attributes

- `workspace_id` — Required workspace UUID; replacement on change. The resource does not silently select the default workspace.
- `type` — Required stable destination type; replacement on change. Provider-specific configuration is validated by OpenRouter. S3 IAM-role bootstrap/onboarding is outside this resource's supported lifecycle.
- `name` — Required destination name.
- `enabled` — Optional, defaults to `false`, unlike the API's enabled-on-create default. Enabling a destination can forward real traffic; opt in deliberately.
- `privacy_mode` — Optional, defaults to `true`; excludes request/response bodies from forwarding. Review identity/request-context metadata separately.
- `sampling_rate` — Optional number in `[0.0001, 1]`, defaults to `1`.
- `regions` — Optional nonempty set of canonical `global`, `europe`, or `us`; defaults to `["global"]`.
- `api_key_hashes` — Optional nonempty set of API-key hashes. `null` clears the filter and permits all keys; removing a configured allowlist therefore broadens coverage.
- `broadcast_generation_cost`, `broadcast_generation_identity`, `broadcast_generation_request_context` — Optional metadata flags, default `false`.
- `config_wo` — Optional sensitive **write-only JSON object string**, required on create and when changing a positive `config_wo_revision`. Supports provider-specific nested configuration without storing it in state. Use an ephemeral sensitive variable rather than committing credentials or supplying them as command arguments.
- `config_wo_revision` — Optional integer, defaults to `0`; creation requires at least `1`. Change this marker and supply `config_wo` to send an intentional configuration update. Changing `config_wo` alone cannot trigger a plan because write-only values are not retained.
- `id`, `created_at`, `updated_at` — Computed remote UUID and timestamps.

## Configuration ownership and drift

OpenRouter masks returned configuration. The provider discards the returned `config` entirely: neither raw credentials nor masks/hashes are retained. Metadata updates do not resend configuration. Configuration writes are **merge-only**: omitted fields retain remote values, masked values are ignored, and deleting a configuration key is not documented. A revision is a write trigger, not a content hash or proof of complete configuration equality. Out-of-band configuration/credential changes cannot be drift-detected. The documented Langfuse credential placeholder literals are rejected, but masking syntax is not universally documented; arbitrary configuration strings are not classified as secrets by punctuation. A successful configuration update cannot prove credential rotation because the API may ignore masked values.

`filter_rules` and beta `broadcast_generation_cost_allocation` are deliberately not owned. Updates preserve these remote fields. Manage no other controller against metadata this resource owns. The API supplies no compare-and-swap for these edits; concurrent external changes require coordination.

Sensitive requests suppress upstream error bodies because they can echo configuration. A failure may therefore report only an HTTP status; inspect the service separately without copying secrets into logs. A malformed successful creation response with a valid UUID and matching requested scope/type retains only that identity in failed state, with the error preserved; a later refresh can recover metadata. Untrusted identities are never adopted. Entirely malformed responses and ambiguous transport failures still require manual reconciliation, not blind retries.

## Import and deletion

```sh
terraform import openrouter_observability_destination.traces <destination_uuid>
```

Import establishes metadata-only ownership with revision `0`; configuration need not be supplied. Match existing metadata in configuration to avoid unintended changes, especially `enabled` and `privacy_mode`. For a later configuration write, supply `config_wo` and set a positive revision. Deletion uses OpenRouter's soft-delete endpoint; it does not delete the external observability platform.

See [create](https://openrouter.ai/docs/api/api-reference/observability/create-an-observability-destination), [update](https://openrouter.ai/docs/api/api-reference/observability/update-an-observability-destination), and [ownership ADR](../adr/0003-identity-observability-and-read-only-apis.md).
