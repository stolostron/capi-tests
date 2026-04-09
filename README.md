# CAPI Test Suite

**ARO:**

[![Management Cluster (ARO)](https://github.com/stolostron/capi-tests/actions/workflows/management-cluster-aro.yml/badge.svg)](https://github.com/stolostron/capi-tests/actions/workflows/management-cluster-aro.yml)
[![Full Cluster Deployment (ARO)](https://github.com/stolostron/capi-tests/actions/workflows/workload-cluster-aro.yml/badge.svg)](https://github.com/stolostron/capi-tests/actions/workflows/workload-cluster-aro.yml)

**ROSA:**

[![Management Cluster (ROSA)](https://github.com/stolostron/capi-tests/actions/workflows/management-cluster-rosa.yml/badge.svg)](https://github.com/stolostron/capi-tests/actions/workflows/management-cluster-rosa.yml)
[![Full Cluster Deployment (ROSA)](https://github.com/stolostron/capi-tests/actions/workflows/workload-cluster-rosa.yml/badge.svg)](https://github.com/stolostron/capi-tests/actions/workflows/workload-cluster-rosa.yml)

**Security Scanning:**

[![govulncheck](https://github.com/stolostron/capi-tests/actions/workflows/security-govulncheck.yml/badge.svg)](https://github.com/stolostron/capi-tests/actions/workflows/security-govulncheck.yml)
[![gosec](https://github.com/stolostron/capi-tests/actions/workflows/security-gosec.yml/badge.svg)](https://github.com/stolostron/capi-tests/actions/workflows/security-gosec.yml)
[![Trivy](https://github.com/stolostron/capi-tests/actions/workflows/security-trivy.yml/badge.svg)](https://github.com/stolostron/capi-tests/actions/workflows/security-trivy.yml)
[![nancy](https://github.com/stolostron/capi-tests/actions/workflows/security-nancy.yml/badge.svg)](https://github.com/stolostron/capi-tests/actions/workflows/security-nancy.yml)

Go-based CAPI test suite supporting ARO (CAPZ/ASO) and ROSA (CAPA) deployment paths.

## Overview

This repository contains a Go-based CAPI test suite, currently supporting CAPZ/ARO and CAPA/ROSA paths. The tests verify the complete deployment workflow from prerequisite verification to final cluster validation.

The test suite is designed to work with the [cluster-api-installer](https://github.com/stolostron/cluster-api-installer).

## What This Tests

The test suite validates:
- **ARO (CAPZ/ASO)** - Azure Red Hat OpenShift via Cluster API Provider Azure and Azure Service Operator
- **ROSA (CAPA)** - Red Hat OpenShift on AWS via Cluster API Provider AWS

## Consumers

Target usage of this test suite will be:

- **OSCI (OpenShift CI)** - Automated continuous integration testing for OpenShift deployments
- **ACM (Advanced Cluster Management)** - Multi-cluster management and validation workflows
- **Manual Testing** - Developer and QA validation of CAPI deployments

## Prerequisites

### Required Tools

The following tools are required for running the test suite:

| Tool | Minimum Version | Tested Version | Purpose |
|------|----------------|----------------|---------|
| **Go** | 1.22 | 1.22+ | Running tests |
| **Docker** or **Podman** | 20.10+ | latest | Container runtime |
| **Kind** | 0.20.0 | 0.20.0 | Kubernetes in Docker for management cluster |
| **Azure CLI** (`az`) | 2.50.0 | latest | Azure authentication and management |
| **kubectl** | 1.28+ | latest | Kubernetes CLI |
| **OpenShift CLI** (`oc`) | 4.14+ | latest | OpenShift cluster interaction |
| **Helm** | 3.12+ | latest | Package manager for Kubernetes |
| **Git** | 2.30+ | latest | Source control |
| **envsubst** | - | latest | YAML templating (part of `gettext`) |
| **jq** | 1.6+ | latest | JSON processing (optional, for scripts) |

**Note**: The Go version is specified in `go.mod` and workflows automatically use this version.

To verify your tool versions:
```bash
go version
docker --version   # or: podman --version
kind version
az version
kubectl version --client
oc version --client
helm version
git --version
envsubst --version
jq --version
```

### Azure Access

- Azure account with appropriate permissions
- Access to Azure subscription for ARO deployment
- Authenticated via one of:
  - **Service principal** (recommended for CI): Set `AZURE_CLIENT_ID`, `AZURE_CLIENT_SECRET`, `AZURE_TENANT_ID`, and `AZURE_SUBSCRIPTION_ID`
  - **Azure CLI** (for development): Run `az login`

## Configuration

Tests are configured via environment variables:

### Repository Configuration

- `ARO_REPO_URL` - cluster-api-installer repository URL (default: `https://github.com/stolostron/cluster-api-installer`)
- `ARO_REPO_BRANCH` - Branch to use (default: `main`)
- `ARO_REPO_DIR` - Local repository directory (default: `/tmp/cluster-api-installer-aro`)

### Infrastructure Provider

- `INFRA_PROVIDER` - Infrastructure provider to use (values: `aro`, `rosa`; default: `aro`)

### AWS/ROSA Authentication

When using `INFRA_PROVIDER=rosa`, the following credentials are required:

- `AWS_ACCESS_KEY_ID` - AWS access key ID
- `AWS_SECRET_ACCESS_KEY` - AWS secret access key
- `AWS_REGION` - AWS region (default: `us-east-1`)
- `OCM_API_URL` - OpenShift Cluster Manager API URL
- `OCM_CLIENT_ID` - OCM OAuth client ID
- `OCM_CLIENT_SECRET` - OCM OAuth client secret

### Cluster Configuration

- `MANAGEMENT_CLUSTER_NAME` - Management cluster name (default: `capz-tests-stage` for ARO, `capa-tests-stage` for ROSA)
  - **Note**: Tests automatically translate this to `KIND_CLUSTER_NAME` for the deployment script
  - Use this variable for configuring tests; `KIND_CLUSTER_NAME` is set internally
- `WORKLOAD_CLUSTER_NAME` - Workload cluster name (default: `capz-tests` for ARO, `capa-tests` for ROSA). Keep short due to cloud provider length limits
- `CS_CLUSTER_NAME` - Cluster name prefix used for YAML generation (default: `${CAPI_USER}-${DEPLOYMENT_ENV}`). The Azure resource group will be named `${CS_CLUSTER_NAME}-resgroup`.
- `OCP_VERSION` - OpenShift version (default: `4.20`)
- `REGION` - Azure region (default: `uksouth`)
- `AZURE_SUBSCRIPTION_NAME` - Azure subscription ID
- `DEPLOYMENT_ENV` - Deployment environment identifier (default: `stage`)
- `CAPI_USER` - User identifier for domain prefix (default: `cate`)
- `WORKLOAD_CLUSTER_NAMESPACE` - Namespace for workload cluster resources. If set, uses the exact value provided (for resume scenarios). If not set, auto-generates a unique namespace per test run using `${WORKLOAD_CLUSTER_NAMESPACE_PREFIX}-${TIMESTAMP}` format.
- `WORKLOAD_CLUSTER_NAMESPACE_PREFIX` - Prefix for auto-generated namespace (default: provider-specific — `capz-test` for ARO, `capa-test` for ROSA). Only used when `WORKLOAD_CLUSTER_NAMESPACE` is not set.

#### Naming Requirements (RFC 1123)

The following variables must be **RFC 1123 compliant** to avoid deployment failures:
- `CAPI_USER`
- `CS_CLUSTER_NAME`
- `DEPLOYMENT_ENV`
- `WORKLOAD_CLUSTER_NAMESPACE`
- `WORKLOAD_CLUSTER_NAMESPACE_PREFIX`

**RFC 1123 naming rules:**
- Only lowercase alphanumeric characters and hyphens (`a-z`, `0-9`, `-`)
- Must start and end with an alphanumeric character
- No uppercase letters, underscores, dots, or spaces

**Example valid values:**
```bash
export CAPI_USER=cate        # Valid
export DEPLOYMENT_ENV=stage  # Valid
export CS_CLUSTER_NAME=cate-stage  # Valid
```

**Example invalid values:**
```bash
export CAPI_USER=RCap        # Invalid - contains uppercase
export DEPLOYMENT_ENV=Stage_1  # Invalid - contains uppercase and underscore
export CS_CLUSTER_NAME=-my-cluster  # Invalid - starts with hyphen
```

The test suite validates naming compliance during the Check Dependencies phase (phase 1), preventing late failures during CR deployment (phase 5).

### Cluster Mode

- `CLUSTER_MODE` - Management cluster deployment mode (values: `kind`, `mce`). When set:
  - `kind`: Automatically sets `USE_KIND=true` for local Kind cluster deployment
  - `mce`: Configures external MCE cluster mode — auto-creates kubeconfig, sets `USE_KUBECONFIG`, disables chart deployment by default

### Kind Mode

- `USE_KIND` - Enable Kind deployment mode (default: `false`). When set to `true`:
  - Creates a local Kind management cluster with CAPI/CAPZ/ASO controllers

### Test Behavior

- `DEPLOYMENT_TIMEOUT` - Control plane deployment timeout (default: `60m`). Use Go duration format: `1h`, `45m`, `90m`, etc.
- `DEPLOYMENT_STALL_TIMEOUT` - Stall detection timeout (default: `30m`). If the deployment makes no progress for this duration, the test fails early instead of waiting for the full timeout. Set to `0` to disable.
- `TEST_VERBOSITY` - Test output verbosity (default: `-v` for verbose). Set to empty string for quiet output: `TEST_VERBOSITY= make test`

#### Makefile Timeout Variables

Individual test phase timeouts can be overridden via Makefile variables:

| Variable | Default | Phase |
|----------|---------|-------|
| `CLUSTER_TIMEOUT` | `30m` | Management cluster deployment |
| `GENERATE_YAMLS_TIMEOUT` | `20m` | YAML generation |
| `DEPLOY_CRS_TIMEOUT` | `60m` | CR deployment and monitoring |
| `VERIFY_TIMEOUT` | `20m` | Workload cluster verification |
| `DELETION_TIMEOUT` | `60m` | Workload cluster deletion |

Example: `DEPLOY_CRS_TIMEOUT=90m make _deploy-crs`

## Getting Started

### Quick Start

1. **Install prerequisites**:
   ```bash
   # Check prerequisites
   make check-prereq
   ```

2. **Authenticate with Azure** (choose one):
   ```bash
   # Option A: Azure CLI (convenient for development)
   az login

   # Option B: Service Principal (recommended for CI/automation)
   export AZURE_CLIENT_ID=<your-client-id>
   export AZURE_CLIENT_SECRET=<your-client-secret>
   export AZURE_TENANT_ID=<your-tenant-id>
   export AZURE_SUBSCRIPTION_ID=<your-subscription-id>
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

#### Using External Cluster (MCE)

Instead of creating a local Kind cluster, you can run tests against an external Kubernetes cluster with pre-installed CAPI/CAPZ/ASO controllers:

```bash
# Extract kubeconfig from your cluster
oc login https://api.mce-cluster.example.com:6443
oc config view --raw > /tmp/mce-kubeconfig.yaml

# Run tests against external cluster
export USE_KUBECONFIG=/tmp/mce-kubeconfig.yaml
export AZURE_CLIENT_ID=<client-id>
export AZURE_CLIENT_SECRET=<client-secret>
export AZURE_TENANT_ID=<tenant-id>
export AZURE_SUBSCRIPTION_ID=<subscription-id>

make test-all
```

When `USE_KUBECONFIG` is set:
- Phase 02 (Setup) is skipped by default - no repository cloning needed if controllers are pre-installed
- Phase 03 (Cluster) validates pre-installed controllers instead of creating Kind cluster
- All other phases work normally using the external cluster

To deploy controllers to an external cluster, set `DEPLOY_CHARTS=true`:
```bash
export USE_KUBECONFIG=/path/to/kubeconfig
export DEPLOY_CHARTS=true
make test-all
```

This will:
- Clone the cluster-api-installer repository (Phase 02)
- Deploy CAPI and infrastructure provider charts to your external cluster (Phase 03)
- Continue with YAML generation and cluster deployment as normal

**Note**: All test targets automatically generate JUnit XML reports in a timestamped `results/` directory. The path to the results directory is displayed when tests run.

### All Make Targets

Run `make help` to see all available targets. Here's the complete reference:

#### Test Targets

| Target | Description |
|--------|-------------|
| `make test` | Run check dependencies tests only (fast, no Azure resources) |
| `make test-all` | Run all test phases sequentially |
| `make summary` | Generate test results summary from latest results |

#### Internal Test Phases

These targets are called by `make test-all` but can be run individually for debugging:

| Target | Description |
|--------|-------------|
| `make _check-dep` | Check software prerequisites needed for a proper test run |
| `make _setup` | Setup and prepare input repositories with helm charts and CRDs |
| `make _management_cluster` | Prepare cluster for testing and operators |
| `make _generate-yamls` | Generate script for resource creation (yaml) |
| `make _deploy-crs` | Deploy CRs and verify deployment |
| `make _verify-workload-cluster` | Verify deployed workload cluster |
| `make _delete-workload-cluster` | Delete workload cluster and verify deletion |
| `make _validate-cleanup` | Validate cleanup operations completed successfully |

#### Cleanup Targets

| Target | Description |
|--------|-------------|
| `make clean` | Interactive cleanup - prompts before deleting each resource |
| `make clean-all` | Delete ALL resources without prompting (local + Azure) |
| `FORCE=1 make clean` | Same as `make clean-all` |
| `make clean-azure` | Delete all Azure resources (RG, orphaned resources, AD apps, SPs) |

#### Setup and Prerequisites

| Target | Description |
|--------|-------------|
| `make check-prereq` | Check if required tools are installed |
| `make setup-submodule` | Add cluster-api-installer as a git submodule |
| `make update-submodule` | Update cluster-api-installer submodule |
| `make install-gotestsum` | Install gotestsum for test summaries |
| `make check-gotestsum` | Check if gotestsum is installed, install if missing |
| `make fix-docker-config` | Fix Docker credential helper configuration issues |

#### Development Targets

| Target | Description |
|--------|-------------|
| `make fmt` | Format Go code |
| `make lint` | Run linters |
| `make deps` | Download Go dependencies |

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
    ├── junit-deploy-apply.xml     # CR deployment test results (apply phase)
    ├── junit-deploy-monitor.xml   # CR deployment test results (monitor phase)
    ├── junit-verify.xml           # Verification test results
    ├── junit-delete.xml           # Deletion test results
    └── junit-cleanup.xml          # Cleanup validation test results
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
- **Azure resource group** (`${CS_CLUSTER_NAME}-resgroup`, e.g., `cate-stage-resgroup`)

This allows you to selectively clean up resources while preserving anything you want to keep.

For automated workflows (CI/CD, scripts) or quick full resets, use:
- `make clean-all` - deletes all resources without prompting (includes Azure resource group)
- `FORCE=1 make clean` - equivalent to `make clean-all`

**Azure Resource Cleanup**: The cleanup commands now include Azure resource group deletion:
- Uses synchronous `--yes` deletion (waits for completion before orphan cleanup)
- Gracefully skips if Azure CLI is not installed or not logged in
- Checks if resource group exists before attempting deletion
- The resource group name is derived from `${CAPI_USER}-${DEPLOYMENT_ENV}-resgroup` (default: `cate-stage-resgroup`)

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

See [CROSS_PLATFORM.md](docs/CROSS_PLATFORM.md) for cross-platform compatibility information including supported operating systems, shell compatibility, and platform-specific installation instructions.

## Test Structure

```
test/
├── 01_check_dependencies_test.go  # Tool and auth verification
├── 02_setup_test.go               # Repository setup
├── 03_cluster_test.go             # Management cluster deployment
├── 04_generate_yamls_test.go      # Resource generation
├── 05_deploy_crs_test.go          # Cluster provisioning monitoring
├── 06_verification_test.go        # Final cluster validation
├── 07_deletion_test.go            # Workload cluster deletion
├── 08_cleanup_test.go             # Cleanup validation
├── config.go                      # Configuration management
├── config_test.go                 # Configuration validation tests
├── helpers.go                     # Shared utilities
├── helpers_test.go                # Helper function tests
├── cluster_monitor_test.go        # Cluster monitoring utilities
├── start_test.go                  # Test suite entry point
└── README.md                      # Detailed test documentation
```

For detailed test documentation, see [test/README.md](test/README.md).

## CI/CD Integration

The test suite integrates with GitHub Actions:

**Test Workflows:**
- **Management Cluster (ARO)** - ARO management cluster deployment tests
- **Management Cluster (ROSA)** - ROSA management cluster deployment tests
- **Full Cluster Deployment (ARO)** - Complete ARO workload cluster lifecycle
- **Full Cluster Deployment (ROSA)** - Complete ROSA workload cluster lifecycle
- **Check Dependencies** - Dependency checks on every push
- **Test Setup** - Repository setup validation
- **Test Kind Cluster** - Kind cluster deployment tests

**Security Scanning:**
- **govulncheck** - Go vulnerability scanning
- **gosec** - Go security analysis
- **Trivy** - Container and dependency vulnerability scanning
- **nancy** - Go dependency vulnerability auditing

**Required Repository Secrets:**
- `QUAY_AUTH` - Base64-encoded quay.io credentials for pulling container images (required by all cluster workflows)

**GitHub Actions Environment: `aro-stage`** (used by ARO workflows):

| Type | Name | Purpose |
|------|------|---------|
| Variable | `AZURE_TENANT_ID` | Azure tenant ID |
| Variable | `AZURE_SUBSCRIPTION_ID` | Azure subscription ID |
| Variable | `AZURE_CLIENT_ID` | Service principal client ID |
| Variable | `REGION` | Azure region (e.g., `uksouth`) |
| Variable | `DEPLOYMENT_ENV` | Environment identifier (e.g., `stage`) |
| Secret | `AZURE_CLIENT_SECRET` | Service principal secret |
| Variable | `MCE_API_URL` | MCE cluster API endpoint (optional, for MCE mode) |
| Variable | `MCE_API_USER` | MCE cluster username (optional, default: `kubeadmin`) |
| Secret | `MCE_API_PASSWORD` | MCE cluster password (optional, for MCE mode) |

**GitHub Actions Environment: `rosa-stage`** (used by ROSA workflows):

| Type | Name | Purpose |
|------|------|---------|
| Variable | `AWS_REGION` | AWS region (e.g., `us-east-1`) |
| Variable | `OCM_API_URL` | OpenShift Cluster Manager API URL |
| Variable | `OCM_CLIENT_ID` | OCM OAuth client ID |
| Variable | `DEPLOYMENT_ENV` | Environment identifier (e.g., `stage`) |
| Secret | `AWS_ACCESS_KEY_ID` | AWS access key ID |
| Secret | `AWS_SECRET_ACCESS_KEY` | AWS secret access key |
| Secret | `OCM_CLIENT_SECRET` | OCM OAuth client secret |
| Variable | `MCE_API_URL` | MCE cluster API endpoint (optional, for MCE mode) |
| Variable | `MCE_API_USER` | MCE cluster username (optional, default: `kubeadmin`) |
| Secret | `MCE_API_PASSWORD` | MCE cluster password (optional, for MCE mode) |

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines on:

- Development setup and prerequisites
- Running tests locally
- Branch naming and commit conventions
- Pull request process

Quick start:
```bash
make check-prereq  # Verify prerequisites
make test          # Run fast tests
make fmt           # Format code
```

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.
