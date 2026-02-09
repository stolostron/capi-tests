# Test 11: TestExternalCluster_03_ControllersReady

**Location:** `test/03_cluster_test.go:301-365`

**Purpose:** Validate CAPI/CAPZ/ASO controllers are installed on the external cluster.

---

## Prerequisites

- `USE_KUBECONFIG` must be set (external cluster mode)

---

## Controllers Checked

| Controller | Namespace | Deployment |
|------------|-----------|------------|
| CAPI | `config.CAPINamespace` | `capi-controller-manager` |
| CAPZ | `config.CAPZNamespace` | `capz-controller-manager` |
| ASO | `config.CAPZNamespace` | `azureserviceoperator-controller-manager` |

---

## Detailed Flow

```
1. Skip if not external cluster mode

2. Check if MCE cluster (for error message context)

3. For each controller:
   └── kubectl get deployment <name> -n <namespace>
       ├── Error → Report missing controller
       │   ├── MCE cluster + MCE_AUTO_ENABLE=false → Suggest enabling
       │   └── Other → Report error
       └── Success → "Controller manager found"
```

---

## Key Notes

- Provides MCE-specific remediation hints when controllers are missing
- If MCE_AUTO_ENABLE is false, suggests enabling it
- Runs after `TestExternalCluster_02_EnableMCE` so controllers should be available
- The CAPINamespace/CAPZNamespace may differ between Kind and MCE deployments
