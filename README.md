# go-app-template

Go Lang Application Template with Github Action Gitops

## Makefile Targets Usage

### code/init Target

The `code/init` target initializes your Go application with the following actions:

- Installs required packages (gitversion, gh, yq)
- Removes the existing go.mod file
- Initializes a new Go module with the current project name
- Runs `go mod tidy` to ensure dependencies are properly managed
- Replaces all instances of "hello-service" with your project name in all Go files

Usage:

```bash
make code/init
```

### version Target

The `version` target creates a VERSION file for your application using GitVersion:

- If the current commit is a Git tag, it extracts the version from the tag
- Otherwise, it uses GitVersion to generate a semantic version
- Replaces '+' with '-' in the version string for compatibility with Docker and Helm

Usage:

```bash
make version
```
