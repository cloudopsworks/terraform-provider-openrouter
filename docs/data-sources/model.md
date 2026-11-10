# openrouter_model

Looks up one model using the official singular `/model/{author}/{slug}` API. This performs no inference and does not reserve model availability or pricing.

```hcl
data "openrouter_model" "selected" {
  slug = var.model_slug
}
```

- `slug` — Required model slug such as `author/model`; supported aliases/variants are resolved by OpenRouter.
- `id` — Resolved model identifier; can differ from the configured alias.
- `canonical_slug`, `name`, `description`, `created`, `context_length`, `expiration_date`, `knowledge_cutoff` — Computed model metadata; nullable API fields remain null.
- `supported_parameters`, `input_modalities`, `output_modalities` — Computed sets.
- `pricing` — Computed map of base price strings. Values retain API decimal precision and units (per token, image, request, or search as applicable), not prices per million tokens. Conditional overrides and discounts are not represented by this base-price snapshot.

Catalog entries, routing and prices can change or disappear between plan and apply. Do not treat this as a service-level guarantee. See [model lookup](https://openrouter.ai/docs/api/api-reference/models/get-a-model-by-its-slug).
