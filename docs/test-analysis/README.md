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
    └── 6. make _verify        Cluster Verification
```

---

## Phase Overview

| Phase | Make Target | Test File | Tests | Timeout | Description |
|-------|-------------|-----------|-------|---------|-------------|
| 1 | [_check-dep](01-check-dependencies/00-Overview.md) | `01_check_dependencies_test.go` | 12 | 2m | Verify tools, authentication, and naming |
| 2 | [_setup](02-setup/00-Overview.md) | `02_setup_test.go` | 3 | 2m | Clone repository, verify scripts |
| 3 | [_cluster](03-cluster/00-Overview.md) | `03_cluster_test.go` | 7 | 30m | Deploy Kind cluster with controllers |
| 4 | [_generate-yamls](04-generate-yamls/00-Overview.md) | `04_generate_yamls_test.go` | 4 | 20m | Generate YAML manifests |
| 5 | [_deploy-crs](05-deploy-crs/00-Overview.md) | `05_deploy_crs_test.go` | 7 | 40m | Apply CRs, wait for deployment |
| 6 | [_verify](06-verification/00-Overview.md) | `06_verification_test.go` | 7 | 20m | Validate workload cluster |

**Total: 40 tests across 6 phases**

---

## Complete Flow Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           PHASE 1: CHECK DEPENDENCIES                        │
│  Tools: docker, kind, az, oc, helm, git, kubectl, go, clusterctl, python3  │
│  Daemon: Docker daemon running check                                         │
│  Auth: Azure authentication (service principal or CLI)                      │
│  Naming: RFC 1123 compliance, domain prefix length validation              │
│  Creds: Docker credential helper check (macOS)                              │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                              PHASE 2: SETUP                                  │
│  Clone: git clone -b ARO-ASO https://github.com/.../cluster-api-installer   │
│  Verify: scripts/deploy-charts-kind-capz.sh, doc/aro-hcp-scripts/aro-hcp-gen│
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                             PHASE 3: CLUSTER                                 │
│  Create: kind create cluster, helm install cert-manager                     │
│  Deploy: helm template charts/cluster-api | kubectl apply                   │
│  Patch: ASO credentials secret with Azure credentials                       │
│  Wait: CAPI, CAPZ, ASO controllers + all webhooks ready                     │
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
│   └── 12-NamingCompliance.md
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
│   └── 07-WebhooksReady.md
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
│   └── 07-CheckClusterConditions.md
└── 06-verification/
    ├── 00-Overview.md
    ├── 01-RetrieveKubeconfig.md
    ├── 02-ClusterNodes.md
    ├── 03-ClusterVersion.md
    ├── 04-ClusterOperators.md
    ├── 05-ClusterHealth.md
    ├── 06-TestedVersionsSummary.md
    └── 07-ControllerLogSummary.md
```
