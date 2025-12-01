.PHONY: test test-prereq test-setup test-kind test-infra test-deploy test-verify test-all test-short clean help

# Default values
CLUSTER_NAME ?= test-cluster
ENV ?= stage
REGION ?= uksouth
KIND_CLUSTER_NAME ?= capz-stage

# Test configuration
GOTESTSUM_FORMAT ?= testname
# Validate GOTESTSUM_FORMAT against allowlist to prevent command injection
ALLOWED_FORMATS := testname pkgname standard-verbose testdox github-actions
ifeq (,$(filter $(GOTESTSUM_FORMAT),$(ALLOWED_FORMATS)))
  $(error Invalid GOTESTSUM_FORMAT "$(GOTESTSUM_FORMAT)". Allowed: $(ALLOWED_FORMATS))
endif
GOTESTSUM := gotestsum --format='$(GOTESTSUM_FORMAT)' --

help: ## Display this help message
	@echo "ARO-CAPZ Test Suite Makefile"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-20s %s\n", $$1, $$2}'

test: check-gotestsum ## Run all tests
	@echo "=== Running All Tests ==="
	@echo ""
	@$(GOTESTSUM) -v ./test -timeout 60m

test-short: check-gotestsum ## Run quick tests only (skip long-running tests)
	@echo "=== Running Quick Tests (Short Mode) ==="
	@echo ""
	@$(GOTESTSUM) -v -short ./test

test-prereq: check-gotestsum ## Run prerequisite verification tests only
	@echo "=== Running Prerequisites Tests ==="
	@echo ""
	@$(GOTESTSUM) -v ./test -run TestPrerequisites

test-setup: check-gotestsum ## Run repository setup tests only
	@echo "=== Running Repository Setup Tests ==="
	@echo ""
	@$(GOTESTSUM) -v ./test -run TestSetup

test-kind: check-gotestsum ## Run Kind cluster deployment tests only
	@echo "=== Running Kind Cluster Deployment Tests ==="
	@echo ""
	@$(GOTESTSUM) -v ./test -run TestKindCluster -timeout 30m

test-infra: check-gotestsum ## Run infrastructure generation tests only
	@echo "=== Running Infrastructure Generation Tests ==="
	@echo ""
	@$(GOTESTSUM) -v ./test -run TestInfrastructure -timeout 20m

test-deploy: check-gotestsum ## Run deployment monitoring tests only
	@echo "=== Running Deployment Monitoring Tests ==="
	@echo ""
	@$(GOTESTSUM) -v ./test -run TestDeployment -timeout 40m

test-verify: check-gotestsum ## Run cluster verification tests only
	@echo "=== Running Cluster Verification Tests ==="
	@echo ""
	@$(GOTESTSUM) -v ./test -run TestVerification -timeout 20m

test-all: ## Run all test phases sequentially
	@echo "========================================"
	@echo "=== Running Full Test Suite ==="
	@echo "========================================"
	@echo ""
	@$(MAKE) --no-print-directory test-prereq && \
	$(MAKE) --no-print-directory test-setup && \
	$(MAKE) --no-print-directory test-kind && \
	$(MAKE) --no-print-directory test-infra && \
	$(MAKE) --no-print-directory test-deploy && \
	$(MAKE) --no-print-directory test-verify && \
	echo "" && \
	echo "=======================================" && \
	echo "=== All Test Phases Completed Successfully ===" && \
	echo "======================================="

clean: ## Clean up test resources
	@echo "Cleaning up test resources..."
	-kind delete cluster --name $(KIND_CLUSTER_NAME)
	-rm -rf /tmp/cluster-api-installer-aro
	-rm -f /tmp/*-kubeconfig.yaml
	@echo "Cleanup complete"

setup-submodule: ## Add cluster-api-installer as a git submodule
	git submodule add -b ARO-ASO https://github.com/RadekCap/cluster-api-installer.git vendor/cluster-api-installer || true
	git submodule update --init --recursive

update-submodule: ## Update cluster-api-installer submodule
	git submodule update --remote vendor/cluster-api-installer

check-prereq: ## Check if required tools are installed
	@echo "Checking prerequisites..."
	@command -v docker >/dev/null 2>&1 || command -v podman >/dev/null 2>&1 || (echo "Error: docker or podman required" && exit 1)
	@command -v kind >/dev/null 2>&1 || (echo "Error: kind required" && exit 1)
	@command -v az >/dev/null 2>&1 || (echo "Error: az (Azure CLI) required" && exit 1)
	@command -v oc >/dev/null 2>&1 || (echo "Error: oc (OpenShift CLI) required" && exit 1)
	@command -v helm >/dev/null 2>&1 || (echo "Error: helm required" && exit 1)
	@command -v git >/dev/null 2>&1 || (echo "Error: git required" && exit 1)
	@command -v kubectl >/dev/null 2>&1 || (echo "Error: kubectl required" && exit 1)
	@echo "All prerequisites are installed!"

install-gotestsum: ## Install gotestsum for test summaries
	@echo "Installing gotestsum v1.13.0..."
	@go install gotest.tools/gotestsum@v1.13.0
	@echo "gotestsum installed successfully!"

check-gotestsum: ## Check if gotestsum is installed, install if missing
	@command -v gotestsum >/dev/null 2>&1 || $(MAKE) install-gotestsum

fmt: ## Format Go code
	go fmt ./...

lint: ## Run linters
	golangci-lint run ./... || go vet ./...

deps: ## Download Go dependencies
	go mod download
	go mod tidy
