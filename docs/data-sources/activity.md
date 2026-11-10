# openrouter_activity

Reads endpoint-level usage aggregates for one explicit completed UTC date. This is a snapshot, not a managed budget, billing enforcement control or inference operation. Requires a management key; OpenRouter retains the last 30 completed UTC days.

```hcl
data "openrouter_activity" "daily" {
  date               = var.usage_date
  workspace_id       = var.workspace_id
  group_by_workspace = true
  max_items          = 1000
}
```

- `date` — Required real date in `YYYY-MM-DD` format. The API enforces the rolling retention window; old dates eventually become unavailable. Pass a deliberate date rather than an implicit moving `timestamp()` window.
- `workspace_id`, `api_key_hash`, `user_id` — Optional filters. A key hash is an identifier, **not** a secret API key. Without a workspace filter, results are account-wide.
- `group_by_workspace` — Optional boolean, defaults to `false`. Requests API workspace grouping so rows include workspace IDs.
- `max_items` — Optional provider-side result/state bound, default `1000`, allowed range `1..10000`. The API has no activity pagination; exceeding the bound returns an error rather than silently truncating. This is not a server-side limit.
- `total_count` — Number of rows in this snapshot, not a spend total.
- `items` — `date`, `model`, `model_permaslug`, `endpoint_id`, `provider_name`, nullable `workspace_id`; `requests`, `prompt_tokens`, `completion_tokens`, `reasoning_tokens`, `cached_tokens`; numeric USD `usage` (OpenRouter credits) and `byok_usage_inference` (external BYOK spend).

Results may be empty and may change on later refreshes. Historical activity recorded before workspace resolution is attributed to the default workspace. No prompts or completion text are read. Protect state because usage metadata and identifiers can be sensitive. See [activity API](https://openrouter.ai/docs/api/api-reference/analytics/get-user-activity-grouped-by-endpoint).
