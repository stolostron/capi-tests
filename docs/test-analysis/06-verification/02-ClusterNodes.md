# Test 2: TestVerification_ClusterNodes

**Location:** `test/06_verification_test.go:90-122`

**Purpose:** Verify the workload cluster has accessible nodes.

---

## Command Executed

| Command | Purpose |
|---------|---------|
| `kubectl get nodes` | List cluster nodes (using workload kubeconfig) |

---

## Detailed Flow

```
1. Check prerequisite:
   └─ os.Getenv("ARO_CLUSTER_KUBECONFIG") != ""?
      └─ No → SKIP: "Run TestVerification_RetrieveKubeconfig first"

2. Check kubeconfig file exists:
   └─ FileExists(kubeconfigPath)?
      └─ No → SKIP: "Kubeconfig file not found"

3. Set KUBECONFIG:
   └─ SetEnvVar(t, "KUBECONFIG", kubeconfigPath)

4. Get nodes:
   └─ kubectl get nodes
      ├─ Success → Log node list, count nodes
      └─ Failure → FAIL

5. Verify node count:
   └─ len(lines) >= 2? (header + at least 1 node)
      └─ No → FAIL: "Expected at least one node"
```

---

## Example Output

```
=== RUN   TestVerification_ClusterNodes
    06_verification_test.go:102: Checking cluster nodes...
    06_verification_test.go:112: Cluster nodes:
NAME                                    STATUS   ROLES    AGE   VERSION
my-cluster-master-0                     Ready    master   30m   v1.27.3
my-cluster-master-1                     Ready    master   29m   v1.27.3
my-cluster-master-2                     Ready    master   28m   v1.27.3
my-cluster-worker-uksouth1-xxxxx        Ready    worker   25m   v1.27.3
my-cluster-worker-uksouth2-xxxxx        Ready    worker   24m   v1.27.3
    06_verification_test.go:121: Cluster has 5 node(s)
--- PASS: TestVerification_ClusterNodes (0.50s)
```

---

## Node Count Validation

```go
lines := strings.Split(output, "\n")
if len(lines) < 2 { // Header + at least one node
    t.Errorf("Expected at least one node")
}
```

---

## Dependency

This test depends on `TestVerification_RetrieveKubeconfig` setting the `ARO_CLUSTER_KUBECONFIG` environment variable.
