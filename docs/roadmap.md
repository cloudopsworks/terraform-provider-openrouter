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

## Deferred APIs

- **Default guardrail:** defer until its lifecycle can be represented without Terraform attempting an unsafe delete or replacement.
- **Provider observability configuration:** defer because the API masks provider-specific configuration, preventing reliable drift detection.
- **SCIM and interns:** defer because their identity and organization lifecycle needs a dedicated design and ownership model.
- **Inference, models, usage, and other read-only APIs:** leave outside this provider unless they become useful stable data sources; they do not currently require a managed-resource lifecycle.

Future additions should meet the same bar: a stable identity, clear create/read/update/delete semantics, and state that is safe to store and compare.
