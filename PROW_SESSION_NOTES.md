# Prow CI Onboarding — Session Notes (2026-03-10)

## Goal

Onboard `stolostron/capi-tests` to OpenShift CI via PR https://github.com/openshift/release/pull/75733
Branch in capi-tests: `configure-prow`
Branch in openshift/release fork (RadekCap/release): `stolostron-capi-tests-ci`

## Current State (as of end of session)

- **Latest commit pushed to openshift/release PR**: `b914d918631` — "ci: move cluster_profile inside steps to fix env var injection"
- **Last rehearsal result**: Failed with `step needs a lease but no lease client provided` — the cluster_profile fix worked but a lease is now needed
- **capi-tests local branch**: reset to `upstream/configure-prow` at `de36995`

## CI Config File

`/Users/radoslavcap/git/release/ci-operator/config/stolostron/capi-tests/stolostron-capi-tests-configure-prow.yaml`

Current content in the PR after all fixes:
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
  timeout: 4h0m0s
```

## Step Registry Refs Created

All under `/Users/radoslavcap/git/release/ci-operator/step-registry/capz/test/`:

| Ref | Script | Status |
|-----|--------|--------|
| `capz-test-check-dependencies` | Sources `openshift-ci/capz-test-env.sh`, runs `make _check-dep` | Created |
| `capz-test-setup` | Sources env, runs `make _setup` | Created |
| `capz-test-install-controllers` | Clones cluster-api-installer, runs `deploy-charts.sh`, patches ASO secret | Created |
| `capz-test-management-cluster` | Sources env, runs `make _management_cluster` | Created |
| `capz-test-generate-yamls` | Sources env, runs `make _generate-yamls` | Created |
| `capz-test-deploy-crs` | Sources env, runs `make _deploy-crs` | Created, **NOT wired in config** |
| `capz-test-verify-workload-cluster` | ? | **NOT created yet** |
| `capz-test-delete-workload-cluster` | Sources env, runs `make _delete-workload-cluster` | Created, **NOT wired in config** |
| `capz-test-validate-cleanup` | ? | **NOT created yet** |
| `capz-test-teardown` | Safety net cleanup (always runs) | Created |

## Planned Step Order (full pipeline)

```
pre:
  1. ipi-azure-pre (chain)        — Provision IPI OpenShift cluster
  2. capz-test-check-dependencies — Validate tools, auth, naming
  3. capz-test-setup              — Clone repository, verify scripts
  4. capz-test-install-controllers — Install CAPI/CAPZ/ASO on the IPI cluster
test:
  5. capz-test-management-cluster — Validate external cluster with controllers
  6. capz-test-generate-yamls     — Generate YAML manifests
  7. capz-test-deploy-crs         — Apply CRs, wait for deployment (NOT WIRED YET)
  8. capz-test-verify-workload-cluster — Validate workload cluster (NOT CREATED YET)
  9. capz-test-delete-workload-cluster — Delete workload cluster (NOT WIRED YET)
  10. capz-test-validate-cleanup   — Validate cleanup (NOT CREATED YET)
post:
  11. capz-test-teardown          — Safety net cleanup
  12. ipi-azure-post (chain)      — Deprovision IPI cluster
```

## Issues Found and Fixed in This Session

### 1. Go vendor inconsistency (FIXED earlier, before this session)
- Error: `gopkg.in/yaml.v3@v3.0.1: is explicitly required in go.mod, but not marked as explicit in vendor/modules.txt`
- Fix: Run `go mod vendor` in capi-tests repo

### 2. `capz-test-install-controllers` was before `capz-test-setup` (FIXED)
- Commit `babf065dcea`: Reordered so setup runs first
- install-controllers depends on repo being cloned by setup

### 3. `workflow: openshift-e2e-azure` removed (commit `b235cc4a0b0`)
- Hypothesis was that workflow + explicit steps conflicted
- **This did NOT fix the issue** — same error persisted
- The workflow removal is still in place and is fine (not needed when steps are explicit)

### 4. `cluster_profile: azure4` at wrong YAML level (FIXED — commit `b914d918631`)
- **ROOT CAUSE of RELEASE_IMAGE_LATEST and CLUSTER_PROFILE_DIR failures**
- `cluster_profile` was a sibling of `steps:` instead of a child
- ci-operator silently ignores it at the wrong level — no validation error
- Verified by inspecting actual pod specs in `ci-operator-step-graph.json` — neither `RELEASE_IMAGE_LATEST` nor `CLUSTER_PROFILE_DIR` were present in ANY step pod env vars
- Fix: Moved `cluster_profile: azure4` inside `steps:`

### 5. Lease required for azure4 cluster profile (NEW — not yet fixed)
- Commit `b914d91` fixed the `cluster_profile` placement, which fixed `RELEASE_IMAGE_LATEST` and `CLUSTER_PROFILE_DIR` injection
- But the run now fails with: `step "capz-e2e" failed validation: step needs a lease but no lease client provided`
- The `azure4` cluster profile requires a **lease** for cloud quota management
- When `workflow: openshift-e2e-azure` was present, it handled the lease automatically
- Without a workflow, the lease must be explicitly configured
- **Fix options**:
  - Add `workflow: openshift-e2e-azure` back (it should go inside `steps:` alongside `cluster_profile`)
  - OR add explicit lease configuration to the test
- The kata-containers config uses BOTH `cluster_profile: azure4` AND `workflow: openshift-e2e-azure` inside `steps:` — this is the proven pattern

## Key Debugging Insights

### How to check step-level results
```bash
# Get latest comments with test results
gh pr view 75733 --repo openshift/release --json comments --jq '.comments[-5:][].body'

