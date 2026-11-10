# ADR-0002: Native workspace policy lifecycle

## Status

Accepted for the workspace policy expansion.

## Evidence

The official [OpenAPI](https://openrouter.ai/openapi.json), [policy guide](https://openrouter.ai/docs/guides/features/guardrails/policies), and [migration guide](https://openrouter.ai/docs/guides/features/guardrails/policies-migration) were inspected on 2026-11-09. The retrieved OpenAPI has `info.version=1.0.0` and SHA-256 `71a832c2ed8d30ca3791e5677bb9f07156053454a7c07b122405c110ba8e6f16`; the version is not a dated release identifier.

## Decision

Add typed budget and model-availability policy resources and singular API-key/member assignment resources. Preserve existing legacy resources without automatically migrating them.

Policy resources own a native, non-default workspace policy's name and config. Their updates replace config and omit both bulk assignment arrays. Separate assignment resources own direct subject bindings and use the API's complete, freshly read `expected_assignment` state. Conflicts remain errors rather than blind retries.

Reject adoption or mutation of legacy-mapped, account-owned, default, and legacy-default-replacement policies. Migrated policy edits can create a new ID, and some migrated/default objects cannot be deleted. Those lifecycles are not ordinary Terraform CRUD and need a dedicated migration/default design.

## Consequences

- Budget policies govern per-member/per-key spend; workspace budgets remain the aggregate control.
- Typed nullable fields make config replacement and clearing explicit.
- Individual assignments do not replace a policy's full shared assignment list.
- Inherited default assignments and default opt-outs remain outside resource ownership.
- Moderation policies, account/default policies, membership lifecycle, and migration are deferred rather than exposed through untyped JSON or ambiguous ownership.
- Validation uses local HTTP fixtures and Terraform/OpenTofu schema checks; no live API mutation is implied by passing local tests.
