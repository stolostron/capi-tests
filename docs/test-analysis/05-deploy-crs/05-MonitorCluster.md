# Test 5: TestDeployment_MonitorCluster

**Location:** `test/05_deploy_crs_test.go:178-248`

**Purpose:** Monitor the ARO cluster deployment status using kubectl and clusterctl.

---

## Commands Executed

| Step | Command | Purpose |
|------|---------|---------|
| 1 | `kubectl --context <ctx> get cluster <name>` | Verify cluster resource exists |
| 2 | `clusterctl describe cluster <name> --show-conditions=all` | Get detailed status |

---

## Detailed Flow

```
1. Check prerequisites:
   ├─ DirExists(config.RepoDir)?
   │  └─ No → SKIP
   │
   └─ Find clusterctl binary:
      ├─ Check <RepoDir>/<ClusterctlBinPath>
      └─ Fallback to system PATH

2. Set kubeconfig:
   └─ KUBECONFIG=$HOME/.kube/config

3. Check cluster resource exists:
   └─ kubectl --context <ctx> get cluster <name>
      ├─ Success → Continue
      └─ Failure → SKIP: "Cluster resource not found"

4. Describe cluster:
   └─ clusterctl describe cluster <name> --show-conditions=all
      ├─ Success → Log detailed status
      └─ Failure → Log warning (non-fatal)
```

---

## clusterctl Output

The `clusterctl describe` command provides a tree view of all cluster resources:

```
NAME                                                   READY  SEVERITY  REASON  SINCE  MESSAGE
Cluster/my-aro-cluster                                 True                     5m
├─ClusterInfrastructure - AROCluster/my-aro-cluster    True                     5m
└─ControlPlane - AROControlPlane/my-aro-cluster        True                     3m
```

---

## Example Output

```
=== Starting Cluster Monitoring Test ===
Checking prerequisites...
✅ Repository directory exists: /tmp/cluster-api-installer-aro
Looking for clusterctl binary...
✅ Using clusterctl from system PATH

=== Monitoring cluster deployment ===
Cluster: <workload-cluster-name>
Context: kind-<management-cluster-name>

Checking if cluster resource exists...
✅ Cluster resource exists

📊 Fetching cluster status with clusterctl...
Running: clusterctl describe cluster <workload-cluster-name> --show-conditions=all
This may take a few moments...

✅ Successfully retrieved cluster status

Cluster Status:
NAME                                                          READY  ...
Cluster/<workload-cluster-name>                               True   ...
...

=== Cluster Monitoring Test Complete ===
```

---

## Notes

- This test is **informational** - it doesn't fail if the cluster isn't fully ready
- It provides visibility into deployment progress
- Useful for debugging when subsequent tests fail
