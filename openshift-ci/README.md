# OpenShift CI (Prow) Integration

Prow configuration for capi-tests lives entirely in
[openshift/release](https://github.com/openshift/release). This file points at it.

## Why there are no copies here

This directory previously held copies of the ci-operator config and step-registry
files. Nothing validated them against upstream, and they drifted badly — by October
2026 they described a single `capz/test/e2e` step that upstream had deleted, were
missing the entire `capz/arohcp` step family, and named a ci-operator config file
(`stolostron-capi-tests-main.yaml`) that does not exist. The copies were removed rather
than re-synced; read the real files upstream instead.

## Where the real files are

| What | Path in openshift/release |
|------|---------------------------|
| ci-operator config + prowgen | `ci-operator/config/stolostron/capi-tests/` |
| Shared CAPZ steps | `ci-operator/step-registry/capz/test/` |
| ARO-HCP CAPZ steps | `ci-operator/step-registry/capz/arohcp/` |
| Generated job definitions | `ci-operator/jobs/stolostron/capi-tests/` |
| Secret collection ownership | `core-services/sync-rover-groups/_config.yaml` |

The `capz/arohcp` command scripts are symlinks to their `capz/test` counterparts, so a
change to the shared script affects both families.

## Jobs that run this suite

| Job | Source config |
|-----|---------------|
| `periodic-ci-stolostron-capi-tests-configure-prow-mgmt-periodics-capz-e2e` | `stolostron/capi-tests` |
| `periodic-ci-Azure-ARO-HCP-main-capz-e2e-production` | `Azure/ARO-HCP` |

Both consume the same `capz` steps and the same credentials, so a credential problem
breaks them together.

## How it works

1. **Dockerfile.prow** (repo root) builds the test image (Go, azure-cli, kubectl, helm,
   gotestsum, clusterctl, oc).
2. The **`write-env`** step resolves Azure and Quay credentials from mounted secrets and
   writes `${SHARED_DIR}/capz-test-env.sh`.
3. Every later step (`setup`, `management-cluster`, `generate-yamls`, `deploy-crs`,
   `verify-workload-cluster`, `delete-workload-cluster`, `validate-cleanup`, `summary`,
   `teardown`) sources that generated file. There is no env script committed in this
   repository.
4. The management cluster is AKS, provisioned by `aks-provision`. Note that the Go test
   `TestKindCluster_*` retains its name even when running against AKS — it does not
   imply Kind or Docker.
5. Results are written as JUnit XML to `${ARTIFACT_DIR}` for Prow to collect.

## Credentials

Stored in Google Secret Manager, not Vault. See
[`docs/CI_CREDENTIALS.md`](../docs/CI_CREDENTIALS.md) for the storage layout, delivery
path and rotation runbook.

## Changing CI configuration

Edit the files in `openshift/release` and open a PR there. Regenerate jobs with
`make ci-operator-config && make jobs`. To exercise a periodic from a PR, comment:

```text
/pj-rehearse <job-name>
```
