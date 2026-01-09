# ARO-CAPZ Test Suite

[![Check Dependencies](https://github.com/RadekCap/CAPZTests/actions/workflows/check-dependencies.yml/badge.svg)](https://github.com/RadekCap/CAPZTests/actions/workflows/check-dependencies.yml)
[![Repository Setup](https://github.com/RadekCap/CAPZTests/actions/workflows/test-setup.yml/badge.svg)](https://github.com/RadekCap/CAPZTests/actions/workflows/test-setup.yml)
[![Cluster Preparation](https://github.com/RadekCap/CAPZTests/actions/workflows/test-kind-cluster.yml/badge.svg)](https://github.com/RadekCap/CAPZTests/actions/workflows/test-kind-cluster.yml)

**Security Scanning:**

[![govulncheck](https://github.com/RadekCap/CAPZTests/actions/workflows/security-govulncheck.yml/badge.svg)](https://github.com/RadekCap/CAPZTests/actions/workflows/security-govulncheck.yml)
[![gosec](https://github.com/RadekCap/CAPZTests/actions/workflows/security-gosec.yml/badge.svg)](https://github.com/RadekCap/CAPZTests/actions/workflows/security-gosec.yml)

[![Trivy](https://github.com/RadekCap/CAPZTests/actions/workflows/security-trivy.yml/badge.svg)](https://github.com/RadekCap/CAPZTests/actions/workflows/security-trivy.yml)
[![nancy](https://github.com/RadekCap/CAPZTests/actions/workflows/security-nancy.yml/badge.svg)](https://github.com/RadekCap/CAPZTests/actions/workflows/security-nancy.yml)

Comprehensive test suite for Azure Red Hat OpenShift (ARO) deployment using Cluster API Provider Azure (CAPZ) and Azure Service Operator (ASO).

## Overview

This repository contains a Go-based test suite that validates the complete ARO cluster deployment workflow on Azure using CAPZ. The tests verify each step of the deployment process, from prerequisite verification to final cluster validation.

The test suite is designed to work with the [cluster-api-installer](https://github.com/RadekCap/cluster-api-installer) ARO-CAPZ implementation.

## What This Tests

The test suite validates:
- **CAPZ on Azure** - Cluster API Provider Azure for deploying Kubernetes infrastructure on Azure
- **ARO Deployment** - Azure Red Hat OpenShift cluster provisioning
- **ASO Integration** - Azure Service Operator for managing Azure resources

## Consumers

Target usage of this test suite will be:

- **OSCI (OpenShift CI)** - Automated continuous integration testing for OpenShift deployments
- **ACM (Advanced Cluster Management)** - Multi-cluster management and validation workflows
- **Manual Testing** - Developer and QA validation of ARO-CAPZ deployments

## Prerequisites

### Required Tools

- **Docker** or **Podman** - Container runtime
- **Kind** - Kubernetes in Docker for management cluster
- **Azure CLI** (`az`) - Azure authentication and management
- **OpenShift CLI** (`oc`) - OpenShift cluster interaction
- **Helm** - Package manager for Kubernetes
- **Git** - Source control
- **kubectl** - Kubernetes CLI
- **Go** 1.21+ - For running tests

### Azure Access

- Azure account with appropriate permissions
- Access to Azure subscription for ARO deployment
- Authenticated via `az login`

## Configuration

Tests are configured via environment variables:

### Repository Configuration

- `ARO_REPO_URL` - cluster-api-installer repository URL (default: `https://github.com/RadekCap/cluster-api-installer.git`)
- `ARO_REPO_BRANCH` - Branch to use (default: `ARO-ASO`)
- `ARO_REPO_DIR` - Local repository directory (default: `/tmp/cluster-api-installer-aro`)

### Cluster Configuration

- `MANAGEMENT_CLUSTER_NAME` - Management cluster name (default: `capz-tests-stage`)
  - **Note**: Tests automatically translate this to `KIND_CLUSTER_NAME` for the deployment script
  - Use this variable for configuring tests; `KIND_CLUSTER_NAME` is set internally
- `WORKLOAD_CLUSTER_NAME` - ARO workload cluster name (default: `capz-tests-cluster`)
- `CS_CLUSTER_NAME` - Cluster name prefix used for YAML generation (default: `${CAPZ_USER}-${DEPLOYMENT_ENV}`). The Azure resource group will be named `${CS_CLUSTER_NAME}-resgroup`.
- `OPENSHIFT_VERSION` - OpenShift version (default: `4.21`)
- `REGION` - Azure region (default: `uksouth`)
- `AZURE_SUBSCRIPTION_NAME` - Azure subscription ID
- `DEPLOYMENT_ENV` - Deployment environment identifier (default: `stage`)
- `CAPZ_USER` - User identifier for domain prefix (default: `rcap`)

#### Naming Requirements (RFC 1123)

The following variables must be **RFC 1123 compliant** to avoid deployment failures:
- `CAPZ_USER`
- `CS_CLUSTER_NAME`
- `DEPLOYMENT_ENV`

**RFC 1123 naming rules:**
- Only lowercase alphanumeric characters and hyphens (`a-z`, `0-9`, `-`)
- Must start and end with an alphanumeric character
- No uppercase letters, underscores, dots, or spaces

**Example valid values:**
```bash
export CAPZ_USER=rcap        # Valid
export DEPLOYMENT_ENV=stage  # Valid
export CS_CLUSTER_NAME=rcap-stage  # Valid
```

**Example invalid values:**
```bash
export CAPZ_USER=RCap        # Invalid - contains uppercase
export DEPLOYMENT_ENV=Stage_1  # Invalid - contains uppercase and underscore
export CS_CLUSTER_NAME=-my-cluster  # Invalid - starts with hyphen
```

The test suite validates naming compliance during the Check Dependencies phase (phase 1), preventing late failures during CR deployment (phase 5).

### Test Behavior

- `DEPLOYMENT_TIMEOUT` - Control plane deployment timeout (default: `45m`). Use Go duration format: `1h`, `45m`, `90m`, etc.
- `TEST_VERBOSITY` - Test output verbosity (default: `-v` for verbose). Set to empty string for quiet output: `TEST_VERBOSITY= make test`

## Getting Started

### Quick Start

1. **Install prerequisites**:
   ```bash
   # Check prerequisites
   make check-prereq
   ```

2. **Authenticate with Azure**:
   ```bash
   az login
   ```

3. **Run check dependencies tests**:
   ```bash
   make test
   ```

4. **Run full test suite**:
   ```bash
   make test-all
   ```

### Running Tests

#### Using Makefile

```bash
# Run check dependencies tests only (fast, no Azure resources created)
make test

# Run full test suite (all phases sequentially)
make test-all

# Run specific test phase using Go test directly
go test -v ./test -run TestSetup
go test -v ./test -run TestKindCluster
go test -v ./test -run TestInfrastructure
go test -v ./test -run TestDeployment
go test -v ./test -run TestVerification

# Run tests with quiet output (no verbose flag)
TEST_VERBOSITY= make test

# Run tests with verbose output (default)
TEST_VERBOSITY=-v make test
```

**Note**: All test targets automatically generate JUnit XML reports in a timestamped `results/` directory. The path to the results directory is displayed when tests run.

#### Using Go Test Directly

```bash
# Run all tests
go test -v ./test -timeout 60m

# Run specific test phase
go test -v ./test -run TestCheckDependencies
go test -v ./test -run TestInfrastructure

# Run with custom configuration
DEPLOYMENT_ENV=prod \
WORKLOAD_CLUSTER_NAME=my-aro-cluster \
REGION=westus2 \
go test -v ./test -timeout 60m
```

### Test Results and Reports

All Makefile test targets automatically generate JUnit XML reports for test results. Each test run creates a unique timestamped directory under `results/` containing the XML reports.

#### Results Directory Structure

```
results/
└── 20251205_093128/               # Timestamp: YYYYMMDD_HHMMSS
    ├── junit-check-dep.xml        # Check dependencies test results
    ├── junit-setup.xml            # Setup test results
    ├── junit-cluster.xml          # Cluster deployment test results
    ├── junit-generate-yamls.xml   # YAML generation test results
    ├── junit-deploy-crs.xml       # CR deployment test results
    └── junit-verify.xml           # Verification test results
```

#### Using Test Results

When you run a test target, the results path is printed to the terminal:

```bash
$ make test
=== Running Check Dependencies Tests ===
Results will be saved to: results/20251205_093128

# ... test output ...

Test results saved to: results/20251205_093128/junit-check-dep.xml
```

The JUnit XML files can be:
- Consumed by CI/CD systems (GitHub Actions, Jenkins, GitLab CI)
- Visualized in test reporting tools
- Parsed for automated analysis
- Archived for historical tracking

#### Cleanup

The `results/` directory is excluded from git (via `.gitignore`) and can be cleaned up with:

```bash
make clean      # Interactive cleanup - prompts for confirmation before deleting each resource
make clean-all  # Non-interactive - deletes ALL resources without prompting
FORCE=1 make clean  # Same as clean-all
make clean-azure  # Delete only Azure resource group (interactive)
```

The `make clean` command will interactively ask you to confirm deletion of:
- Kind cluster (if it exists)
- Cluster-api-installer repository clone in `/tmp`
- Kubeconfig files in `/tmp`
- Results directory
- **Azure resource group** (`${CS_CLUSTER_NAME}-resgroup`, e.g., `rcap-stage-resgroup`)

This allows you to selectively clean up resources while preserving anything you want to keep.

For automated workflows (CI/CD, scripts) or quick full resets, use:
- `make clean-all` - deletes all resources without prompting (includes Azure resource group)
- `FORCE=1 make clean` - equivalent to `make clean-all`

**Azure Resource Cleanup**: The cleanup commands now include Azure resource group deletion:
- Uses `--no-wait` for non-blocking deletion (deletion continues in background)
- Gracefully skips if Azure CLI is not installed or not logged in
- Checks if resource group exists before attempting deletion
- The resource group name is derived from `${CAPZ_USER}-${DEPLOYMENT_ENV}-resgroup` (default: `rcap-stage-resgroup`)

## Integration with cluster-api-installer

The test suite needs access to the cluster-api-installer repository. Three integration approaches are supported:

### Option 1: Git Submodule (Recommended)

```bash
make setup-submodule
export ARO_REPO_DIR="$(pwd)/vendor/cluster-api-installer"
make test-all
```

### Option 2: Automatic Clone

Let tests clone the repository automatically:

```bash
# Tests will clone to /tmp/cluster-api-installer-aro
make test-all
```

### Option 3: Existing Clone

Point to an existing clone:

```bash
export ARO_REPO_DIR="/path/to/cluster-api-installer"
make test-all
```

See [INTEGRATION.md](docs/INTEGRATION.md) for detailed integration patterns.

## Test Structure

```
test/
├── 01_check_dependencies_test.go  # Tool and auth verification
├── 02_setup_test.go           # Repository setup
├── 03_cluster_test.go         # Management cluster deployment
├── 04_generate_yamls_test.go  # Resource generation
├── 05_deploy_crs_test.go      # Cluster provisioning monitoring
├── 06_verification_test.go    # Final cluster validation
├── config.go                  # Configuration management
├── helpers.go                 # Shared utilities
└── README.md               # Detailed test documentation
```

For detailed test documentation, see [test/README.md](test/README.md).

## CI/CD Integration

The test suite integrates with GitHub Actions for continuous testing:

- **Check Dependencies Workflow** - Runs dependency checks on every push
- **Full Test Workflow** - Can be triggered manually for complete validation

## Contributing

Contributions are welcome! Please ensure:

1. All tests pass before submitting PRs
2. New functionality includes appropriate tests
3. Documentation is updated to reflect changes

## License

[License information to be added]
