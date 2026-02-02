HELM_HOME ?= $(shell helm env HELM_DATA_HOME)

VERSION ?= $(shell git describe --dirty --tag --match='v*')
SHA ?= $(shell git describe --match=none --always --abbrev=7 --dirty)

GO_LDFLAGS := -s -w
GO_LDFLAGS += -X github.com/sergelogvinov/helm-resources/cmd.Version=$(VERSION)

OS ?= $(shell go env GOOS)
ARCH ?= $(shell go env GOARCH)
ARCHS = amd64 arm64

TESTARGS ?= "-v"

############

# Help Menu

define HELP_MENU_HEADER
# Getting Started

To build this project, you must have the following installed:

- git
- make
- golang 1.24+
- golangci-lint 2.2.0+

endef

export HELP_MENU_HEADER

help: ## This help menu.
	@echo "$$HELP_MENU_HEADER"
	@grep -E '^[a-zA-Z0-9%_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

############
#
# Build Abstractions
#

build-all-archs:
	@for arch in $(ARCHS); do $(MAKE) ARCH=$${arch} build ; done

.PHONY: clean
clean: ## Clean
	rm -rf bin/ dist/

.PHONY: build
build: ## Build
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) go build -ldflags "$(GO_LDFLAGS)" \
		-o bin/helm-resources-$(ARCH) ./

.PHONY: lint
lint: ## Lint Code
	golangci-lint run --config .golangci.yml

.PHONY: lint-fix
lint-fix: ## Fix Lint Issues
	golangci-lint run --fix --config .golangci.yml

.PHONY: unit
unit: ## Unit Tests
	go test -tags=unit $(shell go list ./...) $(TESTARGS)

.PHONY: test
test: lint unit ## Run all tests

.PHONY: licenses
licenses:
	go-licenses check ./... --disallowed_types=forbidden,restricted,reciprocal,unknown

.PHONY: conformance
conformance: ## Conformance
	docker run --rm -it -v $(PWD):/src -w /src ghcr.io/siderolabs/conform:v0.1.0-alpha.31 enforce

############

install: build
	mkdir -p $(HELM_HOME)/plugins/helm-resources/bin
	cp bin/helm-resources-$(ARCH) $(HELM_HOME)/plugins/helm-resources/bin/resources
	cp plugin.yaml $(HELM_HOME)/plugins/helm-resources/
