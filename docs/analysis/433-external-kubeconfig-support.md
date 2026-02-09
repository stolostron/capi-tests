# Analysis: External Kubernetes Cluster Support via Kubeconfig

**Issue:** [#433 - Add support for external Kubernetes cluster via kubeconfig](https://github.com/RadekCap/CAPZTests/issues/433)

**Date:** 2025-01-28

**Status:** Analysis Complete

---

## Executive Summary

This document describes the implementation approach for adding support to run the ARO-CAPZ test suite against an external Kubernetes cluster (e.g., MCE installation) instead of creating a local Kind cluster.

The solution introduces a single environment variable `USE_KUBECONFIG` that, when set to a kubeconfig file path, switches the test suite to "external cluster mode" where it validates pre-installed controllers rather than deploying them.

---

## Current Architecture

### Test Flow (Kind-based)

```
Phase 01: Check Dependencies     → Validate tools (kubectl, kind, az, etc.)
Phase 02: Setup                  → Clone cluster-api-installer repository
Phase 03: Cluster                → Create Kind cluster, deploy CAPI/CAPZ/ASO
Phase 04: Generate YAMLs         → Generate credentials.yaml, aro.yaml
Phase 05: Deploy CRs             → Apply resources, monitor deployment
Phase 06: Verification           → Retrieve kubeconfig, verify workload cluster
Phase 07: Deletion               → Delete workload cluster
Phase 08: Cleanup                → Validate cleanup status
```

### Current Context Resolution

All tests construct the kubectl context using:

```go
context := fmt.Sprintf("kind-%s", config.ManagementClusterName)
```

This pattern appears in 50+ locations across test files.

### Controller Namespaces

| Mode | CAPI Namespace | CAPZ/ASO Namespace |
|------|----------------|-------------------|
| Kind (USE_KIND=true) | `capi-system` | `capz-system` |
| K8S (USE_K8S=true) | `multicluster-engine` | `multicluster-engine` |

---

## Proposed Solution

### Configuration

**New Environment Variable:**

| Variable | Type | Description |
|----------|------|-------------|
| `USE_KUBECONFIG` | Path | Path to external kubeconfig file. When set, enables external cluster mode. |

**Context Resolution:**
- Uses `current-context` from the kubeconfig file
- No additional `KUBE_CONTEXT` variable (YAGNI principle)
- User sets context before running: `kubectl config use-context <name>`

**Namespace Configuration:**
- External clusters use `multicluster-engine` namespace (MCE installation)
- Leverages existing `USE_K8S=true` pattern for namespace resolution

### Implementation Details

#### 1. Config Changes (`test/config.go`)

```go
type TestConfig struct {
    // ...existing fields...

    // UseKubeconfig is the path to an external kubeconfig file.
    // When set, the test suite runs in "external cluster mode":
    // - Skips Kind cluster creation
    // - Validates pre-installed controllers
    // - Uses current-context from the kubeconfig
    UseKubeconfig string
}

func NewTestConfig() *TestConfig {
    useKubeconfig := os.Getenv("USE_KUBECONFIG")

    // When using external kubeconfig, default to MCE namespaces
    if useKubeconfig != "" {
        os.Setenv("USE_K8S", "true")  // Triggers multicluster-engine namespaces
    }

    return &TestConfig{
        // ...existing initialization...
        UseKubeconfig: useKubeconfig,
    }
}

// GetKubeContext returns the kubectl context to use.
// For external clusters, extracts current-context from kubeconfig.
// For Kind clusters, returns "kind-{ManagementClusterName}".
func (c *TestConfig) GetKubeContext() string {
    if c.UseKubeconfig != "" {
        return extractCurrentContext(c.UseKubeconfig)
    }
    return fmt.Sprintf("kind-%s", c.ManagementClusterName)
}

// IsExternalCluster returns true when using an external kubeconfig.
func (c *TestConfig) IsExternalCluster() bool {
    return c.UseKubeconfig != ""
}
```

#### 2. Context Extraction Helper (`test/helpers.go`)

```go
// extractCurrentContext reads the current-context from a kubeconfig file.
func extractCurrentContext(kubeconfigPath string) string {
    output, err := exec.Command("kubectl", "config", "current-context",
        "--kubeconfig", kubeconfigPath).Output()
    if err != nil {
        return ""
    }
    return strings.TrimSpace(string(output))
}
```

#### 3. Phase Behavior Changes

| Phase | Kind Mode | External Cluster Mode |
|-------|-----------|----------------------|
| 01 Check Dependencies | Validate kind, clusterctl | Validate kubeconfig file exists, kubectl works |
| 02 Setup | Clone cluster-api-installer | Skip (controllers pre-installed) |
| 03 Cluster | Create Kind + deploy controllers | **Validate** controllers are running |
| 04+ | No change | No change (use `GetKubeContext()`) |

#### 4. New Validation Tests (`test/03_cluster_test.go`)

```go
// TestExternalCluster_Connectivity validates the external cluster is reachable
func TestExternalCluster_Connectivity(t *testing.T) {
    config := NewTestConfig()
    if !config.IsExternalCluster() {
        t.Skip("Not using external cluster")
    }

    // Validate kubeconfig file exists
    if !FileExists(config.UseKubeconfig) {
        t.Fatalf("Kubeconfig file not found: %s", config.UseKubeconfig)
    }

    // Set KUBECONFIG for kubectl
    SetEnvVar(t, "KUBECONFIG", config.UseKubeconfig)

    // Test connectivity
    context := config.GetKubeContext()
    output, err := RunCommand(t, "kubectl", "--context", context, "get", "nodes")
    if err != nil {
        t.Fatalf("Cannot connect to cluster: %v", err)
    }
    t.Logf("Cluster nodes:\n%s", output)
}

// TestExternalCluster_ControllersReady validates CAPI/CAPZ/ASO are installed
func TestExternalCluster_ControllersReady(t *testing.T) {
    config := NewTestConfig()
    if !config.IsExternalCluster() {
        t.Skip("Not using external cluster")
    }

    SetEnvVar(t, "KUBECONFIG", config.UseKubeconfig)
    context := config.GetKubeContext()

    // Validate CAPI controller
    _, err := RunCommand(t, "kubectl", "--context", context, "-n", config.CAPINamespace,
        "get", "deployment", "capi-controller-manager")
    if err != nil {
        t.Errorf("CAPI controller not found in %s namespace", config.CAPINamespace)
    }

    // Validate CAPZ controller
    _, err = RunCommand(t, "kubectl", "--context", context, "-n", config.CAPZNamespace,
        "get", "deployment", "capz-controller-manager")
    if err != nil {
        t.Errorf("CAPZ controller not found in %s namespace", config.CAPZNamespace)
    }

    // Validate ASO controller
    _, err = RunCommand(t, "kubectl", "--context", context, "-n", config.CAPZNamespace,
        "get", "deployment", "azureserviceoperator-controller-manager")
    if err != nil {
        t.Errorf("ASO controller not found in %s namespace", config.CAPZNamespace)
    }
}
```

#### 5. Refactor Existing Tests

Replace all occurrences of:
```go
context := fmt.Sprintf("kind-%s", config.ManagementClusterName)
```

With:
```go
context := config.GetKubeContext()
```

And add KUBECONFIG environment variable when in external mode:
```go
if config.IsExternalCluster() {
    SetEnvVar(t, "KUBECONFIG", config.UseKubeconfig)
}
```

---

## Files to Modify

| File | Changes |
|------|---------|
| `test/config.go` | Add `UseKubeconfig` field, `GetKubeContext()`, `IsExternalCluster()` |
| `test/helpers.go` | Add `extractCurrentContext()` helper |
| `test/01_check_dependencies_test.go` | Add kubeconfig validation when `USE_KUBECONFIG` set |
| `test/02_setup_test.go` | Skip when `IsExternalCluster()` |
| `test/03_cluster_test.go` | Add external cluster validation tests, skip Kind tests |
| `test/04_generate_yamls_test.go` | Use `GetKubeContext()` |
| `test/05_deploy_crs_test.go` | Use `GetKubeContext()` |
| `test/06_verification_test.go` | Use `GetKubeContext()` |
| `test/07_deletion_test.go` | Use `GetKubeContext()` |
| `test/08_cleanup_test.go` | Use `GetKubeContext()` |
| `CLAUDE.md` | Document `USE_KUBECONFIG` |
| `README.md` | Add external cluster usage section |

---

## Alternatives Considered

### Alternative 1: Separate Test Suite

**Approach:** Create parallel test files for external cluster mode (e.g., `03_cluster_external_test.go`)

**Pros:**
- No modifications to existing tests
- Clear separation of concerns

**Cons:**
- Code duplication
- Maintenance burden (changes needed in two places)
- Harder to keep in sync

**Decision:** Rejected - conditional logic in single test suite is cleaner

### Alternative 2: USE_KUBECONFIG + KUBE_CONTEXT

**Approach:** Two environment variables for kubeconfig path and context name

**Pros:**
- Explicit context control without modifying kubeconfig
- Useful for multi-context kubeconfigs

**Cons:**
- More configuration complexity
- Potential user confusion

**Decision:** Rejected (YAGNI) - can be added later if needed

### Alternative 3: Auto-detect Cluster Type

**Approach:** Detect if running in Kind or external cluster based on context name

**Pros:**
- No explicit configuration needed

**Cons:**
- Fragile detection logic
- Unclear behavior
- Harder to test

**Decision:** Rejected - explicit configuration is safer

---

## Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| External cluster has different controller versions | Tests may fail unexpectedly | Add version validation in `TestExternalCluster_ControllersReady` |
| Kubeconfig file permissions | Security concern | Validate file permissions (0600) |
| Context doesn't exist in kubeconfig | Silent failure | Validate context exists before proceeding |
| ASO credentials not configured | Deployment will fail | Created issue #435 to address validation |

---

## Testing Plan

### Unit Tests

1. `TestConfig_GetKubeContext_Kind` - Returns `kind-{name}` when no kubeconfig
2. `TestConfig_GetKubeContext_External` - Returns current-context from file
3. `TestConfig_IsExternalCluster` - Returns true/false correctly

### Integration Tests

1. Run full test suite with `USE_KUBECONFIG` pointing to Kind cluster kubeconfig
2. Verify all phases work correctly
3. Verify skip behavior for Kind-specific tests

### Manual Testing

1. Set up MCE cluster with CAPI/CAPZ/ASO
2. Export kubeconfig
3. Run: `USE_KUBECONFIG=/path/to/kubeconfig make test-all`
4. Verify controllers detected and tests proceed

---

## Usage Examples

### Example 1: Run Against MCE Cluster

```bash
# Extract kubeconfig from MCE cluster
oc login https://api.mce-cluster.example.com:6443
oc config view --raw > /tmp/mce-kubeconfig.yaml

# Run tests
export USE_KUBECONFIG=/tmp/mce-kubeconfig.yaml
export AZURE_CLIENT_ID=<client-id>
export AZURE_CLIENT_SECRET=<client-secret>
export AZURE_TENANT_ID=<tenant-id>
export AZURE_SUBSCRIPTION_ID=<subscription-id>

make test-all
```

### Example 2: CI/CD Pipeline (OpenShift CI)

```yaml
# In OpenShift CI job configuration
env:
  USE_KUBECONFIG: /var/run/secrets/ci.openshift.io/kubeconfig

steps:
  - run: make test-all
```

---

## Related Issues

- #433 - Parent issue (this analysis)
- #434 - OpenShift CI and Sippy integration
- #435 - ASO credentials validation in external cluster mode

---

## Appendix: Current Context Usage Locations

Files using `fmt.Sprintf("kind-%s", config.ManagementClusterName)`:

- `test/03_cluster_test.go` (15 occurrences)
- `test/05_deploy_crs_test.go` (10 occurrences)
- `test/06_verification_test.go` (3 occurrences)
- `test/07_deletion_test.go` (5 occurrences)
- `test/helpers.go` (referenced in function parameters)

All occurrences will be refactored to use `config.GetKubeContext()`.
