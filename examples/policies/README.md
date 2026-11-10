# Native workspace policies

This example creates a workspace, an API key, typed budget/model-availability policies, and direct API-key assignments. Set `member_ids` to existing organization member user IDs to add member assignments; this example does not manage membership.

Use a management key through `OPENROUTER_API_KEY`. Terraform/OpenTofu `>= 1.11` is required. After building and installing a provider version containing these resources, initialize and validate the configuration before applying it.

Budget policies apply independently per member/API key. They are not an aggregate workspace spending pool; use `openrouter_workspace_budget` for that control.

The policy resources manage native, non-default workspace policies only. Updates replace config while leaving assignments untouched. Assignment resources manage direct bindings, refuse existing direct bindings on create, and send OpenRouter's just-read `expected_assignment` state. A concurrent change returns an error rather than being retried blindly.

Do not manage the same effective controls through legacy guardrails and these policies. Defaults, default opt-outs, account-owned policies, and legacy-derived policies require separate lifecycle handling and are outside this example.

The example is schema-validated against a locally built provider; it is not applied to a live OpenRouter account. Applying creates billable-account configuration and may change model access.
