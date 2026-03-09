# OpenShift CI (Prow) Integration

This directory contains reference copies of the OpenShift CI configuration files for the capi-tests project. The actual files are submitted to the [openshift/release](https://github.com/openshift/release) repository.

## Directory Structure

```
openshift-ci/
├── README.md                          # This file
├── ci-operator-config.yaml            # ci-operator config reference
└── step-registry/
    ├── capz-test-check-dependencies-ref.yaml        # Step reference YAML
    ├── capz-test-check-dependencies-commands.sh     # Step commands script
    ├── capz-test-install-controllers-ref.yaml       # Install CAPI/CAPZ/ASO controllers
    ├── capz-test-install-controllers-commands.sh     # Controller installation script
    ├── capz-test-e2e-ref.yaml                       # Full e2e test step
    ├── capz-test-e2e-commands.sh                     # E2e test execution script
    └── capz-test-e2e-workflow.yaml                   # Complete e2e workflow definition
```

## Where Files Go in openshift/release

| Local reference file | openshift/release destination |
|---------------------|-------------------------------|
| `ci-operator-config.yaml` | `ci-operator/config/stolostron/capi-tests/stolostron-capi-tests-main.yaml` |
| `step-registry/capz-test-check-dependencies-ref.yaml` | `ci-operator/step-registry/capz/test/check-dependencies/capz-test-check-dependencies-ref.yaml` |
| `step-registry/capz-test-check-dependencies-commands.sh` | `ci-operator/step-registry/capz/test/check-dependencies/capz-test-check-dependencies-commands.sh` |
| `step-registry/capz-test-install-controllers-ref.yaml` | `ci-operator/step-registry/capz/test/install-controllers/capz-test-install-controllers-ref.yaml` |
| `step-registry/capz-test-install-controllers-commands.sh` | `ci-operator/step-registry/capz/test/install-controllers/capz-test-install-controllers-commands.sh` |
| `step-registry/capz-test-e2e-ref.yaml` | `ci-operator/step-registry/capz/test/e2e/capz-test-e2e-ref.yaml` |
| `step-registry/capz-test-e2e-commands.sh` | `ci-operator/step-registry/capz/test/e2e/capz-test-e2e-commands.sh` |
| `step-registry/capz-test-e2e-workflow.yaml` | `ci-operator/step-registry/capz/test/e2e/capz-test-e2e-workflow.yaml` |

## How It Works

1. **Dockerfile.prow** (in repo root) builds a container image with all required tools (Go, azure-cli, kubectl, helm, gotestsum, clusterctl, oc)
2. **ci-operator** uses the config to define test jobs that run against PRs and periodically
3. **Step registry** entries define individual test steps:
   - `check-dependencies` — Phase 01 (tool availability, no cloud resources)
   - `install-controllers` — Pre-step that deploys CAPI/CAPZ/ASO controllers on a CI-provisioned cluster
   - `e2e` — Full test suite (Phases 01-07) using `USE_KUBECONFIG` mode against the CI cluster
4. The **e2e workflow** chains: IPI Azure cluster provisioning → controller installation → e2e tests → deprovisioning
5. Test results are written as JUnit XML to `${ARTIFACT_DIR}` for Prow to collect and display

## Setting Up in openshift/release

### 1. Fork and clone openshift/release

```bash
gh repo fork openshift/release --clone
cd release
```

### 2. Copy files to their destinations

```bash
# ci-operator config
mkdir -p ci-operator/config/stolostron/capi-tests
cp <path-to-capi-tests>/openshift-ci/ci-operator-config.yaml \
   ci-operator/config/stolostron/capi-tests/stolostron-capi-tests-main.yaml

# Step registry — check-dependencies
mkdir -p ci-operator/step-registry/capz/test/check-dependencies
cp <path-to-capi-tests>/openshift-ci/step-registry/capz-test-check-dependencies-ref.yaml \
   ci-operator/step-registry/capz/test/check-dependencies/
cp <path-to-capi-tests>/openshift-ci/step-registry/capz-test-check-dependencies-commands.sh \
   ci-operator/step-registry/capz/test/check-dependencies/

# Step registry — install-controllers
mkdir -p ci-operator/step-registry/capz/test/install-controllers
cp <path-to-capi-tests>/openshift-ci/step-registry/capz-test-install-controllers-ref.yaml \
   ci-operator/step-registry/capz/test/install-controllers/
cp <path-to-capi-tests>/openshift-ci/step-registry/capz-test-install-controllers-commands.sh \
   ci-operator/step-registry/capz/test/install-controllers/

# Step registry — e2e
mkdir -p ci-operator/step-registry/capz/test/e2e
cp <path-to-capi-tests>/openshift-ci/step-registry/capz-test-e2e-ref.yaml \
   ci-operator/step-registry/capz/test/e2e/
cp <path-to-capi-tests>/openshift-ci/step-registry/capz-test-e2e-commands.sh \
   ci-operator/step-registry/capz/test/e2e/
cp <path-to-capi-tests>/openshift-ci/step-registry/capz-test-e2e-workflow.yaml \
   ci-operator/step-registry/capz/test/e2e/
```

### 3. Generate Prow jobs

```bash
make ci-operator-config
make jobs
```

### 4. Submit PR to openshift/release

The generated jobs will appear in `ci-operator/jobs/stolostron/capi-tests/`.

## Testing Locally

### Build the Dockerfile

```bash
docker build -f Dockerfile.prow .
```

### Run check-dependencies in the container

```bash
docker run --rm -it <image> bash -c \
  'gotestsum --junitfile /tmp/junit.xml -- -v ./test -count=1 -run TestCheckDependencies'
```

## E2E Test Architecture

The `capz-e2e` test uses OpenShift CI's IPI Azure workflow to provision a real cluster instead of Kind (which requires Docker-in-Docker, unsupported in Prow pods):

```
IPI Azure Provisioning → Install Controllers → Run E2E Tests → Deprovision
       (~40 min)             (~5 min)             (~90 min)       (~15 min)
```

Key design decisions:
- **`USE_KUBECONFIG`** points tests at the CI-provisioned cluster, skipping Kind creation
- **`USE_K8S=false`** prevents auto-switch to MCE namespaces (controllers go to `capi-system`/`capz-system`)
- **`CAPI_USER=prow`** + **`DEPLOYMENT_ENV=ci`** provide CI-specific naming
- Azure credentials come from `cluster_profile: azure4` via `osServicePrincipal.json`

## Future Work

- Add periodic job config for nightly full e2e runs
- Add step registry entries for additional test phases (e.g., ROSA/CAPA)
