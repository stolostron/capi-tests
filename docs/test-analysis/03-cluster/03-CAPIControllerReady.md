# Test 3: TestKindCluster_CAPIControllerReady

**Location:** `test/03_cluster_test.go:152-205`

**Purpose:** Wait for CAPI controller manager deployment to become available (timeout: 10m).

---

## Command Executed (Polling Loop)

| Command | Purpose |
|---------|---------|
| `kubectl --context kind-<name> -n capi-system get deployment capi-controller-manager -o jsonpath={.status.conditions[?(@.type=='Available')].status}` | Check if deployment is Available |

---

## Configuration

| Parameter | Value |
|-----------|-------|
| Timeout | 10 minutes |
| Poll interval | 10 seconds |
| Namespace | `capi-system` |
| Deployment | `capi-controller-manager` |

---

## Detailed Flow

```
Configuration:
├── Timeout: 10 minutes
├── Poll interval: 10 seconds
└── Target: capi-system/capi-controller-manager

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
=== Waiting for CAPI controller manager ===
Namespace: capi-system
Deployment: capi-controller-manager
Timeout: 10m0s | Poll interval: 10s

[1] Checking deployment status...
[1] 📊 Deployment Available status: False
[2] Checking deployment status...
[2] 📊 Deployment Available status: False
[3] Checking deployment status...
[3] 📊 Deployment Available status: True

✅ CAPI controller manager is available! (took 25s)
```
