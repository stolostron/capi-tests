# ARO-CAPZ Test Suite Analysis

This directory contains detailed analysis of all test phases in the ARO-CAPZ test suite.

---

## Test Execution Order

The tests run sequentially, with each phase depending on the previous phase's success:

```
make test-all
    │
    ├── 1. make _check-dep     Check Dependencies
    │
    ├── 2. make _setup         Repository Setup
    │
    ├── 3. make _cluster       Kind Cluster Deployment
    │
    ├── 4. make _generate-yamls YAML Generation
    │
    ├── 5. make _deploy-crs    CR Deployment
    │
    ├── 6. make _verify        Cluster Verification
    │
    ├── 7. make _delete        Cluster Deletion
    │
    └── 8. make _cleanup       Cleanup Validation
```

---

## Phase Overview

| Phase | Make Target | Test File | Tests | Timeout | Description |
|-------|-------------|-----------|-------|---------|-------------|
| 1 | [_check-dep](01-check-dependencies/00-Overview.md) | `01_check_dependencies_test.go` | 18 | 2m | Verify tools, authentication, and naming |
| 2 | [_setup](02-setup/00-Overview.md) | `02_setup_test.go` | 3 | 2m | Clone repository, verify scripts |
| 3 | [_cluster](03-cluster/00-Overview.md) | `03_cluster_test.go` | 11 | 30m | Deploy Kind/external cluster with controllers |
| 4 | [_generate-yamls](04-generate-yamls/00-Overview.md) | `04_generate_yamls_test.go` | 4 | 20m | Generate YAML manifests |
| 5 | [_deploy-crs](05-deploy-crs/00-Overview.md) | `05_deploy_crs_test.go` | 9 | 40m | Apply CRs, wait for deployment |
| 6 | [_verify](06-verification/00-Overview.md) | `06_verification_test.go` | 7 | 20m | Validate workload cluster |
| 7 | [_delete](07-deletion/00-Overview.md) | `07_deletion_test.go` | 6 | 60m | Delete workload cluster |
| 8 | [_cleanup](08-cleanup/00-Overview.md) | `08_cleanup_test.go` | 18 | 10m | Validate cleanup operations |

**Total: 76 tests across 8 phases**

---

## Complete Flow Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           PHASE 1: CHECK DEPENDENCIES                        │
│  Tools: docker, kind, az, oc, helm, git, kubectl, go, clusterctl, python3  │
│  Optional: jq (for MCE)                                                     │
│  External: Validate USE_KUBECONFIG connectivity                             │
│  Daemon: Docker daemon running check                                         │
│  Auth: Azure authentication (service principal or CLI)                      │
│  Naming: RFC 1123 compliance, domain prefix length validation              │
│  Azure: Region, subscription access, timeout configuration                 │
│  Summary: Comprehensive validation with critical error detection           │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                              PHASE 2: SETUP                                  │
│  Clone: git clone -b main https://github.com/stolostron/cluster-api-installer│
│  Verify: scripts/deploy-charts.sh, doc/aro-hcp-scripts/aro-hcp-gen          │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                             PHASE 3: CLUSTER                                 │
│  Kind mode:                                                                  │
│    Create: kind create cluster, helm install cert-manager                   │
│    Deploy: helm template charts/cluster-api | kubectl apply                 │
│    Patch: ASO credentials secret with Azure credentials                     │
│    Wait: CAPI, CAPZ, ASO controllers + all webhooks ready                   │
│  External mode (USE_KUBECONFIG):                                             │
│    Validate: Cluster connectivity, MCE baseline                             │
│    Enable: MCE CAPI/CAPZ components (auto-enablement)                       │
│    Verify: Pre-installed controllers                                         │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                          PHASE 4: GENERATE YAMLS                             │
│  Run: bash aro-hcp-gen.sh <output-dir>                                       │
│  Output: credentials.yaml, is.yaml, aro.yaml                                 │
│  Validate: YAML syntax check                                                 │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           PHASE 5: DEPLOY CRS                                │
│  Namespace: Create unique per-run namespace (capz-test-YYYYMMDD-HHMMSS)    │
│  Guard: Check for mismatched cluster resources (stale config detection)     │
│  Health: Wait for cluster healthy before applying                           │
│  Apply: kubectl apply -f credentials.yaml, is.yaml, aro.yaml (with retry)  │
│  Monitor: clusterctl describe cluster                                        │
│  Wait: Poll arocontrolplane until status.ready=true (45m timeout)           │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                          PHASE 6: VERIFICATION                               │
│  Kubeconfig: kubectl get secret <cluster>-kubeconfig                        │
│  Nodes: kubectl get nodes                                                    │
│  Operators: oc get clusteroperators                                          │
│  Health: kubectl get pods -A (check for non-running)                        │
│  Summary: Display component versions and save controller logs               │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                            PHASE 7: DELETION                                 │
│  Delete: kubectl delete cluster <cluster-name> --wait=false                 │
│  Wait: Monitor cluster resource until fully deleted                          │
│  Verify: AROControlPlane, MachinePool resources deleted                     │
│  Azure: Verify Azure resource group cleanup                                  │
│  Summary: Deletion status report                                             │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           PHASE 8: CLEANUP                                   │
│  Local: Kind cluster, kubeconfig, repository, results, deployment state    │
│  Azure: Resource group, orphaned resources, AD apps, service principals    │
│  Script: Cleanup script validation (help, dry-run, prefix validation)      │
│  Edge cases: Non-existent resources, prefix matching accuracy               │
│  Summary: Comprehensive cleanup status with actionable commands             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Quick Reference

