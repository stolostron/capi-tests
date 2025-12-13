# Test 6: TestDeployment_WaitForControlPlane

**Location:** `test/05_deploy_crs_test.go:250-305`

**Purpose:** Wait for the ARO control plane to become ready, polling until success or timeout.

---

## Command Executed (Polling Loop)

| Command | Purpose |
|---------|---------|
| `kubectl --context <ctx> get arocontrolplane -A -o jsonpath={.items[0].status.ready}` | Check control plane ready status |

---

## Configuration

| Parameter | Value |
|-----------|-------|
| Timeout | `config.DeploymentTimeout` (default: 45m) |
| Poll interval | 30 seconds |
| Target | `arocontrolplane` resource |
| Expected value | `status.ready = true` |

---

## Detailed Flow

```
Configuration:
├── Timeout: DEPLOYMENT_TIMEOUT (default 45m)
├── Poll interval: 30 seconds
└── Target: arocontrolplane (any namespace)

Loop:
│
├─► Check elapsed time > timeout?
│   └─ Yes → FAIL: "Timeout waiting for control plane"
│
├─► Run kubectl get arocontrolplane ... -o jsonpath=...
│   └─ Returns: "true" | "false" | "" | error
│
├─► status == "true"?
│   └─ Yes → PASS: "Control plane is ready!"
│   └─ No  → Continue
│
├─► ReportProgress(iteration, elapsed, remaining, timeout)
│
└─► Sleep 30 seconds, repeat
```

---

## JSONPath Explained

```
{.items[0].status.ready}
```

This extracts the `ready` field from the first AROControlPlane resource:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
kind: AROControlPlane
metadata:
  name: my-aro-cluster
status:
  ready: true  # ← This is what we check
```

---

## Example Output

```
=== Waiting for control plane to be ready ===
Timeout: 45m0s | Poll interval: 30s

[1] Checking control plane status...
[1] 📊 Control plane ready status: false
[2] Checking control plane status...
[2] 📊 Control plane ready status: false
...
[45] Checking control plane status...
[45] 📊 Control plane ready status: true

✅ Control plane is ready! (took 22m30s)
```

---

## Why AROControlPlane?

Unlike standard Kubernetes clusters that use `KubeadmControlPlane`, ARO uses a custom `AROControlPlane` resource because:
- ARO control plane is managed by Azure
- No direct node access to control plane
- Different lifecycle management

---

## Timeout Configuration

The timeout can be configured via environment variable:

```bash
export DEPLOYMENT_TIMEOUT=60m
make _deploy-crs
```

Default is 45 minutes, which is typically sufficient for ARO deployment.
