# Go Application Template

## Agent command policy

The `Makefile` remains for compatibility, but direct Make invocation is deprecated
for README and Git-flow/version operations. Agents must use the Tronador CLI:

- Run `tronador readme build` to render README output. If committed target/dependency
  documentation also needs refresh, run `tronador docs targets` first.
- Use `tronador versions` for branch and version workflows. Before starting a
  feature, verify that the active GitVersion configuration declares the intended
  `# Agents: WayOfWork=...` policy; missing or contradictory metadata is a blocker.
  `tronador versions feature start "<slug>"` starts from `develop` for GitFlow and
  from the configured primary branch for GitHub Flow or trunk-based development.
  Use the matching `publish` and `finish` subcommands, and use
  `tronador versions tag --publish` when creating and publishing a version tag.
- For destructive cleanup after a verified merge, use
  `tronador versions feature purge ... --allow-network`,
  `tronador versions hotfix purge ... --allow-network`, or
  `tronador versions release purge ... --allow-network`. Fail closed if the
  WayOfWork policy, merge evidence, authentication, or remote state is missing or
  contradictory; do not fall back to Make. For a GitFlow release merged into
  `support/*`, the release must also be back-integrated into `develop` before purge;
  otherwise Tronador intentionally refuses deletion. CI jobs must install the CLI first with
  `uses: cloudopsworks/install-tronador-cli@v1`.
- Run `tronador project version --generate --yes` only after verified GitHub-template
  or source-owner policy explicitly makes `_VERSION` writable. When authorized, the
  command only writes the guarded file; review, stage, and commit it through the normal
  feature-branch and pull-request flow. In downstream implementations `_VERSION` remains
  a protected template marker; marker presence or inherited prose never grants authority.

This repository is a **CloudOps Works Go application template**. It gives you:

- a minimal Go HTTP service scaffold
- CloudOps Works CI/CD wiring under `.cloudopsworks/`
- GitHub Actions workflows for build, scan, preview, release, and deployment
- deployment templates for Kubernetes, Lambda, Elastic Beanstalk, App Engine, and Cloud Run

Use this template when you want a new Go service that already follows the CloudOps Works delivery model.

---

## What this template includes

### Application scaffold
- `main.go` — process entrypoint
- `internal/api/` — sample handlers and tests
- `internal/server/` — HTTP server wiring and tests
- `apifiles/` — API definition placeholders
- `version.go` — version injection target for pipeline builds

### Delivery scaffold
- `.cloudopsworks/cloudopsworks-ci.yaml` — repository governance and environment mapping
- `.cloudopsworks/vars/inputs-global.yaml` — global build/deploy defaults
- `.cloudopsworks/vars/inputs-*.yaml` — target-specific environment templates
- `.cloudopsworks/vars/helm/` — Helm values per environment
- `.cloudopsworks/vars/preview/` — preview-environment defaults
- `.cloudopsworks/gitversion_gitflow.yaml` — reference GitVersion config for GitFlow-based generated repos
- `.cloudopsworks/gitversion_githubflow.yaml` — reference GitVersion config for GitHub Flow-based generated repos
- `.github/workflows/` — reusable CI/CD orchestration

---

## Quick start

### 1. Create a repository from this template
Create your new repository from `cloudopsworks/go-app-template`, then clone it locally.

### 2. Initialize the Go module
Run the bootstrap command from the root of the new repository:

Install Tronador CLI v0.5.0 or newer and GitHub CLI, then authenticate `gh`
(`gh auth status` must succeed). Project initialization resolves the repository
owner through GitHub, so network access must be explicit:

```bash
tronador project init --allow-network
```

This command:
- removes the existing `go.mod`
- initializes a new module with the current directory name
- runs `go mod tidy`
- replaces `hello-service` references in Go sources with your project name

> Rename the repository directory before running `tronador project init --allow-network` if you want the module name to match the final service name.

### 3. Update the sample application
At minimum, review and update:
- `main.go`
- `internal/api/handlers.go`
- `internal/server/server.go`
- `apifiles/`

The template starts with `/hello` and `/health` endpoints so the pipeline has a working baseline.

### 4. Verify locally
```bash
go test ./...
tronador project version
```

