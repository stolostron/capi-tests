# Test 1: TestDeletion_DeleteCluster

**Location:** `test/07_deletion_test.go:13-54`

**Purpose:** Initiate the deletion of the workload cluster from the management cluster.

---

## Commands Executed

| Command | Purpose |
|---------|---------|
| `kubectl --context <ctx> -n <ns> get cluster <name>` | Check if cluster exists |
| `kubectl --context <ctx> -n <ns> delete cluster <name> --wait=false` | Initiate cluster deletion |

---

## Detailed Flow

```
1. Load configuration:
   └── config := NewTestConfig()

2. Set kubeconfig if external cluster mode

3. Get provisioned cluster name from aro.yaml

4. Check if cluster exists:
   │
   ├── kubectl get cluster <name> -n <namespace>
   │   ├── Not found → Skip: "Cluster not found"
   │   └── Found → Continue
   │
   └── Delete cluster:
       └── kubectl delete cluster <name> --wait=false
           ├── Success → Log "Cluster deletion initiated"
           └── Failure → Fatal error
```

---

## Key Design Decisions

- **`--wait=false`**: Returns immediately so the next test (`WaitForClusterDeletion`) can monitor progress with detailed status reporting
- **Skip on not found**: Idempotent - safe to re-run if cluster was already deleted
- **Uses provisioned cluster name**: Reads the actual cluster name from `aro.yaml` rather than using `WORKLOAD_CLUSTER_NAME` directly

---

## Example Output

```
=== RUN   TestDeletion_DeleteCluster
    07_deletion_test.go:39: Deleting cluster 'rcap-stage' from namespace 'capz-test-20260202-135526'
    07_deletion_test.go:53: Cluster deletion initiated: cluster.cluster.x-k8s.io "rcap-stage" deleted
--- PASS: TestDeletion_DeleteCluster (0.25s)
```
