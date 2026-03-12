# Prow CI Onboarding — Session Notes

## Quick Status (updated 2026-03-12 evening)

**Where we are**: 4 out of 6 wired steps are passing. `capz-test-install-controllers` is now FIXED. The latest fix (`USE_K8S=false`) targets `capz-test-management-cluster` namespace mismatch. A rehearsal (build `2032109621262422016`) is running with this fix — check its result first.

**What failed last**: `capz-test-management-cluster` looked for controllers in `multicluster-engine` namespace instead of `capi-system`/`capz-system`, because `USE_KUBECONFIG` being set triggered the Go test suite to assume MCE mode (`USE_K8S=true`).

**What to do next**: Check rehearsal result (see "Next Session" section below).

## Goal

Onboard `stolostron/capi-tests` to OpenShift CI via PR https://github.com/openshift/release/pull/75733
Branch in capi-tests: `configure-prow`
Branch in openshift/release fork (RadekCap/release): `stolostron-capi-tests-ci`

## Current State (as of 2026-03-12 evening)

- **Latest commits pushed to capi-tests** (branch `configure-prow`):
  - `610f1f7` — "fix(ci): set USE_K8S=false for standard controller namespaces"
  - `0764b4c` — "fix: remove redundant azure-service-operator chart argument"
  - `002ed5d` — "fix(ci): set OCP_CONTEXT for deploy-charts.sh on IPI clusters (#577)"
- **Latest commits on openshift/release PR** (branch `stolostron-capi-tests-ci`):
  - `76050f32d` — "fix: remove redundant azure-service-operator chart argument"
  - `7077d5c3e` — "fix: use correct repo/branch and auto-detect kube context"
- **Latest rehearsal**: Build `2032109621262422016` — triggered ~15:02 UTC on 2026-03-12. Contains all fixes including USE_K8S=false. **Result pending.**
- **Previous rehearsal** (build `2032079063358640128`): `capz-test-install-controllers` PASSED (OCP_CONTEXT fix confirmed working). `capz-test-management-cluster` FAILED — namespace mismatch (USE_K8S issue).

## CI Config File

Located at: `~/git/github/openshift/release/ci-operator/config/stolostron/capi-tests/stolostron-capi-tests-configure-prow.yaml`

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

All under `~/git/github/openshift/release/ci-operator/step-registry/capz/test/`:

| Ref | Script | Wired in Config |
|-----|--------|-----------------|
| `capz-test-check-dependencies` | Sources `openshift-ci/capz-test-env.sh`, runs `make _check-dep` | Yes (pre) |
| `capz-test-setup` | Sources env, runs `make _setup` | Yes (pre) |
| `capz-test-install-controllers` | Sources env, clones cluster-api-installer, runs `deploy-charts.sh`, patches ASO secret | Yes (pre) |
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

## Latest Run Results (2026-03-12, build `2032079063358640128`)

| Step | Lifecycle | Status |
|------|-----------|--------|
| ipi-azure-pre (15 substeps) | pre | All passed (IPI cluster created) |
| `capz-test-check-dependencies` | pre | Passed ✅ |
| `capz-test-setup` | pre | Passed ✅ |
| `capz-test-install-controllers` | pre | Passed ✅ (OCP_CONTEXT fix worked!) |
| `capz-test-management-cluster` | test | Failed ❌ — namespace mismatch (looked in `multicluster-engine` instead of `capi-system`) |
| `capz-test-generate-yamls` | test | Not reached |
| `capz-test-teardown` | post | Passed |
| ipi-azure-post (deprovisioning) | post | Passed |

### Previous Runs

| Build | Date | Failed At | Root Cause |
|-------|------|-----------|------------|
| `2032079063358640128` | 2026-03-12 | `capz-test-management-cluster` | USE_K8S namespace mismatch |
| `2032058857164902400` | 2026-03-12 | ABORTED | Prow killed job when new commit pushed |
| `2031995518237806592` | 2026-03-12 | `capz-test-install-controllers` | crc-admin context (repo/branch fix worked) |
| `2031822848447746048` | 2026-03-12 | `capz-test-install-controllers` | Wrong repo/branch + crc-admin context |
| `2031737114856525824` | 2026-03-11 | `capz-test-check-dependencies` | docker/kind not in image |

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

#### 6. docker/kind check fails in external cluster mode (FIXED — commit `624c3b5` in capi-tests)
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

### Session 3 (2026-03-12)

#### 8. install-controllers used wrong repo/branch defaults (FIXED — commit `51f549d` in capi-tests, `3ebe92ab46d` in release)
- `capz-test-install-controllers` step had inline defaults: `RadekCap/cluster-api-installer` branch `ARO-ASO`
- It was the only step that didn't source `openshift-ci/capz-test-env.sh`
- Fix: Refactored to source `capz-test-env.sh` (like all other steps), added `ARO_REPO_URL` and `ARO_REPO_BRANCH` exports to the env file

#### 9. `deploy-charts.sh` hardcodes `--context=crc-admin` (FIXED — commit `002ed5d` in capi-tests, `7077d5c3e` in release)
- `deploy-charts.sh` in `cluster-api-installer` sets `OCP_CONTEXT=${OCP_CONTEXT:-crc-admin}`
- On the IPI-provisioned cluster, `crc-admin` context does not exist
- Error: `error: context "crc-admin" does not exist`
- Fix: Set `export OCP_CONTEXT=$(kubectl config current-context)` in `capz-test-install-controllers-commands.sh` before calling `deploy-charts.sh`
- **Why it works locally**: In `USE_KUBECONFIG` mode, the Go test suite skips Kind creation AND deploy-charts.sh entirely — controllers are pre-installed via MCE. So the crc-admin default was never hit locally.
- Confirmed working in build `2032079063358640128`

