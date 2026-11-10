# openrouter_model_availability_policy

Manages a native, non-default model-availability policy in one workspace.

## Example Usage

```hcl
resource "openrouter_model_availability_policy" "approved" {
  name         = "Approved providers"
  workspace_id = openrouter_workspace.team.id
  config = {
    allowed_providers     = ["openai", "anthropic"]
    enforce_zdr_openai    = true
    enforce_zdr_anthropic = true
    enforce_zdr_xai       = true
  }
}
```

## Argument Reference

- `name` - (Required) Policy name, 1–255 characters.
- `workspace_id` - (Required, Forces replacement) Workspace UUID.
- `config` - (Required) Typed configuration object. Its fields are optional and nullable:
  - Model/provider string sets: `allowed_models`, `allowed_providers`, `ignored_models`, `ignored_providers`.
  - Platform restriction string sets: `platform_banned_authors`, `platform_banned_models`, `platform_banned_providers`.
  - Region string set: `allowed_data_regions`. Use null to remove this constraint; a non-null set must contain at least one item.
  - ZDR booleans: `enforce_zdr`, `enforce_zdr_anthropic`, `enforce_zdr_google`, `enforce_zdr_openai`, `enforce_zdr_other`, `enforce_zdr_xai`.
  - Retention booleans: `allow_safety_retention_google`, `allow_safety_retention_other`.
  - Training/publication booleans: `enable_free_model_publication`, `enable_free_model_training`, `enable_paid_model_training`.
  - Other booleans: `enforce_first_party_harness_models`, `require_eu_data_region`.

A null or omitted config field clears that field on replacement; it does not preserve an old server value. Explicit false values remain false. String sets are unordered; null and empty sets are distinct except where the API rejects emptiness.

## Attribute Reference

- `id` - Policy UUID returned by OpenRouter.

## Ownership and Lifecycle

Updates replace the config but omit member/API-key assignment arrays, preserving externally managed assignments. Use separate direct assignment resources to attach subjects.

This resource rejects legacy-mapped policies, account-owned policies, workspace defaults, and copies replacing an existing legacy default. Do not manage the same effective controls through both legacy guardrails and new policies. The provider does not perform policy migration or default-policy reassignment. See the [migration guide](https://openrouter.ai/docs/guides/features/guardrails/policies-migration).

## Import

```sh
terraform import openrouter_model_availability_policy.approved '<workspace_uuid>/<policy_uuid>'
```

Import adopts only an eligible native, non-default policy. Refresh removes a resource from state on HTTP 404; other API errors remain diagnostics.
