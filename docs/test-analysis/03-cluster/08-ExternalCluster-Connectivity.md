# Test 8: TestExternalCluster_01_Connectivity

**Location:** `test/03_cluster_test.go:14-40`

**Purpose:** Validate the external cluster is reachable via the provided kubeconfig.

---

## Prerequisites

- `USE_KUBECONFIG` must be set (external cluster mode)

---

## Commands Executed

| Command | Purpose |
|---------|---------|
| `kubectl --context <ctx> get nodes` | Verify cluster connectivity |

---

## Detailed Flow

```
1. Skip if not external cluster mode

2. Set KUBECONFIG and extract context

3. Test connectivity:
   └── kubectl --context <ctx> get nodes
       ├── Error → Fatal: "Cannot connect to external cluster"
       └── Success → Display node list
```

---

## Key Notes

- Only runs when `USE_KUBECONFIG` is set
- Uses `current-context` from the provided kubeconfig file
- Provides the baseline connectivity check before MCE tests
