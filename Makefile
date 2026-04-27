export PROJECT ?= $(shell basename $(shell pwd))
TRONADOR_AUTO_INIT := true

GITVERSION ?= $(INSTALL_PATH)/gitversion
GH ?= $(INSTALL_PATH)/gh
YQ ?= $(INSTALL_PATH)/yq

-include $(shell curl -sSL -o .tronador "https://cowk.io/acc"; echo .tronador)

## Version Bump and creates VERSION File - Uses always the FullSemVer from GitVersion (no need to prepend the 'v').
version: packages/install/gitversion
	$(call assert-set,GITVERSION)
ifeq ($(GIT_IS_TAG),1)
	@echo "$(GIT_TAG)" | sed -E 's/^v([0-9]+\.[0-9]+\.[0-9]+((-alpha|-beta).[0-9]?)?)(\+deploy-.*)?$$/\1/g' > VERSION
else
	# Translates + in version to - for helm/docker compatibility
	@echo "$(shell $(GITVERSION) -output json -showvariable FullSemVer | tr '+' '-')" > VERSION
endif

# Modify pom.xml to change the project name with the $(PROJECT) variable
## Code Initialization for GoLang Project
code/init: packages/install/gitversion packages/install/gh packages/install/yq
	$(call assert-set,GITVERSION)
	$(call assert-set,GH)
	$(call assert-set,YQ)
	$(eval $@_OWNER := $(shell $(GH) repo view --json 'name,owner' -q '.owner.login'))
	rm go.mod
	@go mod init $(PROJECT)
	@go mod tidy
ifeq ($(OS),darwin)
	@find . -name "*.go" -exec sed -E -i '' "s/hello-service/${PROJECT}/g" {} \;
else
	@find . -name "*.go" -exec sed -E -i '' "s/hello-service/${PROJECT}/g" {} \;
endif

## Format Go source code.
fmt:
	gofmt -s -w -e .

## Run provider validation checks with go vet.
lint:
	mkdir -p .gocache2 .gomodcache2
	GOCACHE=$(PWD)/.gocache2 GOMODCACHE=$(PWD)/.gomodcache2 go vet ./...

## Run the unit test suite.
test:
	mkdir -p .gocache2 .gomodcache2
	GOCACHE=$(PWD)/.gocache2 GOMODCACHE=$(PWD)/.gomodcache2 go test -v -cover -timeout=120s ./...

## Run acceptance tests against a real OpenRouter account.
testacc:
	mkdir -p .gocache2 .gomodcache2
	TF_ACC=1 GOCACHE=$(PWD)/.gocache2 GOMODCACHE=$(PWD)/.gomodcache2 go test -v -cover -timeout=120m ./...

## Build the provider binaries locally.
build:
	mkdir -p .gocache2 .gomodcache2
	GOCACHE=$(PWD)/.gocache2 GOMODCACHE=$(PWD)/.gomodcache2 go build -v ./...

## Install the provider into the local Go bin directory.
install: build
	mkdir -p .gocache2 .gomodcache2
	GOCACHE=$(PWD)/.gocache2 GOMODCACHE=$(PWD)/.gomodcache2 go install -v ./...

## Build a local unsigned GoReleaser snapshot release.
release-snapshot:
	mkdir -p .gocache2 .gomodcache2
	GOCACHE=$(PWD)/.gocache2 GOMODCACHE=$(PWD)/.gomodcache2 goreleaser release --snapshot --clean --skip=publish --skip=sign

.PHONY: version code/init fmt lint test testacc build install release-snapshot
