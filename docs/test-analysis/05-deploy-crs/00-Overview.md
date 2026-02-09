# Phase 5: Deploy CRs

**Make target:** `make _deploy-crs`
**Test file:** `test/05_deploy_crs_test.go`
**Timeout:** 40 minutes

---

## Purpose

Create the workload cluster namespace, apply the generated YAML manifests to the management cluster, and monitor the ARO cluster deployment until the control plane is ready.

---

## Test Summary

| # | Test | Purpose |
|---|------|---------|
| 1 | [08-CreateNamespace](08-CreateNamespace.md) | Create workload cluster namespace |
| 2 | [09-CheckExistingClusters](09-CheckExistingClusters.md) | Check for mismatched cluster resources |
| 3 | [01-ApplyResources](01-ApplyResources.md) | Apply all YAML files to cluster |
| 4 | [02-ApplyCredentialsYAML](02-ApplyCredentialsYAML.md) | Apply credentials.yaml |
| 5 | [04-ApplyAROClusterYAML](04-ApplyAROClusterYAML.md) | Apply aro.yaml |
| 6 | [05-MonitorCluster](05-MonitorCluster.md) | Monitor deployment with clusterctl |
| 7 | [06-WaitForControlPlane](06-WaitForControlPlane.md) | Poll until control plane is ready |
| 8 | [07-CheckClusterConditions](07-CheckClusterConditions.md) | Check cluster condition status |

---

## Flow Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                    make _deploy-crs                              │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 1: CreateNamespace                                          │
│  ├── Create unique namespace (capz-test-YYYYMMDD-HHMMSS)         │
│  └── Add identification labels (capz-test=true)                  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 2: CheckExistingClusters                                    │
│  ├── Check for mismatched Cluster CRs                             │
│  └── Fail-fast if stale clusters from different config exist      │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Tests 3-5: Apply Resources                                       │
│  ├── kubectl apply -f credentials.yaml                            │
│  └── kubectl apply -f aro.yaml                                    │
│  (with retry logic for transient connection issues)               │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 7: MonitorCluster                                           │
│  ├── kubectl get cluster <name>                                   │
│  └── clusterctl describe cluster <name> --show-conditions=all     │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 8: WaitForControlPlane                                      │
│  └── Poll arocontrolplane until status.ready=true                 │
│      (timeout: DEPLOYMENT_TIMEOUT, default 45m)                   │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 9: CheckClusterConditions                                   │
│  ├── Check InfrastructureReady condition                          │
│  └── Check ControlPlaneReady condition                            │
└─────────────────────────────────────────────────────────────────┘
```

---

## YAML Files Applied

| File | Contains |
|------|----------|
| `credentials.yaml` | Azure credentials secret |
| `aro.yaml` | ARO cluster resources (Cluster, AROControlPlane, AROCluster with ASO resources, MachinePool) |

---

## Namespace Isolation

Each test run creates a unique namespace to enable:
- Parallel test runs on the same cluster
- Easy cleanup of test resources
- Clear separation between test runs

Namespace format: `${WORKLOAD_CLUSTER_NAMESPACE_PREFIX}-${TIMESTAMP}` (e.g., `capz-test-20260202-135526`)

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `DEPLOYMENT_TIMEOUT` | `45m` | Control plane wait timeout |
| `MANAGEMENT_CLUSTER_NAME` | `capz-tests-stage` | kubectl context |
| `WORKLOAD_CLUSTER_NAME` | `capz-tests-cluster` | ARO cluster name |
| `WORKLOAD_CLUSTER_NAMESPACE` | auto-generated | Namespace for cluster resources |
| `WORKLOAD_CLUSTER_NAMESPACE_PREFIX` | `capz-test` | Prefix for auto-generated namespace |
