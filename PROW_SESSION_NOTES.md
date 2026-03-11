# Prow CI Onboarding — Session Notes

## Goal

Onboard `stolostron/capi-tests` to OpenShift CI via PR https://github.com/openshift/release/pull/75733
Branch in capi-tests: `configure-prow`
Branch in openshift/release fork (RadekCap/release): `stolostron-capi-tests-ci`

## Current State (as of 2026-03-11 17:15 UTC)

- **Latest commit pushed to capi-tests**: `bc9ae78` — "docs: update Prow session notes with 2026-03-11 findings"
- **Latest commit on openshift/release PR**: `088afa2e3c` — "ci: temporarily disable e2e steps beyond generate-yamls"
- **Previous rehearsal (build `2031737114856525824`)**: IPI cluster provisioned successfully (58m39s). Failed at `capz-test-check-dependencies` (docker/kind not in image). **Fix committed in `624c3b5`**.
- **Current rehearsal (build `2031767535971471360`)**: PENDING since 16:21 UTC. IPI provisioning in progress (~59min expected). This run includes the docker/kind fix. **Check this result first when resuming.**
- **PR description updated**: Added `capz-test-install-controllers` step, updated descriptions to reflect IPI (not Kind) mode
- **What to do when resuming**: Check build `2031767535971471360` result. If still pending, wait. If failed, parse ci-operator.log for the failing step. Expected next failure point: `capz-test-setup` or `capz-test-install-controllers`.

## CI Config File

Located at: `~/git/release/ci-operator/config/stolostron/capi-tests/stolostron-capi-tests-configure-prow.yaml`

Current content in the PR:
```yaml
build_root:
  project_image:
    dockerfile_path: Dockerfile.prow
releases:
  initial:
    integration:
      name: "4.19"
      namespace: ocp
  latest:
    integration:
      include_built_images: true
      name: "4.19"
      namespace: ocp
resources:
  '*':
    requests:
      cpu: 100m
      memory: 200Mi
tests:
- as: capz-e2e
  steps:
    cluster_profile: azure4
    pre:
    - chain: ipi-azure-pre
    - ref: capz-test-check-dependencies
    - ref: capz-test-setup
    - ref: capz-test-install-controllers
    test:
    - ref: capz-test-management-cluster
    - ref: capz-test-generate-yamls
    post:
    - ref: capz-test-teardown
    - chain: ipi-azure-post
    workflow: openshift-e2e-azure
  timeout: 4h0m0s
```

## Step Registry Refs

All under `~/git/release/ci-operator/step-registry/capz/test/`:

| Ref | Script | Wired in Config |
|-----|--------|-----------------|
| `capz-test-check-dependencies` | Sources `openshift-ci/capz-test-env.sh`, runs `make _check-dep` | Yes (pre) |
| `capz-test-setup` | Sources env, runs `make _setup` | Yes (pre) |
| `capz-test-install-controllers` | Clones cluster-api-installer, runs `deploy-charts.sh`, patches ASO secret | Yes (pre) |
| `capz-test-management-cluster` | Sources env, runs `make _management_cluster` | Yes (test) |
| `capz-test-generate-yamls` | Sources env, runs `make _generate-yamls` | Yes (test) |
| `capz-test-deploy-crs` | Sources env, runs `make _deploy-crs` | Created, **not wired** |
| `capz-test-verify-workload-cluster` | Sources env, runs `make _verify-workload-cluster` | Created, **not wired** |
| `capz-test-delete-workload-cluster` | Sources env, runs `make _delete-workload-cluster` | Created, **not wired** |
| `capz-test-validate-cleanup` | Sources env, runs `make _validate-cleanup` | Created, **not wired** |
| `capz-test-teardown` | Safety net cleanup (always runs, `best_effort: true`) | Yes (post) |

## Planned Step Order (full pipeline)

```
pre:
  1. ipi-azure-pre (chain)         — Provision IPI OpenShift cluster (~59min)
  2. capz-test-check-dependencies  — Validate tools, auth, naming
  3. capz-test-setup               — Clone repository, verify scripts
  4. capz-test-install-controllers — Install CAPI/CAPZ/ASO on the IPI cluster
test:
  5. capz-test-management-cluster  — Validate external cluster with controllers
  6. capz-test-generate-yamls      — Generate YAML manifests
  7. capz-test-deploy-crs          — Apply CRs, wait for deployment (NOT WIRED)
  8. capz-test-verify-workload-cluster — Validate workload cluster (NOT WIRED)
  9. capz-test-delete-workload-cluster — Delete workload cluster (NOT WIRED)
  10. capz-test-validate-cleanup    — Validate cleanup (NOT WIRED)
post:
  11. capz-test-teardown           — Safety net cleanup
  12. ipi-azure-post (chain)       — Deprovision IPI cluster
```

