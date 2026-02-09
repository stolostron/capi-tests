# Test 2: TestDeletion_WaitForClusterDeletion

**Location:** `test/07_deletion_test.go:59-136`

**Purpose:** Wait for the cluster resource to be fully deleted, showing detailed progress information about all resources being deleted.

---

## Commands Executed (Polling Loop)

| Command | Purpose |
|---------|---------|
| `GetDeletionResourceStatus()` | Get comprehensive status of cluster, CAPI resources, and Azure RG |

---

## Configuration

| Parameter | Value |
|-----------|-------|
| Timeout | `config.DeploymentTimeout` (default: 45m) |
| Poll interval | 30 seconds |
| Target | Cluster resource no longer exists |

---

## Detailed Flow

```
Configuration:
├── Timeout: DEPLOYMENT_TIMEOUT (default 45m)
├── Poll interval: 30 seconds
└── Resource group: ${CS_CLUSTER_NAME}-resgroup

Loop:
│
├─► Check elapsed time > timeout?
│   └─ Yes → FAIL with troubleshooting steps
│
├─► GetDeletionResourceStatus(context, namespace, clusterName, resourceGroup)
│   └─ Returns: ClusterExists, CAPI resource status, Azure RG status
│
├─► !status.ClusterExists?
│   └─ Yes → PASS: "Cluster has been deleted"
│   └─ No  → Continue
│
├─► ReportDeletionProgress(iteration, elapsed, remaining, status)
│
└─► Sleep 30 seconds, repeat
```

---

## Progress Reporting

The test provides detailed progress information during deletion:

- Cluster resource existence
- AROControlPlane deletion status
- MachinePool deletion status
- Azure resource group status

---

## Timeout Error Troubleshooting

On timeout, the test provides specific troubleshooting steps:

1. Check cluster status with `kubectl get cluster -o yaml`
2. Check for stuck finalizers
3. Check remaining CAPI resources
4. Check Azure resource group status

Common causes:
- Azure resource deletion taking longer than expected
- Finalizers blocking resource deletion
- Azure resource stuck in 'Deleting' state

---

## Example Output

```
=== RUN   TestDeletion_WaitForClusterDeletion
Waiting for cluster 'rcap-stage' to be deleted...
Namespace: capz-test-20260202-135526 | Timeout: 45m0s | Poll interval: 30s
Azure Resource Group: rcap-stage-resgroup

[1] Cluster: exists | AROControlPlane: deleting | MachinePool: deleting | RG: exists
[2] Cluster: exists | AROControlPlane: deleted | MachinePool: deleting | RG: exists
...
[15] Cluster: deleted

Cluster 'rcap-stage' has been deleted (took 7m30s)
--- PASS: TestDeletion_WaitForClusterDeletion (450.12s)
```
