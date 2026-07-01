# ADR-0001: Official OpenRouter Guardrail API Terraform Schema

## Status

Accepted for implementation on `feature/guardrail-official-api`.

## Context

The provider currently models only the basic OpenRouter guardrail surface: workspace, spend/routing fields, timestamps, and the deprecated aggregate `enforce_zdr` field. The official OpenRouter OpenAPI (`https://openrouter.ai/openapi.json`, OpenAPI 3.1.0, API v1.0.0) includes additional guardrail fields that must ship in the next release:

- Per-provider ZDR controls: `enforce_zdr_anthropic`, `enforce_zdr_google`, `enforce_zdr_openai`, `enforce_zdr_other`.
- Builtin content filters: `content_filter_builtins` with `slug`, `action`, system-assigned `label`, and optional `scan_scope`.
- Custom content filters: `content_filters` with `pattern`, `action`, and optional `label`.

The expansion must be additive and preserve existing configurations using `enforce_zdr`.

## Decision

Implement an additive guardrail schema expansion that:

1. Keeps deprecated `enforce_zdr` read/write for backward compatibility, while documenting per-provider ZDR fields as preferred.
2. Rejects contradictory mixed ZDR configuration: if `enforce_zdr` is configured together with any per-provider ZDR field set to the opposite value, return a diagnostic. Consistent mixed values are allowed; legacy-only configs continue to work.
3. Preserves configured legacy `enforce_zdr` state when OpenRouter returns `null` for the deprecated field and per-provider fields are present, preventing refresh/apply drift for existing users.
4. Models `content_filter_builtins` and `content_filters` as `ListNestedAttribute`, not `SetNestedAttribute`, because nested objects include computed/read-only fields and set hashing would be fragile.
5. Treats API list order as authoritative for the initial implementation and requires no-op acceptance coverage. If live tests show OpenRouter reorders filters, canonicalize before release.
6. Exposes builtin filter `label` as computed-only. OpenRouter marks request `label` as deprecated/ignored and response `label` as system-assigned.
7. Uses permissive string validation for filter `slug`, `action`, and `scan_scope`; known values are documented but not strictly enforced because OpenRouter marks those enums as allowing unknown values.
8. Defines filter list semantics:
   - Omitted/null config: do not send the field; API-returned values remain computed state.
   - Explicit empty list: clear the filter list; update requests serialize this as JSON `null` per OpenRouter’s removal contract.
   - Populated list: send configured objects.
9. Extracts a shared guardrail upsert request builder used by both Create and Update, with direct tests for nil, empty/clear, populated, and mixed-ZDR cases.

## Decision Drivers

- Match the official OpenRouter guardrail API in the next release.
- Preserve backward compatibility for existing Terraform configurations.
- Avoid perpetual diffs from server-normalized, computed, deprecated, or read-only fields.
- Keep the provider forward-compatible with future OpenRouter enum additions.
- Keep create/update behavior maintainable as guardrail schema complexity grows.

## Alternatives Considered

### Use `SetNestedAttribute` for filter collections

Rejected. Sets are unordered and object identity/hash behavior becomes fragile when nested objects contain computed fields such as builtin `label`.

### Canonical-sort all filters before writing state

Deferred. Sorting can avoid API reordering drift, but can also create diffs if users configure a different order and OpenRouter preserves it. Initial implementation treats API order as authoritative and locks that assumption with acceptance no-op tests. Canonical sorting remains the fallback if live tests prove API reordering.

### Remove or make legacy `enforce_zdr` read-only

Rejected. Existing users can already configure `enforce_zdr`; removing write support would be a breaking change.

### Allow all mixed legacy/per-provider ZDR combinations

Rejected. Contradictory Terraform configuration would be confusing and drift-prone. The provider will reject contradictions and recommend per-provider fields.

### Expose builtin filter `label` as configurable

Rejected. OpenRouter documents builtin request `label` as deprecated and ignored, and returns a system-assigned label. User-configurable label would invite permanent diffs.

### Add strict enum validators

Rejected. OpenRouter’s OpenAPI marks filter action, builtin slug, and scan-scope enums with unknown-values allowed. Strict validators would require unnecessary provider releases when OpenRouter adds enum values.

## Consequences

- More conversion code is required than the current simple string-set model.
- Create and Update must share request-building logic to prevent divergence.
- Tests must explicitly cover ZDR compatibility, filter null/empty/populated behavior, computed labels, and no-op behavior after apply.
- Docs must explain that per-provider ZDR fields supersede legacy `enforce_zdr`, and that contradictory mixed config is invalid.
- If acceptance tests show OpenRouter reorders filter arrays, implementation must add canonical ordering before release or revisit the schema choice.

## Follow-ups

- Implement `buildGuardrailUpsertRequest` or equivalent and route both Create and Update through it.
- Add diagnostics for contradictory `enforce_zdr` plus per-provider ZDR values.
- Preserve configured legacy `enforce_zdr` state when API returns null for the deprecated field.
- Add tests for explicit empty-list clearing and JSON `null` serialization on update.
- Add no-op acceptance tests after content-filter create/update/import to catch ordering or normalization drift.
- Update resource/data-source docs and examples for all official guardrail fields.

## References

- OpenRouter OpenAPI: `https://openrouter.ai/openapi.json`.
- Current client guardrail model/request: `internal/client/types.go`.
- Current guardrail resource schema and request wiring: `internal/provider/guardrail_resource.go`.
- Current guardrails data source mapping: `internal/provider/guardrails_data_source.go`.
- Current guardrail acceptance tests: `internal/provider/guardrail_resource_acc_test.go`.
- Planning artifact: `.omx/plans/guardrail-official-api-coverage.md`.
