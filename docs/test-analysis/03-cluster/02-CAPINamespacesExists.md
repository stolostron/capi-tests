# Test 2: TestKindCluster_CAPINamespacesExists

**Location:** `test/03_cluster_test.go:103-150`

**Purpose:** Verify CAPI and CAPZ namespaces exist in the management cluster.

---

## Commands Executed

| Step | Command | Purpose |
|------|---------|---------|
| 1 | `kubectl --context kind-<name> get namespace capi-system` | Check if CAPI namespace exists |
| 2 | `kubectl --context kind-<name> get namespace capz-system` | Check if CAPZ namespace exists |
| 3 | *(5 second sleep)* | Wait for controllers to initialize |
| 4 | `kubectl --context kind-<name> get pods -A --selector=cluster.x-k8s.io/provider` | List all CAPI-related pods |

---

## Detailed Flow

```
1. Loop through expected namespaces:
   - capi-system
   - capz-system

   For each namespace:
   └─ Run: kubectl --context kind-<name> get namespace <ns>
      └─ Success → Log "Found namespace: <ns>"
      └─ Failure → Log warning (non-fatal, test continues)

2. Sleep 5 seconds (wait for controllers)

3. Run: kubectl get pods -A --selector=cluster.x-k8s.io/provider
   └─ Lists pods with CAPI provider label across all namespaces
   └─ Failure → Log warning (non-fatal)
   └─ Success → Log pod list
```

---

## Key Observations

- **Non-blocking test**: Failures here only produce warnings, not test failures
- **Informational**: This test is more about visibility than validation
- The `--selector=cluster.x-k8s.io/provider` finds pods labeled by CAPI providers

---

## Example Output

```
=== Checking for CAPI namespaces ===
Checking namespace: capi-system...
✅ Found namespace: capi-system
Checking namespace: capz-system...
✅ Found namespace: capz-system

Waiting 5 seconds for controllers to initialize...

=== Checking for CAPI pods ===
Running: kubectl get pods -A --selector=cluster.x-k8s.io/provider
✅ CAPI pods found:
NAMESPACE     NAME                                     READY   STATUS
capi-system   capi-controller-manager-xxx              1/1     Running
capz-system   capz-controller-manager-xxx              1/1     Running
capz-system   azureserviceoperator-controller-xxx      1/1     Running
```
