# Phase 5: Deploy CRs

**Make target:** `make _deploy-crs`
**Test file:** `test/05_deploy_crs_test.go`
**Timeout:** 40 minutes

---

## Purpose

Apply the generated YAML manifests to the Kind cluster and monitor the ARO cluster deployment until the control plane is ready.

---

## Test Summary

| # | Test | Purpose |
|---|------|---------|
| 1 | [01-ApplyResources](01-ApplyResources.md) | Apply all YAML files to cluster |
| 2 | [02-ApplyCredentialsYAML](02-ApplyCredentialsYAML.md) | Apply credentials.yaml |
| 3 | [03-ApplyInfrastructureSecretsYAML](03-ApplyInfrastructureSecretsYAML.md) | Apply is.yaml |
| 4 | [04-ApplyAROClusterYAML](04-ApplyAROClusterYAML.md) | Apply aro.yaml |
| 5 | [05-MonitorCluster](05-MonitorCluster.md) | Monitor deployment with clusterctl |
| 6 | [06-WaitForControlPlane](06-WaitForControlPlane.md) | Poll until control plane is ready |
| 7 | [07-CheckClusterConditions](07-CheckClusterConditions.md) | Check cluster condition status |

---

## Flow Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                    make _deploy-crs                              │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Tests 1-4: Apply Resources                                      │
│  ├── kubectl apply -f credentials.yaml                           │
│  ├── kubectl apply -f is.yaml                                    │
│  └── kubectl apply -f aro.yaml                                   │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 5: MonitorCluster                                          │
│  ├── kubectl get cluster <name>                                  │
│  └── clusterctl describe cluster <name> --show-conditions=all    │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 6: WaitForControlPlane                                     │
│  └── Poll arocontrolplane until status.ready=true                │
│      (timeout: DEPLOYMENT_TIMEOUT, default 45m)                  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 7: CheckClusterConditions                                  │
│  ├── Check InfrastructureReady condition                         │
│  └── Check ControlPlaneReady condition                           │
└─────────────────────────────────────────────────────────────────┘
```

---

## YAML Files Applied

| File | Contains |
|------|----------|
| `credentials.yaml` | Azure credentials secret |
| `is.yaml` | Infrastructure secrets |
| `aro.yaml` | ARO cluster resources |

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `DEPLOYMENT_TIMEOUT` | `45m` | Control plane wait timeout |
| `MANAGEMENT_CLUSTER_NAME` | `capz-tests-stage` | kubectl context |
| `WORKLOAD_CLUSTER_NAME` | `capz-tests-cluster` | ARO cluster name |
