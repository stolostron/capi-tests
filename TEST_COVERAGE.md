# Test Coverage Documentation

This document provides comprehensive information about the test coverage for the ARO-CAPZ deployment test suite.

## Review History

| Version | Date | Issue | Notes |
|---------|------|-------|-------|
| V1 | 2026-01-23 | #393 | Initial coverage documentation |
| V1.1 | 2026-02-08 | ACM-29883 | Updated for external cluster mode, MCE, namespace generation |

## Overview

The test suite provides end-to-end coverage of the Azure Red Hat OpenShift (ARO) deployment process using Cluster API Provider Azure (CAPZ) and Azure Service Operator (ASO). The tests validate each step of the deployment workflow.

### Coverage Metrics Summary

| Metric | Value |
|--------|-------|
| Production code | 3,237 lines (helpers.go + config.go) |
| Test code | 7,852 lines (10 test files + 2 unit test files) |
| Test-to-production ratio | 2.4:1 |
| Unit test functions | 58 (helpers_test.go: 52, config_test.go: 6) |
| Integration test functions | 76 (across 10 phase files) |
| Total test functions | 134 |
| Unit test coverage (statements) | 32.1% |
| gosec issues | 0 (7 #nosec annotations) |

**Note on unit test coverage**: The 32.1% statement coverage reflects that many helper functions (e.g., `RunCommand`, `SetEnvVar`, MCE functions) require a live Kubernetes cluster or Azure environment to execute. These are covered by integration tests during `make test-all` runs but cannot be exercised in isolated unit tests.

---

## Test Structure

### Unit Tests

Unit tests validate pure logic functions that don't require external dependencies.

#### helpers_test.go (52 tests, 3,348 lines)

Functions with unit test coverage:

| Function | Coverage | Test Count | Notes |
|----------|----------|------------|-------|
| `IsKubectlApplySuccess` | via integration | 1 | Table-driven with success/failure cases |
| `ExtractClusterNameFromYAML` | 88.9% | 1 | Table-driven, multi-document YAML |
| `CheckYAMLConfigMatch` | via integration | 1 | **V1.1** - Namespace mismatch detection |
| `ValidateYAMLFile` | 92.9% | 1 | YAML syntax and structure validation |
| `ExtractNamespaceFromYAML` | 100% | 1 | **V1.1** - Namespace extraction from YAML |
| `DeploymentState_Namespace` | via integration | 1 | **V1.1** - State file read/write/delete cycle |
| `FormatAROControlPlaneConditions` | 100% | 1 | Condition formatting |
| `isWaitingCondition` | 100% | 1 | Waiting condition detection |
| `GetDomainPrefix` | 100% | 1 | Domain prefix generation |
| `ValidateDomainPrefix` | 100% | 2 | RFC compliance, max length |
| `ValidateRFC1123Name` | 100% | 2 | RFC 1123 compliance |
| `GetExternalAuthID` | 100% | 1 | External auth ID generation |
| `ValidateExternalAuthID` | 100% | 2 | Auth ID validation |
| `isRetryableKubectlError` | via integration | 1 | Retry logic |
| `extractVersionFromImage` | 100% | 1 | Version extraction from container images |
| `FormatComponentVersions` | 96.3% | 3 | Version table formatting |
| `ParseControllerLogs` | 100% | 1 | Controller log parsing |
| `FormatControllerLogSummaries` | 85.7% | 2 | Log summary formatting |
| `DetectAzureError` | via integration | 1 | Azure error detection |
| `FormatAzureError` | 100% | 1 | Azure error formatting |
| `HasServicePrincipalCredentials` | via integration | 1 | SP credential detection |
| `GetAzureAuthDescription` | via integration | 1 | Auth mode description |
| `ClonedRepositoryTracking` | 100% | 1 | Repository tracking |
| `ValidateAzureRegion` | 88.9% | 1 | Azure region validation |
| `findSimilarRegions` | 85.7% | 1 | Fuzzy region matching |
| `ValidateTimeout` | 100% | 1 | Timeout validation |
| `ValidateDeploymentTimeout` | 100% | 1 | Deployment timeout validation |
| `ValidateASOControllerTimeout` | 100% | 1 | ASO timeout validation |
| `ValidateAllConfigurations` | 76.2% | 2 | Full config validation |
| `FormatValidationResults` | 92.9% | 1 | Validation result formatting |
| `formatRemediationSteps` | 100% | 2 | Remediation step formatting |
| `CheckForMismatchedClusters` | via integration | 1 | **V1.1** - Cluster mismatch detection |
| `FormatMismatchedClustersError` | 100% | 2 | **V1.1** - Mismatch error formatting |

#### config_test.go (6 tests, 151 lines)

| Function | Test Count | Notes |
|----------|------------|-------|
| `getDefaultRepoDir` | 3 | Env var override, consistency (sync.Once), path format |
| `parseDeploymentTimeout` | 3 | Default, valid durations, invalid durations |

### Integration Tests (Phase Files)

Integration tests exercise the full deployment workflow and cover functions that require external dependencies.

#### Phase 1: Check Dependencies (`01_check_dependencies_test.go`) - 18 tests, 879 lines

| Test | Coverage | V1.1 Changes |
|------|----------|-------------|
| `TestCheckDependencies_ToolAvailable` | Tool validation | - |
| `TestCheckDependencies_AzureCLILogin_IsLoggedIn` | Azure auth | - |
| `TestCheckDependencies_DockerCredentialHelper` | Docker config | - |
| `TestCheckDependencies_ExternalKubeconfig` | External cluster validation | **V1.1** - validates `USE_KUBECONFIG`, calls `ExtractCurrentContext()` |
| `TestCheckDependencies_ValidateConfiguration` | Config validation | **V1.1** - RFC 1123, namespace validation |
| + 13 more tests | Various | - |

#### Phase 3: Cluster (`03_cluster_test.go`) - 11 tests, 1,023 lines

| Test | Coverage | V1.1 Changes |
|------|----------|-------------|
| `TestExternalCluster_01_Connectivity` | **V1.1** | External cluster node access |
| `TestExternalCluster_01b_MCEBaselineStatus` | **V1.1** | MCE component baseline, calls `SetMCEComponentState()` |
| `TestExternalCluster_02_EnableMCE` | **V1.1** | MCE enablement, calls `EnableMCEComponent()`, `WaitForMCEController()` |
| `TestExternalCluster_03_ControllersReady` | **V1.1** | Controller readiness on external cluster |
| `TestKindCluster_*` (7 tests) | Kind cluster deployment | - |

#### Phase 4: Generate YAMLs (`04_generate_yamls_test.go`) - 4 tests, 321 lines

| Test | Coverage | V1.1 Changes |
|------|----------|-------------|
| `TestInfrastructure_GenerateResources` | YAML generation | **V1.1** - uses `CheckYAMLConfigMatch()` for namespace mismatch detection |

#### Phase 5: Deploy CRs (`05_deploy_crs_test.go`) - 9 tests, 574 lines

| Test | Coverage | V1.1 Changes |
|------|----------|-------------|
| `TestDeployment_00_CreateNamespace` | **V1.1** | Namespace creation for workload cluster |
| `TestDeployment_01_CheckExistingClusters` | **V1.1** | Calls `GetExistingClusterNames()`, `CheckForMismatchedClusters()` |
| `TestDeployment_ApplyResources` | CR deployment | **V1.1** - uses `ApplyWithRetryInNamespace()` |
| + 6 more tests | Monitoring, conditions | - |

#### Remaining Phases

| Phase | Tests | Lines | V1.1 Changes |
|-------|-------|-------|-------------|
| 02 Setup | 3 | 129 | - |
| 06 Verification | 7 | 354 | `IsExternalCluster()` guards |
| 07 Deletion | 6 | 304 | `IsExternalCluster()` guards |
| 08 Cleanup | 18 | 769 | - |

---

## V1.1 Coverage Assessment

### New Helper Functions

| Function | Unit Tests | Integration Tests | Assessment |
|----------|-----------|-------------------|------------|
| `SetMCEComponentState()` | None | `TestExternalCluster_01b_MCEBaselineStatus` | Adequate - requires live MCE cluster |
| `EnableMCEComponent()` | None | `TestExternalCluster_02_EnableMCE` | Adequate - requires live MCE cluster |
| `WaitForMCEController()` | None | `TestExternalCluster_02_EnableMCE` | Adequate - requires live cluster |
| `ExtractNamespaceFromYAML()` | 100% | `TestInfrastructure_GenerateResources` | Well covered |
| `ExtractCurrentContext()` | None | `TestCheckDependencies_ExternalKubeconfig` | Adequate - requires kubeconfig file |
| `CheckYAMLConfigMatch()` | Unit test exists | `TestInfrastructure_GenerateResources` | Well covered |
| `GetExistingClusterNames()` | None | `TestDeployment_01_CheckExistingClusters` | Adequate - requires live cluster |
| `CheckForMismatchedClusters()` | Unit test (logic) | `TestDeployment_01_CheckExistingClusters` | Well covered |
| `FormatMismatchedClustersError()` | 100% | `TestDeployment_01_CheckExistingClusters` | Well covered |
| `IsMCECluster()` | None | `TestExternalCluster_02_EnableMCE` | Adequate - requires live cluster |
| `GetMCEComponentStatus()` | None | `TestExternalCluster_01b_MCEBaselineStatus` | Adequate - requires live cluster |
| `ApplyWithRetryInNamespace()` | None | `TestDeployment_ApplyResources` | Adequate - requires live cluster |

### New Config Methods

| Method | Unit Tests | Integration Tests | Assessment |
|--------|-----------|-------------------|------------|
| `IsExternalCluster()` | None | Used in 5 phase files as guard condition | Well covered by integration |
| `GetKubeContext()` | 0% statement | Used in 4 phase files | Covered by integration, not unit |
| `getWorkloadClusterNamespace()` | Indirect via `TestDeploymentState_Namespace` | Used throughout | Adequate |

### New Integration Test Phases

| Test | Purpose | Code Path Covered |
|------|---------|-------------------|
| `TestExternalCluster_01_Connectivity` | Validates external cluster access | `IsExternalCluster()`, `GetKubeContext()`, kubectl node listing |
| `TestExternalCluster_01b_MCEBaselineStatus` | Ensures MCE component baseline | `GetMCEComponentStatus()`, `SetMCEComponentState()` |
| `TestExternalCluster_02_EnableMCE` | Enables CAPI/CAPZ on MCE | `EnableMCEComponent()`, `WaitForMCEController()`, `IsMCECluster()` |
| `TestExternalCluster_03_ControllersReady` | Validates controller deployments | Controller namespace lookups, deployment checks |
| `TestDeployment_00_CreateNamespace` | Creates workload cluster namespace | Namespace generation, `kubectl create namespace` |
| `TestDeployment_01_CheckExistingClusters` | Detects stale clusters | `GetExistingClusterNames()`, `CheckForMismatchedClusters()` |

### Coverage Gaps

#### Functions without unit tests (require external dependencies)

These functions have 0% unit test coverage but are exercised by integration tests during `make test-all`:

| Function | Reason | Integration Coverage |
|----------|--------|---------------------|
| `SetMCEComponentState()` | Requires MCE cluster | `TestExternalCluster_01b_MCEBaselineStatus` |
| `EnableMCEComponent()` | Requires MCE cluster | `TestExternalCluster_02_EnableMCE` |
| `WaitForMCEController()` | Requires MCE cluster | `TestExternalCluster_02_EnableMCE` |
| `GetMCEComponentStatus()` | Requires MCE cluster | `TestExternalCluster_01b_MCEBaselineStatus` |
| `IsMCECluster()` | Requires kubectl | `TestExternalCluster_02_EnableMCE` |
| `ExtractCurrentContext()` | Requires kubeconfig + kubectl | `TestCheckDependencies_ExternalKubeconfig` |
| `GetExistingClusterNames()` | Requires kubectl | `TestDeployment_01_CheckExistingClusters` |
| `ApplyWithRetryInNamespace()` | Requires kubectl | `TestDeployment_ApplyResources` |
| `GetKubeContext()` | Requires kubeconfig or Kind | Used in 4 phase files |

These are not feasibly unit-testable without mocking `exec.Command`, which would add complexity without proportional value in a test suite that already exercises these paths via integration tests.

#### Config methods without dedicated unit tests

| Method | Reason | Covered By |
|--------|--------|------------|
| `IsExternalCluster()` | Trivial one-liner (`c.UseKubeconfig != ""`) | Used as guard in 5 phase files |
| `GetKubeContext()` | Delegates to `ExtractCurrentContext()` or string format | Integration tests in 4 phase files |

These are simple enough that dedicated unit tests would not add meaningful value.

---

## Test Execution Modes

### Quick Validation (Unit Tests Only)

```bash
make test
```

**Duration**: ~15 seconds
**Coverage**: 32.1% of statements (pure logic functions)

### Full Test Suite

```bash
make test-all
```

**Duration**: 30-60 minutes (depends on Azure provisioning)
**Phases Executed**:
1. Check Dependencies (`_check-dep`)
2. Setup (`_setup`)
3. Cluster (`_cluster`) - Kind or External cluster mode
4. Generate YAMLs (`_generate-yamls`)
5. Deploy CRs (`_deploy-crs`)
6. Verification (`_verify`)
7. Delete (`_delete`)
8. Cleanup (`_cleanup`)

### External Cluster Mode

When `USE_KUBECONFIG` is set, the test suite exercises the MCE code path:

```bash
USE_KUBECONFIG=/path/to/kubeconfig make test-all
```

**Additional phases activated**:
- `TestExternalCluster_01_Connectivity`
- `TestExternalCluster_01b_MCEBaselineStatus`
- `TestExternalCluster_02_EnableMCE`
- `TestExternalCluster_03_ControllersReady`

**Phases skipped**:
- `TestSetup_CloneRepository` (controllers pre-installed)
- `TestKindCluster_*` (using external cluster)

---

## Phase Coverage Summary

| Phase | File | Tests | Lines | V1.1 New Tests |
|-------|------|-------|-------|---------------|
| 01 Check Dependencies | `01_check_dependencies_test.go` | 18 | 879 | External kubeconfig validation |
| 02 Setup | `02_setup_test.go` | 3 | 129 | - |
| 03 Cluster | `03_cluster_test.go` | 11 | 1,023 | +4 external cluster/MCE tests |
| 04 Generate YAMLs | `04_generate_yamls_test.go` | 4 | 321 | Namespace mismatch detection |
| 05 Deploy CRs | `05_deploy_crs_test.go` | 9 | 574 | +2 namespace/cluster checks |
| 06 Verification | `06_verification_test.go` | 7 | 354 | External cluster guards |
| 07 Deletion | `07_deletion_test.go` | 6 | 304 | External cluster guards |
| 08 Cleanup | `08_cleanup_test.go` | 18 | 769 | - |
| Unit: helpers | `helpers_test.go` | 52 | 3,348 | +6 new test functions |
| Unit: config | `config_test.go` | 6 | 151 | - |
| **Total** | **10 files** | **134** | **7,852** | |

---

## Deployment Workflow Coverage

The test suite covers 100% of the documented ARO-CAPZ deployment workflow:

- Check dependencies verification
- Repository setup
- Management cluster deployment (Kind or External)
- CAPI/CAPZ/ASO controller validation
- MCE component management (V1.1)
- Infrastructure resource generation
- Namespace creation and validation (V1.1)
- Resource application with retry
- Cluster provisioning monitoring
- Cluster verification
- Cluster deletion
- Cleanup validation

---

## CI/CD Coverage

### GitHub Actions Workflows

| Workflow | Trigger | Coverage |
|----------|---------|----------|
| `check-dependencies.yml` | Push, PR | Unit tests + dependency checks |
| `test.yml` | Manual | Full integration suite |
| `security-gosec.yml` | Daily | Static analysis |
| `security-govulncheck.yml` | Daily | Vulnerability scanning |
| `security-nancy.yml` | Daily | OSS Index scanning |
| `security-trivy.yml` | Daily | Comprehensive scanning |

---

## Future Considerations

1. **Interface-based mocking**: MCE and kubectl functions could be made unit-testable by introducing interfaces, but this would add abstraction complexity to a test suite
2. **Coverage target**: The 32.1% unit test coverage is appropriate for this codebase where most value comes from integration tests against real infrastructure
3. **External cluster CI**: Consider adding a CI job with `USE_KUBECONFIG` to exercise the MCE code path automatically

## Verification Commands

```bash
# Run unit tests with coverage
make test

# Get function-level coverage breakdown
go test -coverprofile=coverage.out ./test/ -short
go tool cover -func=coverage.out

# Run full integration suite
make test-all

# Run specific phase
go test -v ./test -run TestExternalCluster -timeout 30m
go test -v ./test -run TestDeployment -timeout 60m
```
