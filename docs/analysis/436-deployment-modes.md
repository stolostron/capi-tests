# Deployment Modes for ARO-CAPZ Test Suite

**Issue:** [#436 - Document supported testing paths and deployment modes](https://github.com/RadekCap/CAPZTests/issues/436)

**Date:** 2025-01-28

**Status:** Analysis Complete

---

## Summary

| Mode | Trigger | Cluster | Controllers | Namespace | CRD Source | Use Case | CAPZ Tests |
|------|---------|---------|-------------|-----------|-----------|----------|------------|
| Kind | `USE_KIND=true` | Local Kind cluster | Deployed by tests | `capi-system` / `capz-system` | cluster-api-installer | Local development | v1 |
| K8S | `USE_K8S=true` | Local K8s cluster | Deployed by tests | `multicluster-engine` | cluster-api-installer | Generic Kubernetes | v1 |
| OCP | (default) | Local CRC/OpenShift | Deployed by tests | `multicluster-engine` | cluster-api-installer | Local OpenShift testing | v1 |
| MCE | `USE_KUBECONFIG=<path>` | MCE installation | Pre-installed | `multicluster-engine` | Backplane operator | Production-like testing, OpenShift CI | v2 |

---

## Mode Details

### Kind Mode

**Trigger:** `USE_KIND=true`

**Description:**
Creates a local Kind (Kubernetes in Docker) cluster and deploys CAPI, CAPZ, and ASO controllers. This is the most self-contained mode - everything is provisioned by the test suite.

**Prerequisites:**
- Docker installed and running
- `kind` CLI installed
- Azure credentials for ASO configuration

**Controller Deployment:**
- CAPI controller deployed to `capi-system` namespace
- CAPZ controller deployed to `capz-system` namespace
- ASO controller deployed to `capz-system` namespace
- Controllers are deployed by the test suite via `deploy-charts.sh`

**Test Phases:**
1. Check Dependencies - Validates tools and Azure auth
2. Setup - Clones cluster-api-installer repository
3. Cluster - Creates Kind cluster, deploys controllers
4. Generate YAMLs - Generates deployment manifests
5. Deploy CRs - Applies resources to cluster
6. Verification - Validates workload cluster
7. Deletion - Cleans up workload cluster
8. Cleanup - Validates cleanup status

**Example:**
```bash
export USE_KIND=true
export AZURE_CLIENT_ID=<client-id>
export AZURE_CLIENT_SECRET=<client-secret>
export AZURE_TENANT_ID=<tenant-id>
export AZURE_SUBSCRIPTION_ID=<subscription-id>

make test-all
```

---

### K8S Mode

**Trigger:** `USE_K8S=true`

**Description:**
Uses a local Kubernetes cluster with deployed CAPI, CAPZ, and ASO controllers. The test suite deploys controllers to the local cluster and proceeds with workload cluster deployment.

**Prerequisites:**
- Local Kubernetes cluster running
- `kubectl` CLI installed and configured
- Azure credentials for ASO configuration

**Controller Deployment:**
- CAPI controller deployed to `capi-system` namespace
- CAPZ controller deployed to `capz-system` namespace
- ASO controller deployed to `capz-system` namespace
- Controllers are deployed by the test suite

**Test Phases:**
- Same as Kind mode (controllers deployed by tests)

**Example:**
```bash
export USE_K8S=true
export AZURE_CLIENT_ID=<client-id>
export AZURE_CLIENT_SECRET=<client-secret>
export AZURE_TENANT_ID=<tenant-id>
export AZURE_SUBSCRIPTION_ID=<subscription-id>

make test-all
```

**Open Questions:**
- See issue #437 for namespace clarification

---

### OCP Mode (Default)

**Trigger:** None (default when no `USE_KIND` or `USE_K8S` set)

**Description:**
Connects to a local OpenShift installation running via CRC (CodeReady Containers). This is the implicit default mode - tests assume a local OpenShift instance is available.

**Prerequisites:**
- CRC installed and running (`crc start`)
- `oc` CLI configured to connect to CRC
- Azure credentials for ASO configuration

**Controller Deployment:**
- CAPI controller deployed to `capi-system` namespace
- CAPZ controller deployed to `capz-system` namespace
- ASO controller deployed to `capz-system` namespace
- Controllers are deployed by the test suite

**Test Phases:**
- Same as Kind mode (controllers deployed by tests)

**Example:**
```bash
# Start CRC
crc start

# Login to CRC
eval $(crc oc-env)
oc login -u developer

# Run tests (no USE_* variables needed)
export AZURE_CLIENT_ID=<client-id>
export AZURE_CLIENT_SECRET=<client-secret>
export AZURE_TENANT_ID=<tenant-id>
export AZURE_SUBSCRIPTION_ID=<subscription-id>

make test-all
```

**Note:** If CRC is not running, tests will fail when trying to connect.

---

### MCE Mode (v2)

**Trigger:** `USE_KUBECONFIG=<path>`

**Description:**
Uses an external MCE (Multicluster Engine) installation via kubeconfig. This mode is designed for production-like testing and OpenShift CI integration. The test suite skips cluster creation and controller deployment, validating that everything is pre-installed.

**Prerequisites:**
- Access to MCE cluster
- CAPI, CAPZ, ASO controllers pre-installed in `multicluster-engine` namespace
- Kubeconfig file with cluster access
- Azure credentials

**Controller Deployment:**
- Controllers must be pre-installed by MCE
- Located in `multicluster-engine` namespace
- ASO credentials assumed to be configured (see #435)

**Test Phases:**
1. Check Dependencies - Validates kubeconfig file exists
2. Setup - **Skipped** (controllers pre-installed)
3. Cluster - **Validates** controllers exist (no creation)
4. Generate YAMLs - Same as other modes
5. Deploy CRs - Same as other modes
6. Verification - Same as other modes
7. Deletion - Same as other modes
8. Cleanup - Same as other modes

**Example:**
```bash
# Extract kubeconfig from MCE cluster
oc login https://api.mce-cluster.example.com:6443
oc config view --raw > /tmp/mce-kubeconfig.yaml

# Run tests
export USE_KUBECONFIG=/tmp/mce-kubeconfig.yaml
export AZURE_CLIENT_ID=<client-id>
export AZURE_CLIENT_SECRET=<client-secret>
export AZURE_TENANT_ID=<tenant-id>
export AZURE_SUBSCRIPTION_ID=<subscription-id>

make test-all
```

**Implementation:** See issue #433 for implementation details.

---

## Decision Flowchart

```
Start
  |
  v
Do you have MCE cluster access?
  |
  +-- Yes --> Use MCE mode (USE_KUBECONFIG=<path>)
  |
  +-- No
        |
        v
      Do you have local K8s cluster?
        |
        +-- Yes --> Use K8S mode (USE_K8S=true)
        |
        +-- No
              |
              v
            Do you have CRC running?
              |
              +-- Yes --> Use OCP mode (default)
              |
              +-- No --> Use Kind mode (USE_KIND=true)
```

---

## Namespace Configuration

| Mode | CAPI Namespace | CAPZ/ASO Namespace |
|------|----------------|-------------------|
| Kind | `capi-system` | `capz-system` |
| K8S | `capi-system` | `capz-system` |
| OCP | `capi-system` | `capz-system` |
| MCE | `multicluster-engine` | `multicluster-engine` |

**Override:** Namespaces can be overridden via environment variables:
- `CAPI_NAMESPACE` - Override CAPI controller namespace
- `CAPZ_NAMESPACE` - Override CAPZ/ASO controller namespace

---

## Related Issues

- #433 - Add support for external Kubernetes cluster via kubeconfig (MCE mode implementation)
- #434 - Integrate with OpenShift CI and Sippy reporting
- #435 - Validate ASO credentials configuration in external cluster mode
- #437 - Clarify namespace configuration for K8S deployment mode