## Latest Run Results (2026-03-11, build `2031737114856525824`)

| Step | Lifecycle | Status |
|------|-----------|--------|
| ipi-azure-pre (15 substeps) | pre | All passed (IPI cluster created in 58m39s) |
| `capz-test-check-dependencies` | pre | Failed (1m4s) — docker/kind not in image |
| `capz-test-setup` | pre | Skipped (blocked by above) |
| `capz-test-install-controllers` | pre | Skipped (blocked by above) |
| `capz-test-management-cluster` | test | Skipped |
| `capz-test-generate-yamls` | test | Skipped |
| `capz-test-teardown` | post | Passed (1m27s) |
| ipi-azure-post (deprovisioning) | post | Passed (19m25s) |

**Total run time**: 1h38m54s

## Issues Found and Fixed

### Session 1 (2026-03-10)

#### 1. Go vendor inconsistency (FIXED)
- Error: `gopkg.in/yaml.v3@v3.0.1: is explicitly required in go.mod, but not marked as explicit in vendor/modules.txt`
- Fix: Run `go mod vendor` in capi-tests repo

#### 2. `capz-test-install-controllers` was before `capz-test-setup` (FIXED)
- Commit `babf065dcea`: Reordered so setup runs first
- install-controllers depends on repo being cloned by setup

#### 3. `workflow: openshift-e2e-azure` removed (commit `b235cc4a0b0`)
- Hypothesis was that workflow + explicit steps conflicted
- **This did NOT fix the issue** — same error persisted
- The workflow removal is still in place and is fine (not needed when steps are explicit)

#### 4. `cluster_profile: azure4` at wrong YAML level (FIXED — commit `b914d918631`)
- **ROOT CAUSE of RELEASE_IMAGE_LATEST and CLUSTER_PROFILE_DIR failures**
- `cluster_profile` was a sibling of `steps:` instead of a child
- ci-operator silently ignores it at the wrong level — no validation error
- Verified by inspecting actual pod specs in `ci-operator-step-graph.json`
- Fix: Moved `cluster_profile: azure4` inside `steps:`

#### 5. Lease required for azure4 cluster profile (FIXED)
- Without `workflow: openshift-e2e-azure`, the lease was not provided
- Fix: Added `workflow: openshift-e2e-azure` back INSIDE `steps:` (kata-containers pattern)
- Both `cluster_profile` and `workflow` go inside `steps:` alongside explicit `pre`/`test`/`post`

### Session 2 (2026-03-11)

#### 6. docker/kind check fails in external cluster mode (FIXED — commit `624c3b5`)
- IPI cluster provisioned successfully, but `capz-test-check-dependencies` failed
- `TestCheckDependencies_ToolAvailable` unconditionally checked for `docker` and `kind`
- In external cluster mode (`USE_KUBECONFIG`), these tools are not needed
- Fix: Skip `docker`/`kind` in tool check, `DockerDaemonRunning`, and `Kind_IsAvailable` when `config.IsExternalCluster()` is true
- **Why not caught earlier**: March 6 run tested `main` branch (different status check context), `configure-prow` branch has this IPI setup

#### 7. `gen.sh` missing script (March 6 run — NOT an issue on configure-prow)
- March 6 run failed at `capz-test-setup` with `scripts/aro-hcp/gen.sh` not found
- That run tested `main` branch which used old repo defaults (`RadekCap/cluster-api-installer`, branch `ARO-ASO`)
- `configure-prow` branch already has correct defaults (`marek-veber/cluster-api-installer`, branch `capi-tests`)
- `gen.sh` exists in `marek-veber/cluster-api-installer` on the `capi-tests` branch

## Key Debugging Insights

