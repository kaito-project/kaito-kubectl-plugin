# Version and build information
VERSION ?= $(shell git describe --tags --always --dirty)
COMMIT ?= $(shell git rev-parse HEAD)
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Build configuration
BINARY_NAME = kubectl-kaito
BINARY_PATH = bin/$(BINARY_NAME)
PKG = github.com/kaito-project/kaito-kubectl-plugin
CMD_PKG = ./cmd/kubectl-kaito
LDFLAGS = -ldflags "-X ${PKG}/pkg/cmd.version=${VERSION} -X ${PKG}/pkg/cmd.commit=${COMMIT} -X ${PKG}/pkg/cmd.date=${DATE}"

# Scripts
GO_INSTALL := ./hack/go-install.sh
TOOLS_DIR := hack/tools
TOOLS_BIN_DIR := $(abspath $(TOOLS_DIR)/bin)

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

GOLANGCI_LINT_VER := latest
GOLANGCI_LINT_BIN := golangci-lint
GOLANGCI_LINT := $(abspath $(TOOLS_BIN_DIR)/$(GOLANGCI_LINT_BIN)-$(GOLANGCI_LINT_VER))

# Build the binary
.PHONY: build
build:
	mkdir -p bin
	go build ${LDFLAGS} -o ${BINARY_PATH} ${CMD_PKG}

# Run unit tests with race detection and coverage report
.PHONY: unit-tests
unit-tests:
	@echo "Running unit tests with race detection and coverage..."
	go test -v -race -coverprofile=coverage.out -covermode=atomic ./cmd/... ./pkg/...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"
	@go tool cover -func=coverage.out | grep total

# Run e2e tests with AKS cluster (creates billable resources)
.PHONY: test-e2e-aks
test-e2e-aks:
	cd tests/e2e && go test -v -timeout=30m -run "TestAKSClusterOperations"

# Setup AKS cluster for manual testing
.PHONY: setup-aks
setup-aks:
	./hack/test/e2e/setup-aks.sh

# Cleanup AKS cluster
.PHONY: cleanup-aks
cleanup-aks:
	./hack/test/e2e/cleanup-aks.sh

# Lint the code

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download and install golangci-lint locally.

.PHONY: ginkgo
ginkgo: $(GOLANGCI_LINT) ## Download and install ginkgo locally.

$(GOLANGCI_LINT): ## Download and install golangci-lint locally.
	GOBIN=$(TOOLS_BIN_DIR) $(GO_INSTALL) github.com/golangci/golangci-lint/cmd/golangci-lint $(GOLANGCI_LINT_BIN) $(GOLANGCI_LINT_VER)


.PHONY: lint
lint: $(GOLANGCI_LINT) ## Run golangci-lint against code.
	$(GOLANGCI_LINT) run -v


# Format the code
.PHONY: fmt
fmt:
	go fmt ./...

# Vet the code
.PHONY: vet
vet:
	go vet ./...

# Tidy dependencies
.PHONY: tidy
tidy:
	go mod tidy

# Clean build artifacts
.PHONY: clean
clean:
	rm -rf bin/
	rm -rf dist/
	rm -f coverage.out coverage.html

# Quick check before commit
.PHONY: check
check: fmt vet lint unit-tests pre-commit-run-staged

# Run the binary (for testing)
.PHONY: run
run: build
	${BINARY_PATH}