#### 10. Redundant `azure-service-operator` chart argument (FIXED — commit `0764b4c` in capi-tests, `76050f32d` in release)
- `capz-test-install-controllers` was calling: `bash scripts/deploy-charts.sh cluster-api cluster-api-provider-azure azure-service-operator`
- `deploy-charts.sh` skipped it: `!!!!!!!!! SKIP DEPLOY: charts/azure-service-operator` (no such chart directory)
- ASO is bundled INSIDE the `cluster-api-provider-azure` chart (see deploy-charts.sh line 45: `DEPLOYMENTS[$NAMESPACE]="${T}-controller-manager azureserviceoperator-controller-manager"`)
- Fix: Removed `azure-service-operator` from the deploy-charts.sh arguments — now just `cluster-api cluster-api-provider-azure`

#### 11. USE_K8S namespace mismatch in management-cluster step (FIXED — commit `610f1f7` in capi-tests, pending sync to release PR)
- `capz-test-install-controllers` passed, but `capz-test-management-cluster` failed
- The Go test suite (via `config.go`) sets `USE_K8S=true` when `USE_KUBECONFIG` is set, causing it to look for controllers in `multicluster-engine` namespace
- But `deploy-charts.sh` with `USE_K8S=false` installs controllers into standard namespaces (`capi-system`, `capz-system`)
- Fix: Added `export USE_K8S=false` to `openshift-ci/capz-test-env.sh`
- The `USE_K8S=false` override ensures the Go test suite checks standard namespaces, matching where deploy-charts.sh actually installed the controllers
- **Awaiting confirmation** in build `2032109621262422016`

## Key Technical Insights

### Two deployment paths — understanding the architecture

| Mode | How controllers are installed | Where controllers live | Who calls deploy-charts.sh |
|------|------------------------------|----------------------|---------------------------|
| **Local (USE_KUBECONFIG + MCE)** | MCE auto-enablement in Go test suite | `multicluster-engine` namespace | Nobody — MCE manages them |
| **Local (USE_KIND)** | Go test suite calls deploy-charts.sh | `capi-system`/`capz-system` | Go test suite (03_cluster_test.go) |
| **Prow CI (IPI cluster)** | `capz-test-install-controllers` step calls deploy-charts.sh | `capi-system`/`capz-system` | Prow step script |

This is why issues #9 and #11 were never caught locally — the Prow CI path (IPI + deploy-charts.sh without Kind) is a new combination.

### Step registry file duplication

Step scripts exist in TWO places:
1. `capi-tests/openshift-ci/step-registry/` — source of truth (in capi-tests repo)
2. `openshift/release/ci-operator/step-registry/capz/test/` — what Prow actually executes (in release PR)

Pushing to capi-tests does NOT update the release PR. Both must be updated manually and kept in sync.

### deploy-charts.sh context logic

```bash
if [ "$USE_KIND" = true -o "$USE_K8S" = true ] ; then
    KUBE_CONTEXT="--context=kind-$KIND_CLUSTER_NAME"    # Kind mode
else
    OCP_CONTEXT=${OCP_CONTEXT:-crc-admin}               # OCP mode (defaults to crc-admin!)
    KUBE_CONTEXT="--context=$OCP_CONTEXT"
fi
```

For Prow CI, we set `OCP_CONTEXT` before calling deploy-charts.sh to override the `crc-admin` default.

### IPI Azure cluster defaults

The IPI-provisioned cluster uses:
- VM size: `Standard_D4s_v3` (4 vCPUs, 16 GB RAM)
- 3 master + 3 worker nodes (6 total)
- Provisioning time: ~60 minutes
- Defined in the `ipi-conf-azure` step (step registry, not configurable from our CI config)

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
- Prow kills running jobs when new commits are pushed — must retrigger manually

## Local Repos

- **capi-tests**: `~/git/github/stolostron/capi-tests` (branch: `configure-prow`)
  - Remote `origin`: `https://github.com/RadekCap/capi-tests.git`
  - Remote `upstream`: `https://github.com/stolostron/capi-tests.git`
- **openshift/release fork**: `~/git/github/openshift/release` (branch: `stolostron-capi-tests-ci`)
  - Remote `origin`: `https://github.com/RadekCap/release.git`
  - Remote `upstream`: `https://github.com/openshift/release.git`

## Next Session — What To Do

### Step 1: Check rehearsal result (build `2032109621262422016`)

```bash
gh pr view 75733 --repo openshift/release --json statusCheckRollup \
  --jq '.statusCheckRollup[] | select(.context | test("capz")) | "\(.state) | \(.targetUrl)"'
```

Or use the `/prow-status` slash command.

**Expected outcome**: With USE_K8S=false, `capz-test-management-cluster` should now find controllers in `capi-system`/`capz-system` and pass. If it passes, `capz-test-generate-yamls` will run next (first time!).

**NOTE**: The USE_K8S=false fix is in capi-tests (`610f1f7`) but the openshift/release PR step scripts may need updating too. Check if `capz-test-management-cluster-commands.sh` in the release PR sources `capz-test-env.sh` — if yes, it picks up USE_K8S=false automatically from the capi-tests source code. If not, update the release PR copy.

### Step 2: If management-cluster passes, debug generate-yamls

`capz-test-generate-yamls` runs `make _generate-yamls` which calls the cluster-api-installer's `gen.sh` script. Potential issues:
- Missing env vars for YAML generation (AZURE_SUBSCRIPTION_ID, etc.)
- Script path differences between Kind and IPI modes
- Check if `gen.sh` needs additional env vars not in `capz-test-env.sh`

### Step 3: Wire remaining test steps

Once the first 6 steps pass, add the remaining steps to the CI config in the openshift/release PR:
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
