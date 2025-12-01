.PHONY: test test-prereq test-setup test-kind test-infra test-deploy test-verify test-all test-short clean help

# Default values
CLUSTER_NAME ?= test-cluster
ENV ?= stage
REGION ?= uksouth
KIND_CLUSTER_NAME ?= capz-stage

help: ## Display this help message
	@echo "ARO-CAPZ Test Suite Makefile"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-20s %s\n", $$1, $$2}'

test: ## Run all tests
	go test -v ./test -timeout 60m

test-short: ## Run quick tests only (skip long-running tests)
	go test -v -short ./test

test-prereq: ## Run prerequisite verification tests only
	go test -v ./test -run TestPrerequisites

test-setup: ## Run repository setup tests only
	go test -v ./test -run TestSetup

test-kind: ## Run Kind cluster deployment tests only
	go test -v ./test -run TestKindCluster -timeout 30m

test-infra: ## Run infrastructure generation tests only
	go test -v ./test -run TestInfrastructure -timeout 20m

test-deploy: ## Run deployment monitoring tests only
	go test -v ./test -run TestDeployment -timeout 40m

test-verify: ## Run cluster verification tests only
	go test -v ./test -run TestVerification -timeout 20m

test-all: test-prereq test-setup test-kind test-infra test-deploy test-verify ## Run all test phases sequentially

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

fmt: ## Format Go code
	go fmt ./...

lint: ## Run linters
	golangci-lint run ./... || go vet ./...

deps: ## Download Go dependencies
	go mod download
	go mod tidy
