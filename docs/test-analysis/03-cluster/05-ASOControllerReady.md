# Test 5: TestKindCluster_ASOControllerReady

**Location:** `test/03_cluster_test.go:262-315`

**Purpose:** Wait for Azure Service Operator controller manager to become available (timeout: 10m).

---

## Command Executed (Polling Loop)

| Command | Purpose |
|---------|---------|
| `kubectl --context kind-<name> -n capz-system get deployment azureserviceoperator-controller-manager -o jsonpath={.status.conditions[?(@.type=='Available')].status}` | Check if deployment is Available |

---

## Configuration

| Parameter | Value |
|-----------|-------|
| Timeout | 10 minutes |
| Poll interval | 10 seconds |
| Namespace | `capz-system` |
| Deployment | `azureserviceoperator-controller-manager` |

---

## Detailed Flow

```
Configuration:
├── Timeout: 10 minutes
├── Poll interval: 10 seconds
└── Target: capz-system/azureserviceoperator-controller-manager

Loop:
│
├─► Check elapsed time > 10m?
│   └─ Yes → FAIL test, exit
│
├─► Run kubectl get deployment ... -o jsonpath=...
│   └─ Returns: "True" | "False" | "" | error
│
├─► Status == "True"?
│   └─ Yes → PASS test, exit
│   └─ No  → Continue
│
├─► Log progress (iteration, elapsed, remaining)
│
└─► Sleep 10 seconds, repeat
```

---

## JSONPath Explained

```
{.status.conditions[?(@.type=='Available')].status}
```

This extracts the `status` field from the condition where `type == "Available"`:

```yaml
# Example deployment status:
status:
  conditions:
    - type: Available
      status: "True"      # ← This is what we extract
    - type: Progressing
      status: "True"
```

---

## Example Output

```
=== Waiting for Azure Service Operator controller manager ===
Namespace: capz-system
Deployment: azureserviceoperator-controller-manager
Timeout: 10m0s | Poll interval: 10s

[1] Checking deployment status...
[1] 📊 Deployment Available status: False
[2] Checking deployment status...
[2] 📊 Deployment Available status: False
[3] Checking deployment status...
[3] 📊 Deployment Available status: True

✅ Azure Service Operator controller manager is available! (took 25s)
```

---

## What is Azure Service Operator (ASO)?

ASO is a Kubernetes operator that enables management of Azure resources directly from Kubernetes. It:

- Translates Kubernetes custom resources into Azure API calls
- Manages lifecycle of Azure resources (create, update, delete)
- Reports Azure resource status back to Kubernetes

In the context of CAPZ, ASO is used to provision Azure infrastructure (VNets, subnets, managed identities, etc.) for workload clusters.

---

## Comparison of All Controller Tests

| Test | Namespace | Deployment | Purpose |
|------|-----------|------------|---------|
| Test 3 | `capi-system` | `capi-controller-manager` | Core Cluster API |
| Test 4 | `capz-system` | `capz-controller-manager` | Azure provider for CAPI |
| Test 5 | `capz-system` | `azureserviceoperator-controller-manager` | Azure resource management |

All three tests use identical polling logic with 10m timeout and 10s intervals.
