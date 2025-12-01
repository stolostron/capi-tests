# ARO-CAPZ Test Suite

Comprehensive test suite for Azure Red Hat OpenShift (ARO) deployment using Cluster API Provider Azure (CAPZ) and Azure Service Operator (ASO).

## Overview

This repository contains a Go-based test suite that validates the complete ARO cluster deployment workflow on Azure using CAPZ. The tests verify each step of the deployment process, from prerequisite verification to final cluster validation.

The test suite is designed to work with the [cluster-api-installer](https://github.com/RadekCap/cluster-api-installer) ARO-CAPZ implementation.

## What This Tests

The test suite validates:
- **CAPZ on Azure** - Cluster API Provider Azure for deploying Kubernetes infrastructure on Azure
- **ARO Deployment** - Azure Red Hat OpenShift cluster provisioning
- **ASO Integration** - Azure Service Operator for managing Azure resources

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

- `KIND_CLUSTER_NAME` - Management cluster name (default: `capz-stage`)
- `CLUSTER_NAME` - ARO cluster name (default: `test-cluster`)
- `RESOURCE_GROUP` - Azure resource group
- `OPENSHIFT_VERSION` - OpenShift version (default: `4.18`)
- `REGION` - Azure region (default: `uksouth`)
- `AZURE_SUBSCRIPTION_NAME` - Azure subscription ID
- `ENV` - Environment identifier (default: `stage`)
- `USER` - User identifier

### Test Behavior

- `DEPLOYMENT_TIMEOUT` - Control plane deployment timeout (default: `30m`). Use Go duration format: `1h`, `45m`, `90m`, etc.

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

3. **Run prerequisite tests**:
   ```bash
   make test-prereq
   ```

4. **Run full test suite**:
   ```bash
   make test-all
   ```

### Running Tests

#### Using Makefile

```bash
# Run prerequisite tests only (fast, no Azure resources created)
make test

# Run full test suite (all phases sequentially)
make test-all

# Run specific test phase
make test-setup       # Repository setup
make test-kind        # Kind cluster deployment
make test-infra       # Infrastructure generation
make test-deploy      # Deployment monitoring
make test-verify      # Cluster verification

# Run quick tests (skip long-running operations)
make test-short
```

#### Using Go Test Directly

```bash
# Run all tests
go test -v ./test -timeout 60m

# Run specific test phase
go test -v ./test -run TestPrerequisites
go test -v ./test -run TestInfrastructure

# Run with custom configuration
ENV=prod \
CLUSTER_NAME=my-aro-cluster \
REGION=westus2 \
go test -v ./test -timeout 60m
```

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
├── 01_prerequisites_test.go   # Tool and auth verification
├── 02_setup_test.go           # Repository setup
├── 03_kind_cluster_test.go    # Management cluster deployment
├── infrastructure_test.go     # Resource generation
├── deployment_test.go         # Cluster provisioning monitoring
├── verification_test.go       # Final cluster validation
├── config.go                  # Configuration management
├── helpers.go                 # Shared utilities
└── README.md               # Detailed test documentation
```

For detailed test documentation, see [test/README.md](test/README.md).

## CI/CD Integration

The test suite integrates with GitHub Actions for continuous testing:

- **Prerequisites Workflow** - Runs prerequisite checks on every push
- **Full Test Workflow** - Can be triggered manually for complete validation

## Contributing

Contributions are welcome! Please ensure:

1. All tests pass before submitting PRs
2. New functionality includes appropriate tests
3. Documentation is updated to reflect changes

## License

[License information to be added]
