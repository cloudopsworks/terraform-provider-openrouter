# GoLang Development Template - Documentation

## Overview
This template provides a standardized structure for GoLang application development with integrated CI/CD pipelines using [Cloud Ops Works Blueprints](https://github.com/cloudopsworks/blueprints). It supports multiple deployment targets including Kubernetes (EKS, AKS, GKE), AWS Lambda, AWS Elastic Beanstalk, Google App Engine, and Google Cloud Run.

The automation is based on the Cloud Ops Works Blueprint system, where actions are referenced via the `./bp` prefix during CI/CD execution.

## Available Workflows
The CI/CD processes are managed through GitHub Actions, leveraging the Cloud Ops Works automation blueprints.

### Main Workflows
- **Release Build (`main-build.yml`)**: Triggered on pushes to `develop`, `support/**`, `release/**`, and version tags (`v*.*.*`).
    - **Code Build**: Compiles Go sources, runs unit tests, and generates code coverage reports.
    - **SBOM Generation**: Automatically generates a Software Bill of Materials (CycloneDX) for dependency tracking.
    - **Containerization**: Builds and pushes Docker images to the configured registry.
    - **Deployment**: Orchestrates deployment to the target environment defined in the branch mapping.
    - **Security Scanning**: Integrates with SonarQube, Snyk, and Semgrep for comprehensive security and quality analysis.
- **PR Build (`pr-build.yml`)**: Triggered on Pull Requests. Validates code, runs tests, and supports the creation of temporary **Preview Environments**.
- **Scan (`scan.yml`)**: Reusable workflow for executing security and quality scans.
- **Deploy Workflows**:
    - `deploy.yml`: Standard deployment to cloud targets.
    - `deploy-container.yml`: Handles container-specific deployment tasks.
    - `deploy-blue-green.yml`: Orchestrates blue-green deployment strategies for zero-downtime releases.

### Utility Workflows
- **Automerge**: Automatically merges Pull Requests that pass all checks and approvals.
- **Jira Integration**: Synchronizes release tags and deployment status with Jira issues for better project tracking.
- **Environment Management**: Manual workflows to `unlock` or `destroy` specific environments.
- **Slash Commands**: Enables interaction with PRs via comments (e.g., `/retry`, `/approve`).

---

## Module Configuration (`.cloudopsworks/cloudopsworks-ci.yaml`)
This file is the brain of the pipeline, defining how the repository interacts with the CI/CD system.

### Configuration Options:
- **`zipGlobs` / `excludeGlobs`**: Control which files are bundled into artifacts or excluded from the build process (e.g., excluding `Dockerfile`, `README.md`).
- **`config`**:
    - `branchProtection`: Set to `true` to enforce GitHub branch protection.
    - `gitFlow`: Enablement for Git Flow branch naming conventions.
    - `protectedSources`: List of critical files (e.g., `.tf`, `.github`) that require extra scrutiny.
    - `requiredReviewers`: Minimum number of approvals for PRs.
    - `reviewers` / `owners` / `contributors`: RBAC for the repository.
- **`cd`**:
    - `automatic`: Enables automated merges/deploys to lower environments.
    - **`deployments`**: **Crucial Mapping Section**. Maps Git branches to environment labels:
        - `develop` ➔ `env: dev`
        - `release/**` (Pushes) ➔ `env: prod`
        - `test` (Internal) ➔ `env: uat`
        - `prerelease` (Tags) ➔ `env: demo`
        - `support/**` ➔ Matches specific versions to environments.

---

## Global Configuration (`.cloudopsworks/vars/inputs-global.yaml`)
The base configuration used across all environments. This file should be documented with high detail as it sets the default behavior.

### Detailed Parameters:
- **Project Identity**:
    - `organization_name` / `organization_unit`: Organizational metadata.
    - `environment_name`: **[Required]** Should match the repository name for alignment.
    - `repository_owner`: **[Required]** The GitHub organization or user owning the repo.
- **GoLang Settings (`golang`)**:
    - `main_file`: Entry point (e.g., `.`).
    - `version`: Target Go version (e.g., `1.25`).
    - `dist` / `arch`: Target OS and architecture (e.g., `linux/amd64`).
    - `image_variant`: Base Docker image (e.g., `alpine:3.21`).
    - `disable_cgo`: Set `true` for pure Go binaries.
- **Security & Analysis**:
    - `snyk` / `semgrep` / `sonarqube`: Toggle enablement and configure paths for analysis.
    - `dependencyTrack`: Configures SBOM upload. Type can be `Application`, `Library`, etc.
- **Integrations**:
    - `jira`: Configure `project_id` and `project_key` for release tracking.
- **Docker Build Customization**:
    - `docker_inline`: Append custom steps to the generated Dockerfile.
    - `docker_args`: Pass `--build-arg` values to Docker.
    - `custom_run_command`: Override the default `startup.sh` script.
    - `custom_usergroup`: Script to create specific users/groups inside the container.
- **Advanced Features**:
    - `api_files_dir`: Location of API specs (default: `./apifiles`).
    - `preview`: Enable/disable PR-based preview environments.
    - `apis.enabled`: Toggle API Gateway deployment.
    - `observability`: Configure agents (`xray`, `datadog`, `newrelic`, `dynatrace`) and their specific settings.
- **Deployment defaults**:
    - `cloud`: `aws`, `azure`, or `gcp`.
    - `cloud_type`: Deployment target type (e.g., `eks`, `lambda`, `cloudrun`).
    - `runner_set`: Default GitHub runner to use.

---

## Environmental Configuration (`.cloudopsworks/vars/inputs-***.yaml`)
Each environment (defined in `cloudopsworks-ci.yaml`) must have a corresponding `inputs-<env>.yaml` file. **Disclaimer: Only one deployment target configuration (Kubernetes, Lambda, etc.) can be used per environment.**

### Kubernetes (`inputs-KUBERNETES-ENV.yaml`)
Used when `cloud_type` is `kubernetes`, `eks`, `aks`, or `gke`.
- `cluster_name` / `namespace`: K8s target.
- `container_registry`: Environment-specific registry.
- `secret_files` / `config_map`: Map repository files to K8s volumes.
- `helm_values_overrides`: Direct overrides for the Helm chart.
- `external_secrets`: Configuration for fetching secrets from Cloud Secret Managers into K8s.

### AWS Lambda (`inputs-LAMBDA-ENV.yaml`)
Used when `cloud_type` is `lambda`.
- `versions_bucket`: S3 bucket for code deployment.
- `lambda.runtime` / `lambda.handler`: Execution environment.
- `lambda.iam`: Inline definition of IAM roles and policies.
- `lambda.vpc`: VPC and Security Group association.
- `lambda.triggers`: Configure events from S3, SQS, or DynamoDB.

### AWS Elastic Beanstalk (`inputs-BEANSTALK-ENV.yaml`)
Used when `cloud_type` is `beanstalk`.
- `beanstalk.solution_stack`: EB Platform (e.g., `go`).
- `beanstalk.instance`: Instance types and scaling limits.
- `beanstalk.port_mappings`: ELB to Instance port routing.
- `beanstalk.extra_settings`: Direct Elastic Beanstalk configuration options.

### Google App Engine (`inputs-APPENGINE.yaml`) / Cloud Run (`inputs-CLOUDRUN.yaml`)
- `appengine.runtime`: e.g., `golang1.25`.
- `appengine.instance`: Scaling and class settings.

---

## Helm Chart Configuration
The module uses a standardized Helm Chart. All options below can be overridden in `.cloudopsworks/vars/helm/values-<env>.yaml`.

| Category | Key | Description |
|----------|-----|-------------|
| **Scaling** | `replicaCount` | Number of pods. |
| | `hpa.enabled` | Enable Horizontal Pod Autoscaler. |
| **Image** | `image.repository` | Docker image name. |
| | `image.tag` | Image version tag. |
| **Service** | `service.type` | `ClusterIP`, `NodePort`, or `LoadBalancer`. |
| | `service.externalPort`| Port exposed by the service. |
| **Ingress** | `ingress.enabled` | Enable external access. |
| | `ingress.ingressClass`| e.g., `nginx`. |
| **Probes** | `probe.path` | Health check endpoint (default `/healthz`). |
| | `startupProbe.enabled`| Enable startup check. |
| **Workloads**| `statefulset.enabled`| Deploy as StatefulSet. |
| | `job.enabled` | Deploy as a one-time Job. |
| | `cronjob.enabled` | Deploy as a CronJob. |
| **Advanced**| `keda.enabled` | Enable KEDA autoscaling. |
| | `canary.enabled` | Enable Canary releases via Flagger/Istio. |
| | `affinity` / `tolerations`| Pod placement rules. |

---

## Preview Environments
Files located in `.cloudopsworks/vars/preview/` are used for Pull Request previews.
- **`inputs.yaml`**: Environment settings.
- **`values.yaml`**: Standard Helm values.
All preview configuration files are fully compatible with Helm configuration standards.

---

## GoLang Application Sample
- **`main.go`**: Initializes the server using environment variables (e.g., `PORT`).
- **`internal/server`**: Sets up the HTTP server and registers routes.
- **`internal/api`**: Implementation of handlers for business logic and health checks.
- **`apifiles/`**: Contains OpenAPI/REST definitions for AWS API Gateway or other API management tools.
- **`version.go`**: Managed by the pipeline to inject the current SemVer during build.
