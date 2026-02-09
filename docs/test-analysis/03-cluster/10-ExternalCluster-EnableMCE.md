# Test 10: TestExternalCluster_02_EnableMCE

**Location:** `test/03_cluster_test.go:187-296`

**Purpose:** Enable CAPI and CAPZ components in MCE if not already enabled.

---

## Prerequisites

- `USE_KUBECONFIG` must be set (external cluster mode)
- Cluster must be an MCE installation
- `MCE_AUTO_ENABLE` must be `true` (default when `USE_KUBECONFIG` is set)

---

## MCE Components Enabled

| Component | MCE Name |
|-----------|----------|
| CAPI | `cluster-api` |
| CAPZ | `cluster-api-provider-azure-preview` |

---

## Detailed Flow

```
1. Skip if: not external cluster, not MCE, or MCE_AUTO_ENABLE=false

2. Check each component status:
   └── GetMCEComponentStatus(context, component)
       ├── Already enabled → Skip
       └── Disabled → EnableMCEComponent(context, component)
           ├── HyperShift exclusivity error → Fatal with remediation steps
           └── Other error → Fatal with troubleshooting

3. If any components were enabled:
   ├── Wait 30 seconds for MCE reconciliation
   └── Wait for each controller deployment to become available:
       ├── CAPI controller (capi-controller-manager)
       ├── CAPZ controller (capz-controller-manager)
       └── ASO controller (azureserviceoperator-controller-manager)
       └── Timeout: MCE_ENABLEMENT_TIMEOUT (default 15m)
```

---

## HyperShift Exclusivity

MCE enforces component exclusivity between HyperShift and Cluster API. If HyperShift is enabled, CAPI/CAPZ cannot be enabled simultaneously. The test provides remediation steps:

```bash
kubectl patch mce multiclusterengine --type=merge -p '
  {"spec":{"overrides":{"components":[
    {"name":"hypershift","enabled":false},
    {"name":"hypershift-local-hosting","enabled":false}
  ]}}}'
```

---

## Key Notes

- Uses `jq` for JSON transformation when patching MCE (hence optional `jq` dependency)
- Initial 30-second wait allows MCE operator to start reconciling
- Controller readiness is verified by checking deployment Available condition
