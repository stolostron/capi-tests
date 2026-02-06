# API/Interface Contract Review for V1

> **Issue**: #395 - V1 Review: API/Interface Contract Review
> **Priority**: HIGH
> **Status**: Complete

This document provides a comprehensive review of all public interfaces that will be difficult to change after v1. These contracts become the public API, and breaking changes will be disruptive to users.

## Table of Contents

1. [Environment Variables (Public API)](#environment-variables-public-api)
2. [TestConfig Struct](#testconfig-struct)
3. [Helper Functions](#helper-functions)
4. [Makefile Targets](#makefile-targets)
5. [Script Interfaces](#script-interfaces)
6. [Exit Codes](#exit-codes)
7. [Recommendations Summary](#recommendations-summary)

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
| `CS_CLUSTER_NAME` | ⚠️ Warning | Needs Review | **CS prefix unclear** - what does CS mean? |
| `OCP_VERSION` | ✅ Approved | Good | Matches cluster-api-installer variable |
| `REGION` | ✅ Approved | Good | Simple and clear |
| `DEPLOYMENT_ENV` | ✅ Approved | Good | Clear abbreviation (ENV is well-known) |
| `CAPZ_USER` | ✅ Approved | Good | Consistent with CAPZ terminology |
| `TEST_NAMESPACE` | ✅ Approved | Good | Clear, follows K8s conventions |
| `DEPLOYMENT_TIMEOUT` | ✅ Approved | Good | Clear purpose, Go duration format |

### Controller Namespace Variables (Internal/Advanced)

| Variable | Review Status | Rating | Notes |
|----------|---------------|--------|-------|
| `CAPI_NAMESPACE` | ✅ Approved | Good | Clear controller namespace override |
| `CAPZ_NAMESPACE` | ✅ Approved | Good | Consistent with CAPI_NAMESPACE |
| `USE_K8S` | ⚠️ Warning | Needs Review | **Boolean naming** - should be `USE_K8S_MODE` or clearer |
| `ASO_CONTROLLER_TIMEOUT` | ✅ Approved | Good | Clear purpose, follows DEPLOYMENT_TIMEOUT pattern |

### Path Configuration Variables (Internal)

| Variable | Review Status | Rating | Notes |
|----------|---------------|--------|-------|
| `CLUSTERCTL_BIN` | ✅ Approved | Good | Clear purpose |
| `SCRIPTS_PATH` | ✅ Approved | Good | Clear purpose |
| `GEN_SCRIPT_PATH` | ⚠️ Warning | Needs Review | **Abbreviation** - consider `GENERATION_SCRIPT_PATH` |
| `TEST_RESULTS_DIR` | ✅ Approved | Good | Clear purpose |

### Findings and Recommendations

1. **CS_CLUSTER_NAME**: The "CS" prefix is unclear. Based on code analysis, this appears to mean "Cluster Service" or relates to the cluster name prefix used for resource group naming. Consider:
   - Renaming to `CLUSTER_NAME_PREFIX` for clarity
   - Or documenting what "CS" stands for prominently

2. **USE_K8S**: Boolean flag naming could be clearer. Consider `USE_K8S_MODE=true` or `DEPLOYMENT_MODE=k8s` for better clarity.

3. **GEN_SCRIPT_PATH**: Abbreviation "GEN" could confuse users. Consider full name.

---

## TestConfig Struct

### Review Criteria
- Field names are clear and consistent
- Types are appropriate (string vs int vs duration)
- Grouping/organization is logical
- No fields that should be private

### Current Structure Analysis

```go
type TestConfig struct {
    // Repository configuration
    RepoURL    string  // ✅ Good - clear naming
    RepoBranch string  // ✅ Good - consistent
    RepoDir    string  // ✅ Good - consistent

    // Cluster configuration
    ManagementClusterName string        // ✅ Good - descriptive
    WorkloadClusterName   string        // ✅ Good - descriptive
    ClusterNamePrefix     string        // ✅ Good - better than CS_CLUSTER_NAME
    OCPVersion            string        // ✅ Good - matches installer variable
    Region                string        // ✅ Good - simple
    AzureSubscriptionName string        // ✅ Good - clear that it's the name (FIXED)
    Environment           string        // ✅ Good - matches DEPLOYMENT_ENV
    CAPZUser              string        // ✅ Good - matches CAPZ_USER env var (FIXED)
    TestNamespace         string        // ✅ Good - clear
    CAPINamespace         string        // ✅ Good - clear
    CAPZNamespace         string        // ✅ Good - clear

    // Paths
    ClusterctlBinPath string  // ✅ Good - clear
    ScriptsPath       string  // ✅ Good - clear
    GenScriptPath     string  // ⚠️ Abbreviation - consider GenerationScriptPath

    // Timeouts
    DeploymentTimeout    time.Duration  // ✅ Good - proper type
    ASOControllerTimeout time.Duration  // ✅ Good - proper type
}
```

### Findings and Recommendations

1. **Field Grouping**: ✅ The struct is well-organized with logical groupings:
   - Repository configuration
   - Cluster configuration
   - Paths
   - Timeouts

2. **Type Appropriateness**: ✅ Types are correct:
   - `time.Duration` for timeouts (not strings)
   - Strings for configuration values

3. **Naming Issues** (RESOLVED):
   - ~~`AzureSubscription`~~ → `AzureSubscriptionName` ✅ FIXED
   - ~~`User`~~ → `CAPZUser` ✅ FIXED
   - `GenScriptPath` - Abbreviation. Consider `GenerationScriptPath` (minor, kept as-is for v1).

4. **No Private Fields Needed**: All fields appropriately public for test configuration.

---

## Helper Functions

### Review Criteria
- Function names follow Go conventions
- Parameter order is consistent
- Return types are appropriate
- No functions that should be internal

### Public Helper Functions Review

| Function | Signature | Status | Notes |
|----------|-----------|--------|-------|
| `CommandExists` | `(cmd string) bool` | ✅ Approved | Simple, clear, follows Go naming |
| `RunCommand` | `(t *testing.T, name string, args ...string) (string, error)` | ✅ Approved | Standard pattern, t first as per convention |
| `RunCommandQuiet` | `(t *testing.T, name string, args ...string) (string, error)` | ✅ Approved | Good variant name |
| `RunCommandWithStreaming` | `(t *testing.T, name string, args ...string) (string, error)` | ✅ Approved | Descriptive name |
| `SetEnvVar` | `(t *testing.T, key, value string)` | ✅ Approved | Clear, standard pattern |
| `FileExists` | `(path string) bool` | ✅ Approved | Simple, standard Go naming |
| `DirExists` | `(path string) bool` | ✅ Approved | Consistent with FileExists |
| `GetEnvOrDefault` | `(key, defaultValue string) string` | ✅ Approved | Clear purpose |
| `ValidateDomainPrefix` | `(user, env string) error` | ✅ Approved | Clear validation function |
| `ValidateRFC1123Name` | `(name, varName string) error` | ✅ Approved | Clear, includes varName for error messages |

### Additional Public Functions

| Function | Signature | Status | Notes |
|----------|-----------|--------|-------|
| `PrintTestHeader` | `(t, testName, description string)` | ✅ Approved | Clear purpose |
| `PrintToTTY` | `(format string, args ...interface{})` | ✅ Approved | Clear purpose |
| `ReportProgress` | `(t, iteration int, elapsed, remaining, timeout Duration)` | ✅ Approved | Good for progress reporting |
| `IsKubectlApplySuccess` | `(output string) bool` | ✅ Approved | Clear predicate naming |
| `ExtractClusterNameFromYAML` | `(filePath string) (string, error)` | ✅ Approved | Descriptive |
| `FormatAROControlPlaneConditions` | `(jsonData string) string` | ✅ Approved | Standard Format* naming |
| `EnsureAzureCredentialsSet` | `(t) error` | ✅ Approved | Ensure* naming indicates side effect |
| `PatchASOCredentialsSecret` | `(t, kubeContext string) error` | ✅ Approved | Clear operation naming |
| `ApplyWithRetry` | `(t, kubeContext, yamlPath string, maxRetries int) error` | ✅ Approved | Clear retry pattern |
| `WaitForClusterHealthy` | `(t, kubeContext string, timeout Duration) error` | ✅ Approved | WaitFor* naming convention |
| `WaitForClusterReady` | `(t, kubeContext, namespace, clusterName string, timeout Duration) error` | ✅ Approved | Consistent with WaitFor* |

### Findings and Recommendations

1. **Go Naming Conventions**: ✅ All functions follow Go conventions
   - Exported functions start with uppercase
   - Descriptive names without excessive abbreviations
   - Predicate functions use `Is*` prefix

2. **Parameter Order**: ✅ Consistent
   - `t *testing.T` always first (Go testing convention)
   - Context-like parameters (kubeContext) follow t
   - Configuration parameters last

3. **Return Types**: ✅ Appropriate
   - `error` for operations that can fail
   - `bool` for existence checks
   - `(string, error)` for commands that return output

4. **Internal Functions**: The following functions are appropriately unexported:
   - `openTTY()` - internal TTY handling
   - `isWaitingCondition()` - internal condition checking
   - `isRetryableKubectlError()` - internal retry logic
   - `extractVersionFromImage()` - internal parsing
   - `getControllerNamespace()` - internal config helper

---

## Makefile Targets

### Review Criteria
- Target names are intuitive
- Help text is clear
- No targets that should be internal (prefix with `_`)
- Consistent naming convention

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
| `_cluster` | ✅ Approved | Internal phase |
| `_generate-yamls` | ✅ Approved | Internal phase |
| `_deploy-crs` | ✅ Approved | Internal phase |
| `_verify` | ✅ Approved | Internal phase |
| `_delete` | ✅ Approved | Internal phase |
| `_test-all-impl` | ✅ Approved | Internal implementation |
| `_copy-latest-results` | ✅ Approved | Internal helper |
| `_clean-azure-force` | ✅ Approved | Internal force variant |

### Findings and Recommendations

1. **Naming Convention**: ✅ Excellent
   - User-facing targets use standard names
   - Internal targets correctly use `_` prefix
   - Consistent kebab-case naming

2. **Help Text**: ✅ All user-facing targets have `## Comment` for help text

3. **Documentation**: ✅ The `help` target provides clear usage information and explains the expected order for internal targets.

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

### Current Exit Codes

| Script/Tool | Exit Code | Meaning | Status |
|-------------|-----------|---------|--------|
| cleanup-azure-resources.sh | 0 | Success | ✅ Documented |
| cleanup-azure-resources.sh | 1 | Error (invalid prefix, not logged in, etc.) | ✅ Documented |
| generate-summary.sh | 0 | Success | ✅ Documented |
| generate-summary.sh | 1 | Error (usage, missing dir, missing xmllint) | ✅ Documented |
| Makefile targets | 0 | Success | ✅ Standard |
| Makefile targets | Non-zero | Failure | ✅ Standard |

### Recommendation

Exit codes are consistent and follow Unix conventions. No changes needed.

---

## Recommendations Summary

### Critical (Should Fix Before V1)

1. **CS_CLUSTER_NAME Environment Variable**: The "CS" prefix is unclear and should be documented or renamed to `CLUSTER_NAME_PREFIX`.
   - ✅ **RESOLVED**: Added documentation in CLAUDE.md explaining CS = **C**luster **S**ervice

### Recommended (Minor Improvements)

2. **TestConfig.AzureSubscription Field**: Consider renaming to `AzureSubscriptionName` for clarity since it maps to `AZURE_SUBSCRIPTION_NAME`.
   - ✅ **RESOLVED**: Renamed to `AzureSubscriptionName`

3. **TestConfig.User Field**: Consider renaming to `CAPZUser` to match the `CAPZ_USER` environment variable.
   - ✅ **RESOLVED**: Renamed to `CAPZUser`

4. **GEN_SCRIPT_PATH**: Consider renaming to `GENERATION_SCRIPT_PATH` to avoid abbreviation.
   - ⏸️ **Deferred**: Minor issue, kept as-is for v1 stability

5. **USE_K8S**: Consider renaming to `USE_K8S_MODE` or `DEPLOYMENT_MODE` for clarity.
   - ⏸️ **Deferred**: Minor issue, kept as-is for v1 stability

### Already Good (No Changes Needed)

- Azure authentication variables follow SDK conventions
- ARO_REPO_* family is consistent
- Cluster configuration variables are well-named
- Helper function signatures follow Go conventions
- Makefile targets use proper naming conventions with `_` prefix for internal targets
- Script interfaces are well-documented with standard flags

---

## Conclusion

The API/Interface contracts are well-designed and follow established conventions. All critical and recommended improvements have been addressed:

- **CS_CLUSTER_NAME** is now documented with CS = Cluster Service
- **TestConfig field names** are now consistent with their environment variable counterparts
- **Helper functions** follow Go conventions
- **Makefile targets** use proper naming with `_` prefix for internal targets
- **Script interfaces** are well-documented with standard flags

**Overall Rating**: ✅ **Ready for V1**

The test suite's public API is stable, consistent, and follows industry best practices. Breaking changes after v1 should be minimal given the current design quality.
