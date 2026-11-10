# Management-plane roadmap

This provider prioritizes OpenRouter APIs that represent durable configuration with a clear Terraform lifecycle. This is a record of qualitative research and implementation decisions, not a prevalence survey.

## Signals considered

- An [upstream provider request for guardrail-key assignment](https://github.com/OpenRouterTeam/terraform-provider-openrouter/issues/130) directly requested a Terraform resource for associating a key with a guardrail. That maps to a durable, exclusive assignment and is implemented as `openrouter_guardrail_key_assignment`.
- An [upstream request for a default guardrail](https://github.com/OpenRouterTeam/terraform-provider-openrouter/issues/129) identifies a desired control, but its lazy, undeletable lifecycle is not currently safe to model as an ordinary Terraform resource.
- Community discussions describe concern about [runaway spend and caps](https://www.reddit.com/r/openrouter/comments/1ttk5c9/) and [usage-limit behavior](https://www.reddit.com/r/openrouter/comments/1seo9w1/), which support managing per-workspace budgets. These posts are individual reports, not evidence of request volume.
- A discussion of [sharing OpenRouter access](https://www.reddit.com/r/openrouter/comments/1sjy14f/) supports scoped access and credential-management workflows, but likewise is qualitative evidence only.

## Implemented priorities

1. **Guardrail-key assignment** — exclusive, importable association of an existing API key hash with a guardrail.
2. **Workspace budgets** — independent `daily`, `weekly`, `monthly`, and `lifetime` limits, represented by an immutable workspace/interval identity.
3. **BYOK credentials** — metadata, allowlists, and write-only credential rotation. The OpenRouter [BYOK create](https://openrouter.ai/docs/api/api-reference/byok/create-a-byok-provider-credential), [update](https://openrouter.ai/docs/api/api-reference/byok/update-a-byok-provider-credential), and [list](https://openrouter.ai/docs/api/api-reference/byok/list-byok-provider-credentials) endpoints support this lifecycle; raw credential material remains outside Terraform state.
4. **Native workspace policies** — typed `openrouter_budget_policy` and `openrouter_model_availability_policy` resources for name/config ownership, with singular `openrouter_api_key_policy_assignment` and `openrouter_member_policy_assignment` resources using optimistic concurrency.
5. **SCIM group mappings** — importable associations of existing IdP groups to workspaces; deletion preserves workspace membership by default. Directory provisioning and one-shot sync jobs are not modeled as CRUD resources.
6. **Observability destinations** — typed metadata lifecycle with write-only, revision-triggered provider configuration. Returned masked configuration is discarded; drift guarantees apply to metadata, not secret/configuration contents.
7. **Read-only discovery and usage** — paginated SCIM groups/workspace members, bounded model catalog pages, single-model lookup, and explicit-date activity snapshots.

## API review: 2026-11-09

The current [OpenAPI](https://openrouter.ai/openapi.json), [policy guide](https://openrouter.ai/docs/guides/features/guardrails/policies), and [migration guide](https://openrouter.ai/docs/guides/features/guardrails/policies-migration) document workspace policy CRUD and concurrency-checked assignments. These are durable configuration, unlike inference operations. See [ADR-0002](adr/0002-native-workspace-policy-lifecycle.md) for the retrieved schema fingerprint and ownership decision.

This expansion covers native, non-default budget/model-availability policies and direct subject bindings. Policy updates preserve assignment lists. It does not migrate legacy controls, manage workspace/account defaults, or turn inherited defaults into opt-outs. Existing legacy resources remain supported; avoid overlapping ownership of the same effective controls.

The accepted follow-up adds identity/observability configuration and useful read-only discovery. [ADR-0003](adr/0003-identity-observability-and-read-only-apis.md) records the same-day schema fingerprint, masked-secret approach, snapshot limits and membership safety decision. Successful GET probes establish read capability only, not live lifecycle coverage; the test account's SCIM groups route returned 404.

## Deferred APIs

- **Moderation policy authoring:** documented CRUD exists, but its larger nested classifier/filter surface needs a separate typed schema and lifecycle tests. Direct assignments can reference an existing eligible moderation policy.
- **Account/workspace defaults and policy migration:** default coverage and opt-outs affect other subjects/workspaces. Editing legacy-mapped policies can create new IDs, and some legacy/default objects cannot be deleted; defer until ownership and migration are explicitly modeled.
- **Workspace membership mutations:** read-only listing is included. Default membership is implicit, but the documented Workspace/WorkspaceMember responses contain no reliable default/SCIM ownership discriminator. Defer managed mutations and import/destroy ownership until an authoritative contract exists; never guess based on names/slugs or delete API keys to bypass removal blockers.
- **Rate-limit policies:** assignment enums mention `rate_limit`, but the retrieved OpenAPI does not provide corresponding policy CRUD; no resource is inferred from that enum.
- **Presets:** the retrieved API does not expose a complete ordinary CRUD lifecycle; do not treat versioned creation alone as Terraform ownership.
- **Default guardrail:** defer until its lifecycle can be represented without Terraform attempting an unsafe delete or replacement.
- **Advanced observability:** initial destinations support metadata and write-only configuration writes, not full config drift or key removal through merge patches. Filter-rule authoring, beta cost-allocation metadata and S3 IAM-role external-ID bootstrap remain deferred.
- **SCIM directory provisioning, sync jobs and interns:** SCIM group mappings are included, but external IdP identities and action-like sync jobs do not have ordinary Terraform ownership in this provider. Intern organization lifecycle remains deferred.
- **Inference and further analytics:** inference remains excluded. Models and dated usage are read-only data sources, not resources. Effective-account model catalogs, full analytics dashboards, credit purchases and stored generation content remain outside this increment.

Future additions should meet the same bar: a stable identity, clear create/read/update/delete semantics, and state that is safe to store and compare.
