# Test 1: TestVerification_RetrieveKubeconfig

**Location:** `test/06_verification_test.go:12-88`

**Purpose:** Retrieve the kubeconfig for the workload cluster from the management cluster.

---

## Commands Executed

| Method | Command | Purpose |
|--------|---------|---------|
| 1 | `kubectl get secret <cluster>-kubeconfig -o jsonpath={.data.value}` | Get base64 kubeconfig |
| 2 | `clusterctl get kubeconfig <cluster>` | Fallback method |

---

## Detailed Flow

```
1. Build secret name:
   └─ secretName = "<WorkloadClusterName>-kubeconfig"

2. Method 1 - kubectl get secret:
   │
   └─► kubectl --context <ctx> get secret <name> -o jsonpath={.data.value}
       │
       ├─ Success:
       │  ├─ Validate output not empty
       │  ├─ base64.StdEncoding.DecodeString(output)
       │  ├─ Validate decoded not empty
       │  └─ os.WriteFile(kubeconfigPath, decoded, 0600)
       │
       └─ Failure → Try Method 2

3. Method 2 - clusterctl (fallback):
   │
   ├─► Find clusterctl binary:
   │   ├─ Check <RepoDir>/<ClusterctlBinPath>
   │   └─ Fallback to system PATH
   │
   └─► clusterctl get kubeconfig <cluster>
       ├─ Success → os.WriteFile(kubeconfigPath, output, 0600)
       └─ Failure → FAIL: "Both methods failed"

4. Set environment variable:
   └─ ARO_CLUSTER_KUBECONFIG = kubeconfigPath
```

---

## Kubeconfig Path

```go
kubeconfigPath := filepath.Join(os.TempDir(),
    fmt.Sprintf("%s-kubeconfig.yaml", config.WorkloadClusterName))
```

Example: `/tmp/capz-tests-cluster-kubeconfig.yaml`

---

## Base64 Decoding

The kubeconfig is stored as a base64-encoded secret:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: my-cluster-kubeconfig
type: cluster.x-k8s.io/secret
data:
  value: YXBpVmVyc2lvbjog...  # base64 encoded
```

The test uses Go's `encoding/base64` package for safe decoding (no shell command injection risk).

---

## Example Output

```
=== RUN   TestVerification_RetrieveKubeconfig
    06_verification_test.go:21: Retrieving kubeconfig for cluster 'capz-tests-cluster'
    06_verification_test.go:26: Attempting Method 1: kubectl get secret capz-tests-cluster-kubeconfig...
    06_verification_test.go:83: Kubeconfig retrieved using kubectl and saved to /tmp/capz-tests-cluster-kubeconfig.yaml
--- PASS: TestVerification_RetrieveKubeconfig (0.15s)
```

---

## Security

- Kubeconfig is written with `0600` permissions (owner read/write only)
- The path is stored in environment variable for subsequent tests
- No sensitive data is logged
