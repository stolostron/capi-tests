.PHONY: test _check-dep _setup _cluster _generate-yamls _deploy-crds _verify test-all clean help

# Default values
DEPLOYMENT_ENV ?= stage
REGION ?= uksouth
MANAGEMENT_CLUSTER_NAME ?= capz-stage

# Test configuration
GOTESTSUM_FORMAT ?= testname
# Validate GOTESTSUM_FORMAT against allowlist to prevent command injection
ALLOWED_FORMATS := testname pkgname standard-verbose testdox github-actions
ifeq (,$(filter $(GOTESTSUM_FORMAT),$(ALLOWED_FORMATS)))
  $(error Invalid GOTESTSUM_FORMAT "$(GOTESTSUM_FORMAT)". Allowed: $(ALLOWED_FORMATS))
endif

# Test verbosity configuration
# Set to -v for verbose output (default), or empty string for quiet output
TEST_VERBOSITY ?= -v

# Test timeout configuration
# Individual phase timeouts (format: Go duration like 30m, 1h, etc.)
CLUSTER_TIMEOUT ?= 30m
GENERATE_YAMLS_TIMEOUT ?= 20m
DEPLOY_CRS_TIMEOUT ?= 40m
VERIFY_TIMEOUT ?= 20m

# Results directory configuration
# Create unique results directory for each test run using timestamp
TIMESTAMP := $(shell date +%Y%m%d_%H%M%S)
RESULTS_DIR := results/$(TIMESTAMP)
LATEST_RESULTS_DIR := results/latest

# Determine Go binary installation path
# Prefer GOBIN if set, otherwise use GOPATH/bin, with fallback to $HOME/go/bin
GOBIN := $(shell if [ -n "$$(go env GOBIN 2>/dev/null)" ]; then go env GOBIN; else echo "$$(go env GOPATH 2>/dev/null || echo "$$HOME/go")/bin"; fi)
GOTESTSUM := $(GOBIN)/gotestsum --format='$(GOTESTSUM_FORMAT)'

