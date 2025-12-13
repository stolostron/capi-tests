# Test 3: TestVerification_ClusterVersion

**Location:** `test/06_verification_test.go:124-147`

**Purpose:** Verify the OpenShift cluster version.

---

## Command Executed

| Command | Purpose |
|---------|---------|
| `oc version` | Get OpenShift client and server version |

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

4. Get version:
   └─ oc version
      ├─ Success → Log version info
      └─ Failure → Log warning (non-fatal, cluster may still be provisioning)
```

---

## Example Output

```
=== RUN   TestVerification_ClusterVersion
    06_verification_test.go:136: Checking OpenShift cluster version...
    06_verification_test.go:146: OpenShift version:
Client Version: 4.14.0
Kustomize Version: v5.0.1
Server Version: 4.14.5
Kubernetes Version: v1.27.6+b49f9d1
--- PASS: TestVerification_ClusterVersion (0.30s)
```

---

## Non-Fatal Failure

This test uses `t.Logf` instead of `t.Errorf` for failures because:
- The cluster may still be provisioning
- Server version requires API access which may not be ready
- Client version is still useful information

---

## Why `oc` Instead of `kubectl`?

OpenShift CLI (`oc`) provides:
- OpenShift-specific version information
- Server version with OpenShift release number
- Better compatibility with OpenShift clusters
