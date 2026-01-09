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
| 6 | [06-TestedVersionsSummary](06-TestedVersionsSummary.md) | Display component version summary |
| 7 | [07-ControllerLogSummary](07-ControllerLogSummary.md) | Summarize and save controller logs |

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
│  ├── Check: cluster phase is "Provisioned"                      │
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
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 6: TestedVersionsSummary                                   │
│  └── Display CAPI, CAPZ, ASO controller versions                │
│  └── Show OpenShift version and cluster info                    │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 7: ControllerLogSummary                                    │
│  ├── Fetch logs from CAPI, CAPZ, ASO controllers                │
│  ├── Count errors and warnings                                   │
│  └── Save complete logs to results/<timestamp>/                  │
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

---

## Output Files

Controller logs are saved to the results directory:

| File | Description |
|------|-------------|
| `results/<timestamp>/capi-controller.log` | CAPI controller logs |
| `results/<timestamp>/capz-controller.log` | CAPZ controller logs |
| `results/<timestamp>/aso-controller.log` | ASO controller logs |
| `results/latest/*.log` | Copies for easy access |

---

## Summary Tests

Tests 6 and 7 are informational tests that provide useful debugging and documentation information:

| Test | Purpose | Failure Behavior |
|------|---------|------------------|
| TestedVersionsSummary | Document tested versions | Does not fail |
| ControllerLogSummary | Save logs for debugging | Does not fail |

These tests always pass but provide valuable information for troubleshooting and reproducibility.
