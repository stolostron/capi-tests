# Test 7: TestDeployment_CheckClusterConditions

**Location:** `test/05_deploy_crs_test.go:307-361`

**Purpose:** Check various cluster conditions to verify deployment health.

---

## Commands Executed

| Step | Command | Purpose |
|------|---------|---------|
| 1 | `kubectl get cluster <name> -o yaml` | Get full cluster status |
| 2 | `kubectl get cluster <name> -o jsonpath={...InfrastructureReady...}` | Check infra condition |
| 3 | `kubectl get cluster <name> -o jsonpath={...ControlPlaneReady...}` | Check control plane condition |

---

## Detailed Flow

```
1. Get full cluster status:
   └─ kubectl --context <ctx> get cluster <name> -o yaml
      ├─ Success → Check for "status:" and "conditions:" sections
      └─ Failure → FAIL

2. Check InfrastructureReady condition:
   └─ kubectl ... -o jsonpath={.status.conditions[?(@.type=='InfrastructureReady')].status}
      └─ Log result (True/False/Unknown)

3. Check ControlPlaneReady condition:
   └─ kubectl ... -o jsonpath={.status.conditions[?(@.type=='ControlPlaneReady')].status}
      └─ Log result (True/False/Unknown)
```

---

## Cluster Conditions

CAPI clusters have standard conditions:

| Condition | Description |
|-----------|-------------|
| `InfrastructureReady` | Infrastructure (AROCluster) is provisioned |
| `ControlPlaneReady` | Control plane is running and healthy |
| `Ready` | Cluster is fully ready |

---

## JSONPath for Conditions

```
{.status.conditions[?(@.type=='InfrastructureReady')].status}
```

This filters conditions by type and extracts the status:

```yaml
status:
  conditions:
    - type: InfrastructureReady
      status: "True"           # ← Extracted
      reason: AROClusterReady
    - type: ControlPlaneReady
      status: "True"           # ← Extracted
      reason: AROControlPlaneReady
```

---

## Example Output

```
=== Checking cluster conditions ===
Cluster: capz-tests-cluster

Fetching cluster status...
✅ Cluster has status information
✅ Cluster conditions are available

Checking InfrastructureReady condition...
📊 InfrastructureReady status: True

Checking ControlPlaneReady condition...
📊 ControlPlaneReady status: True

=== Cluster condition check complete ===
```

---

## Informational Test

This test is primarily **informational**:
- Provides visibility into cluster state
- Helps debug issues when verification tests fail
- Does not fail if conditions are not yet True (deployment may still be in progress)