`tronador project version` writes a `VERSION` file using GitVersion semantics. On a tagged commit it uses the tag value; otherwise it derives the version from branch history.

---

## Required template configuration

### `.cloudopsworks/cloudopsworks-ci.yaml`
This file controls repository behavior and deployment routing.

Update these sections first:

#### `config`
- `branchProtection` — enable branch protection automation
- `gitFlow.enabled` — keep `true` if you use GitFlow branch conventions
- `gitFlow.supportBranches` — enable only if you maintain long-lived support branches
- `requiredReviewers`, `reviewers`, `owners`, `contributors` — repository governance

#### `cd.deployments`
This maps branch/tag flows to deployment environments.

Default mapping in this template:
- `develop` -> `dev`
- `release/**` -> `prod`
- internal `test` stage -> `uat`
- prerelease tags -> `demo`
- `hotfix` -> `hotfix`
- optional `support` mappings by version match

Adjust these names to match your environments and promotion flow.

### `.cloudopsworks/vars/inputs-global.yaml`
This is the main global configuration file used by the workflows.

Set these values before your first pipeline run:
- `organization_name`
- `organization_unit`
- `environment_name`
- `repository_owner`
- `cloud`
- `cloud_type`

Use `cloud: none` and `cloud_type: none` only for repositories that should build/scan without deployment. In that mode, your upstream blueprint configuration must resolve deployment to disabled.

