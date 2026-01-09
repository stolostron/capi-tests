.PHONY: test _check-dep _setup _cluster _generate-yamls _deploy-crds _verify test-all clean clean-all clean-azure help summary

# Default values
# Extract CAPZ_USER default from Go config to maintain single source of truth
CAPZ_USER_DEFAULT := $(shell grep 'DefaultCAPZUser = ' test/config.go | grep -o '"[^"]*"' | tr -d '"')
CAPZ_USER ?= $(CAPZ_USER_DEFAULT)
DEPLOYMENT_ENV ?= stage
REGION ?= uksouth
MANAGEMENT_CLUSTER_NAME ?= capz-tests-stage
CS_CLUSTER_NAME ?= $(CAPZ_USER)-$(DEPLOYMENT_ENV)
AZURE_RESOURCE_GROUP ?= $(CS_CLUSTER_NAME)-resgroup

# Deployment state file - written by tests to record actual deployed configuration
DEPLOYMENT_STATE_FILE := .deployment-state.json

# Read from deployment state file if it exists (for cleanup to target correct resources)
# This ensures cleanup targets the same resources that were actually deployed,
# even if environment variables or defaults have changed since deployment.
STATE_RESOURCE_GROUP := $(shell if [ -f $(DEPLOYMENT_STATE_FILE) ]; then cat $(DEPLOYMENT_STATE_FILE) | grep '"resource_group"' | sed 's/.*: *"\([^"]*\)".*/\1/'; fi)
STATE_MANAGEMENT_CLUSTER := $(shell if [ -f $(DEPLOYMENT_STATE_FILE) ]; then cat $(DEPLOYMENT_STATE_FILE) | grep '"management_cluster_name"' | sed 's/.*: *"\([^"]*\)".*/\1/'; fi)

# Use state file values if available, otherwise use defaults
CLEANUP_RESOURCE_GROUP := $(if $(STATE_RESOURCE_GROUP),$(STATE_RESOURCE_GROUP),$(AZURE_RESOURCE_GROUP))
CLEANUP_MANAGEMENT_CLUSTER := $(if $(STATE_MANAGEMENT_CLUSTER),$(STATE_MANAGEMENT_CLUSTER),$(MANAGEMENT_CLUSTER_NAME))

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
DEPLOY_CRS_TIMEOUT ?= 60m
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

