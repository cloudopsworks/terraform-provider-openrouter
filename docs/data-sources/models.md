# openrouter_models

Reads one bounded page of the model catalog. No inference, unbounded automatic catalog retrieval or model creation occurs.

```hcl
data "openrouter_models" "page" {
  search = "reasoning"
  limit  = 20
  offset = 0
}
```

- `offset` — Optional nonnegative page offset, default `0`.
- `limit` — Optional page size, default `100`, maximum `1000`.
- `search` — Optional search text (`q` in the API).
- `input_modalities`, `output_modalities` — Optional API comma-separated modality filters. The API defaults output modality to text; use its supported `all` selector deliberately when required.
- `total_count` — API catalog count for this query, not necessarily the page size.
- `has_more` — Whether the API reports more results.
- `items` — One page, deterministically sorted by model ID, with the metadata and base string pricing described in [openrouter_model](model.md).

Follow pages explicitly when necessary and bound state size. `total_count`, availability and prices are snapshots, not stable assertions for live tests. See [catalog API](https://openrouter.ai/docs/api/api-reference/models/list-all-models-and-their-properties).