Common optional sections:
- `golang` — package name override, Go version, target OS/arch, image variant, CGO toggle, and optional GoReleaser publishing (including macOS and Chocolatey/Windows runners)
- `pages` — GitHub Pages publishing for Jekyll documentation (see [GitHub Pages documentation](#github-pages-documentation))
- `preview` — PR preview environment behavior
- `apis` — API Gateway publishing
- `observability` — tracing/monitoring agent configuration
- `snyk`, `semgrep`, `trivy`, `sonarqube`, `dependencyTrack` — security/quality tooling
- `docker_inline`, `docker_args`, `custom_run_command`, `custom_usergroup` — container customization

To override the binary/package name used during build and release (defaults to the `go.mod` module name or repository name), set `package_name` under `golang`:

```yaml
golang:
  package_name: my-cli
```

To enable GoReleaser for tagged releases, configure the `goreleaser` block under `golang`:

```yaml
golang:
  goreleaser:
    enabled: true     # run GoReleaser on the default runner with .goreleaser.yml
    macos: true       # optional: run GoReleaser on a macOS runner with macos.goreleaser.yml
    chocolatey: true  # optional: run GoReleaser on a Windows runner with windows.goreleaser.yml
```

| Key | Runner | Config file (repository root) | Typical use |
|---|---|---|---|
| `enabled` | `runner_set` (default runner) | `.goreleaser.yml` | Cross-platform binaries, archives, checksums, Homebrew tap |
| `macos` | `macos-latest` | `macos.goreleaser.yml` | macOS code signing and notarization |
| `chocolatey` | `windows-latest` | `windows.goreleaser.yml` | Chocolatey package build and push |

Each key opts in independently and defaults to disabled. Enabling any of them makes the release job create the GitHub release as a draft for GoReleaser to complete. The macOS and Chocolatey jobs run first, and the main GoReleaser job runs after them. A failure in the macOS or Chocolatey job does not block the main job. All GoReleaser jobs run only after a successful release job, and only for release builds without a prerelease qualifier.

The boolean form `golang.goreleaser: true` is deprecated but still supported; it is equivalent to `goreleaser.enabled: true`.

Leave GoReleaser unset when the repository should only build container/image artifacts. The release workflow keeps GoReleaser disabled by default.

> **Required GoReleaser configuration:** The CloudOps Works release workflow creates a GitHub draft release before invoking GoReleaser. Every GoReleaser config file you enable (`.goreleaser.yml`, `macos.goreleaser.yml`, `windows.goreleaser.yml`) must include `release.use_existing_draft: true` so GoReleaser updates that draft instead of attempting to create a new release. Without this setting GoReleaser will fail because the release already exists.
>
> ```yaml
> release:
>   use_existing_draft: true
> ```

> **Required secrets when any GoReleaser job is enabled:** The GoReleaser steps sign release artifacts using GPG. Before enabling `enabled`, `macos`, or `chocolatey` you must add the following secrets at the repository or organization level:
>
> - `GPG_PRIVATE_KEY` — armored GPG private key used to sign released artifacts
> - `GPG_PASSPHRASE` — passphrase for the GPG private key
>
> The workflow will fail at the GoReleaser signing step if either secret is absent. It is also recommended to set `cloud_type: none` in `inputs-global.yaml` when using GoReleaser so that no cloud deployment is attempted alongside the release.
>
> **Optional GoReleaser distribution secrets:** If your GoReleaser configuration publishes to Homebrew, Chocolatey, or signed macOS binaries, also add:
>
> - `HOMEBREW_TAP_TOKEN` — GitHub personal access token with write access to your Homebrew tap repository. Falls back to `BOT_TOKEN` when absent, so Homebrew tap commits use the bot identity instead of failing.
> - `CHOCOLATEY_API_KEY` — Chocolatey community repository API key for publishing packages (used by the `chocolatey` job)
> - `APPLE_SIGN_CERTIFICATE_BASE64` — base64-encoded Apple Developer certificate (P12) for macOS binary signing. Falls back to `XCODE_BUILD_CERTIFICATE_BASE64` when absent.
> - `APPLE_SIGN_CERTIFICATE_PASS` — passphrase for the Apple Developer certificate. Falls back to `XCODE_BUILD_CERTIFICATE_PASS` when absent.
> - `APPLE_KEYCHAIN_PASSWORD` — password for the temporary signing keychain on the macOS runner. Falls back to `XCODE_KEYCHAIN_PASSWORD` when absent.
> - `APPLE_STORE_CONNECT_KEY_BASE64` — base64-encoded App Store Connect API key for macOS notarization
> - `APPLE_STORE_CONNECT_KEY_ID` — App Store Connect API key identifier
> - `APPLE_STORE_CONNECT_ISSUER_ID` — App Store Connect API issuer identifier
>
> These secrets are passed through to GoReleaser automatically when present. The workflow proceeds without them if the GoReleaser configuration does not reference those publishers or signing steps.

---

## Choose one deployment target per environment

Each active environment should have exactly one matching deployment-target file under `.cloudopsworks/vars/`.

### Kubernetes / EKS / AKS / GKE
Use `inputs-KUBERNETES-ENV.yaml`.

Key fields:
- `container_registry`
- `cluster_name`
- `namespace`
- target-cloud credentials/settings
- optional Helm, secret, and external-secret overrides

### AWS Lambda
Use `inputs-LAMBDA-ENV.yaml`.

Key fields:
- `versions_bucket`
- `aws.region`
- Lambda runtime/handler settings
- IAM, VPC, and trigger configuration

### AWS Elastic Beanstalk
Use `inputs-BEANSTALK-ENV.yaml`.

Key fields:
- `versions_bucket`
- `container_registry`
- `aws.region`
- Beanstalk platform, instance, port, and extra settings

`runner_set` is optional and only needed when you use self-hosted runners.

### Google App Engine
Use `inputs-APPENGINE.yaml`.

Key fields:
- `container_registry`
- `gcp.region`
- `gcp.project_id`
- `appengine.runtime`
- `appengine.type`
- `appengine.entrypoint_shell` — startup command App Engine should execute, for example `./your-service-binary`

### Google Cloud Run
Use `inputs-CLOUDRUN.yaml`.

Key fields:
- `container_registry`
- `gcp.region`
- `gcp.project_id`
- `cloudrun.type`

---

## Preview environments

Preview environments are configured from:
- `.cloudopsworks/vars/preview/inputs.yaml`
- `.cloudopsworks/vars/preview/values.yaml`

Enable them in `inputs-global.yaml`:

```yaml
preview:
  enabled: true
```

Use preview environments when pull requests from `feature/**` or `hotfix/**` should deploy an isolated review environment.

---

## GitHub Pages documentation

The `docs-pages.yml` workflow builds a Jekyll site from your repository docs and deploys it to GitHub Pages. It is disabled by default. Enable it in `inputs-global.yaml`:

```yaml
pages:
  enabled: true
  docs_path: "docs"         # Jekyll source folder (default: docs)
  pages_path: "docs-pages"  # build output folder uploaded to Pages (default: docs-pages)
```

Requirements:
- Jekyll-format docs under `docs_path`, including a `_config.yml`
- `Gemfile` and `Gemfile.lock` at the repository root that pin Jekyll and its plugins
- Repository **Settings → Pages → Build and deployment → Source** set to **GitHub Actions**

The workflow runs on pushes to `master` that change `docs/**`, `Gemfile`, or `Gemfile.lock`, and on manual dispatch. If you set a custom `docs_path`, pushes under that folder do not trigger it automatically, so run it manually (`docs-pages.yml` is template-owned and is overwritten on upgrade). When `pages.enabled` is not `true`, the build step and deploy job are skipped.

---

## GitHub Actions workflow model

Important workflows in this template:

- `main-build.yml` — build, test, containerize, scan, and release/deploy on branch/tag events
- `pr-build.yml` — PR validation and optional preview deployment
- `deploy-container.yml` — push application container artifacts
- `deploy.yml` — standard deployment flow
- `deploy-blue-green.yml` — blue/green deployment flow
- `scan.yml` — SAST/SCA/DAST orchestration
- `environment-unlock.yml` / `environment-destroy.yml` — environment operations
- `docs-pages.yml` — Jekyll build and GitHub Pages deployment when `pages.enabled: true`
- `automerge.yml`, slash-command workflows, Jira integration workflows — repo automation

When GoReleaser is enabled under `golang.goreleaser`, `main-build.yml` runs GoReleaser publication jobs after the standard GitHub release is created: `goreleaser_mac` (macOS runner) and `goreleaser_chocolatey` (Windows runner) when their flags are set, then `goreleaser_main`. Use that mode only when the generated repository includes valid GoReleaser configuration files and the signing secrets described below.

This template now also includes dedicated GitVersion reference files for both GitFlow and GitHub Flow release models. If your generated repository wants to use one of them directly, wire it explicitly in your generator/build logic rather than assuming automatic selection.

---

## Secrets and variables expected by workflows

The workflows expect GitHub repository or organization configuration for build, preview, and deploy credentials.

Typical examples:
- `BOT_TOKEN`
- `BUILD_AWS_ACCESS_KEY_ID` / `BUILD_AWS_SECRET_ACCESS_KEY`
- `DEPLOYMENT_AWS_ACCESS_KEY_ID` / `DEPLOYMENT_AWS_SECRET_ACCESS_KEY`
- `BUILD_GCP_CREDENTIALS` / `DEPLOYMENT_GCP_CREDENTIALS`
- `BUILD_AZURE_SERVICE_ID` / `BUILD_AZURE_SERVICE_SECRET`
- `DEPLOYMENT_AZURE_SERVICE_ID` / `DEPLOYMENT_AZURE_SERVICE_SECRET`
- runner, registry, region, and state configuration variables

If you enable any GoReleaser job under `golang.goreleaser`, the following secrets are **required** at the repository or organization level — the workflow will fail without them:
- `GPG_PRIVATE_KEY` — armored GPG private key for signing release artifacts
- `GPG_PASSPHRASE` — passphrase for the GPG private key

The following secrets are **optional** and only needed when your GoReleaser configuration targets those distribution channels or signing steps:
- `HOMEBREW_TAP_TOKEN` — GitHub personal access token with write access to your Homebrew tap repository. Falls back to `BOT_TOKEN` when absent.
- `CHOCOLATEY_API_KEY` — Chocolatey community repository API key for publishing packages
- `APPLE_SIGN_CERTIFICATE_BASE64` — base64-encoded Apple Developer certificate (P12) for macOS binary signing. Falls back to `XCODE_BUILD_CERTIFICATE_BASE64`.
- `APPLE_SIGN_CERTIFICATE_PASS` — passphrase for the Apple Developer certificate. Falls back to `XCODE_BUILD_CERTIFICATE_PASS`.
- `APPLE_KEYCHAIN_PASSWORD` — password for the temporary signing keychain on the macOS runner. Falls back to `XCODE_KEYCHAIN_PASSWORD`.
- `APPLE_STORE_CONNECT_KEY_BASE64` — base64-encoded App Store Connect API key for macOS notarization
- `APPLE_STORE_CONNECT_KEY_ID` — App Store Connect API key identifier
- `APPLE_STORE_CONNECT_ISSUER_ID` — App Store Connect API issuer identifier

`GITHUB_TOKEN` is supplied automatically by GitHub Actions and is used by the GoReleaser release step for repository publication.

Review the `with:` and `secrets:` blocks in the workflow files and align your repository settings before enabling deployments.

---

## Release and versioning expectations

This template repository follows semantic versioning.

- documentation/template-only fixes -> patch release
- backward-compatible template capability additions -> minor release
- breaking workflow or template contract changes -> major release

Version calculation is GitVersion-based, and release automation relies on commit/PR annotations such as:
- `+semver: patch`
- `+semver: fix`
- `+semver: minor`
- `+semver: feature`
- `+semver: major`

If you use the CloudOps Works release workflow, keep changes grouped by release intent so the generated version bump stays predictable.

---

## Recommended first-pass checklist for new repositories

- [ ] Create repo from template
- [ ] Run `tronador project init --allow-network`
- [ ] Rename/update the sample service code
- [ ] Update `.cloudopsworks/cloudopsworks-ci.yaml`
- [ ] Update `.cloudopsworks/vars/inputs-global.yaml`
- [ ] Configure exactly one target file per active environment
- [ ] Configure preview settings if needed
- [ ] Configure GitHub Pages docs (`pages`) and GoReleaser runners (`golang.goreleaser`) if needed
- [ ] Add required GitHub secrets and variables
- [ ] Run `go test ./...`
- [ ] Open a PR and verify `pr-build.yml`
- [ ] Merge and verify the first environment deployment

---

## Notes

- `.omx/`, `.claude/`, `.opencode/`, and similar agent/tooling directories are intentionally ignored and are not part of the application template contract.
- The template is designed for CloudOps Works blueprint-backed automation; if you remove that integration, also prune the related workflows and `.cloudopsworks/` configuration.

---

## Upgrading from the Template

Repositories derived from this template stay in sync with upstream releases using the
`tronador repos upgrade` command family. An agent asked to "upgrade", "update from template",
"sync with template", "apply template changes", or "bump template version" should use
these commands — never fetch or apply template changes manually.

### Available upgrade commands

| Target | When to use |
|---|---|
| `tronador repos upgrade` | **Default — patch upgrade.** Pulls the latest patch within the **same minor version**. No breaking changes. Use for routine maintenance. |
| `tronador repos upgrade major` | Pulls the latest release within the **same major version**. May include workflow-level changes. |
| `tronador repos upgrade master` | Pulls from the template's `master` branch tip. Use only when explicitly asked to track the latest unreleased template state. |
| `tronador repos upgrade develop` | Pulls from the template's `develop` branch. Use only for pre-release or preview upgrades. |
| `tronador repos available` | Lists the latest available patch and major versions without modifying anything. Run this first to see what is available. |

### Upgrade workflow for agents

1. Run `tronador repos available` to see the current and latest available versions.
2. Choose the appropriate command (default: `tronador repos upgrade` for a routine patch upgrade).
3. Review the diff — the upgrade overwrites `.github/workflows/` and selected `.cloudopsworks/` metadata; application source files are never touched.
4. Commit the result with: `chore: upgrade from <template-name> <old-version> → <new-version> +semver: patch`
5. Use `/cw-release` to create and merge the hotfix PR (see [Release Workflow — use `cw-release`](#release-workflow--use-cw-release)).

> **Note:** `Makefile`, `.github/`, `.cloudopsworks/labeler.yml`, `.cloudopsworks/Makefile`,
> and `.cloudopsworks/LICENSE` are owned by the template and will be overwritten on every upgrade.
> Do not edit these files manually in derived repositories.

---

## AI-assisted upgrade of `.cloudopsworks/vars` configuration files

This section is a machine-readable protocol for AI agents performing a seamless, non-destructive upgrade of all configuration files under `.cloudopsworks/vars/` when a new template version is released. Follow the steps below in order.

### Upgrade overview

The template version locked into this repository is recorded in `.cloudopsworks/_VERSION`. The canonical upstream source is the GitHub repository `cloudopsworks/go-app-template`, pinned to the tag that matches the content of `_VERSION`.

An upgrade merges new keys, updated comments, and structural changes from the upstream template into local files **without overwriting values the operator has already set**.

---

### Step 1 — determine current and target versions

1. Read `.cloudopsworks/_VERSION` to get the **current locked version** (e.g., `v1.4.15`).
2. The **target version** is either supplied by the operator or is the latest release tag on `cloudopsworks/go-app-template`.
3. Fetch any upstream file from GitHub using the pattern:
   ```
   https://raw.githubusercontent.com/cloudopsworks/go-app-template/<version>/<path>
   ```
   Example:
   ```
   https://raw.githubusercontent.com/cloudopsworks/go-app-template/v1.4.15/.cloudopsworks/vars/inputs-global.yaml
   ```

---

### Step 2 — identify the deployment type for each environment file

Each `inputs-<name>.yaml` file under `.cloudopsworks/vars/` maps to a specific upstream template. Determine the type using the following priority order:

**Priority 1 — `Agents:` header comment**

If the file contains an `# Agents:` line in its header block, read `cloud` and `cloud_type` directly from it:

```yaml
# Agents: cloud=aws ; cloud_type=lambda
```

Multiple valid combinations may be listed separated by `|`:

```yaml
# Agents: cloud=aws|gcp|azure ; cloud_type=kubernetes
```

**Priority 2 — fallback to `inputs-global.yaml`**

If no `# Agents:` line is present, read the active `cloud` and `cloud_type` values from `.cloudopsworks/vars/inputs-global.yaml` and apply the mapping table below.

**`cloud` / `cloud_type` → upstream template file:**

| `cloud`                  | `cloud_type`                   | Upstream template file         |
|--------------------------|--------------------------------|--------------------------------|
| `aws`                    | `eks` or `kubernetes`          | `inputs-KUBERNETES-ENV.yaml`   |
| `azure`                  | `aks` or `kubernetes`          | `inputs-KUBERNETES-ENV.yaml`   |
| `gcp`                    | `gke` or `kubernetes`          | `inputs-KUBERNETES-ENV.yaml`   |
| `aws`                    | `lambda`                       | `inputs-LAMBDA-ENV.yaml`       |
| `aws`                    | `beanstalk`                    | `inputs-BEANSTALK-ENV.yaml`    |
| `gcp`                    | `appengine`                    | `inputs-APPENGINE.yaml`        |
| `gcp`                    | `cloudrun`                     | `inputs-CLOUDRUN.yaml`         |
| `aws` / `gcp` / `azure`  | `none` or library mode         | `inputs-LIB-ENV.yaml`          |

`inputs-global.yaml` always maps to the upstream `inputs-global.yaml` regardless of cloud type.

---

### Step 3 — upgrade deployment target files

The deployment target files identified by the Step 2 mapping table — such as `inputs-KUBERNETES-ENV.yaml`, `inputs-LAMBDA-ENV.yaml`, `inputs-BEANSTALK-ENV.yaml`, `inputs-APPENGINE.yaml`, `inputs-CLOUDRUN.yaml`, `inputs-LIB-ENV.yaml`, and mobile equivalents such as `inputs-ANDROID-ENV.yaml` and `inputs-XCODE-ENV.yaml` — are **scaffolding templates**. They provide placeholder structures and documented examples, not finalized operator configuration.

**Do not merge these files. Overwrite them.**

Upgrade procedure for each deployment target file:

1. **Before overwriting** — inspect the local file and record any operator-configured values (keys that have been uncommented and set to non-placeholder values).
2. **Replace the file** — overwrite the local file entirely with the upstream template version.
3. **Re-apply operator values** — after overwriting, set each previously recorded operator-configured value at its corresponding key in the new file.
4. **Copy in absent files** — if a deployment target file is present in the upstream template but absent locally, copy it in from the upstream template as a new file.

---

### Step 4 — merge `inputs-global.yaml`

`inputs-global.yaml` requires special handling because it contains mandatory operator identity fields alongside a large body of optional commented-out sections.

Merge procedure:

1. **Retain the four mandatory identity fields** verbatim at the top of the file:
   ```yaml
   organization_name: "..."
   organization_unit: "..."
   environment_name: "..."
   repository_owner: "..."
   ```
2. **Retain `cloud` and `cloud_type`** exactly as the operator set them.
3. **For every optional commented-out section** in the upstream template, check the local file:
   - If the operator **has uncommented and configured it** — keep the operator's values; update only surrounding comment text if it changed upstream.
   - If the section **is still fully commented out locally** — replace the entire commented block with the upstream version, capturing any new fields or updated documentation within it.
4. **Append new optional sections** that appear in the upstream template but are entirely absent locally, in fully commented-out form, preserving their upstream position and comments.

---

### Step 5 — upgrade subdirectory files

Apply the merge rules from Step 4 to every file in the following subdirectories, matching each local file to its corresponding upstream file at the same relative path:

- `.cloudopsworks/vars/preview/inputs.yaml`
- `.cloudopsworks/vars/preview/values.yaml`
- `.cloudopsworks/vars/apigw/apis-global.yaml`
- `.cloudopsworks/vars/apigw/apis-dev.yaml`
- `.cloudopsworks/vars/apigw/apis-uat.yaml`
- `.cloudopsworks/vars/apigw/apis-prod.yaml`
- `.cloudopsworks/vars/helm/values-dev.yaml`
- `.cloudopsworks/vars/helm/values-uat.yaml`
- `.cloudopsworks/vars/helm/values-prod.yaml`

---

### Step 6 — verify the protected `_VERSION` marker

Do not write `.cloudopsworks/_VERSION` during a manual downstream merge. It is a protected template-generation marker and may change only through the repository's verified, authorized template-upgrade owner (for example `tronador repos upgrade`). After all merges are verified, confirm the authorized upgrade recorded the expected template version; otherwise leave the marker unchanged and report the mismatch.

---

### Upgrade invariants

An agent performing this upgrade must **never**:

- Overwrite a field the operator has explicitly set to a non-placeholder value.
- Remove a commented-out operator value without first reporting it.
- Change the YAML structure of any active (uncommented) operator section.
- Alter a file's opening description comment (`# This file contains...`) unless the upstream version changed it.
- Modify `.cloudopsworks/cloudopsworks-ci.yaml`, `gitversion_*.yaml`, or any file under `.github/workflows/` as part of a vars upgrade — those follow their own upgrade path.
- Hand-edit or stage `_VERSION`; only the verified, authorized template-upgrade owner may change it after the file merges are complete.

---

### Conflict resolution

When a merge cannot be resolved automatically (for example, the upstream template restructured a section that the operator has customized):

1. Emit a diff showing both the upstream template block and the local operator block side by side.
2. Pause and present the conflict to the operator, asking which version to keep or whether a manual merge is needed.
3. Never silently choose one side.

---

## Release Workflow — use `cw-release`

All releases **must** be performed using the `cw-release` skill from the CloudOps Works skill set. Do **not** create release branches, hotfix branches, version tags, or release PRs manually — the skill owns the full GitFlow-aware release lifecycle for this repository.

### When to invoke `cw-release`

Use it whenever you are asked to:
- Release, ship, or publish a new version (patch, minor, or major)
- Create a hotfix or patch release
- Create a release branch or feature-merge PR
- Tag and publish a version

### How to run it

In Claude Code (CLI, IDE extension, or web):

```
/cw-release
```

### What the skill does

1. Detects the GitVersion flow in use (`gitversion_gitflow.yaml` or `gitversion_githubflow.yaml`).
2. Reads the repo-local release policy from `.cloudopsworks/cloudopsworks-ci.yaml`.
3. Drives the shared `tronador` CLI / `gh` release path end-to-end.
4. Creates the correct branch, PR, tag, and GitHub Release in the right sequence.

> **Do not** run `git tag`, `gh release create`, or `tronador versions tag --publish` directly. Always let `cw-release` orchestrate these steps to keep version history and CI consistent.
