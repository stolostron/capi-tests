Check the status of the Prow CI rehearsal for the openshift/release PR.

## Instructions

1. Get the latest PR comments from the openshift/release PR #75733:
   ```bash
   gh pr view 75733 --repo openshift/release --json comments --jq '.comments[-5:][].body'
   ```

2. Parse the comments to find the most recent rehearsal result for `rehearse-75733-pull-ci-stolostron-capi-tests-configure-prow-capz-e2e`.

3. If a build ID is found, fetch the junit_operator.xml to check step results:
   - URL pattern: `https://gcsweb-ci.apps.ci.l2s4.p1.openshiftapps.com/gcs/test-platform-results/pr-logs/pull/openshift_release/75733/rehearse-75733-pull-ci-stolostron-capi-tests-configure-prow-capz-e2e/<BUILD_ID>/artifacts/`
   - Key files to check: `junit_operator.xml`, `ci-operator.log`

4. Display results as a CAPZ pipeline status table. Map junit results to our pipeline steps and show this table:

   | Step | Lifecycle | Status |
   |------|-----------|--------|
   | capz-test-check-dependencies | pre | ? |
   | capz-test-setup | pre | ? |
   | capz-test-install-controllers | pre | ? |
   | capz-test-management-cluster | test | ? |
   | capz-test-generate-yamls | test | ? |
   | capz-test-deploy-crs | test | ? |
   | capz-test-verify-workload-cluster | test | ? |
   | capz-test-delete-workload-cluster | test | ? |
   | capz-test-validate-cleanup | test | ? |
   | capz-test-teardown | post | ? |

   Status values:
   - **PASSED** — step ran and succeeded
   - **FAILED** — step ran and failed (include error message)
   - **NOT REACHED** — step didn't run because a prior step failed
   - **NOT WIRED** — step ref exists but is not yet included in the CI config
   - **NOT CREATED** — step ref does not exist yet in the step registry

   To determine status: match junit test names containing each step name (e.g. "capz-test-check-dependencies").
   Steps that are NOT WIRED: `capz-test-deploy-crs`, `capz-test-delete-workload-cluster`
   Steps that are NOT CREATED: `capz-test-verify-workload-cluster`, `capz-test-validate-cleanup`

5. After the table, provide:
   - Link to the Prow job page
   - Brief summary of what failed and why
   - Suggested next action

6. If there's no recent rehearsal, report that and suggest triggering one with:
   ```bash
   gh pr comment 75733 --repo openshift/release --body "/pj-rehearse pull-ci-stolostron-capi-tests-configure-prow-capz-e2e"
   ```
