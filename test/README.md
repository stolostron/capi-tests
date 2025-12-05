# ARO-CAPZ Test Suite

Comprehensive test suite for Azure Red Hat OpenShift (ARO) deployment using Cluster API Provider Azure (CAPZ) and Azure Service Operator (ASO).

## Overview

This test suite validates each step of the ARO deployment process as documented in the [cluster-api-installer ARO-CAPZ documentation](https://github.com/RadekCap/cluster-api-installer/blob/ARO-ASO/doc/ARO-capz.md).

## Test Structure

### Test Files

1. **`01_check_dependencies_test.go`** - Verifies required tools and authentication
   - Checks for required CLI tools (docker/podman, kind, az, oc, helm, git)
   - Validates Azure CLI login status
   - Verifies tool versions

2. **`02_setup_test.go`** - Repository setup and preparation
   - Clones cluster-api-installer repository
   - Verifies repository structure
   - Sets script permissions

3. **`03_cluster_test.go`** - Kind cluster deployment
   - Deploys Kind cluster with CAPZ components
   - Verifies cluster accessibility
   - Checks CAPI components installation

4. **`04_generate_yamls_test.go`** - Infrastructure resource generation
   - Generates ARO infrastructure resources
   - Validates generated YAML files
   - Applies resources to the management cluster

5. **`05_deploy_crds_test.go`** - Cluster deployment monitoring
   - Monitors ARO cluster deployment
   - Waits for control plane readiness
   - Checks cluster conditions

6. **`06_verification_test.go`** - Cluster verification
   - Retrieves cluster kubeconfig
   - Verifies cluster nodes
   - Checks OpenShift version and operators
   - Performs health checks

### Helper Files

- **`config.go`** - Test configuration management
- **`helpers.go`** - Shared utility functions

## Prerequisites

### Required Tools

- Docker or Podman
- Kind (Kubernetes in Docker)
- Azure CLI (`az`)
- OpenShift CLI (`oc`)
- Helm
- Git
- kubectl
- clusterctl (optional, can be provided by cluster-api-installer)

### Azure Access

- Red Hat account with access to Azure tenant
- Access to "ARO Hosted Control Planes" subscription
- Authenticated via `az login`

## Configuration

Tests are configured via environment variables:

### Repository Configuration

- `ARO_REPO_URL` - Repository URL (default: `https://github.com/RadekCap/cluster-api-installer.git`)
- `ARO_REPO_BRANCH` - Branch to clone (default: `ARO-ASO`)
- `ARO_REPO_DIR` - Local repository directory (default: `/tmp/cluster-api-installer-aro`)

### Cluster Configuration

- `KIND_CLUSTER_NAME` - Kind cluster name (default: `capz-tests-stage`)
- `CLUSTER_NAME` - ARO cluster name (default: `capz-tests-cluster`)
- `RESOURCE_GROUP` - Azure resource group
- `OPENSHIFT_VERSION` - OpenShift version (default: `4.18`)
- `REGION` - Azure region (default: `uksouth`)
- `AZURE_SUBSCRIPTION_NAME` - Azure subscription ID
- `ENV` - Environment (stage/prod) (default: `stage`)
- `USER` - User identifier

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
ENV=prod \
CLUSTER_NAME=my-aro-cluster \
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

The test suite is designed to be idempotent - you can re-run tests and they will skip steps that are already complete.

## Integration with cluster-api-installer

### Option 1: Git Submodule (Recommended)

Add cluster-api-installer as a git submodule:

```bash
git submodule add -b ARO-ASO https://github.com/RadekCap/cluster-api-installer.git vendor/cluster-api-installer
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
# Delete Kind cluster
kind delete cluster --name capz-tests-stage

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
2. Azure credentials are configured as secrets
3. Tests run with appropriate timeout values (deployments can take 30+ minutes)

Example for GitHub Actions is provided in `.github/workflows/test.yml`.
