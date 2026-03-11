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

4. Display results as a full pipeline status table. The table must include ALL steps — both IPI infrastructure chain steps and our custom CAPZ test steps. Map junit test names to each step.

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
   | gather-must-gather | post (ipi-azure-post) | ? |
   | gather-extra | post (ipi-azure-post) | ? |
   | gather-audit-logs | post (ipi-azure-post) | ? |
   | gather-azure-cli | post (ipi-azure-post) | ? |
   | azure-deprovision-sp-and-custom-role | post (ipi-azure-post) | ? |
   | ipi-deprovision-deprovision | post (ipi-azure-post) | ? |

   Status values:
   - **PASSED** — step ran and succeeded
   - **FAILED** — step ran and failed (include error message)
   - **NOT REACHED** — step didn't run because a prior step failed
   - **NOT WIRED** — step ref exists but is not yet included in the CI config
   - **NOT CREATED** — step ref does not exist yet in the step registry

   To determine status: match junit test names containing each step name (e.g. "capz-test-check-dependencies", "ipi-install-install").
   Steps that are NOT WIRED: `capz-test-deploy-crs`, `capz-test-delete-workload-cluster`
   Steps that are NOT CREATED: `capz-test-verify-workload-cluster`, `capz-test-validate-cleanup`

   If a step appears in the junit XML, use its result. If it doesn't appear and a prior step failed, mark it NOT REACHED.

5. After the table, provide:
   - Link to the Prow job page
   - Brief summary of what failed and why
   - Suggested next action

6. If there's no recent rehearsal, report that and suggest triggering one with:
   ```bash
   gh pr comment 75733 --repo openshift/release --body "/pj-rehearse pull-ci-stolostron-capi-tests-configure-prow-capz-e2e"
   ```
