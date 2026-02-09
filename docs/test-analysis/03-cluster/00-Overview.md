# Phase 3: Cluster

**Make target:** `make _cluster`
**Test file:** `test/03_cluster_test.go`
**Timeout:** 30 minutes

---

## Purpose

Deploy a Kind cluster with CAPI, CAPZ, and ASO controllers, then verify all controllers are ready. When using an external cluster (`USE_KUBECONFIG`), validates pre-installed controllers and optionally enables MCE components.

---

## Test Summary

### External Cluster Tests (when USE_KUBECONFIG is set)

| # | Test | Purpose |
|---|------|---------|
| 1 | [08-ExternalCluster-Connectivity](08-ExternalCluster-Connectivity.md) | Validate external cluster connectivity |
| 2 | [09-ExternalCluster-MCEBaselineStatus](09-ExternalCluster-MCEBaselineStatus.md) | Validate and configure MCE component baseline |
| 3 | [10-ExternalCluster-EnableMCE](10-ExternalCluster-EnableMCE.md) | Enable CAPI/CAPZ components in MCE |
| 4 | [11-ExternalCluster-ControllersReady](11-ExternalCluster-ControllersReady.md) | Validate pre-installed controllers |

### Kind Cluster Tests (default mode)

| # | Test | Purpose |
|---|------|---------|
| 5 | [01-KindClusterReady](01-KindClusterReady.md) | Deploy Kind cluster with CAPI/CAPZ/ASO controllers |
| 6 | [02-CAPINamespacesExists](02-CAPINamespacesExists.md) | Verify CAPI namespaces exist |
| 7 | [03-CAPIControllerReady](03-CAPIControllerReady.md) | Wait for CAPI controller (10m timeout) |
| 8 | [04-CAPZControllerReady](04-CAPZControllerReady.md) | Wait for CAPZ controller (10m timeout) |
| 9 | [05-ASOCredentialsConfigured](05-ASOCredentialsConfigured.md) | Validate ASO credentials secret |
| 10 | [06-ASOControllerReady](06-ASOControllerReady.md) | Wait for ASO controller (configurable timeout) |
| 11 | [07-WebhooksReady](07-WebhooksReady.md) | Wait for CAPI/CAPZ/ASO/MCE webhooks (5m timeout) |

---

## Flow Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                    make _cluster                                 │
└─────────────────────────────────────────────────────────────────┘
                              │
                    ┌─────────┴─────────┐
                    │                   │
           USE_KUBECONFIG?          Kind mode
                    │                   │
                    ▼                   ▼
┌──────────────────────────┐  ┌──────────────────────────┐
│  EXTERNAL CLUSTER PATH    │  │  KIND CLUSTER PATH        │
│                           │  │                           │
│  1. Connectivity check    │  │  5. Deploy Kind cluster   │
│  2. MCE baseline status   │  │  6. Check CAPI namespaces │
│  3. Enable MCE CAPI/CAPZ  │  │  7. Wait CAPI controller  │
│  4. Verify controllers    │  │  8. Wait CAPZ controller  │
│                           │  │  9. Verify ASO credentials│
└──────────────────────────┘  │  10. Wait ASO controller   │
                              │  11. Wait for webhooks     │
                              └──────────────────────────┘
```

---

## Components Deployed (Kind Mode)

| Component | Namespace | Description |
|-----------|-----------|-------------|
| cert-manager | `cert-manager` | TLS certificate management |
| CAPI | `capi-system` | Cluster API core controllers |
| CAPZ | `capz-system` | Cluster API Azure provider |
| ASO | `capz-system` | Azure Service Operator |

---

## MCE Components (External Cluster Mode)

| Component | MCE Name | Expected State |
|-----------|----------|---------------|
| CAPI | `cluster-api` | enabled |
| CAPZ | `cluster-api-provider-azure-preview` | enabled |
| HyperShift | `hypershift` | **disabled** (mutual exclusion with CAPI) |

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `MANAGEMENT_CLUSTER_NAME` | `capz-tests-stage` | Kind cluster name |
| `ARO_REPO_DIR` | `/tmp/cluster-api-installer-aro` | Path to cluster-api-installer |
| `USE_KUBECONFIG` | (unset) | Path to external cluster kubeconfig |
| `MCE_AUTO_ENABLE` | `true` (when USE_KUBECONFIG set) | Auto-enable MCE components |
| `MCE_ENABLEMENT_TIMEOUT` | `15m` | Timeout for MCE component enablement |
| `ASO_CONTROLLER_TIMEOUT` | `10m` | Timeout for ASO controller readiness |

---

## Source File

All tests are defined in: `test/03_cluster_test.go`