# Internal target to copy latest results
.PHONY: _copy-latest-results
_copy-latest-results:
	@mkdir -p $(LATEST_RESULTS_DIR)
	@cp -f $(RESULTS_DIR)/*.xml $(LATEST_RESULTS_DIR)/ 2>/dev/null || true

help: ## Display this help message
	@echo "ARO-CAPZ Test Suite Makefile"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-20s %s\n", $$1, $$2}'
	@echo ""
	@echo "Expected order for manual execution of internal targets:"
	@echo "  1. make _check-dep       # Check software prerequisites needed for a proper test run"
	@echo "  2. make _setup           # Setup and prepare input repositories with helm charts and CRDs"
	@echo "  3. make _cluster         # Prepare cluster for testing, and prepare operators needed for testing"
	@echo "  4. make _generate-yamls  # Generate script for resource creation (yaml)"
	@echo "  5. make _deploy-crs      # Deploy CRs and verify deployment"
	@echo "  6. make _verify          # Verify deployed cluster"

test: _check-dep ## Run check dependencies tests only

_check-dep: check-gotestsum
	@mkdir -p $(RESULTS_DIR)
	@echo "=== Running Check Dependencies Tests ==="
	@echo "Results will be saved to: $(RESULTS_DIR)"
	@echo ""
	@$(GOTESTSUM) --junitfile=$(RESULTS_DIR)/junit-check-dep.xml -- $(TEST_VERBOSITY) ./test -run TestCheckDependencies
	@$(MAKE) --no-print-directory _copy-latest-results
	@echo ""
	@echo "Test results saved to: $(RESULTS_DIR)/junit-check-dep.xml"
	@echo "Latest results copied to: $(LATEST_RESULTS_DIR)/"
	@echo "✅ Check Dependencies Tests completed"
	@echo ""

_setup: check-gotestsum
	@mkdir -p $(RESULTS_DIR)
	@echo "=== Running Repository Setup Tests ==="
	@echo "Results will be saved to: $(RESULTS_DIR)"
	@echo ""
	@$(GOTESTSUM) --junitfile=$(RESULTS_DIR)/junit-setup.xml -- $(TEST_VERBOSITY) ./test -run TestSetup
	@$(MAKE) --no-print-directory _copy-latest-results
	@echo ""
	@echo "Test results saved to: $(RESULTS_DIR)/junit-setup.xml"
	@echo "Latest results copied to: $(LATEST_RESULTS_DIR)/"
	@echo "✅ Repository Setup Tests completed"
	@echo ""

_cluster: check-gotestsum
	@mkdir -p $(RESULTS_DIR)
	@echo "=== Running Cluster Deployment Tests ==="
	@echo "Results will be saved to: $(RESULTS_DIR)"
	@echo ""
	@$(GOTESTSUM) --junitfile=$(RESULTS_DIR)/junit-cluster.xml -- $(TEST_VERBOSITY) ./test -run TestKindCluster -timeout $(CLUSTER_TIMEOUT)
	@$(MAKE) --no-print-directory _copy-latest-results
	@echo ""
	@echo "Test results saved to: $(RESULTS_DIR)/junit-cluster.xml"
	@echo "Latest results copied to: $(LATEST_RESULTS_DIR)/"
	@echo "✅ Cluster Deployment Tests completed"
	@echo ""

_generate-yamls: check-gotestsum
	@mkdir -p $(RESULTS_DIR)
	@echo "=== Running YAML Generation Tests ==="
	@echo "Results will be saved to: $(RESULTS_DIR)"
	@echo ""
	@$(GOTESTSUM) --junitfile=$(RESULTS_DIR)/junit-generate-yamls.xml -- $(TEST_VERBOSITY) ./test -run TestInfrastructure -timeout $(GENERATE_YAMLS_TIMEOUT)
	@$(MAKE) --no-print-directory _copy-latest-results
	@echo ""
	@echo "Test results saved to: $(RESULTS_DIR)/junit-generate-yamls.xml"
	@echo "Latest results copied to: $(LATEST_RESULTS_DIR)/"
	@echo "✅ YAML Generation Tests completed"
	@echo ""

_deploy-crs: check-gotestsum
	@mkdir -p $(RESULTS_DIR)
	@echo "=== Running CR Deployment Tests ==="
	@echo "Results will be saved to: $(RESULTS_DIR)"
	@echo ""
	@$(GOTESTSUM) --junitfile=$(RESULTS_DIR)/junit-deploy-crs.xml -- $(TEST_VERBOSITY) ./test -run TestDeployment -timeout $(DEPLOY_CRS_TIMEOUT)
	@$(MAKE) --no-print-directory _copy-latest-results
	@echo ""
	@echo "Test results saved to: $(RESULTS_DIR)/junit-deploy-crs.xml"
	@echo "Latest results copied to: $(LATEST_RESULTS_DIR)/"
	@echo "✅ CR Deployment Tests completed"
	@echo ""

_verify: check-gotestsum
	@mkdir -p $(RESULTS_DIR)
	@echo "=== Running Cluster Verification Tests ==="
	@echo "Results will be saved to: $(RESULTS_DIR)"
	@echo ""
	@$(GOTESTSUM) --junitfile=$(RESULTS_DIR)/junit-verify.xml -- $(TEST_VERBOSITY) ./test -run TestVerification -timeout $(VERIFY_TIMEOUT)
	@$(MAKE) --no-print-directory _copy-latest-results
	@echo ""
	@echo "Test results saved to: $(RESULTS_DIR)/junit-verify.xml"
	@echo "Latest results copied to: $(LATEST_RESULTS_DIR)/"
	@echo "✅ Cluster Verification Tests completed"
	@echo ""

test-all: ## Run all test phases sequentially
	@mkdir -p $(RESULTS_DIR)
	@echo "========================================"
	@echo "=== Running Full Test Suite ==="
	@echo "========================================"
	@echo ""
	@echo "All test results will be saved to: $(RESULTS_DIR)"
	@echo ""
	@$(MAKE) --no-print-directory _check-dep || ( \
		echo ""; \
		echo "❌ ERROR: Check dependencies phase failed. Cannot continue with test suite."; \
		echo "   Please ensure all required tools are installed and try again."; \
		echo ""; \
		exit 1 \
	)
	@$(MAKE) --no-print-directory _setup || ( \
		echo ""; \
		echo "❌ ERROR: Repository setup phase failed. Cannot continue with test suite."; \
		echo "   Previous stage (check dependencies) completed successfully."; \
		echo ""; \
		exit 1 \
	)
	@$(MAKE) --no-print-directory _cluster || ( \
		echo ""; \
		echo "❌ ERROR: Cluster deployment phase failed. Cannot continue with test suite."; \
		echo "   Previous stages (check dependencies, setup) completed successfully."; \
		echo ""; \
		exit 1 \
	)
	@$(MAKE) --no-print-directory _generate-yamls || ( \
		echo ""; \
		echo "❌ ERROR: YAML generation phase failed. Cannot continue with test suite."; \
		echo "   Previous stages (check dependencies, setup, cluster) completed successfully."; \
		echo ""; \
		exit 1 \
	)
	@$(MAKE) --no-print-directory _deploy-crs || ( \
		echo ""; \
		echo "❌ ERROR: CR deployment phase failed. Cannot continue with test suite."; \
		echo "   Previous stages (check dependencies, setup, cluster, YAML generation) completed successfully."; \
		echo ""; \
		exit 1 \
	)
	@$(MAKE) --no-print-directory _verify || ( \
		echo ""; \
		echo "❌ ERROR: Cluster verification phase failed."; \
		echo "   Previous stages completed successfully but final verification encountered issues."; \
		echo ""; \
		exit 1 \
	)
	@echo ""
	@echo "======================================="
	@echo "=== All Test Phases Completed Successfully ==="
	@echo "======================================="
	@echo ""
	@echo "All test results saved to: $(RESULTS_DIR)"
	@echo "Latest results copied to: $(LATEST_RESULTS_DIR)/"

clean: ## Clean up test resources (interactive)
	@echo "========================================"
	@echo "=== Interactive Cleanup ==="
	@echo "========================================"
	@echo ""
	@echo "This will guide you through cleaning up test resources."
	@echo "You can choose what to delete."
	@echo ""
	@# Check if management cluster exists
	@if kind get clusters 2>/dev/null | grep -q "^$(MANAGEMENT_CLUSTER_NAME)$$"; then \
		echo "Management cluster '$(MANAGEMENT_CLUSTER_NAME)' exists."; \
		read -p "Delete management cluster '$(MANAGEMENT_CLUSTER_NAME)'? [y/N] " -n 1 -r; \
		echo ""; \
		if [[ $$REPLY =~ ^[Yy]$$ ]]; then \
			echo "Deleting management cluster..."; \
			kind delete cluster --name $(MANAGEMENT_CLUSTER_NAME) || echo "Failed to delete cluster"; \
		else \
			echo "Skipped management cluster deletion."; \
		fi; \
	else \
		echo "Management cluster '$(MANAGEMENT_CLUSTER_NAME)' not found (already clean)."; \
	fi
	@echo ""
	@# Check if cluster-api-installer directory exists
	@if [ -d "/tmp/cluster-api-installer-aro" ]; then \
		echo "Directory /tmp/cluster-api-installer-aro exists."; \
		read -p "Delete /tmp/cluster-api-installer-aro? [y/N] " -n 1 -r; \
		echo ""; \
		if [[ $$REPLY =~ ^[Yy]$$ ]]; then \
			echo "Deleting directory..."; \
			rm -rf /tmp/cluster-api-installer-aro || echo "Failed to delete directory"; \
		else \
			echo "Skipped directory deletion."; \
		fi; \
	else \
		echo "Directory /tmp/cluster-api-installer-aro not found (already clean)."; \
	fi
	@echo ""
	@# Check if kubeconfig files exist
	@if ls /tmp/*-kubeconfig.yaml 1> /dev/null 2>&1; then \
		echo "Kubeconfig files found in /tmp:"; \
		ls -1 /tmp/*-kubeconfig.yaml; \
		read -p "Delete kubeconfig files? [y/N] " -n 1 -r; \
		echo ""; \
		if [[ $$REPLY =~ ^[Yy]$$ ]]; then \
			echo "Deleting kubeconfig files..."; \
			rm -f /tmp/*-kubeconfig.yaml || echo "Failed to delete kubeconfig files"; \
		else \
			echo "Skipped kubeconfig files deletion."; \
		fi; \
	else \
		echo "No kubeconfig files found in /tmp (already clean)."; \
	fi
	@echo ""
	@# Check if results directory exists
	@if [ -d "results" ]; then \
		echo "Results directory exists."; \
		echo "Contents:"; \
		du -sh results/* 2>/dev/null || echo "  (empty)"; \
		read -p "Delete results directory? [y/N] " -n 1 -r; \
		echo ""; \
		if [[ $$REPLY =~ ^[Yy]$$ ]]; then \
			echo "Deleting results directory..."; \
			rm -rf results || echo "Failed to delete results directory"; \
		else \
			echo "Skipped results directory deletion."; \
		fi; \
	else \
		echo "Results directory not found (already clean)."; \
	fi
	@echo ""
	@echo "======================================="
	@echo "=== Cleanup Complete ==="
	@echo "======================================="

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
	@command -v go >/dev/null 2>&1 || (echo "Error: go required (install from https://golang.org/dl/)" && exit 1)
	@echo "All prerequisites are installed!"

install-gotestsum: ## Install gotestsum for test summaries
	@echo "Installing gotestsum v1.13.0..."
	@command -v go >/dev/null 2>&1 || (echo "Error: go is required to install gotestsum. Install Go from https://golang.org/dl/" && exit 1)
	@go install gotest.tools/gotestsum@v1.13.0
	@echo "gotestsum installed successfully to $(GOBIN)/gotestsum"
	@if ! echo ":$$PATH:" | grep -q ":$(GOBIN):"; then \
		echo ""; \
		echo "⚠️  Warning: $(GOBIN) is not in your PATH"; \
		echo "   Add it to your PATH by running:"; \
		echo "   export PATH=\"\$$PATH:$(GOBIN)\""; \
		echo ""; \
		echo "   To make it permanent, add this line to your ~/.zshrc or ~/.bash_profile"; \
		echo "   The Makefile will use the full path, so tests will still work."; \
	fi

check-gotestsum: ## Check if gotestsum is installed, install if missing
	@test -f $(GOBIN)/gotestsum || $(MAKE) install-gotestsum

fix-docker-config: ## Fix Docker credential helper configuration issues
	@DOCKER_CONFIG_DIR="$${DOCKER_CONFIG:-$$HOME/.docker}"; \
	CONFIG_FILE="$$DOCKER_CONFIG_DIR/config.json"; \
	BACKUP_FILE="$$DOCKER_CONFIG_DIR/config.json.backup"; \
	TMP_FILE="$$DOCKER_CONFIG_DIR/config.json.tmp"; \
	echo "Fixing Docker credential helper configuration..."; \
	if [ ! -f "$$CONFIG_FILE" ]; then \
		echo "✅ No Docker config file found - nothing to fix"; \
		exit 0; \
	fi; \
	echo "Current Docker config:"; \
	cat "$$CONFIG_FILE"; \
	echo ""; \
	echo "Creating backup at $$BACKUP_FILE..."; \
	cp "$$CONFIG_FILE" "$$BACKUP_FILE"; \
	echo "Removing credsStore from Docker config..."; \
	if command -v jq >/dev/null 2>&1; then \
		if jq 'del(.credsStore) | del(.credHelpers)' "$$CONFIG_FILE" > "$$TMP_FILE" 2>/dev/null; then \
			mv "$$TMP_FILE" "$$CONFIG_FILE" && \
			echo "✅ Docker config fixed using jq"; \
		else \
			rm -f "$$TMP_FILE"; \
			echo "❌ Failed to fix Docker config with jq"; \
			exit 1; \
		fi; \
	else \
		echo "⚠️  jq not found - using sed fallback"; \
		sed -E '/"credsStore":/d; /"credHelpers":/,/}/d' "$$CONFIG_FILE" > "$$TMP_FILE" && \
		sed -E -i '' 's/,\s*([}]])/\1/g' "$$TMP_FILE" && \
		mv "$$TMP_FILE" "$$CONFIG_FILE"; \
		echo "✅ Docker config fixed using sed"; \
	fi; \
	echo ""; \
	echo "Updated Docker config:"; \
	cat "$$CONFIG_FILE"; \
	echo ""; \
	echo "✅ Docker credential helper configuration fixed!"; \
	echo "   Backup saved to $$BACKUP_FILE"; \
	echo ""; \
	echo "You can now run 'make test-all' to deploy the Kind cluster and run all tests"

fmt: ## Format Go code
	go fmt ./...

lint: ## Run linters
	golangci-lint run ./... || go vet ./...

deps: ## Download Go dependencies
	go mod download
	go mod tidy
