# API/Interface Contract Review

> **V1 Issue**: #395 - V1 Review: API/Interface Contract Review
> **V1.1 Issue**: ACM-29881 - V1.1 Review: API/Interface Contract Review
> **Status**: Complete

This document provides a comprehensive review of all public interfaces. These contracts become the public API, and breaking changes will be disruptive to users.

## Table of Contents

1. [Breaking Changes (V1 → V1.1)](#breaking-changes-v1--v11)
2. [Environment Variables (Public API)](#environment-variables-public-api)
3. [TestConfig Struct](#testconfig-struct)
4. [Helper Functions](#helper-functions)
5. [Makefile Targets](#makefile-targets)
6. [Script Interfaces](#script-interfaces)
7. [Exit Codes](#exit-codes)
8. [Recommendations Summary](#recommendations-summary)

---

## Breaking Changes (V1 → V1.1)

### Environment Variable Renames

| V1 Variable | V1.1 Variable | Migration |
|-------------|---------------|-----------|
| `OPENSHIFT_VERSION` | `OCP_VERSION` | Rename in your env/scripts |
| `TEST_NAMESPACE` | `WORKLOAD_CLUSTER_NAMESPACE` | Rename; note that v1.1 auto-generates a unique namespace per run if not set |

### Behavior Changes

| Change | V1 Behavior | V1.1 Behavior |
|--------|-------------|---------------|
| Namespace | Static `TEST_NAMESPACE` (default: `default`) | Auto-generated unique namespace per test run (`capz-test-YYYYMMDD-HHMMSS`). Set `WORKLOAD_CLUSTER_NAMESPACE` to override. |
| `TestNamespace` field | `TestConfig.TestNamespace` | Renamed to `TestConfig.WorkloadClusterNamespace` |

### New Environment Variables (V1.1)

| Variable | Required | Default | Purpose |
|----------|----------|---------|---------|
| `USE_KUBECONFIG` | No | _(unset)_ | Path to external kubeconfig; enables external cluster mode |
| `WORKLOAD_CLUSTER_NAMESPACE` | No | _(auto-generated)_ | Explicit namespace override for resume scenarios |
| `WORKLOAD_CLUSTER_NAMESPACE_PREFIX` | No | `capz-test` | Prefix for auto-generated namespace |
| `MCE_AUTO_ENABLE` | No | `true` (when `USE_KUBECONFIG` set) | Auto-enable MCE CAPI/CAPZ components |
| `MCE_ENABLEMENT_TIMEOUT` | No | `15m` | Timeout for MCE component enablement |

### New TestConfig Methods (V1.1)

| Method | Signature | Purpose |
|--------|-----------|---------|
| `IsExternalCluster` | `() bool` | Returns true when using external kubeconfig |
| `GetKubeContext` | `() string` | Returns kubectl context (Kind or external) |

### New Constants (V1.1)

| Constant | Value | Purpose |
|----------|-------|---------|
| `MCEComponentCAPI` | `"cluster-api"` | MCE component name for CAPI |
| `MCEComponentCAPZ` | `"cluster-api-provider-azure-preview"` | MCE component name for CAPZ |
| `DefaultMCEEnablementTimeout` | `15 * time.Minute` | Default MCE enablement timeout |

---

## Environment Variables (Public API)

### Review Criteria
- Names are clear and consistent (prefix conventions)
- No abbreviations that might confuse users
- Aligned with industry conventions where applicable
- Complete list documented in one place

### Azure Authentication Variables

| Variable | Review Status | Rating | Notes |
|----------|---------------|--------|-------|
| `AZURE_CLIENT_ID` | ✅ Approved | Good | Standard Azure SDK naming convention |
| `AZURE_CLIENT_SECRET` | ✅ Approved | Good | Standard Azure SDK naming convention |
| `AZURE_TENANT_ID` | ✅ Approved | Good | Standard Azure SDK naming convention |
| `AZURE_SUBSCRIPTION_ID` | ✅ Approved | Good | Standard Azure SDK naming convention |
| `AZURE_SUBSCRIPTION_NAME` | ✅ Approved | Good | Alternative to ID, follows Azure patterns |

### Repository Configuration Variables

| Variable | Review Status | Rating | Notes |
|----------|---------------|--------|-------|
| `ARO_REPO_URL` | ✅ Approved | Good | Clear prefix (ARO_), describes purpose |
| `ARO_REPO_BRANCH` | ✅ Approved | Good | Consistent with ARO_REPO_URL |
| `ARO_REPO_DIR` | ✅ Approved | Good | Consistent with ARO_REPO_* family |

### Cluster Configuration Variables

| Variable | Review Status | Rating | Notes |
|----------|---------------|--------|-------|
| `MANAGEMENT_CLUSTER_NAME` | ✅ Approved | Good | Clear, descriptive, no abbreviations |
| `WORKLOAD_CLUSTER_NAME` | ✅ Approved | Good | Clear, descriptive, no abbreviations |
| `CS_CLUSTER_NAME` | ⚠️ Warning | Acceptable | **CS** = **C**luster **S**ervice; documented in CLAUDE.md |
| `OCP_VERSION` | ✅ Approved | Good | Matches cluster-api-installer variable. Renamed from `OPENSHIFT_VERSION` in v1.1 |
| `REGION` | ✅ Approved | Good | Simple and clear |
| `DEPLOYMENT_ENV` | ✅ Approved | Good | Clear abbreviation (ENV is well-known) |
| `CAPZ_USER` | ✅ Approved | Good | Consistent with CAPZ terminology |
| `WORKLOAD_CLUSTER_NAMESPACE` | ✅ Approved | Good | Clear, follows `WORKLOAD_CLUSTER_*` prefix. Replaces `TEST_NAMESPACE` from v1 |
| `WORKLOAD_CLUSTER_NAMESPACE_PREFIX` | ✅ Approved | Good | Consistent with `WORKLOAD_CLUSTER_*` family |
| `DEPLOYMENT_TIMEOUT` | ✅ Approved | Good | Clear purpose, Go duration format |

### External Cluster Variables (New in V1.1)

| Variable | Review Status | Rating | Notes |
|----------|---------------|--------|-------|
| `USE_KUBECONFIG` | ✅ Approved | Good | Clear intent, file path value |
| `MCE_AUTO_ENABLE` | ✅ Approved | Good | Clear boolean, `MCE_*` prefix consistent |
| `MCE_ENABLEMENT_TIMEOUT` | ✅ Approved | Good | Consistent with `DEPLOYMENT_TIMEOUT` pattern |

### Controller Namespace Variables (Internal/Advanced)

| Variable | Review Status | Rating | Notes |
|----------|---------------|--------|-------|
| `CAPI_NAMESPACE` | ✅ Approved | Good | Clear controller namespace override |
| `CAPZ_NAMESPACE` | ✅ Approved | Good | Consistent with CAPI_NAMESPACE |
| `USE_K8S` | ⚠️ Warning | Acceptable | Auto-set when `USE_KUBECONFIG` is provided; naming kept for backward compatibility |
| `ASO_CONTROLLER_TIMEOUT` | ✅ Approved | Good | Clear purpose, follows DEPLOYMENT_TIMEOUT pattern |

### Path Configuration Variables (Internal)

| Variable | Review Status | Rating | Notes |
|----------|---------------|--------|-------|
| `CLUSTERCTL_BIN` | ✅ Approved | Good | Clear purpose |
| `SCRIPTS_PATH` | ✅ Approved | Good | Clear purpose |
| `GEN_SCRIPT_PATH` | ⚠️ Warning | Acceptable | Abbreviation kept for backward compatibility |
| `TEST_RESULTS_DIR` | ✅ Approved | Good | Clear purpose |

### Findings and Recommendations

1. **CS_CLUSTER_NAME**: ✅ Documented as **C**luster **S**ervice in CLAUDE.md (resolved in v1).

2. **USE_K8S**: Now auto-set when `USE_KUBECONFIG` is provided. Naming kept for backward compatibility. Low priority to rename.

3. **GEN_SCRIPT_PATH**: Abbreviation kept for backward compatibility. Low priority to rename.

---

## TestConfig Struct

### Review Criteria
- Field names are clear and consistent
- Types are appropriate (string vs int vs duration)
- Grouping/organization is logical
- No fields that should be private

### Current Structure Analysis (V1.1)

```go
type TestConfig struct {
    // Repository configuration
    RepoURL    string  // ✅ Good - clear naming
    RepoBranch string  // ✅ Good - consistent
    RepoDir    string  // ✅ Good - consistent

    // Cluster configuration
    ManagementClusterName    string  // ✅ Good - descriptive
    WorkloadClusterName      string  // ✅ Good - descriptive
    ClusterNamePrefix        string  // ✅ Good - maps to CS_CLUSTER_NAME
    OCPVersion               string  // ✅ Good - matches installer variable
    Region                   string  // ✅ Good - simple
    AzureSubscriptionName    string  // ✅ Good - clear (FIXED in v1)
    Environment              string  // ✅ Good - matches DEPLOYMENT_ENV
    CAPZUser                 string  // ✅ Good - matches CAPZ_USER (FIXED in v1)
    WorkloadClusterNamespace string  // ✅ Good - replaces TestNamespace (v1.1)
    CAPINamespace            string  // ✅ Good - clear
    CAPZNamespace            string  // ✅ Good - clear

    // External cluster configuration (New in V1.1)
    UseKubeconfig string  // ✅ Good - file path, maps to USE_KUBECONFIG

    // Paths
    ClusterctlBinPath string  // ✅ Good - clear
    ScriptsPath       string  // ✅ Good - clear
    GenScriptPath     string  // ⚠️ Abbreviation - kept for backward compatibility

    // Timeouts
    DeploymentTimeout    time.Duration  // ✅ Good - proper type
    ASOControllerTimeout time.Duration  // ✅ Good - proper type

    // MCE configuration (New in V1.1)
    MCEAutoEnable        bool           // ✅ Good - clear boolean
    MCEEnablementTimeout time.Duration  // ✅ Good - proper type, consistent naming
}
```

### V1.1 Changes

| Change | Details | Status |
|--------|---------|--------|
| `TestNamespace` → `WorkloadClusterNamespace` | Renamed to match env var and clarify purpose | ✅ Breaking but necessary |
| `UseKubeconfig` added | External cluster support | ✅ New field, no breakage |
| `MCEAutoEnable` added | MCE auto-enablement control | ✅ New field, no breakage |
| `MCEEnablementTimeout` added | MCE timeout configuration | ✅ New field, `time.Duration` type correct |

### Findings and Recommendations

1. **Field Grouping**: ✅ Well-organized with logical groupings (repository, cluster, external cluster, paths, timeouts, MCE).

2. **Type Appropriateness**: ✅ Types are correct:
   - `time.Duration` for all timeouts
   - `bool` for `MCEAutoEnable`
   - Strings for paths and identifiers

3. **WorkloadClusterNamespace Resolution**: This field is computed from multiple sources (env var → state file → auto-generated). The resolution logic is well-documented in `getWorkloadClusterNamespace()` (`config.go:65-109`).

---

## Helper Functions

### Review Criteria
- Function names follow Go conventions
- Parameter order is consistent
- Return types are appropriate
- No functions that should be internal

### V1 Public Helper Functions

| Function | Signature | Status | Notes |
|----------|-----------|--------|-------|
| `CommandExists` | `(cmd string) bool` | ✅ Approved | Simple, clear |
| `RunCommand` | `(t, name string, args ...string) (string, error)` | ✅ Approved | Standard pattern |
| `RunCommandQuiet` | `(t, name string, args ...string) (string, error)` | ✅ Approved | Good variant |
| `RunCommandWithStreaming` | `(t, name string, args ...string) (string, error)` | ✅ Approved | Descriptive |
| `SetEnvVar` | `(t, key, value string)` | ✅ Approved | Clear |
| `FileExists` | `(path string) bool` | ✅ Approved | Standard |
| `DirExists` | `(path string) bool` | ✅ Approved | Consistent |
| `GetEnvOrDefault` | `(key, defaultValue string) string` | ✅ Approved | Clear |
| `ValidateDomainPrefix` | `(user, env string) error` | ✅ Approved | Clear |
| `ValidateRFC1123Name` | `(name, varName string) error` | ✅ Approved | Clear |
| `PrintTestHeader` | `(t, testName, description string)` | ✅ Approved | Clear |
| `PrintToTTY` | `(format string, args ...interface{})` | ✅ Approved | Clear |
| `ReportProgress` | `(t, iteration int, elapsed, remaining, timeout Duration)` | ✅ Approved | Good |
| `IsKubectlApplySuccess` | `(output string) bool` | ✅ Approved | Clear predicate |
| `ExtractClusterNameFromYAML` | `(filePath string) (string, error)` | ✅ Approved | Descriptive |
| `FormatAROControlPlaneConditions` | `(jsonData string) string` | ✅ Approved | Standard |
| `EnsureAzureCredentialsSet` | `(t) error` | ✅ Approved | Ensure* naming |
| `PatchASOCredentialsSecret` | `(t, kubeContext string) error` | ✅ Approved | Clear |
| `ApplyWithRetry` | `(t, kubeContext, yamlPath string, maxRetries int) error` | ✅ Approved | Clear |
| `WaitForClusterHealthy` | `(t, kubeContext string, timeout Duration) error` | ✅ Approved | WaitFor* |
| `WaitForClusterReady` | `(t, kubeContext, namespace, clusterName string, timeout Duration) error` | ✅ Approved | Consistent |

### New V1.1 Helper Functions

| Function | Signature | Status | Notes |
|----------|-----------|--------|-------|
| `ExtractCurrentContext` | `(kubeconfigPath string) string` | ✅ Approved | Pure function, no `t` needed |
| `IsMCECluster` | `(t, kubeContext string) bool` | ✅ Approved | `Is*` predicate naming |
| `GetMCEComponentStatus` | `(t, kubeContext, componentName string) (*MCEComponentStatus, error)` | ✅ Approved | `Get*` naming, returns struct pointer |
| `SetMCEComponentState` | `(t, kubeContext, componentName string, enabled bool) error` | ✅ Approved | `Set*` naming, clear bool parameter |
| `EnableMCEComponent` | `(t, kubeContext, componentName string) error` | ⚠️ Refactor | Currently duplicates `SetMCEComponentState` logic; should delegate to it (ACM-29872) |
| `WaitForMCEController` | `(t, kubeContext, namespace, deploymentName string, timeout Duration) error` | ✅ Approved | `WaitFor*` consistent |
| `CheckYAMLConfigMatch` | `(t, aroYAMLPath, expectedPrefix string) (bool, string)` | ✅ Approved | Named returns for clarity |
| `ExtractNamespaceFromYAML` | `(filePath string) (string, error)` | ✅ Approved | Pure function, no `t` needed |
| `ApplyWithRetryInNamespace` | `(t, kubeContext, namespace, yamlPath string, maxRetries int) error` | ✅ Approved | Namespace-explicit variant of `ApplyWithRetry` |
| `GetExistingClusterNames` | `(t, kubeContext, namespace string) ([]string, error)` | ✅ Approved | `Get*` naming |
| `CheckForMismatchedClusters` | `(t, kubeContext, namespace, expectedPrefix string) ([]string, error)` | ✅ Approved | Returns slice of mismatched names |
| `FormatMismatchedClustersError` | `(mismatched []string, expectedPrefix, namespace string) string` | ✅ Approved | `Format*` naming, pure function |
| `ReadDeploymentState` | `() (*DeploymentState, error)` | ✅ Approved | No `t` needed (utility) |
| `WriteDeploymentState` | `(config *TestConfig) error` | ✅ Approved | No `t` needed (utility) |
| `GetClusterPhase` | `(t, kubeContext, namespace, clusterName string) (string, error)` | ✅ Approved | `Get*` naming |
| `GetDeletionResourceStatus` | `(t, kubeContext, namespace, clusterName, resourceGroup string) DeletionResourceStatus` | ✅ Approved | Returns value type |

### Findings and Recommendations

1. **Naming Conventions**: ✅ All V1.1 functions follow established patterns:
   - Predicates: `Is*` (IsMCECluster)
   - Getters: `Get*` (GetMCEComponentStatus)
   - Setters: `Set*` (SetMCEComponentState)
   - Wait functions: `WaitFor*` (WaitForMCEController)

2. **Parameter Order**: ✅ Consistent — `t *testing.T` always first when present.

3. **Internal Functions**: Appropriately unexported:
   - `openTTY()` - internal TTY handling
   - `isWaitingCondition()` - internal condition checking
   - `isRetryableKubectlError()` - internal retry logic
   - `extractVersionFromImage()` - internal parsing
   - `getControllerNamespace()` - internal config helper
   - `getWorkloadClusterNamespace()` - namespace resolution (V1.1)
   - `parseMCEAutoEnable()` - config parsing (V1.1)
   - `parseMCEEnablementTimeout()` - config parsing (V1.1)

---

## Makefile Targets

### User-Facing Targets

| Target | Status | Notes |
|--------|--------|-------|
| `test` | ✅ Approved | Standard target, runs quick tests |
| `test-all` | ✅ Approved | Clear that it runs all tests |
| `clean` | ✅ Approved | Standard cleanup target |
| `clean-all` | ✅ Approved | Clear variant for non-interactive |
| `clean-azure` | ✅ Approved | Specific to Azure resources |
| `help` | ✅ Approved | Standard help target |
| `summary` | ✅ Approved | Clear purpose |
| `check-prereq` | ✅ Approved | Clear purpose |
| `install-gotestsum` | ✅ Approved | Clear purpose |
| `check-gotestsum` | ✅ Approved | Clear purpose |
| `setup-submodule` | ✅ Approved | Clear purpose |
| `update-submodule` | ✅ Approved | Clear purpose |
| `fix-docker-config` | ✅ Approved | Clear purpose |
| `fmt` | ✅ Approved | Standard Go target |
| `lint` | ✅ Approved | Standard Go target |
| `deps` | ✅ Approved | Standard dependency target |

### Internal Targets (Correctly Prefixed)

| Target | Status | Notes |
|--------|--------|-------|
| `_check-dep` | ✅ Approved | Internal phase, underscore prefix correct |
| `_setup` | ✅ Approved | Internal phase |
| `_management_cluster` | ✅ Approved | Internal phase |
| `_generate-yamls` | ✅ Approved | Internal phase |
| `_deploy-crs` | ✅ Approved | Internal phase |
| `_verify` | ✅ Approved | Internal phase |
| `_delete` | ✅ Approved | Internal phase |
| `_cleanup` | ✅ Approved | Internal phase (V1.1) |
| `_test-all-impl` | ✅ Approved | Internal implementation |
| `_copy-latest-results` | ✅ Approved | Internal helper |
| `_clean-azure-force` | ✅ Approved | Internal force variant |

No new user-facing Makefile targets were added in V1.1.

---

## Script Interfaces

### cleanup-azure-resources.sh

| Interface | Status | Notes |
|-----------|--------|-------|
| `--prefix PREFIX` | ✅ Approved | Clear, consistent with env var |
| `--resource-group RG` | ✅ Approved | Clear parameter |
| `--dry-run` | ✅ Approved | Standard flag name |
| `--force` | ✅ Approved | Standard flag name |
| `--help` / `-h` | ✅ Approved | Standard help flags |
| Exit code 0 | ✅ Approved | Success |
| Exit code 1 | ✅ Approved | Error |

**Input Validation**: ✅ The script validates the prefix against RFC 1123 pattern to prevent OData filter injection.

### generate-summary.sh

| Interface | Status | Notes |
|-----------|--------|-------|
| `<results-directory>` | ✅ Approved | Positional argument, clear |
| Exit code 0 | ✅ Approved | Success |
| Exit code 1 | ✅ Approved | Error (missing arg, missing dir, missing xmllint) |

**Output**: Creates `summary.txt` in the results directory (documented).

---

## Exit Codes

| Script/Tool | Exit Code | Meaning | Status |
|-------------|-----------|---------|--------|
| cleanup-azure-resources.sh | 0 | Success | ✅ Documented |
| cleanup-azure-resources.sh | 1 | Error (invalid prefix, not logged in, etc.) | ✅ Documented |
| generate-summary.sh | 0 | Success | ✅ Documented |
| generate-summary.sh | 1 | Error (usage, missing dir, missing xmllint) | ✅ Documented |
| Makefile targets | 0 | Success | ✅ Standard |
| Makefile targets | Non-zero | Failure | ✅ Standard |

Exit codes are consistent and follow Unix conventions.

---

## Recommendations Summary

### Resolved in V1

1. **CS_CLUSTER_NAME**: ✅ Documented as **C**luster **S**ervice in CLAUDE.md
2. **TestConfig.AzureSubscription**: ✅ Renamed to `AzureSubscriptionName`
3. **TestConfig.User**: ✅ Renamed to `CAPZUser`

### Resolved in V1.1

4. **TEST_NAMESPACE → WORKLOAD_CLUSTER_NAMESPACE**: ✅ Renamed with auto-generation support
5. **OPENSHIFT_VERSION → OCP_VERSION**: ✅ Renamed to match cluster-api-installer

### Deferred (Low Priority)

6. **GEN_SCRIPT_PATH**: Abbreviation kept for backward compatibility
7. **USE_K8S**: Naming kept for backward compatibility; now auto-set when `USE_KUBECONFIG` is provided

---

## Conclusion

The API/Interface contracts are well-designed and follow established conventions:

- **Environment variables** follow SCREAMING_SNAKE_CASE with logical prefixes (`AZURE_*`, `ARO_*`, `MCE_*`, `WORKLOAD_CLUSTER_*`)
- **TestConfig fields** use appropriate types (`time.Duration` for timeouts, `bool` for flags)
- **Helper functions** follow Go conventions (`Is*`, `Get*`, `Set*`, `WaitFor*`, `t *testing.T` first)
- **Makefile targets** use `_` prefix for internal targets
- **Script interfaces** are well-documented with standard flags
- **Breaking changes** from V1 are documented with migration paths

**Overall Rating**: ✅ **Ready for V1.1**
