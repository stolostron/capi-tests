# Test Coverage Documentation

This document provides comprehensive information about the test coverage for the ARO-CAPZ deployment test suite.

## Overview

The test suite provides end-to-end coverage of the Azure Red Hat OpenShift (ARO) deployment process using Cluster API Provider Azure (CAPZ) and Azure Service Operator (ASO). The tests validate each step of the deployment workflow as documented in the [cluster-api-installer ARO-CAPZ documentation](https://github.com/RadekCap/cluster-api-installer/blob/ARO-ASO/doc/ARO-capz.md).

## Test Structure

### Test Files and Coverage

#### 1. Check Dependencies Verification (`test/01_check_dependencies_test.go`)

**Purpose**: Validates that all required tools and authentication are in place before attempting deployment.

**Coverage**:
- ✓ Docker/Podman availability check
- ✓ Kind (Kubernetes in Docker) installation verification
- ✓ Azure CLI (`az`) presence and version
- ✓ OpenShift CLI (`oc`) availability
- ✓ Helm installation check
- ✓ Git version verification
- ✓ kubectl availability
- ✓ Azure CLI authentication status
- ✓ Tool version compatibility

**Key Test Cases**:
- `TestCheckDependencies_ToolAvailable` - Verifies all CLI tools are installed
- `TestCheckDependencies_AzureCLILogin_IsLoggedIn` - Validates Azure authentication
- `TestCheckDependencies_DockerCredentialHelper` - Checks Docker credential helper configuration

**Why This Matters**: Catching missing tools early prevents failures deep into the deployment process.

---

#### 2. Repository Setup (`test/02_setup_test.go`)

**Purpose**: Ensures the cluster-api-installer repository is properly cloned and configured.

**Coverage**:
- ✓ Repository cloning from specified branch
- ✓ Directory structure validation
- ✓ Essential scripts presence verification
- ✓ Script permissions configuration
- ✓ Documentation availability check

**Key Test Cases**:
- `TestSetup_CloneRepository` - Clones cluster-api-installer repository
- `TestSetup_VerifyStructure` - Validates repository structure
- `TestSetup_CheckScripts` - Ensures required scripts exist and are executable

**Integration Points**: Supports multiple integration approaches (submodule, dynamic clone, vendored scripts).

---

#### 3. Kind Cluster Deployment (`test/03_cluster_test.go`)

**Purpose**: Deploys and validates the management cluster that will orchestrate ARO deployment.

**Coverage**:
- ✓ Kind cluster creation
- ✓ Cluster accessibility verification
- ✓ CAPI (Cluster API) components installation
- ✓ CAPZ (Cluster API Provider Azure) deployment
- ✓ ASO (Azure Service Operator) installation
- ✓ Component health checks
- ✓ API server readiness

**Key Test Cases**:
- `TestKindCluster_Deploy` - Creates Kind cluster with CAPZ
- `TestKindCluster_VerifyAccess` - Validates cluster connectivity
- `TestKindCluster_CheckCAPIComponents` - Verifies CAPI installation

**Resources Created**:
- Kind cluster (default: `capz-tests-stage`)
- CAPI controllers
- CAPZ provider
- Azure Service Operator

---

#### 4. Infrastructure Generation (`test/04_generate_yamls_test.go`)

**Purpose**: Generates and applies ARO infrastructure resources to the management cluster.

**Coverage**:
- ✓ Infrastructure resource generation via script
- ✓ YAML file validation
- ✓ Credentials secret generation
- ✓ Infrastructure secrets creation
- ✓ Cluster configuration generation
- ✓ Resource application to management cluster
- ✓ Resource status verification

**Key Test Cases**:
- `TestInfrastructure_GenerateResources` - Runs generation script
- `TestInfrastructure_VerifyGeneratedFiles` - Validates YAML outputs
- `TestInfrastructure_ApplyResources` - Applies resources to cluster

**Generated Resources**:
- Azure credentials secrets
- Infrastructure configuration
- Cluster manifests
- Network configuration
- Identity and access resources

---

#### 5. Deployment Monitoring (`test/05_deploy_crds_test.go`)

**Purpose**: Monitors the ARO cluster deployment progress and validates successful provisioning.

**Coverage**:
- ✓ Cluster resource status monitoring
- ✓ Control plane readiness checks
- ✓ Infrastructure provisioning validation
- ✓ Cluster condition monitoring
- ✓ Deployment timeout handling
- ✓ Error detection and reporting

**Key Test Cases**:
- `TestDeployment_MonitorCluster` - Tracks deployment using clusterctl
- `TestDeployment_WaitForControlPlane` - Waits for control plane ready
- `TestDeployment_CheckClusterConditions` - Validates cluster health

**Monitoring Includes**:
- InfrastructureReady condition
- ControlPlaneReady condition
- Cluster provisioning state
- Node readiness
- Component health

**Timeout**: Configurable, default 30 minutes for control plane

---

#### 6. Cluster Verification (`test/06_verification_test.go`)

**Purpose**: Performs comprehensive validation of the deployed ARO cluster.

**Coverage**:
- ✓ Kubeconfig retrieval
- ✓ Cluster API connectivity
- ✓ Node count and status verification
- ✓ OpenShift version validation
- ✓ Cluster operator health checks
- ✓ Console accessibility
- ✓ API endpoint validation
- ✓ Authentication verification

**Key Test Cases**:
- `TestVerification_GetKubeconfig` - Retrieves cluster credentials
- `TestVerification_VerifyNodes` - Checks node health
- `TestVerification_CheckOpenShiftVersion` - Validates OCP version
- `TestVerification_CheckClusterOperators` - Verifies operator status

**Validation Points**:
- All nodes in Ready state
- Expected OpenShift version deployed
- All cluster operators available and not degraded
- API server responsive
- Authentication configured correctly

---

### Supporting Infrastructure

#### Configuration Management (`test/config.go`)

**Purpose**: Centralized configuration with environment variable support.

**Features**:
- Environment variable-based configuration
- Sensible defaults for all settings
- Support for multiple environments (stage, prod)
- Path configuration for scripts and tools
- Azure subscription management

**Configurable Parameters**:
- Repository settings (URL, branch, directory)
- Cluster settings (name, region, version)
- Azure settings (subscription, resource group)
- Tool paths (clusterctl, scripts)

---

#### Helper Functions (`test/helpers.go`)

**Purpose**: Shared utilities used across all test files.

**Utilities Provided**:
- `CommandExists()` - Check tool availability
- `RunCommand()` - Execute shell commands with output capture
- `SetEnvVar()` - Manage environment variables in tests
- `FileExists()` / `DirExists()` - Filesystem checks
- `GetEnvOrDefault()` - Configuration with fallbacks

---

## Test Execution Modes

### Full Test Suite

Runs complete end-to-end deployment and verification.

```bash
make test
```

**Duration**: 30-60 minutes (depending on Azure provisioning time)
**Requirements**: Full Azure credentials and permissions

---

### Quick Validation

Check dependencies validation without deployment tests.

```bash
go test -v ./test -run TestCheckDependencies
```

**Duration**: < 2 minutes
**Requirements**: Local tools and Azure CLI login
**Coverage**: Prerequisites validation only

**Note**: Since `make test-short` was removed, use the Go test command above to run only prerequisite tests. The `make test` target runs the complete test suite.

---

### Phase-Specific Testing

Run individual test phases for targeted validation.

```bash
# Check dependencies only (fast, no Azure resources)
make test

# Run all test phases sequentially
make test-all

# Run specific test phase using Go test directly
go test -v ./test -run TestSetup
go test -v ./test -run TestKindCluster
go test -v ./test -run TestInfrastructure
go test -v ./test -run TestDeployment
go test -v ./test -run TestVerification
```

---

## Coverage Metrics

### Test Phase Coverage

| Phase | Test Files | Test Cases | Lines of Code | Coverage |
|-------|-----------|------------|---------------|----------|
| Check Dependencies | 1 | 6 | 80 | Tool validation, auth checks |
| Setup | 1 | 3 | 116 | Repository structure, scripts |
| Kind Cluster | 1 | 3 | 142 | Cluster deployment, CAPI |
| Infrastructure | 1 | 3 | 163 | Resource generation, YAML validation |
| Deployment | 1 | 3 | 138 | Monitoring, health checks |
| Verification | 1 | 6 | 209 | Cluster validation, operators |
| **Total** | **6** | **21** | **848** | **End-to-end workflow** |

### Deployment Workflow Coverage

The test suite covers 100% of the documented ARO-CAPZ deployment workflow:

- ✅ Check dependencies verification
- ✅ Repository setup
- ✅ Management cluster deployment
- ✅ CAPI components installation
- ✅ Infrastructure resource generation
- ✅ Resource application
- ✅ Cluster provisioning
- ✅ Deployment monitoring
- ✅ Cluster verification
- ✅ Health validation

---

## Integration Testing

### Integration Approaches Tested

1. **Git Submodule Integration**
   - Repository as submodule
   - Version pinning
   - Update workflows

2. **Dynamic Clone Integration**
   - Runtime repository cloning
   - Branch specification
   - Automatic cleanup

3. **Vendored Scripts Integration**
   - Offline operation
   - No external dependencies
   - Manual sync validation

All three approaches are documented and tested in `docs/INTEGRATION.md`.

---

## CI/CD Coverage

### GitHub Actions Integration

Two workflow files provide automated testing:

1. **`check-dependencies.yml`**
   - Runs on all pushes and PRs
   - Validates dependency checks
   - Quick feedback (< 2 minutes)

2. **`test.yml`**
   - Full test suite execution
   - Can be triggered manually
   - Requires Azure credentials as secrets

**Coverage**: CI/CD pipeline validates dependencies on every commit.

---

## Test Features

### Idempotency

All tests are designed to be idempotent:
- Re-running tests skips already-completed steps
- Safe to run multiple times
- Helpful for debugging and development

### Error Handling

Comprehensive error handling:
- Clear error messages
- Context-aware failures
- Helpful troubleshooting guidance
- Graceful degradation where appropriate

### Configurability

Every aspect is configurable via environment variables:
- Cluster names and regions
- Repository locations
- Tool paths
- Timeout values
- Environment (stage/prod)

### Logging

Detailed logging throughout:
- Step-by-step progress
- Command outputs
- Status updates
- Error context

---

## Coverage Gaps and Future Enhancements

### Current Gaps

1. **Network Testing**: No specific network policy or connectivity tests
2. **Scaling Tests**: No validation of cluster scaling operations
3. **Upgrade Tests**: No testing of cluster upgrades
4. **Backup/Restore**: No disaster recovery testing
5. **Performance Tests**: No load or performance validation
6. **Security Scanning**: No security posture validation

### Planned Enhancements

1. **Additional Test Scenarios**
   - Cluster scaling (up and down)
   - Version upgrades
   - Node replacement
   - Disaster recovery

2. **Performance Testing**
   - Cluster provisioning time benchmarks
   - Resource utilization monitoring
   - API response time validation

3. **Security Testing**
   - RBAC validation
   - Network policy enforcement
   - Secret management verification
   - Compliance checking

4. **Multi-Environment Testing**
   - Test across multiple Azure regions
   - Validate different OpenShift versions
   - Test various cluster sizes

---

## Running the Test Suite

### Prerequisites

Ensure all required tools are installed:
```bash
make check-prereq
```

### Full Test Run

```bash
# Set configuration
export CLUSTER_NAME=my-test-cluster
export REGION=uksouth
export AZURE_SUBSCRIPTION_NAME=your-subscription-id

# Run all tests
make test
```

### Cleanup

After testing, clean up resources:
```bash
make clean
```

---

## Troubleshooting Test Failures

### Check Dependencies Failures

**Symptom**: Tests fail during dependency checks
**Solution**: Install missing tools or update versions
```bash
make check-prereq
```

### Setup Failures

**Symptom**: Repository clone or structure validation fails
**Solution**: Check network connectivity and repository access
```bash
git clone -b ARO-ASO https://github.com/RadekCap/cluster-api-installer.git /tmp/cluster-api-installer-aro
```

### Kind Cluster Failures

**Symptom**: Kind cluster deployment fails
**Solution**: Check Docker/Podman is running
```bash
docker info  # or podman info
kind get clusters
```

### Infrastructure Failures

**Symptom**: Resource generation or application fails
**Solution**: Verify Azure credentials and permissions
```bash
az login
az account show
```

### Deployment Timeout

**Symptom**: Tests timeout waiting for cluster
**Solution**: Check Azure portal for resource status, increase timeout
```bash
# Increase timeout in 05_deploy_crds_test.go
timeout := 60 * time.Minute  # default is 30m
```

---

## Documentation Coverage

### Comprehensive Documentation

1. **Main README** (`README.md`)
   - Framework overview
   - Getting started guide
   - Check Dependencies

2. **Test README** (`test/README.md`)
   - Detailed test documentation
   - Configuration guide
   - Usage examples

3. **Integration Guide** (`docs/INTEGRATION.md`)
   - Integration approaches
   - Setup instructions
   - Best practices

4. **Test Coverage** (`TEST_COVERAGE.md` - this document)
   - Test structure
   - Coverage details
   - Troubleshooting

5. **Makefile**
   - Self-documenting targets
   - Usage help

**Total Documentation**: ~1,200 lines covering all aspects of the test suite

---

## Continuous Improvement

The test suite is designed to evolve with the ARO-CAPZ deployment process:

1. **Test Additions**: New tests added as features are developed
2. **Coverage Expansion**: Increasing coverage of edge cases and scenarios
3. **Documentation Updates**: Keeping documentation in sync with tests
4. **Performance Optimization**: Improving test execution time
5. **Feedback Integration**: Incorporating user feedback and real-world usage

---

## Summary

This test suite provides comprehensive coverage of the ARO-CAPZ deployment workflow:

- **6 test files** covering all deployment phases
- **21 test cases** validating each step
- **Idempotent execution** for safe re-runs
- **Configurable** via environment variables
- **Well-documented** with 4 documentation files
- **CI/CD ready** with GitHub Actions workflows
- **Multiple integration options** for different use cases

The test suite ensures reliable, repeatable ARO deployments and serves as both validation and documentation of the deployment process.
