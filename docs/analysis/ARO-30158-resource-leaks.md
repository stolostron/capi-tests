# ARO-30158: CAPZ ARO resource leaks after control-plane wait failures

**Investigation date:** 2026-10-16
**Ticket:** [ARO-30158](https://redhat.atlassian.net/browse/ARO-30158)
**Scope:** Explain the reported Azure resource-group leaks after failed ARO deployment waits, and recommend cleanup and validation changes.

## Findings

The failure path skips the normal workload-cluster deletion phase. CI then runs its fallback cleanup, but that cleanup deletes Azure resources before stopping the Kind management cluster. All nine cited runs show the Azure resource-group delete command returning successfully, followed by an Azure AD application-list failure. That failure aborts `make clean-all` before it reaches Kind deletion.

The still-running management cluster retains the CAPZ resources and controllers. They may recreate Azure resources after the direct resource-group deletion. This is the leading explanation for the later resource-group inventory, but the available logs do not prove recreation. Azure Activity Log entries between deletion and the 2026-10-12 inventory are needed to distinguish recreation from a deletion that did not persist.

## Evidence

1. The ticket reports that all nine runs failed `TestDeployment_WaitForControlPlane` after roughly 41–44 minutes. The phase runner stops after a failed deployment-monitor phase, so `_delete-workload-cluster` is not reached ([`_test-all-impl`](../../Makefile#L407), [`_deploy-crs`](../../Makefile#L273)).
2. The ARO workflow has an `if: always()` cleanup step that runs `FORCE=1 make clean-all` ([`Cleanup resources`](../../.github/workflows/workload-cluster-aro.yml#L519)).
3. [`clean-all`](../../Makefile#L670) invokes `_clean-azure-conditional` before its Kind-cluster deletion recipe. GNU Make stops the target when the Azure cleanup submake fails.
4. [`cleanup-azure-resources.sh`](../../scripts/cleanup-azure-resources.sh#L208) calls `az group delete --name ... --yes` without `--no-wait`; Azure CLI therefore waits for the delete operation by default ([Microsoft Learn: `az group delete`](https://learn.microsoft.com/en-us/cli/azure/group?view=azure-cli-lts#az-group-delete)). It prints “Resource group deleted” when that command exits successfully. It then searches Azure Resource Graph and lists Azure AD applications. In every cited run, the log shows “No resources found matching prefix …” followed by “Failed to list Azure AD Applications …” and a nonzero `clean-all` exit. The script suppresses the Azure CLI error output, so the underlying app-list failure is unknown.
5. Because `_clean-azure-conditional` fails, the later Kind deletion recipe is skipped. The workflow marks the cleanup step `continue-on-error: true`, so its cleanup failure does not trigger a second cleanup attempt.
6. The ticket records a live Azure check on 2026-10-12: all nine resource groups were present, with five managed identities across three groups. The ticket does not name those identities or specify which three groups contained them; no Jira comments or attachments add that detail.

The code ordering and cleanup failure are confirmed. Resource recreation by active CAPZ controllers is an inference from that ordering and the later inventory, not a proven Azure event sequence.

## Affected runs and resource groups

| Run | ARO branch | Resource group | Resource prefix |
|---|---|---|---|
| [37095733197](https://github.com/stolostron/capi-tests/actions/runs/37095733197) | `backplane-2.17` | `capz-tests-9c558-resgroup` | `runner-9c558` |
| [37179874787](https://github.com/stolostron/capi-tests/actions/runs/37179874787) | `backplane-2.17` | `capz-tests-776fd-resgroup` | `runner-776fd` |
| [37186513378](https://github.com/stolostron/capi-tests/actions/runs/37186513378) | `backplane-5.1` | `capz-tests-18793-resgroup` | `runner-18793` |
| [37267009133](https://github.com/stolostron/capi-tests/actions/runs/37267009133) | `backplane-5.1` | `capz-tests-f10b0-resgroup` | `runner-f10b0` |
| [37270663224](https://github.com/stolostron/capi-tests/actions/runs/37270663224) | `backplane-5.1` | `capz-tests-88d24-resgroup` | `runner-88d24` |
| [37287057279](https://github.com/stolostron/capi-tests/actions/runs/37287057279) | `backplane-5.0` | `capz-tests-9de68-resgroup` | `runner-9de68` |
| [37311189508](https://github.com/stolostron/capi-tests/actions/runs/37311189508) | `backplane-5.1` | `capz-tests-3423d-resgroup` | `runner-3423d` |
| [37412231371](https://github.com/stolostron/capi-tests/actions/runs/37412231371) | `backplane-2.17` | `capz-tests-67bd2-resgroup` | `runner-67bd2` |
| [37579171147](https://github.com/stolostron/capi-tests/actions/runs/37579171147) | `backplane-5.1` | `capz-tests-372da-resgroup` | `runner-372da` |

Each log records a successful Azure CLI resource-group deletion message, an empty immediate Resource Graph prefix search, and then the same Azure AD application-list failure. The resource-group and managed-identity counts above are the 2026-10-12 snapshot from the Jira ticket, not a current Azure inventory.

## Recommended mitigation

1. After a deployment failure, attempt graceful deletion of the workload `Cluster` and wait for its finalizers with a bounded timeout.
2. Stop/delete the Kind management cluster before the fallback Azure resource-group sweep, so CAPZ controllers cannot reconcile resources while cleanup is running.
3. Make cleanup best-effort across independent steps: record cloud-cleanup errors, still remove local test infrastructure, then return a combined failure status. Preserve the Azure CLI error output for failed app/service-principal discovery.
4. After the controllers are stopped, delete the resource group and verify it is absent. Search for run-tagged/prefixed Azure resources and report any that remain. Record the exact resource IDs and query Azure Activity Logs when a group remains or reappears.

## Validation recommendation

Exercise a failed deployment-monitor path in a disposable test subscription, then verify the cleanup sequence in CI logs:

- The workload-cluster deletion attempt is bounded and its outcome is reported.
- The Kind management cluster is stopped even when Azure AD application discovery fails.
- Azure cleanup runs after controller shutdown and reports failure if the resource group still exists after deletion.
- A post-cleanup Resource Graph query returns no resources tagged for or prefixed with that run; any surviving managed identities are listed by exact resource ID.
- The workflow remains failed for the deployment error and separately exposes any cleanup error.

This ticket asks for investigation and recommendations. It does not require applying the cleanup redesign; the validation items above are proposed follow-up work.
