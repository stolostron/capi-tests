# Phase 3: Cluster

**Make target:** `make _cluster`
**Test file:** `test/03_cluster_test.go`
**Timeout:** 30 minutes

---

## Purpose

Deploy a Kind cluster with CAPI, CAPZ, and ASO controllers, then verify all controllers are ready.

## Test Summary

| # | Test | File | Purpose |
|---|------|------|---------|
| 1 | [01-KindClusterReady](01-KindClusterReady.md) | `TestKindCluster_KindClusterReady` | Deploy Kind cluster with CAPI/CAPZ/ASO controllers |
| 2 | [02-CAPINamespacesExists](02-CAPINamespacesExists.md) | `TestKindCluster_CAPINamespacesExists` | Verify CAPI namespaces exist |
| 3 | [03-CAPIControllerReady](03-CAPIControllerReady.md) | `TestKindCluster_CAPIControllerReady` | Wait for CAPI controller (10m timeout) |
| 4 | [04-CAPZControllerReady](04-CAPZControllerReady.md) | `TestKindCluster_CAPZControllerReady` | Wait for CAPZ controller (10m timeout) |
| 5 | [05-ASOControllerReady](05-ASOControllerReady.md) | `TestKindCluster_ASOControllerReady` | Wait for ASO controller (10m timeout) |

---

## Flow Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                    make _cluster                                 │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 1: KindClusterReady                                        │
│  ├── Check if cluster exists (kind get clusters)                 │
│  ├── If not: run deploy-charts-kind-capz.sh                      │
│  │   ├── Create Kind cluster                                     │
│  │   ├── Install cert-manager                                    │
│  │   ├── Deploy CAPI charts                                      │
│  │   ├── Deploy CAPZ charts                                      │
│  │   └── Wait for controllers                                    │
│  └── Verify: kubectl get nodes                                   │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 2: CAPINamespacesExists                                    │
│  ├── Check namespace: capi-system                                │
│  ├── Check namespace: capz-system                                │
│  └── List CAPI pods (informational)                              │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 3: CAPIControllerReady                                     │
│  └── Poll until capi-controller-manager Available=True           │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 4: CAPZControllerReady                                     │
│  └── Poll until capz-controller-manager Available=True           │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 5: ASOControllerReady                                      │
│  └── Poll until azureserviceoperator-controller-manager          │
│      Available=True                                              │
└─────────────────────────────────────────────────────────────────┘
```

---

## Components Deployed

| Component | Namespace | Description |
|-----------|-----------|-------------|
| cert-manager | `cert-manager` | TLS certificate management |
| CAPI | `capi-system` | Cluster API core controllers |
| CAPZ | `capz-system` | Cluster API Azure provider |
| ASO | `capz-system` | Azure Service Operator |

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `MANAGEMENT_CLUSTER_NAME` | `capz-tests-stage` | Kind cluster name |
| `ARO_REPO_DIR` | `/tmp/cluster-api-installer-aro` | Path to cluster-api-installer |
| `CLUSTER_TIMEOUT` | `30m` | Make target timeout |

---

## Source File

All tests are defined in: `test/03_cluster_test.go`
