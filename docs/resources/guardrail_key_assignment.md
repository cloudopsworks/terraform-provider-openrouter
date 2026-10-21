# openrouter_guardrail_key_assignment

Assigns one existing OpenRouter API key to one guardrail.

## Example Usage

```hcl
resource "openrouter_guardrail_key_assignment" "service" {
  guardrail_id = openrouter_guardrail.team.id
  key_hash     = openrouter_api_key.service.id
}
```

## Argument Reference

- `guardrail_id` - (Required) Guardrail identifier.
- `key_hash` - (Required, Forces replacement) API key hash. This is the `id` exported by `openrouter_api_key`, not its sensitive `key` value.

## Attribute Reference

- `id` - Composite assignment identity in `<guardrail_id>/<key_hash>` format.

## Ownership and Reassignment

An API key can be assigned to only one guardrail. Manage each key with exactly one `openrouter_guardrail_key_assignment` resource. Creation refuses to overwrite an assignment owned by a different guardrail; import that assignment or remove its other owner first.

Changing `guardrail_id` reassigns the key through OpenRouter. The provider assigns the destination before verifying it and never attempts a compensating unassignment after a failed verification, because a concurrent external reassignment could otherwise leave the key unguarded. This reduces risk but does not provide a cross-process lock: avoid concurrent Terraform applies or out-of-band reassignment for the same key.

## Import

```sh
terraform import openrouter_guardrail_key_assignment.service <guardrail_id>/<key_hash>
```

Use the guardrail identifier and API key hash separated by one slash.
