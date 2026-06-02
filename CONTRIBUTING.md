# Contributing to terraform-provider-openrouter

Thanks for your interest in improving the OpenRouter Terraform/OpenTofu provider.
Contributions of all sizes are welcome — bug reports, docs fixes, new resources,
and acceptance tests.

## Ways to contribute

- **Report a bug or request a feature** via the
  [issue tracker](https://github.com/cloudopsworks/terraform-provider-openrouter/issues).
- **Pick up a [`good first issue`](https://github.com/cloudopsworks/terraform-provider-openrouter/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22)**
  if you're new to the codebase.
- **Improve coverage of the OpenRouter management API** — new resources, data
  sources, or arguments.

## Development setup

Requirements:

- Go `1.25+`
- Terraform `>= 1.7` or OpenTofu `>= 1.7`
- An OpenRouter **management** API key (for acceptance tests)

Common tasks:

```sh
make fmt      # format Go code
make lint     # run golangci-lint
make test     # unit tests
make build    # build the provider binary
```

Acceptance tests talk to the live OpenRouter management API and create real
resources, so they require a key and are opt-in:

```sh
export OPENROUTER_API_KEY="your-management-key"
make testacc
```

## Pull request flow

1. Fork the repo and create a topic branch.
2. Make your change, keeping it focused and reasonably small.
3. Run `make fmt lint test` and, where relevant, `make testacc`.
4. Update or add docs under `docs/` and an example under `examples/` for any new
   resource, data source, or argument.
5. Open a pull request using the
   [PR template](.github/PULL_REQUEST_TEMPLATE.md) and describe the change and how
   you tested it.

Maintainers aim to triage new issues and PRs within a couple of business days.

## Docs and examples

`docs/` is the source for the Terraform Registry documentation; every resource
and data source should have an `## Example Usage` block with runnable HCL. The
flagship multi-team example lives in [`examples/governance/`](examples/governance/).

## Code of conduct

Be respectful and constructive. Cloud Ops Works reserves the right to moderate
discussion that doesn't follow that standard.

## License

By contributing, you agree that your contributions are licensed under the
[Apache License 2.0](LICENSE).