# Find build ID from the results comment, then fetch junit
# Example URL pattern:
# https://gcsweb-ci.apps.ci.l2s4.p1.openshiftapps.com/gcs/test-platform-results/pr-logs/pull/openshift_release/75733/rehearse-75733-pull-ci-stolostron-capi-tests-configure-prow-capz-e2e/<BUILD_ID>/artifacts/junit_operator.xml

# Raw ci-operator log:
# .../<BUILD_ID>/artifacts/ci-operator.log

# Step graph (JSON with pod specs):
# .../<BUILD_ID>/artifacts/ci-operator-step-graph.json
```

### How to trigger a rehearsal
```bash
gh pr comment 75733 --repo openshift/release --body "/pj-rehearse pull-ci-stolostron-capi-tests-configure-prow-capz-e2e"
```

### How to verify env var injection
Parse `ci-operator-step-graph.json`, find the `capz-e2e` substeps, and check `spec.containers[].env` for `RELEASE_IMAGE_LATEST` and `CLUSTER_PROFILE_DIR`. If missing, the test infrastructure config is wrong.

### Reference configs that work
- `kata-containers/kata-containers` — uses `ipi-azure-pre` chain + `workflow: openshift-e2e-azure` with `cluster_profile` INSIDE `steps:`
  - File: `/Users/radoslavcap/git/release/ci-operator/config/kata-containers/kata-containers/kata-containers-kata-containers-main.yaml`
- `openshift-priv/azure-disk-csi-driver` — overrides pre/test/post without workflow, `cluster_profile` INSIDE `steps:`

### ci-operator env var injection rules
- `RELEASE_IMAGE_LATEST` — injected when `releases.latest` is defined AND `cluster_profile` is properly inside `steps:`
- `CLUSTER_PROFILE_DIR` — injected when `cluster_profile` is inside `steps:`, mounts the profile secret as a volume
- `SHARED_DIR` — always available, used for passing data between steps
- Steps using `from: src` get the test repo source code
- Steps using `from_image:` get an external image (centos, azure-cli, etc.)

## Local Repos

- **capi-tests**: `/Users/radoslavcap/git/capi-tests` (branch: `configure-prow`, synced with `upstream/configure-prow`)
- **openshift/release fork**: `/Users/radoslavcap/git/release` (branch: `stolostron-capi-tests-ci`)
  - Remote `origin`: `https://github.com/RadekCap/release.git`
  - Remote `upstream`: `https://github.com/openshift/release.git`

## Next Session — What To Do

### Step 1: Fix the lease issue (BLOCKING)

The latest rehearsal failed with `step needs a lease but no lease client provided`. The `azure4` cluster profile requires a lease for cloud quota management.

**Recommended fix**: Add `workflow: openshift-e2e-azure` back INSIDE `steps:` in the openshift/release config. This is how kata-containers does it — they have both `cluster_profile` and `workflow` inside `steps:` alongside explicit `pre`/`test`/`post` overrides.

Edit `/Users/radoslavcap/git/release/ci-operator/config/stolostron/capi-tests/stolostron-capi-tests-configure-prow.yaml`:
```yaml
tests:
- as: capz-e2e
  steps:
    cluster_profile: azure4
    workflow: openshift-e2e-azure    # <-- ADD THIS LINE
    pre:
    ...
```

Then commit, push to `RadekCap/release` branch `stolostron-capi-tests-ci`, and trigger:
```bash
gh pr comment 75733 --repo openshift/release --body "/pj-rehearse pull-ci-stolostron-capi-tests-configure-prow-capz-e2e"
```

### Step 2: Debug IPI provisioning failures (if any)

If the lease fix works, `ipi-azure-pre` chain will actually try to provision an OpenShift cluster. This takes ~30-40 minutes. Watch for:
- `ipi-conf` step — needs `RELEASE_IMAGE_LATEST` (should now be injected)
- `ipi-install-install` step — actual cluster installation
- Azure quota/permission errors

### Step 3: Debug CAPZ test step failures

Once IPI provisioning succeeds, the CAPZ test steps will run. Likely failures:
- `capz-test-check-dependencies` — may fail if tools aren't in the `src` image (check `Dockerfile.prow`)
- `capz-test-setup` — clones cluster-api-installer repo
- `capz-test-install-controllers` — deploys CAPI/CAPZ/ASO via Helm charts onto the IPI cluster
- `capz-test-management-cluster` — validates controllers are running

Check step logs at:
```
https://gcsweb-ci.apps.ci.l2s4.p1.openshiftapps.com/gcs/test-platform-results/pr-logs/pull/openshift_release/75733/rehearse-75733-pull-ci-stolostron-capi-tests-configure-prow-capz-e2e/<BUILD_ID>/artifacts/test/
```

### Step 4: Wire remaining test steps

Once the first 6 steps pass, add the remaining steps to the config:
```yaml
    test:
    - ref: capz-test-management-cluster
    - ref: capz-test-generate-yamls
    - ref: capz-test-deploy-crs              # ADD
    - ref: capz-test-verify-workload-cluster  # ADD (needs ref created first)
    - ref: capz-test-delete-workload-cluster  # ADD
    - ref: capz-test-validate-cleanup         # ADD (needs ref created first)
```

### Step 5: Create missing step refs

Two refs still need to be created in `/Users/radoslavcap/git/release/ci-operator/step-registry/capz/test/`:
- `capz-test-verify-workload-cluster` — runs `make _verify-workload-cluster`
- `capz-test-validate-cleanup` — runs `make _validate-cleanup`

Follow the pattern of existing refs (e.g., `capz-test-deploy-crs`).

### Step 6: Update this file

After each session, update `PROW_SESSION_NOTES.md` with new findings and push to `upstream/configure-prow`.
