Check the status of the Prow CI rehearsal for the openshift/release PR.

## Instructions

### Step 1: Detect the latest job (running, pending, or finished)

Use `gh pr checks` to find the current rehearsal status and build ID:
```bash
gh pr checks 75733 --repo openshift/release 2>&1 | grep "capi-tests/configure-prow/capz-e2e"
```

Output format: `name\tstatus\t...\tURL\tdescription`
- The URL contains the build ID (the numeric segment after the job name)
- Status is `pending`, `pass`, or `fail`

Extract the build ID from the URL and note the status.

### Step 2: Determine job state and fetch data

Based on the check status from Step 1:

**If `pending` (job is scheduled or running):**
1. Check if `started.json` exists:
   ```bash
   curl -sL "https://storage.googleapis.com/test-platform-results/pr-logs/pull/openshift_release/75733/rehearse-75733-pull-ci-stolostron-capi-tests-configure-prow-capz-e2e/<BUILD_ID>/started.json"
   ```
   - If NoSuchKey: job is **SCHEDULED** (not started yet). Report that and show the Prow link.
   - If exists: job is **RUNNING**. Calculate elapsed time from the `timestamp` field.

2. For running jobs, fetch live logs to determine step progress:
   ```bash
   curl -sL "https://prow.ci.openshift.org/log?container=test&id=<BUILD_ID>&job=rehearse-75733-pull-ci-stolostron-capi-tests-configure-prow-capz-e2e" 2>&1 | grep -E "INFO.*Step|phase"
   ```
   Parse the log lines to determine which steps have passed, which is currently running, and which are pending.

**If `pass` or `fail` (job finished):**
1. Fetch the junit XML for detailed results:
   ```bash
   curl -sL "https://storage.googleapis.com/test-platform-results/pr-logs/pull/openshift_release/75733/rehearse-75733-pull-ci-stolostron-capi-tests-configure-prow-capz-e2e/<BUILD_ID>/artifacts/rehearse-75733-pull-ci-stolostron-capi-tests-configure-prow-capz-e2e/capz-e2e/artifacts/junit_operator.xml"
   ```
2. Parse each `<testcase>` element. Match test names to steps (e.g. test name containing "capz-test-check-dependencies" maps to that step).
3. A testcase with a `<failure>` child element = FAILED. Without = PASSED.

### Step 3: Display pipeline status table

Show results as a full pipeline status table with ALL steps:

| Step | Lifecycle | Status |
|------|-----------|--------|
| ipi-conf | pre (ipi-azure-pre) | ? |
| ipi-conf-azure | pre (ipi-azure-pre) | ? |
| ipi-conf-telemetry | pre (ipi-azure-pre) | ? |
| rhcos-conf-osstream | pre (ipi-azure-pre) | ? |
| ipi-azure-rbac | pre (ipi-azure-pre) | ? |
| azure-provision-service-principal | pre (ipi-azure-pre) | ? |
| azure-provision-custom-role | pre (ipi-azure-pre) | ? |
| ipi-install-rbac | pre (ipi-azure-pre) | ? |
| ipi-install-install | pre (ipi-azure-pre) | ? |
| ipi-install-monitoringpvc | pre (ipi-azure-pre) | ? |
| ipi-install-hosted-loki | pre (ipi-azure-pre) | ? |
| ipi-install-times-collection | pre (ipi-azure-pre) | ? |
| openshift-cluster-bot-rbac | pre (ipi-azure-pre) | ? |
| multiarch-validate-nodes | pre (ipi-azure-pre) | ? |
| nodes-readiness | pre (ipi-azure-pre) | ? |
| **capz-test-check-dependencies** | **pre** | ? |
| **capz-test-setup** | **pre** | ? |
| **capz-test-management-cluster** | **test** | ? |
| **capz-test-generate-yamls** | **test** | ? |
| **capz-test-deploy-crs** | **test** | ? |
| **capz-test-verify-workload-cluster** | **test** | ? |
| **capz-test-delete-workload-cluster** | **test** | ? |
| **capz-test-validate-cleanup** | **test** | ? |
| **capz-test-teardown** | **post** | ? |
| gather-must-gather | post (ipi-azure-post) | ? |
| gather-extra | post (ipi-azure-post) | ? |
| gather-audit-logs | post (ipi-azure-post) | ? |
| gather-azure-cli | post (ipi-azure-post) | ? |
| azure-deprovision-sp-and-custom-role | post (ipi-azure-post) | ? |
| ipi-deprovision-deprovision | post (ipi-azure-post) | ? |

Status values:
- 🟢 **PASSED** — step ran and succeeded (include duration if known)
- 🔴 **FAILED** — step ran and failed (include error message)
- 🟡 **RUNNING** — step is currently executing
- ⚪ **PENDING** — step hasn't started yet (job still on earlier steps)
- ⚫ **NOT REACHED** — step didn't run because a prior step failed
- ⚪ **SKIPPED** — step was skipped (e.g. prerequisites not met)

For running jobs, use live log lines like `Step capz-e2e-X succeeded after Ys` to mark steps as PASSED, the last `Running step capz-e2e-X` as RUNNING, and remaining steps as PENDING.

If a step appears in the junit XML, use its result. If it doesn't appear and a prior step failed, mark it NOT REACHED.

### Step 4: Summary

After the table, provide:
- Link to the Prow job page
- For finished jobs: brief summary of what failed and why, suggested next action
- For running jobs: estimated progress based on elapsed time and which step is active
- For scheduled jobs: note that it's queued and suggest checking back later

### Step 5: No rehearsal found

If `gh pr checks` shows no capz-e2e check at all, suggest triggering one:
```bash
gh pr comment 75733 --repo openshift/release --body "/pj-rehearse pull-ci-stolostron-capi-tests-configure-prow-capz-e2e"
```
