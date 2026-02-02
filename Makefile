REGISTRY ?= ghcr.io
USERNAME ?= sergelogvinov
PROJECT ?= helm-resources
IMAGE ?= $(REGISTRY)/$(USERNAME)/$(PROJECT)
HELMREPO ?= $(REGISTRY)/$(USERNAME)/charts
PLATFORM ?= linux/arm64,linux/amd64
PUSH ?= false

VERSION ?= $(shell git describe --dirty --tag --match='v*')
SHA ?= $(shell git describe --match=none --always --abbrev=7 --dirty)
TAG ?= $(VERSION)

GO_LDFLAGS := -s -w
GO_LDFLAGS += -X k8s.io/component-base/version.gitVersion=$(VERSION)

OS ?= $(shell go env GOOS)
ARCH ?= $(shell go env GOARCH)
ARCHS = amd64 arm64

TESTARGS ?= "-v"

BUILD_ARGS := --platform=$(PLATFORM)
ifeq ($(PUSH),true)
BUILD_ARGS += --push=$(PUSH)
BUILD_ARGS += --output type=image,annotation-index.org.opencontainers.image.source="https://github.com/$(USERNAME)/$(PROJECT)",annotation-index.org.opencontainers.image.description="Helm resource plugin"
else
BUILD_ARGS += --output type=docker
endif

COSING_ARGS ?=

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
	rm -rf bin

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
	docker run --rm -it -v $(PWD):/src -w /src ghcr.io/siderolabs/conform:v0.1.0-alpha.30 enforce

############
#
# Docker Abstractions
#

docker-init:
	@docker run --rm --privileged multiarch/qemu-user-static -p yes ||:

	@docker context create multiarch ||:
	@docker buildx create --name multiarch --driver docker-container --use ||:
	@docker context use multiarch
	@docker buildx inspect --bootstrap multiarch

.PHONY: images
images: ## Build images
	docker buildx build $(BUILD_ARGS) \
		--build-arg VERSION="$(VERSION)" \
		--build-arg TAG="$(TAG)" \
		--build-arg SHA="$(SHA)" \
		-t $(IMAGE):$(TAG) \
		-f Dockerfile .

.PHONY: images-checks
images-checks: images
	trivy image --exit-code 1 --ignore-unfixed --severity HIGH,CRITICAL --no-progress $(IMAGE):$(TAG)

.PHONY: images-cosign
images-cosign:
	@cosign sign --yes $(COSING_ARGS) --recursive $(IMAGE):$(TAG)
