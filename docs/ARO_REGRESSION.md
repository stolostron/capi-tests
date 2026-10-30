# ARO-HCP lifecycle regression (draft)

This opt-in path runs the existing CAPZ lifecycle through creation, workload
verification, control-plane/node-pool version update, and deletion. It uses the
same run context and result directory throughout. The upgrade reconciler checks
ASO readiness and the workload ClusterVersion before continuing to deletion.

## Run locally

Configure the existing ARO lifecycle prerequisites and an initial deployment
version supported by your environment. Select an approved upgrade target:

```bash
export INFRA_PROVIDER=aro
export ARO_UPGRADE_VERSION=<supported-target-version>
# Optional: a supported node-pool version no newer than the control plane.
export ARO_UPGRADE_NODE_POOL_VERSION=<supported-node-pool-version>
make test-regression
```

Versions must have the form `X.Y` or `X.Y.Z`. The upgrade phase validates the
relationship to the live cluster and generated manifest. This is not a catalog
of supported versions or upgrade paths. Supplying a target alone does not change
`make test-all`; use `test-regression` explicitly.

A failed create or update stops later lifecycle phases. MCE teardown still runs.
For local diagnosis, workload deletion is skipped after an upgrade failure;
inspect the failure and use the existing documented deletion/cleanup procedure.
CI retains its existing always-run resource cleanup after a valid preflight.
Do not run this target against a cluster you intend to preserve: successful
regression includes deletion.

## Existing GitHub Actions integration

The `workload-cluster-aro.yml` workflow accepts these manual/reusable inputs:

- `regression_upgrade_version`: opt into regression with a control-plane target.
- `regression_node_pool_version`: optional node-pool target (defaults to control plane).

Repository variables `ARO_REGRESSION_UPGRADE_VERSION` and
`ARO_REGRESSION_NODE_POOL_VERSION` provide fallback values, including for the
existing nightly schedule. Explicit nonempty inputs take precedence. With an
empty control-plane target and no node-pool target, the existing create/delete
workflow runs. Clear the repository variables to disable the scheduled opt-in.
The existing schedule's code-change filter still applies.

CI uses the existing deployment timeout for both create and update. Its preflight
requires the selected create + update + delete timeouts plus 60 minutes of
headroom to fit within the 360-minute job limit. Invalid regression settings fail
before provisioning and skip resource cleanup. The default 60-minute phase
budgets fit. The existing environment, installer branch, and management-cluster
inputs remain available; they are not a supported-configuration matrix.

Upgrade results are saved as `junit-upgrade.xml` and copied to `results/latest`
on success or failure, included in the existing GHA artifact, summary, and
failure annotations. Existing phase JUnit files remain separate.

## Pending external contracts

This draft supplies orchestration and CI integration, not all acceptance criteria
of [ARO-24307](https://redhat.atlassian.net/browse/ARO-24307).

Before claiming cross-platform stability coverage, obtain Vincent Boulos's
approved matrix: configuration axes, supported version/upgrade paths, required
management-cluster and runner platforms, environments, exclusions, and stability
criteria. The current CI runner remains Ubuntu. No matrix entries are assumed.

Polarion publication is pending its endpoint/project, authentication secret
contract, test-case mapping, run identifiers, import format, and publication
failure/retry policy. JUnit artifacts are available for that integration; this
change neither uploads to Polarion nor claims that publication succeeds.

## Offline validation

With Python 3 and PyYAML installed:

```bash
python3 scripts/tests/test_aro_regression.py
```

The fixtures execute the actual Make recipes and workflow shell snippets with
fake phase runners. They check opt-in selection, phase ordering, shared run state,
failure short-circuiting, MCE teardown, JUnit retention, and CI budget validation.
They never authenticate, dispatch workflows, or provision/delete cloud resources.
