# Test 4: TestDeployment_ApplyAROClusterYAML

**Location:** `test/05_deploy_crs_test.go:143-176`

**Purpose:** Apply `aro.yaml` (ARO cluster configuration) to the Kind cluster.

---

## Command Executed

| Command | Purpose |
|---------|---------|
| `kubectl --context kind-<name> apply -f <path>/aro.yaml` | Apply ARO cluster resources |

---

## Detailed Flow

```
1. Check prerequisite:
   └─ DirExists(outputDir)?
      └─ No → SKIP: "Output directory does not exist"

2. Check file exists:
   └─ FileExists(filePath)?
      └─ No → FAIL: "aro.yaml not found"

3. Build kubectl context:
   └─ context = "kind-<ManagementClusterName>"

4. Apply file:
   └─ kubectl --context <ctx> apply -f <path>
      ├─ Success → Log "Successfully applied"
      └─ Failure → Check IsKubectlApplySuccess(output)
         ├─ True → Continue (resource unchanged)
         └─ False → FAIL
```

---

## Example Output

```
=== Applying aro.yaml (ARO cluster configuration) ===
✅ Successfully applied aro.yaml
```

---

## What aro.yaml Contains

The main ARO cluster definition with multiple resources:

| Resource | API Version | Description |
|----------|-------------|-------------|
| `Cluster` | `cluster.x-k8s.io/v1beta1` | CAPI Cluster resource |
| `AROCluster` | `infrastructure.cluster.x-k8s.io/v1alpha1` | ARO infrastructure |
| `AROControlPlane` | `infrastructure.cluster.x-k8s.io/v1alpha1` | ARO control plane |

---

## What Happens After Apply

Once `aro.yaml` is applied:
1. CAPI controller creates the Cluster resource
2. CAPZ controller reconciles AROCluster
3. ASO creates Azure resources (VNet, subnets, etc.)
4. AROControlPlane controller provisions the ARO cluster in Azure

This triggers the long-running deployment process monitored by subsequent tests.
