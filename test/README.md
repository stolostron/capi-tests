# CAPI Test Suite

Go-based CAPI test suite, currently supporting CAPZ/ARO and CAPA/ROSA paths. The tests verify the complete deployment workflow from prerequisites to cluster verification.

## Overview

This test suite validates each step of the deployment process using the [cluster-api-installer](https://github.com/stolostron/cluster-api-installer).

## Test Structure

### Test Files

1. **`01_check_dependencies_test.go`** - Verifies required tools and authentication
   - Checks for required CLI tools (docker/podman, kind, az/aws, oc, helm, git)
   - Validates cloud provider authentication
   - Verifies tool versions

2. **`02_setup_test.go`** - Repository setup and preparation
   - Clones cluster-api-installer repository
   - Verifies repository structure
   - Sets script permissions

3. **`03_cluster_test.go`** - Kind cluster deployment
   - Deploys Kind cluster with CAPI and infrastructure provider components
   - Verifies cluster accessibility
   - Checks CAPI components installation

4. **`04_generate_yamls_test.go`** - Infrastructure resource generation
   - Generates provider-specific infrastructure resources (ARO/ROSA)
   - Validates generated YAML files
   - Applies resources to the management cluster

5. **`05_deploy_crs_test.go`** - Cluster deployment monitoring
   - Monitors workload cluster deployment via JSON monitor
   - Waits for control plane readiness
   - Checks cluster conditions

6. **`06_verification_test.go`** - Cluster verification
   - Retrieves cluster kubeconfig
   - Verifies cluster nodes
   - Checks OpenShift version and operators
   - Performs health checks

7. **`07_deletion_test.go`** - Cluster deletion
   - Deletes workload cluster from management cluster
   - Waits for cluster deletion to complete
   - Verifies cloud resources are cleaned up

8. **`08_cleanup_test.go`** - Cleanup validation
   - Validates local resource cleanup (Kind cluster, kubeconfig, repositories)
   - Validates cloud resource cleanup (resource groups, orphaned resources)
   - Tests cleanup modes (interactive, force, dry-run)

### Helper Files

- **`config.go`** - Test configuration management
- **`helpers.go`** - Shared utility functions
- **`cluster_monitor.go`** - Cluster monitoring via JSON monitor script
- **`cluster_monitor_test.go`** - Tests for cluster monitoring functions

## Prerequisites

### Required Tools

- Docker or Podman
- Kind (Kubernetes in Docker)
- Azure CLI (`az`) for ARO, or AWS CLI (`aws`) for ROSA
- OpenShift CLI (`oc`)
- Helm
- Git
- kubectl
- clusterctl (optional, can be provided by cluster-api-installer)

### Cloud Access

- For ARO: Azure account with appropriate subscription and permissions
- For ROSA: AWS account with appropriate IAM permissions and OCM credentials
- Authenticated via `az login` (ARO) or `aws configure` (ROSA)

## Configuration

Tests are configured via environment variables:

### Repository Configuration

- `ARO_REPO_URL` - Repository URL (default: `https://github.com/stolostron/cluster-api-installer`)
- `ARO_REPO_BRANCH` - Branch to clone (default: `main`)
- `ARO_REPO_DIR` - Local repository directory (default: `/tmp/cluster-api-installer-aro`)

### Infrastructure Provider

- `INFRA_PROVIDER` - Infrastructure provider to use (values: `aro`, `rosa`; default: `aro`)

### Cluster Configuration

- `MANAGEMENT_CLUSTER_NAME` - Management cluster name (default: `capz-tests-stage` for ARO, `capa-tests-stage` for ROSA)
  - **Note**: Tests automatically translate this to `KIND_CLUSTER_NAME` for the deployment script
  - Use this variable for configuring tests; `KIND_CLUSTER_NAME` is set internally
- `WORKLOAD_CLUSTER_NAME` - Workload cluster name (default: `capz-tests` for ARO, `capa-tests` for ROSA)
- `CS_CLUSTER_NAME` - Cluster name prefix used for YAML generation (default: `${CAPI_USER}-${random5hex}`). The Azure resource group is named `${WORKLOAD_CLUSTER_NAME}-resgroup`.
- `OCP_VERSION` - OpenShift version (default: `4.19`)
- `REGION` - Azure region (default: `uksouth`)
- `AZURE_SUBSCRIPTION_NAME` - Azure subscription ID
- `DEPLOYMENT_ENV` - Deployment environment (stage/prod) (default: `stage`)
- `CAPI_USER` - User identifier for domain prefix (default: `cate`)
- `WORKLOAD_CLUSTER_NAMESPACE` - Namespace for workload cluster resources (auto-generated if not set)
- `WORKLOAD_CLUSTER_NAMESPACE_PREFIX` - Prefix for auto-generated namespace (default: provider-specific — `capz-test` for ARO, `capa-test` for ROSA)

## Running Tests

### Run All Tests

```bash
go test -v ./test
```

### Run Specific Test Phase

```bash
# Check dependencies only
go test -v ./test -run TestCheckDependencies

# Setup only
go test -v ./test -run TestSetup

# Infrastructure generation
go test -v ./test -run TestInfrastructure

# Verification
go test -v ./test -run TestVerification
```

### Run with Custom Configuration

```bash
DEPLOYMENT_ENV=prod \
WORKLOAD_CLUSTER_NAME=my-cluster \
REGION=westus2 \
AZURE_SUBSCRIPTION_NAME=your-subscription-id \
go test -v ./test
```

### Quick Test (Skip Long-Running Tests)

```bash
go test -v -short ./test
```

## Test Execution Order

For full deployment validation, tests should run in this order:

1. Check dependencies verification
2. Repository setup
3. Kind cluster deployment
4. Infrastructure generation
5. Resource application
6. Deployment monitoring
7. Cluster verification
8. Cluster deletion
9. Cleanup validation

The test suite is designed to be idempotent - you can re-run tests and they will skip steps that are already complete.

## Cluster Monitoring

The test suite includes a JSON-based cluster monitoring system for tracking deployment progress.

### Shell Script

`scripts/monitor-cluster-json.sh` produces a structured JSON snapshot of cluster state:

```bash
# Usage
./scripts/monitor-cluster-json.sh [--context <context>] <namespace> <cluster-name>

# Example
./scripts/monitor-cluster-json.sh --context kind-capz-tests-stage capz-test-20260203 capz-tests
```

The output includes cluster phase, infrastructure readiness, control plane status, machine pool replicas, node status, and a summary with overall readiness.

### Go Integration

`cluster_monitor.go` provides Go functions that wrap the shell script:

- **`MonitorCluster()`** - Takes a single monitoring snapshot and returns structured `ClusterMonitorData`
- **`MonitorClusterUntilReady()`** - Polls the cluster at regular intervals until it reaches a ready state or the timeout expires
- **`MonitorClusterUntilDeleted()`** - Polls until the cluster resources are fully deleted or the timeout expires

These functions are used by Phase 05 (deployment monitoring) and Phase 07 (deletion monitoring) to track workload cluster lifecycle.

## Integration with cluster-api-installer

### Option 1: Git Submodule (Recommended)

Add cluster-api-installer as a git submodule:

```bash
git submodule add https://github.com/stolostron/cluster-api-installer.git vendor/cluster-api-installer
git submodule update --init --recursive
```

Set `ARO_REPO_DIR` to point to the submodule:

```bash
export ARO_REPO_DIR="$(pwd)/vendor/cluster-api-installer"
```

### Option 2: Clone at Test Time

Let the tests clone the repository automatically to `/tmp/cluster-api-installer-aro`:

```bash
# Tests will handle cloning
go test -v ./test
```

### Option 3: Use Existing Clone

If you already have cluster-api-installer cloned:

```bash
export ARO_REPO_DIR="/path/to/your/cluster-api-installer"
go test -v ./test
```

## Cleanup

To clean up test resources:

```bash
# Delete Kind cluster (provider-specific names)
kind delete cluster --name capz-tests-stage  # ARO
kind delete cluster --name capa-tests-stage  # ROSA

# Remove repository clone (if using temp directory)
rm -rf /tmp/cluster-api-installer-aro

# Clean up generated resources
rm -rf /tmp/*-kubeconfig.yaml
```

## Troubleshooting

### Tests Failing Due to Missing Tools

Ensure all prerequisites are installed. Run:

```bash
go test -v ./test -run TestCheckDependencies
```

### Azure Authentication Issues

Ensure you're logged in with the correct account:

```bash
az login
az account show
```

### Kind Cluster Issues

Check Kind cluster status:

```bash
kind get clusters
kubectl cluster-info --context kind-capz-tests-stage
```

### Resource Application Failures

Verify the management cluster is accessible and CAPI components are running:

```bash
kubectl get pods -A --context kind-capz-tests-stage
```

## CI/CD Integration

These tests can be integrated into GitHub Actions or other CI/CD systems. Ensure:

1. Required tools are installed in the CI environment
2. Cloud provider credentials are configured as secrets
3. Tests run with appropriate timeout values (deployments can take 30+ minutes)

Example for GitHub Actions is provided in `.github/workflows/test.yml`.
