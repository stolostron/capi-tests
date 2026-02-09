# Phase 4: Generate YAMLs

**Make target:** `make _generate-yamls`
**Test file:** `test/04_generate_yamls_test.go`
**Timeout:** 20 minutes

---

## Purpose

Generate Kubernetes YAML manifests for ARO infrastructure resources using the `aro-hcp-gen.sh` script, then validate the generated files.

---

## Test Summary

| # | Test | Purpose |
|---|------|---------|
| 1 | [01-GenerateResources](01-GenerateResources.md) | Run generation script and create YAML files |
| 2 | [02-VerifyCredentialsYAML](02-VerifyCredentialsYAML.md) | Validate credentials.yaml syntax |
| 3 | [04-VerifyAROClusterYAML](04-VerifyAROClusterYAML.md) | Validate aro.yaml syntax |

---

## Flow Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                    make _generate-yamls                          │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 1: GenerateResources                                       │
│  ├── Set environment variables                                   │
│  ├── cd to ARO_REPO_DIR                                          │
│  ├── Run: bash aro-hcp-gen.sh <output-dir>                       │
│  └── Verify output files created                                 │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 2: VerifyCredentialsYAML                                   │
│  └── ValidateYAMLFile(credentials.yaml)                          │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 3: VerifyAROClusterYAML                                    │
│  └── ValidateYAMLFile(aro.yaml)                                  │
└─────────────────────────────────────────────────────────────────┘
```

---

## Generated Files

| File | Description |
|------|-------------|
| `credentials.yaml` | Azure credentials secret |
| `aro.yaml` | ARO cluster configuration (Cluster, AROControlPlane, AROCluster with ASO resources, MachinePool) |

---

## Environment Variables Used

| Variable | Purpose |
|----------|---------|
| `DEPLOYMENT_ENV` | Environment identifier (stage, prod) |
| `USER` | User identifier for naming |
| `WORKLOAD_CLUSTER_NAME` | Name for the ARO cluster |
| `REGION` | Azure region |
| `AZURE_SUBSCRIPTION_NAME` | Azure subscription ID |

---

## Output Directory

Files are generated to: `<ARO_REPO_DIR>/<DEPLOYMENT_ENV>-<USER>-<WORKLOAD_CLUSTER_NAME>/`

Example: `/tmp/cluster-api-installer-aro/stage-radek-capz-tests-cluster/`
