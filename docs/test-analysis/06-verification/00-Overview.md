# Phase 6: Verification

**Make target:** `make _verify`
**Test file:** `test/06_verification_test.go`
**Timeout:** 20 minutes

---

## Purpose

Verify the deployed ARO cluster is accessible and healthy by retrieving kubeconfig and running validation commands.

---

## Test Summary

| # | Test | Purpose |
|---|------|---------|
| 1 | [01-RetrieveKubeconfig](01-RetrieveKubeconfig.md) | Get kubeconfig from cluster secret |
| 2 | [02-ClusterNodes](02-ClusterNodes.md) | Verify nodes are available |
| 3 | [03-ClusterVersion](03-ClusterVersion.md) | Check OpenShift version |
| 4 | [04-ClusterOperators](04-ClusterOperators.md) | Verify cluster operators |
| 5 | [05-ClusterHealth](05-ClusterHealth.md) | Check overall cluster health |

---

## Flow Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                    make _verify                                  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 1: RetrieveKubeconfig                                      │
│  ├── Method 1: kubectl get secret <cluster>-kubeconfig           │
│  └── Method 2: clusterctl get kubeconfig <cluster>               │
│  └── Save to /tmp/<cluster>-kubeconfig.yaml                      │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 2: ClusterNodes                                            │
│  └── kubectl get nodes (using retrieved kubeconfig)              │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 3: ClusterVersion                                          │
│  └── oc version                                                  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 4: ClusterOperators                                        │
│  └── oc get clusteroperators                                     │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 5: ClusterHealth                                           │
│  ├── kubectl get pods -n kube-system                             │
│  └── kubectl get pods -A --field-selector=status.phase!=Running  │
└─────────────────────────────────────────────────────────────────┘
```

---

## Kubeconfig Flow

```
Management Cluster (Kind)          Workload Cluster (ARO)
┌─────────────────────┐            ┌─────────────────────┐
│                     │            │                     │
│  Secret:            │  decode    │                     │
│  <cluster>-kubeconfig├──────────►│  kubectl/oc access  │
│                     │  base64    │                     │
└─────────────────────┘            └─────────────────────┘
```

---

## Environment Variables

| Variable | Description |
|----------|-------------|
| `ARO_CLUSTER_KUBECONFIG` | Path to workload cluster kubeconfig (set by Test 1) |
| `WORKLOAD_CLUSTER_NAME` | Name of the ARO cluster |
| `MANAGEMENT_CLUSTER_NAME` | Name of the Kind cluster |
