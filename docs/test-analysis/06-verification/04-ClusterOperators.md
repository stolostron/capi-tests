# Test 4: TestVerification_ClusterOperators

**Location:** `test/06_verification_test.go:149-172`

**Purpose:** Check the status of OpenShift cluster operators.

---

## Command Executed

| Command | Purpose |
|---------|---------|
| `oc get clusteroperators` | List all cluster operators and their status |

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

4. Get cluster operators:
   └─ oc get clusteroperators
      ├─ Success → Log operator status
      └─ Failure → Log warning (non-fatal)
```

---

## What are Cluster Operators?

OpenShift uses Cluster Operators to manage core components:

| Operator | Description |
|----------|-------------|
| `authentication` | OAuth and authentication |
| `console` | Web console |
| `dns` | CoreDNS |
| `etcd` | etcd cluster |
| `ingress` | Ingress controller |
| `kube-apiserver` | Kubernetes API server |
| `machine-api` | Machine management |
| `network` | Cluster networking |
| `openshift-apiserver` | OpenShift API server |
| `storage` | Storage management |

---

## Example Output

```
=== RUN   TestVerification_ClusterOperators
    06_verification_test.go:161: Checking cluster operators...
    06_verification_test.go:171: Cluster operators:
NAME                                       VERSION   AVAILABLE   PROGRESSING   DEGRADED   SINCE
authentication                             4.14.5    True        False         False      25m
cloud-controller-manager                   4.14.5    True        False         False      30m
cloud-credential                           4.14.5    True        False         False      30m
cluster-autoscaler                         4.14.5    True        False         False      28m
console                                    4.14.5    True        False         False      20m
dns                                        4.14.5    True        False         False      28m
...
--- PASS: TestVerification_ClusterOperators (0.40s)
```

---

## Operator Status Columns

| Column | Meaning |
|--------|---------|
| `AVAILABLE` | Operator is functioning |
| `PROGRESSING` | Operator is updating |
| `DEGRADED` | Operator has issues |

Healthy state: `AVAILABLE=True`, `PROGRESSING=False`, `DEGRADED=False`

---

## Non-Fatal Failure

This test logs failures rather than failing because:
- Some operators may still be initializing
- Provides useful diagnostic information
- Doesn't block other verification tests
