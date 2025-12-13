# Test 5: TestVerification_ClusterHealth

**Location:** `test/06_verification_test.go:174-208`

**Purpose:** Perform basic health checks on the workload cluster.

---

## Commands Executed

| Step | Command | Purpose |
|------|---------|---------|
| 1 | `kubectl get pods -n kube-system` | Check system pods |
| 2 | `kubectl get pods -A --field-selector=status.phase!=Running,status.phase!=Succeeded` | Find non-running pods |

---

## Detailed Flow

```
1. Check prerequisite:
   └─ os.Getenv("ARO_CLUSTER_KUBECONFIG") != ""?
      └─ No → SKIP

2. Check kubeconfig file exists:
   └─ FileExists(kubeconfigPath)?
      └─ No → SKIP

3. Set KUBECONFIG:
   └─ SetEnvVar(t, "KUBECONFIG", kubeconfigPath)

4. Check system pods:
   └─ kubectl get pods -n kube-system
      ├─ Success → Log pod list
      └─ Failure → Log warning

5. Find non-running pods:
   └─ kubectl get pods -A --field-selector=status.phase!=Running,status.phase!=Succeeded
      └─ Check output:
         ├─ Empty or header only → Log "All pods are in Running or Succeeded state"
         └─ Has content → Log warning with pod list
```

---

## Field Selector Explained

```
--field-selector=status.phase!=Running,status.phase!=Succeeded
```

This finds pods in problematic states:
- `Pending` - Waiting to be scheduled
- `Failed` - Pod has failed
- `Unknown` - Pod status cannot be determined
- `ContainerCreating` - Containers still starting

---

## Example Output

### Healthy Cluster
```
=== RUN   TestVerification_ClusterHealth
    06_verification_test.go:189: Checking system pods...
    06_verification_test.go:195: System pods:
NAME                                      READY   STATUS    RESTARTS   AGE
coredns-xxxxx                             1/1     Running   0          30m
coredns-yyyyy                             1/1     Running   0          30m
kube-proxy-zzzzz                          1/1     Running   0          30m
    06_verification_test.go:205: All pods are in Running or Succeeded state
--- PASS: TestVerification_ClusterHealth (0.35s)
```

### Cluster with Issues
```
=== RUN   TestVerification_ClusterHealth
    06_verification_test.go:189: Checking system pods...
    06_verification_test.go:195: System pods:
...
    06_verification_test.go:203: Warning: Found non-running pods:
NAMESPACE     NAME                    READY   STATUS    RESTARTS   AGE
kube-system   failing-pod-xxxxx       0/1     Pending   0          5m
openshift     stuck-pod-yyyyy         0/1     Failed    3          10m
--- PASS: TestVerification_ClusterHealth (0.40s)
```

---

## Key Namespaces Checked

| Namespace | Contents |
|-----------|----------|
| `kube-system` | Core Kubernetes components |
| `openshift-*` | OpenShift components |
| All (`-A`) | Complete cluster view |

---

## Summary

This final test provides a comprehensive health check:
1. Verifies core system pods are running
2. Identifies any pods in problematic states
3. Provides diagnostic information for troubleshooting