### How to check step-level results
```bash
# Get latest comments with test results
gh pr view 75733 --repo openshift/release --json comments --jq '.comments[-5:][].body'

# Find build ID from status checks
gh pr view 75733 --repo openshift/release --json statusCheckRollup \
  --jq '.statusCheckRollup[] | select(.context | test("capz")) | "\(.state) | \(.targetUrl)"'

# Raw ci-operator log:
# https://gcsweb-ci.apps.ci.l2s4.p1.openshiftapps.com/gcs/test-platform-results/pr-logs/pull/openshift_release/75733/rehearse-75733-pull-ci-stolostron-capi-tests-configure-prow-capz-e2e/<BUILD_ID>/artifacts/ci-operator.log

# Step results summary:
curl -sL "<ci-operator.log URL>" | grep -E '"msg":"(Step |Running step|Some steps)' | sed 's/.*"msg":"//;s/".*//'
```

### How to trigger/abort a rehearsal
```bash
# Trigger
gh pr comment 75733 --repo openshift/release --body "/pj-rehearse pull-ci-stolostron-capi-tests-configure-prow-capz-e2e"

# Abort
gh pr comment 75733 --repo openshift/release --body "/abort rehearse-75733-pull-ci-stolostron-capi-tests-configure-prow-capz-e2e"
```

### How to verify env var injection
Parse `ci-operator-step-graph.json`, find the `capz-e2e` substeps, and check `spec.containers[].env` for `RELEASE_IMAGE_LATEST` and `CLUSTER_PROFILE_DIR`. If missing, the test infrastructure config is wrong.

### Reference configs that work
- `kata-containers/kata-containers` — uses `ipi-azure-pre` chain + `workflow: openshift-e2e-azure` with `cluster_profile` INSIDE `steps:`
- `openshift-priv/azure-disk-csi-driver` — overrides pre/test/post without workflow, `cluster_profile` INSIDE `steps:`

### ci-operator env var injection rules
- `RELEASE_IMAGE_LATEST` — injected when `releases.latest` is defined AND `cluster_profile` is properly inside `steps:`
- `CLUSTER_PROFILE_DIR` — injected when `cluster_profile` is inside `steps:`, mounts the profile secret as a volume
- `SHARED_DIR` — always available, used for passing data between steps
- Steps using `from: src` get the test repo source code
- Steps using `from_image:` get an external image (centos, azure-cli, etc.)

### Prow job flakiness
- Rehearsal triggers sometimes fail with `failed to submit all rehearsal jobs` — retrying usually works
- If a run is stuck as PENDING, abort it and retrigger
- Lease renewal 502 warnings are transient and harmless

## Local Repos

- **capi-tests**: `~/git/github/stolostron/capi-tests` (branch: `configure-prow`)
  - Remote `origin`: `https://github.com/RadekCap/capi-tests.git`
  - Remote `upstream`: `https://github.com/stolostron/capi-tests.git`
- **openshift/release fork**: `~/git/release` (branch: `stolostron-capi-tests-ci`)
  - Remote `origin`: `https://github.com/RadekCap/release.git`
  - Remote `upstream`: `https://github.com/openshift/release.git`

## Next Session — What To Do

### Step 1: Check rehearsal result

The docker/kind fix has been pushed. A rehearsal was triggered at 15:43 UTC on 2026-03-11. Check the result:
```bash
gh pr view 75733 --repo openshift/release --json statusCheckRollup \
  --jq '.statusCheckRollup[] | select(.context | test("capz")) | "\(.state) | \(.targetUrl)"'
```

### Step 2: Debug next failure

Expected progression after docker/kind fix:
1. `capz-test-check-dependencies` — should now pass (docker/kind skipped in external cluster mode)
2. `capz-test-setup` — clones `marek-veber/cluster-api-installer` (branch `capi-tests`), verifies scripts
3. `capz-test-install-controllers` — deploys CAPI/CAPZ/ASO via Helm charts onto the IPI cluster
4. `capz-test-management-cluster` — validates controllers are running
5. `capz-test-generate-yamls` — generates YAML manifests

### Step 3: Wire remaining test steps

Once the first 6 steps pass, add the remaining steps to the config in the openshift/release PR:
```yaml
    test:
    - ref: capz-test-management-cluster
    - ref: capz-test-generate-yamls
    - ref: capz-test-deploy-crs              # ADD
    - ref: capz-test-verify-workload-cluster  # ADD
    - ref: capz-test-delete-workload-cluster  # ADD
    - ref: capz-test-validate-cleanup         # ADD
```

### Step 4: Update this file

After each session, update `PROW_SESSION_NOTES.md` with new findings and push to `upstream/configure-prow`.