### Run All Tests
```bash
make test-all
```

### Run Individual Phases
```bash
make _check-dep      # Phase 1
make _setup          # Phase 2
make _cluster        # Phase 3
make _generate-yamls # Phase 4
make _deploy-crs     # Phase 5
make _verify         # Phase 6
make _delete         # Phase 7
make _cleanup        # Phase 8
```

### Key Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `MANAGEMENT_CLUSTER_NAME` | `capz-tests-stage` | Kind cluster name |
| `WORKLOAD_CLUSTER_NAME` | `capz-tests-cluster` | ARO cluster name |
| `ARO_REPO_DIR` | `/tmp/cluster-api-installer-aro` | Repository path |
| `DEPLOYMENT_TIMEOUT` | `45m` | Control plane wait timeout |
| `DEPLOYMENT_ENV` | `stage` | Environment identifier |
| `REGION` | `uksouth` | Azure region |
| `CAPZ_USER` | `rcap` | User identifier (RFC 1123 compliant) |
| `USE_KUBECONFIG` | (unset) | Path to external cluster kubeconfig |

---

## Directory Structure

```
docs/test-analysis/
├── README.md                      # This file
├── 01-check-dependencies/
│   ├── 00-Overview.md
│   ├── 01-ToolAvailable.md
│   ├── 02-DockerDaemonRunning.md
│   ├── 03-AzureCLILogin.md
│   ├── 04-AzureEnvironment.md
│   ├── 05-OpenShiftCLI.md
│   ├── 06-Helm.md
│   ├── 07-Kind.md
│   ├── 08-Clusterctl.md
│   ├── 09-DockerCredentialHelper.md
│   ├── 10-PythonVersion.md
│   ├── 11-NamingConstraints.md
│   ├── 12-NamingCompliance.md
│   ├── 13-OptionalTools.md
│   ├── 14-ExternalKubeconfig.md
│   ├── 15-AzureRegion.md
│   ├── 16-AzureSubscriptionAccess.md
│   ├── 17-TimeoutConfiguration.md
│   └── 18-ComprehensiveValidation.md
├── 02-setup/
│   ├── 00-Overview.md
│   ├── 01-CloneRepository.md
│   ├── 02-VerifyRepositoryStructure.md
│   └── 03-ScriptPermissions.md
├── 03-cluster/
│   ├── 00-Overview.md
│   ├── 01-KindClusterReady.md
│   ├── 02-CAPINamespacesExists.md
│   ├── 03-CAPIControllerReady.md
│   ├── 04-CAPZControllerReady.md
│   ├── 05-ASOCredentialsConfigured.md
│   ├── 06-ASOControllerReady.md
│   ├── 07-WebhooksReady.md
│   ├── 08-ExternalCluster-Connectivity.md
│   ├── 09-ExternalCluster-MCEBaselineStatus.md
│   ├── 10-ExternalCluster-EnableMCE.md
│   └── 11-ExternalCluster-ControllersReady.md
├── 04-generate-yamls/
│   ├── 00-Overview.md
│   ├── 01-GenerateResources.md
│   ├── 02-VerifyCredentialsYAML.md
│   ├── 03-VerifyInfrastructureSecretsYAML.md
│   └── 04-VerifyAROClusterYAML.md
├── 05-deploy-crs/
│   ├── 00-Overview.md
│   ├── 01-ApplyResources.md
│   ├── 02-ApplyCredentialsYAML.md
│   ├── 03-ApplyInfrastructureSecretsYAML.md
│   ├── 04-ApplyAROClusterYAML.md
│   ├── 05-MonitorCluster.md
│   ├── 06-WaitForControlPlane.md
│   ├── 07-CheckClusterConditions.md
│   ├── 08-CreateNamespace.md
│   └── 09-CheckExistingClusters.md
├── 06-verification/
│   ├── 00-Overview.md
│   ├── 01-RetrieveKubeconfig.md
│   ├── 02-ClusterNodes.md
│   ├── 03-ClusterVersion.md
│   ├── 04-ClusterOperators.md
│   ├── 05-ClusterHealth.md
│   ├── 06-TestedVersionsSummary.md
│   └── 07-ControllerLogSummary.md
├── 07-deletion/
│   ├── 00-Overview.md
│   ├── 01-DeleteCluster.md
│   ├── 02-WaitForClusterDeletion.md
│   ├── 03-VerifyAROControlPlaneDeletion.md
│   ├── 04-VerifyMachinePoolDeletion.md
│   ├── 05-VerifyAzureResourcesDeletion.md
│   └── 06-Summary.md
└── 08-cleanup/
    ├── 00-Overview.md
    ├── 01-VerifyKindClusterDeletion.md
    ├── 02-VerifyKubeconfigRemoval.md
    ├── 03-VerifyClonedRepositoryRemoval.md
    ├── 04-VerifyResultsDirectoryRemoval.md
    ├── 05-VerifyDeploymentStateFile.md
    ├── 06-AzureCLIAvailability.md
    ├── 07-AzureAuthentication.md
    ├── 08-VerifyResourceGroupStatus.md
    ├── 09-VerifyOrphanedResources.md
    ├── 10-VerifyADApplications.md
    ├── 11-VerifyServicePrincipals.md
    ├── 12-ScriptExists.md
    ├── 13-ScriptHelpWorks.md
    ├── 14-DryRunMode.md
    ├── 15-PrefixValidation.md
    ├── 16-NonExistentResourcesNoError.md
    ├── 17-ResourceDiscoveryPrefixMatching.md
    └── 18-Summary.md
```
