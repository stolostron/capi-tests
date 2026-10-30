# Manual CRD removal test decision

Decision for [ARO-24406](https://redhat.atlassian.net/browse/ARO-24406).
Evidence reviewed: 2026-10-30. This records the proposed test policy; it does
not implement or execute CRD deletion.

## Decision

Keep manual CRD removal out of the normal workload lifecycle and cleanup
phases. Recommend a separate, explicitly selected uninstall/cleanup/reinstall
scenario on a disposable OpenShift hub dedicated to that scenario. Implement
that scenario only after the version-specific CAPZ removal procedure and CRD
inventory are reviewed through
[ARO-24405](https://redhat.atlassian.net/browse/ARO-24405).

Routine workload deletion should let the installed controllers reconcile the
workload's resources. In external MCE mode, retain the management installation
and restore component settings changed by the run, as
[`TestTeardown_RevertMCEComponents`](../test/09_teardown_test.go) does. A remaining
management CRD alone is not a failed workload cleanup assertion. Investigate
remaining workload CRs and cloud resources separately; deleting their API
definitions is not evidence that their infrastructure was cleaned up.

## Evidence and scope

The [ACM 2.14 uninstall guide](https://docs.redhat.com/en/documentation/red_hat_advanced_cluster_management_for_kubernetes/2.14/html-single/install/index#uninstalling)
distinguishes removing the MultiClusterHub instance from uninstalling the
operator, and says a complete operator uninstall can leave CRDs. Its
prerequisites include detaching managed clusters other than the self-managed
hub and removing enabled services as applicable.

The [ACM 2.14 artifact cleanup guide](https://docs.redhat.com/en/documentation/red_hat_advanced_cluster_management_for_kubernetes/2.14/html-single/install/index#cleanup-artifacts)
addresses reinstalling on a previously used hub, including installing an older
ACM version. It requires cluster administrator access and contains explicit
CRD and other artifact removals. The published list reviewed here contains ACM
CRDs; it does not establish a CAPZ CRD inventory.

ARO-24405 requests published CAPZ CRD cleanup instructions and remains the
source for that missing product guidance. This decision does not infer a
supported CAPZ removal procedure from the general ACM script. Any future run
must use documentation matching its installed product versions.

## Proposed isolated scenario

This is a test design for a follow-up implementation, not a runnable procedure
or a newly supported uninstall path.

1. Provision a dedicated disposable hub through an approved test environment.
   Record the cluster identity, ACM/MCE/CAPZ versions and exact documentation
   revision. Use an explicit kubeconfig; reject shared or unidentified hubs.
2. Complete the workload lifecycle first, with controllers still available.
   Confirm the scenario's workload CRs and cloud resources are gone. Complete
   the product uninstall prerequisites, including attached-cluster and service
   checks. Abort on remaining dependents or incomplete cleanup.
3. Inventory the exact CRDs named in the approved version-specific procedure,
   their custom resources across all namespaces and cluster scope as applicable,
   and the other artifacts named by that procedure. Confirm that no retained
   component depends on them. Save this evidence before any destructive step.
4. Follow the documented uninstall sequence. Capture any residual CRDs before
   manual cleanup so the report distinguishes operator behavior from manual
   intervention. Require explicit authorization for the complete destructive
   scenario, including cluster-scoped cleanup; a namespace alone is not an
   isolation boundary for CRDs.
5. Apply only the approved explicit artifact list. Never discover deletion
   targets using a broad name/group wildcard. Unexpected CRs, unknown ownership,
   stuck deletion or authorization failures must stop the scenario and retain
   diagnostics; do not remove finalizers to make it pass.
6. Verify the selected residual CRDs and documented artifacts are absent with
   successful API queries. Treat permission, connection and discovery failures
   as failures, not absence. Repeat the approved cleanup and verify its
   documented behavior for already absent targets, without assuming the script
   is idempotent. Record unexpected errors.
7. Reinstall the documented supported target version on the same hub. Verify
   operator readiness, recreated required CRDs and a minimal CAPZ workload
   create/verify/delete cycle. Report reinstall or reconciliation failure even
   when every selected CRD was removed.

Acceptance requires captured before/after inventories, successful targeted
removal, successful reinstall and workload reconciliation, and no unexplained
changes to resources outside the approved scope. An absent optional CRD at the
start is reported as not exercised, not counted as proof of successful removal.
Retain command outcomes and diagnostics without credentials or secret contents.

## Alternatives and follow-up

Adding CRD deletion to phases 07–09 would couple workload cleanup to management
platform destruction and make reuse of an external hub unsafe. Read-only
residual inventory is useful diagnostic evidence but cannot prove the manual
removal and reinstall path works. The isolated scenario provides that evidence
without changing the ordinary lifecycle contract.

No new environment flag, Make target, CI job or deletion helper is introduced
by this decision. A follow-up implementation needs the reviewed CAPZ artifact
list from ARO-24405, a disposable hub owner and supported version combination,
and an explicit opt-in execution and authorization mechanism. Until those are
available, record this scenario as deferred rather than claiming automated
coverage. Live deletion and reinstall validation have not been performed for
this decision.
