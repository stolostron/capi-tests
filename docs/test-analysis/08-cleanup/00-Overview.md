# Phase 8: Cleanup Validation

**Make target:** `make _cleanup`
**Test file:** `test/08_cleanup_test.go`
**Timeout:** 10 minutes

---

## Purpose

Validate that cleanup operations work correctly for local resources (Kind cluster, kubeconfig, repositories, temp files) and Azure resources (resource groups, orphaned resources, AD applications, service principals). Also validates the cleanup script behavior.

---

## Test Categories

### Local Cleanup Tests

| # | Test | Purpose |
|---|------|---------|
| 1 | [01-VerifyKindClusterDeletion](01-VerifyKindClusterDeletion.md) | Verify Kind cluster can be identified for cleanup |
| 2 | [02-VerifyKubeconfigRemoval](02-VerifyKubeconfigRemoval.md) | Verify kubeconfig files can be identified |
| 3 | [03-VerifyClonedRepositoryRemoval](03-VerifyClonedRepositoryRemoval.md) | Verify cloned repository can be identified |
| 4 | [04-VerifyResultsDirectoryRemoval](04-VerifyResultsDirectoryRemoval.md) | Verify results directory can be identified |
| 5 | [05-VerifyDeploymentStateFile](05-VerifyDeploymentStateFile.md) | Verify deployment state file can be identified |

### Azure Cleanup Tests

| # | Test | Purpose |
|---|------|---------|
| 6 | [06-AzureCLIAvailability](06-AzureCLIAvailability.md) | Verify Azure CLI is available for cleanup |
| 7 | [07-AzureAuthentication](07-AzureAuthentication.md) | Verify Azure authentication for cleanup |
| 8 | [08-VerifyResourceGroupStatus](08-VerifyResourceGroupStatus.md) | Check Azure resource group status |
| 9 | [09-VerifyOrphanedResources](09-VerifyOrphanedResources.md) | Discover orphaned Azure resources |
| 10 | [10-VerifyADApplications](10-VerifyADApplications.md) | Check for Azure AD Applications |
| 11 | [11-VerifyServicePrincipals](11-VerifyServicePrincipals.md) | Check for Service Principals |

### Cleanup Script Validation Tests

| # | Test | Purpose |
|---|------|---------|
| 12 | [12-ScriptExists](12-ScriptExists.md) | Verify cleanup script exists and is executable |
| 13 | [13-ScriptHelpWorks](13-ScriptHelpWorks.md) | Verify cleanup script --help option |
| 14 | [14-DryRunMode](14-DryRunMode.md) | Verify cleanup script dry-run mode |
| 15 | [15-PrefixValidation](15-PrefixValidation.md) | Verify prefix validation in cleanup script |

### Edge Case Tests

| # | Test | Purpose |
|---|------|---------|
| 16 | [16-NonExistentResourcesNoError](16-NonExistentResourcesNoError.md) | Verify graceful handling of non-existent resources |
| 17 | [17-ResourceDiscoveryPrefixMatching](17-ResourceDiscoveryPrefixMatching.md) | Verify prefix matching accuracy |

### Summary

| # | Test | Purpose |
|---|------|---------|
| 18 | [18-Summary](18-Summary.md) | Comprehensive cleanup status summary |

---

## Flow Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                    make _cleanup                                  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  LOCAL CLEANUP TESTS (1-5)                                        │
│  ├── Kind cluster status (kind get clusters)                      │
│  ├── Kubeconfig files (*-kubeconfig.yaml in temp dir)             │
│  ├── Cloned repository (cluster-api-installer-aro)                │
│  ├── Results directory (results/)                                 │
│  └── Deployment state file (.deployment-state.json)               │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  AZURE CLEANUP TESTS (6-11)                                       │
│  ├── Azure CLI availability and version                           │
│  ├── Azure authentication status                                  │
│  ├── Resource group status (${CS_CLUSTER_NAME}-resgroup)          │
│  ├── Orphaned resources (Azure Resource Graph query)              │
│  ├── AD Applications (az ad app list --filter)                    │
│  └── Service Principals (az ad sp list --filter)                  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  CLEANUP SCRIPT VALIDATION (12-15)                                │
│  ├── Script existence and permissions                             │
│  ├── --help output validation                                     │
│  ├── --dry-run mode verification                                  │
│  └── Prefix validation (invalid/valid prefixes)                   │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  EDGE CASES (16-17)                                               │
│  ├── Non-existent resource deletion (graceful handling)           │
│  └── Prefix matching accuracy (startswith vs contains)            │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  SUMMARY (18)                                                     │
│  ├── Local resources: Kind, kubeconfig, repo, results, state     │
│  ├── Azure resources: RG, AD apps                                 │
│  └── Available cleanup commands                                   │
└─────────────────────────────────────────────────────────────────┘
```

---

## Standalone Phase

This phase is designed to run independently - it does **not** modify or delete any resources. It only reports the current cleanup status and validates the cleanup tooling.

To actually clean up resources, use:
- `make clean` - Interactive cleanup (prompts for each resource)
- `make clean-all` - Non-interactive cleanup (deletes everything)
- `make clean-azure` - Azure resources only

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `MANAGEMENT_CLUSTER_NAME` | `capz-tests-stage` | Kind cluster name to check |
| `ARO_REPO_DIR` | `/tmp/cluster-api-installer-aro` | Repository path to check |
| `CAPZ_USER` | `rcap` | Prefix for Azure resource discovery |
| `CS_CLUSTER_NAME` | `${CAPZ_USER}-${DEPLOYMENT_ENV}` | Resource group name prefix |

---

## Source File

All tests are defined in: `test/08_cleanup_test.go`