# Default target - show help when running 'make' with no arguments
.DEFAULT_GOAL := help

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
	@EXIT_CODE=0; \
	$(GOTESTSUM) --junitfile=$(RESULTS_DIR)/junit-check-dep.xml -- $(TEST_VERBOSITY) ./test -count=1 -run TestCheckDependencies || EXIT_CODE=$$?; \
	mkdir -p $(LATEST_RESULTS_DIR); \
	cp -f $(RESULTS_DIR)/*.xml $(LATEST_RESULTS_DIR)/ 2>/dev/null || true; \
	echo ""; \
	echo "Test results saved to: $(RESULTS_DIR)/junit-check-dep.xml"; \
	echo "Latest results copied to: $(LATEST_RESULTS_DIR)/"; \
	if [ $$EXIT_CODE -eq 0 ]; then \
		echo "✅ Check Dependencies Tests completed"; \
	else \
		echo "❌ Check Dependencies Tests failed"; \
	fi; \
	echo ""; \
	exit $$EXIT_CODE

_setup: check-gotestsum
	@mkdir -p $(RESULTS_DIR)
	@echo "=== Running Repository Setup Tests ==="
	@echo "Results will be saved to: $(RESULTS_DIR)"
	@echo ""
	@EXIT_CODE=0; \
	$(GOTESTSUM) --junitfile=$(RESULTS_DIR)/junit-setup.xml -- $(TEST_VERBOSITY) ./test -count=1 -run TestSetup || EXIT_CODE=$$?; \
	mkdir -p $(LATEST_RESULTS_DIR); \
	cp -f $(RESULTS_DIR)/*.xml $(LATEST_RESULTS_DIR)/ 2>/dev/null || true; \
	echo ""; \
	echo "Test results saved to: $(RESULTS_DIR)/junit-setup.xml"; \
	echo "Latest results copied to: $(LATEST_RESULTS_DIR)/"; \
	if [ $$EXIT_CODE -eq 0 ]; then \
		echo "✅ Repository Setup Tests completed"; \
	else \
		echo "❌ Repository Setup Tests failed"; \
	fi; \
	echo ""; \
	exit $$EXIT_CODE

_cluster: check-gotestsum
	@mkdir -p $(RESULTS_DIR)
	@echo "=== Running Cluster Deployment Tests ==="
	@echo "Results will be saved to: $(RESULTS_DIR)"
	@echo ""
	@EXIT_CODE=0; \
	$(GOTESTSUM) --junitfile=$(RESULTS_DIR)/junit-cluster.xml -- $(TEST_VERBOSITY) ./test -count=1 -run TestKindCluster -timeout $(CLUSTER_TIMEOUT) -failfast || EXIT_CODE=$$?; \
	mkdir -p $(LATEST_RESULTS_DIR); \
	cp -f $(RESULTS_DIR)/*.xml $(LATEST_RESULTS_DIR)/ 2>/dev/null || true; \
	echo ""; \
	echo "Test results saved to: $(RESULTS_DIR)/junit-cluster.xml"; \
	echo "Latest results copied to: $(LATEST_RESULTS_DIR)/"; \
	if [ $$EXIT_CODE -eq 0 ]; then \
		echo "✅ Cluster Deployment Tests completed"; \
	else \
		echo "❌ Cluster Deployment Tests failed"; \
	fi; \
	echo ""; \
	exit $$EXIT_CODE

_generate-yamls: check-gotestsum
	@mkdir -p $(RESULTS_DIR)
	@echo "=== Running YAML Generation Tests ==="
	@echo "Results will be saved to: $(RESULTS_DIR)"
	@echo ""
	@EXIT_CODE=0; \
	$(GOTESTSUM) --junitfile=$(RESULTS_DIR)/junit-generate-yamls.xml -- $(TEST_VERBOSITY) ./test -count=1 -run TestInfrastructure -timeout $(GENERATE_YAMLS_TIMEOUT) || EXIT_CODE=$$?; \
	mkdir -p $(LATEST_RESULTS_DIR); \
	cp -f $(RESULTS_DIR)/*.xml $(LATEST_RESULTS_DIR)/ 2>/dev/null || true; \
	echo ""; \
	echo "Test results saved to: $(RESULTS_DIR)/junit-generate-yamls.xml"; \
	echo "Latest results copied to: $(LATEST_RESULTS_DIR)/"; \
	if [ $$EXIT_CODE -eq 0 ]; then \
		echo "✅ YAML Generation Tests completed"; \
	else \
		echo "❌ YAML Generation Tests failed"; \
	fi; \
	echo ""; \
	exit $$EXIT_CODE

_deploy-crs: check-gotestsum
	@mkdir -p $(RESULTS_DIR)
	@echo "=== Running CR Deployment Tests ==="
	@echo "Results will be saved to: $(RESULTS_DIR)"
	@echo ""
	@EXIT_CODE=0; \
	$(GOTESTSUM) --junitfile=$(RESULTS_DIR)/junit-deploy-crs.xml -- $(TEST_VERBOSITY) ./test -count=1 -run TestDeployment -timeout $(DEPLOY_CRS_TIMEOUT) || EXIT_CODE=$$?; \
	mkdir -p $(LATEST_RESULTS_DIR); \
	cp -f $(RESULTS_DIR)/*.xml $(LATEST_RESULTS_DIR)/ 2>/dev/null || true; \
	echo ""; \
	echo "Test results saved to: $(RESULTS_DIR)/junit-deploy-crs.xml"; \
	echo "Latest results copied to: $(LATEST_RESULTS_DIR)/"; \
	if [ $$EXIT_CODE -eq 0 ]; then \
		echo "✅ CR Deployment Tests completed"; \
	else \
		echo "❌ CR Deployment Tests failed"; \
	fi; \
	echo ""; \
	exit $$EXIT_CODE

_verify: check-gotestsum
	@mkdir -p $(RESULTS_DIR)
	@echo "=== Running Cluster Verification Tests ==="
	@echo "Results will be saved to: $(RESULTS_DIR)"
	@echo ""
	@EXIT_CODE=0; \
	TEST_RESULTS_DIR=$(RESULTS_DIR) $(GOTESTSUM) --junitfile=$(RESULTS_DIR)/junit-verify.xml -- $(TEST_VERBOSITY) ./test -count=1 -run TestVerification -timeout $(VERIFY_TIMEOUT) || EXIT_CODE=$$?; \
	mkdir -p $(LATEST_RESULTS_DIR); \
	cp -f $(RESULTS_DIR)/*.xml $(LATEST_RESULTS_DIR)/ 2>/dev/null || true; \
	cp -f $(RESULTS_DIR)/*.log $(LATEST_RESULTS_DIR)/ 2>/dev/null || true; \
	echo ""; \
	echo "Test results saved to: $(RESULTS_DIR)/junit-verify.xml"; \
	echo "Latest results copied to: $(LATEST_RESULTS_DIR)/"; \
	if [ $$EXIT_CODE -eq 0 ]; then \
		echo "✅ Cluster Verification Tests completed"; \
	else \
		echo "❌ Cluster Verification Tests failed"; \
	fi; \
	echo ""; \
	exit $$EXIT_CODE

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
	@# Generate test results summary from LATEST_RESULTS_DIR which contains all phases
	@# Each phase copies its results to LATEST_RESULTS_DIR, so the summary aggregates all
	@if [ -x scripts/generate-summary.sh ]; then \
		echo ""; \
		./scripts/generate-summary.sh $(LATEST_RESULTS_DIR); \
	fi
	@echo ""
	@echo "All test results saved to: $(LATEST_RESULTS_DIR)/"

clean: ## Clean up test resources (interactive, use FORCE=1 to skip prompts)
	@if [ "$(FORCE)" = "1" ]; then \
		$(MAKE) --no-print-directory clean-all; \
	else \
		echo "========================================"; \
		echo "=== Interactive Cleanup ==="; \
		echo "========================================"; \
		echo ""; \
		echo "This will guide you through cleaning up test resources."; \
		echo "You can choose what to delete."; \
		echo "Tip: Use 'make clean-all' or 'FORCE=1 make clean' to skip prompts."; \
		echo ""; \
		if [ -f "$(DEPLOYMENT_STATE_FILE)" ]; then \
			echo "📝 Using deployment state from $(DEPLOYMENT_STATE_FILE)"; \
			echo "   Resource group: $(CLEANUP_RESOURCE_GROUP)"; \
			echo "   Management cluster: $(CLEANUP_MANAGEMENT_CLUSTER)"; \
			echo ""; \
		fi; \
		if kind get clusters 2>/dev/null | grep -q "^$(CLEANUP_MANAGEMENT_CLUSTER)$$"; then \
			echo "Management cluster '$(CLEANUP_MANAGEMENT_CLUSTER)' exists."; \
			read -p "Delete management cluster '$(CLEANUP_MANAGEMENT_CLUSTER)'? [y/N] " -n 1 -r; \
			echo ""; \
			if [[ $$REPLY =~ ^[Yy]$$ ]]; then \
				echo "Deleting management cluster..."; \
				kind delete cluster --name $(CLEANUP_MANAGEMENT_CLUSTER) || echo "Failed to delete cluster"; \
			else \
				echo "Skipped management cluster deletion."; \
			fi; \
		else \
			echo "Management cluster '$(CLEANUP_MANAGEMENT_CLUSTER)' not found (already clean)."; \
		fi; \
		echo ""; \
		if [ -d "/tmp/cluster-api-installer-aro" ]; then \
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
		fi; \
		echo ""; \
		if ls /tmp/*-kubeconfig.yaml 1> /dev/null 2>&1; then \
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
		fi; \
		echo ""; \
		if [ -d "results" ]; then \
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
		fi; \
		echo ""; \
		echo "--- Azure Resources ---"; \
		echo "Target resource group: $(CLEANUP_RESOURCE_GROUP)"; \
		echo ""; \
		if ! command -v az >/dev/null 2>&1; then \
			echo "⚠️  Azure CLI (az) not available - skipping Azure cleanup"; \
		elif ! az account show >/dev/null 2>&1; then \
			echo "⚠️  Not logged in to Azure - skipping Azure cleanup"; \
			echo "   Run 'az login' to authenticate"; \
		elif az group show --name $(CLEANUP_RESOURCE_GROUP) >/dev/null 2>&1; then \
			echo "Resource group '$(CLEANUP_RESOURCE_GROUP)' exists."; \
			echo "⚠️  Warning: This will delete ALL resources in the resource group!"; \
			echo ""; \
			read -p "Delete Azure resource group '$(CLEANUP_RESOURCE_GROUP)'? [y/N] " -n 1 -r; \
			echo ""; \
			if [[ $$REPLY =~ ^[Yy]$$ ]]; then \
				echo "Deleting Azure resource group (this may take several minutes)..."; \
				az group delete --name $(CLEANUP_RESOURCE_GROUP) --yes --no-wait && \
				echo "✅ Resource group deletion initiated (running in background)"; \
			else \
				echo "Skipped Azure resource group deletion."; \
			fi; \
		else \
			echo "Azure resource group '$(CLEANUP_RESOURCE_GROUP)' not found (already clean)."; \
		fi; \
		echo ""; \
		if [ -f "$(DEPLOYMENT_STATE_FILE)" ]; then \
			echo "Removing deployment state file..."; \
			rm -f "$(DEPLOYMENT_STATE_FILE)"; \
		fi; \
		echo "======================================="; \
		echo "=== Cleanup Complete ==="; \
		echo "======================================="; \
	fi

clean-all: ## Clean up ALL test resources without prompting (local + Azure)
	@echo "========================================"
	@echo "=== Non-Interactive Cleanup ==="
	@echo "========================================"
	@echo ""
	@if [ -f "$(DEPLOYMENT_STATE_FILE)" ]; then \
		echo "📝 Using deployment state from $(DEPLOYMENT_STATE_FILE)"; \
		echo "   Resource group: $(CLEANUP_RESOURCE_GROUP)"; \
		echo "   Management cluster: $(CLEANUP_MANAGEMENT_CLUSTER)"; \
		echo ""; \
	fi
	@echo "Deleting all test resources without prompts..."
	@echo ""
	@# Delete Azure resource group first (before local resources)
	@$(MAKE) --no-print-directory _clean-azure-force
	@echo ""
	@# Delete management cluster
	@if kind get clusters 2>/dev/null | grep -q "^$(CLEANUP_MANAGEMENT_CLUSTER)$$"; then \
		echo "Deleting management cluster '$(CLEANUP_MANAGEMENT_CLUSTER)'..."; \
		kind delete cluster --name $(CLEANUP_MANAGEMENT_CLUSTER) || echo "Failed to delete cluster"; \
	else \
		echo "Management cluster '$(CLEANUP_MANAGEMENT_CLUSTER)' not found (already clean)."; \
	fi
	@echo ""
	@# Delete cluster-api-installer directory
	@if [ -d "/tmp/cluster-api-installer-aro" ]; then \
		echo "Deleting /tmp/cluster-api-installer-aro..."; \
		rm -rf /tmp/cluster-api-installer-aro || echo "Failed to delete directory"; \
	else \
		echo "Directory /tmp/cluster-api-installer-aro not found (already clean)."; \
	fi
	@echo ""
	@# Delete kubeconfig files
	@if ls /tmp/*-kubeconfig.yaml 1> /dev/null 2>&1; then \
		echo "Deleting kubeconfig files:"; \
		ls -1 /tmp/*-kubeconfig.yaml; \
		rm -f /tmp/*-kubeconfig.yaml || echo "Failed to delete kubeconfig files"; \
	else \
		echo "No kubeconfig files found in /tmp (already clean)."; \
	fi
	@echo ""
	@# Delete results directory
	@if [ -d "results" ]; then \
		echo "Deleting results directory..."; \
		rm -rf results || echo "Failed to delete results directory"; \
	else \
		echo "Results directory not found (already clean)."; \
	fi
	@echo ""
	@# Delete deployment state file
	@if [ -f "$(DEPLOYMENT_STATE_FILE)" ]; then \
		echo "Removing deployment state file..."; \
		rm -f "$(DEPLOYMENT_STATE_FILE)"; \
	fi
	@echo "======================================="
	@echo "=== All Resources Cleaned ==="
	@echo "======================================="

clean-azure: ## Delete Azure resource group created by deployment (interactive)
	@echo "=== Azure Resource Cleanup ==="
	@echo ""
	@if [ -f "$(DEPLOYMENT_STATE_FILE)" ]; then \
		echo "📝 Using deployment state from $(DEPLOYMENT_STATE_FILE)"; \
	fi
	@echo "Target resource group: $(CLEANUP_RESOURCE_GROUP)"
	@echo ""
	@# Check if Azure CLI is available
	@if ! command -v az >/dev/null 2>&1; then \
		echo "❌ Error: Azure CLI (az) is not installed or not in PATH"; \
		echo "   Install from: https://docs.microsoft.com/en-us/cli/azure/install-azure-cli"; \
		exit 1; \
	fi
	@# Check if logged in to Azure
	@if ! az account show >/dev/null 2>&1; then \
		echo "❌ Error: Not logged in to Azure CLI"; \
		echo "   Run 'az login' to authenticate"; \
		exit 1; \
	fi
	@# Check if resource group exists
	@if az group show --name $(CLEANUP_RESOURCE_GROUP) >/dev/null 2>&1; then \
		echo "Resource group '$(CLEANUP_RESOURCE_GROUP)' exists."; \
		echo "⚠️  Warning: This will delete ALL resources in the resource group!"; \
		echo ""; \
		read -p "Delete Azure resource group '$(CLEANUP_RESOURCE_GROUP)'? [y/N] " -n 1 -r; \
		echo ""; \
		if [[ $$REPLY =~ ^[Yy]$$ ]]; then \
			echo "Deleting Azure resource group '$(CLEANUP_RESOURCE_GROUP)' (this may take several minutes)..."; \
			az group delete --name $(CLEANUP_RESOURCE_GROUP) --yes --no-wait && \
			echo "✅ Resource group deletion initiated (running in background)"; \
			echo "   Use 'az group show --name $(CLEANUP_RESOURCE_GROUP)' to check status"; \
		else \
			echo "Skipped Azure resource group deletion."; \
		fi; \
	else \
		echo "Resource group '$(CLEANUP_RESOURCE_GROUP)' not found (already clean or never created)."; \
	fi

# Internal target: force delete Azure resource group without prompting
.PHONY: _clean-azure-force
_clean-azure-force:
	@# Check if Azure CLI is available
	@if ! command -v az >/dev/null 2>&1; then \
		echo "⚠️  Azure CLI (az) not available - skipping Azure cleanup"; \
	elif ! az account show >/dev/null 2>&1; then \
		echo "⚠️  Not logged in to Azure - skipping Azure cleanup"; \
	elif az group show --name $(CLEANUP_RESOURCE_GROUP) >/dev/null 2>&1; then \
		echo "Deleting Azure resource group '$(CLEANUP_RESOURCE_GROUP)' (running in background)..."; \
		az group delete --name $(CLEANUP_RESOURCE_GROUP) --yes --no-wait && \
		echo "✅ Resource group deletion initiated"; \
	else \
		echo "Azure resource group '$(CLEANUP_RESOURCE_GROUP)' not found (already clean)."; \
	fi

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

summary: ## Generate test results summary from latest results
	@if [ -d "$(LATEST_RESULTS_DIR)" ]; then \
		./scripts/generate-summary.sh $(LATEST_RESULTS_DIR); \
	elif [ -n "$$(ls -d results/2* 2>/dev/null | tail -1)" ]; then \
		LATEST_RUN=$$(ls -d results/2* 2>/dev/null | tail -1); \
		./scripts/generate-summary.sh "$$LATEST_RUN"; \
	else \
		echo "Error: No test results found. Run 'make test-all' first."; \
		exit 1; \
	fi

fmt: ## Format Go code
	go fmt ./...

lint: ## Run linters
	golangci-lint run ./... || go vet ./...

deps: ## Download Go dependencies
	go mod download
	go mod tidy
