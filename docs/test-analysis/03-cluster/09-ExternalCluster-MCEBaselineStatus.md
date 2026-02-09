# Test 9: TestExternalCluster_01b_MCEBaselineStatus

**Location:** `test/03_cluster_test.go:70-180`

**Purpose:** Validate and configure MCE component baseline before enabling CAPI/CAPZ. Ensures HyperShift is disabled (required for CAPI/CAPZ due to MCE component exclusivity).

---

## Prerequisites

- `USE_KUBECONFIG` must be set (external cluster mode)
- Cluster must be an MCE installation

---

## Expected MCE Component States

| Component | Expected State |
|-----------|---------------|
| `local-cluster` | enabled |
| `assisted-service` | enabled |
| `cluster-lifecycle` | enabled |
| `cluster-manager` | enabled |
| `discovery` | enabled |
| `hive` | enabled |
| `server-foundation` | enabled |
| `cluster-proxy-addon` | enabled |
| `managedserviceaccount` | enabled |
| `hypershift` | **disabled** |
| `hypershift-local-hosting` | **disabled** |

---

## Detailed Flow

```
1. Skip if not external cluster or not MCE cluster

2. For each expected component:
   └── GetMCEComponentStatus(context, component)
       ├── Matches expected → Display status
       └── Doesn't match → Queue for correction

3. If components need correction:
   └── For each component to fix:
       └── SetMCEComponentState(context, name, enabled)
           ├── Success → Report change
           └── Failure → Fatal error
```

---

## Key Notes

- HyperShift and Cluster API components are **mutually exclusive** in MCE
- This test automatically corrects mismatched component states
- Must run before `TestExternalCluster_02_EnableMCE` to ensure proper baseline
