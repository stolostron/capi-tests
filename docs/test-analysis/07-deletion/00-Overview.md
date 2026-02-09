# Phase 7: Deletion

**Make target:** `make _delete`
**Test file:** `test/07_deletion_test.go`
**Timeout:** 60 minutes

---

## Purpose

Delete the workload cluster from the management cluster and verify all associated resources (Kubernetes CRs and Azure resources) are cleaned up.

---

## Test Summary

| # | Test | Purpose |
|---|------|---------|
| 1 | [01-DeleteCluster](01-DeleteCluster.md) | Initiate workload cluster deletion |
| 2 | [02-WaitForClusterDeletion](02-WaitForClusterDeletion.md) | Wait for cluster resource to be fully deleted |
| 3 | [03-VerifyAROControlPlaneDeletion](03-VerifyAROControlPlaneDeletion.md) | Verify AROControlPlane resource is deleted |
| 4 | [04-VerifyMachinePoolDeletion](04-VerifyMachinePoolDeletion.md) | Verify MachinePool resources are deleted |
| 5 | [05-VerifyAzureResourcesDeletion](05-VerifyAzureResourcesDeletion.md) | Verify Azure resource group is cleaned up |
| 6 | [06-Summary](06-Summary.md) | Provide deletion status summary |

---

## Flow Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                    make _delete                                   │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 1: DeleteCluster                                            │
│  ├── Check if cluster exists (kubectl get cluster)                │
│  ├── Skip if cluster not found                                    │
│  └── kubectl delete cluster <name> --wait=false                   │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 2: WaitForClusterDeletion                                   │
│  ├── Poll until cluster resource no longer exists                 │
│  ├── Show detailed deletion progress (CAPI resources, Azure RG)  │
│  └── Timeout: DEPLOYMENT_TIMEOUT (default 45m)                    │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 3: VerifyAROControlPlaneDeletion                            │
│  └── kubectl get arocontrolplane --ignore-not-found               │
│  └── Warn if resources still exist                                │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 4: VerifyMachinePoolDeletion                                │
│  └── kubectl get machinepool --ignore-not-found                   │
│  └── Warn if resources still exist                                │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 5: VerifyAzureResourcesDeletion                             │
│  ├── Check Azure CLI availability and authentication              │
│  ├── az group show --name <rg-name>                               │
│  └── List remaining resources if RG still exists                  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 6: Summary                                                  │
│  ├── Check remaining cluster resources                            │
│  ├── Check remaining CAPI resources (arocontrolplane, machinepool)│
│  └── Display deletion status summary                              │
└─────────────────────────────────────────────────────────────────┘
```

---

## Deletion Mechanism

Deleting the CAPI `Cluster` resource triggers a cascading deletion:

```
kubectl delete cluster <name>
        │
        ├── CAPI deletes AROControlPlane
        │   └── CAPZ deletes Azure control plane resources
        │
        ├── CAPI deletes MachinePool
        │   └── CAPZ deletes Azure worker node resources
        │
        └── CAPZ deletes Azure resource group
            └── All Azure resources within the RG are deleted
```

The `--wait=false` flag is used so the delete command returns immediately, allowing the next test to monitor progress.

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `DEPLOYMENT_TIMEOUT` | `45m` | Timeout for waiting for deletion to complete |
| `WORKLOAD_CLUSTER_NAME` | `capz-tests-cluster` | Name of the cluster to delete |
| `WORKLOAD_CLUSTER_NAMESPACE` | auto-generated | Namespace containing cluster resources |
| `CS_CLUSTER_NAME` | `${CAPZ_USER}-${DEPLOYMENT_ENV}` | Prefix for Azure resource group name |

---

## Source File

All tests are defined in: `test/07_deletion_test.go`
