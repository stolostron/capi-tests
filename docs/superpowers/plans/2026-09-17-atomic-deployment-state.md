# Atomic Deployment State Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the persisted deployment state atomic, phase-aware, schema-versioned, and resource-aware for ARO-24296 recommendation 2.

**Architecture:** Keep `DeploymentState` as the durable recovery record and extend it with versioned lifecycle and managed-resource data. Route every state mutation through one atomic same-directory temporary-file writer, while preserving the run-context path and legacy-file read compatibility from PR #854. Add narrow phase/resource mutation helpers and call them at existing phase boundaries without changing the sequential test workflow.

**Tech Stack:** Go 1.21+, Go testing package, JSON state files, POSIX filesystem rename/sync semantics, existing Makefile phase orchestration.

**Spec:** `docs/superpowers/specs/2026-09-17-atomic-deployment-state-design.md`

## Global Constraints

- Use the run-context-derived `.deployment-state.json` path and preserve the historical fallback path.
- Treat missing schema version as version 1 and write version 2 on the next successful update.
- Never fall back to generated configuration after malformed or unreadable persisted state.
- Preserve the previous state file when any atomic write step fails.
- Do not add secrets to resource records.
- Do not introduce CR rollback or Azure deletion waiting; those are separate recommendations.
- Use TDD: each production behavior starts with a failing focused test.
- Keep phase tests sequential; do not add `t.Parallel()`.
- Stage explicit files only and use `Assisted-by: Codex GPT-5 <noreply@anthropic.com>` in commits.

---

### Task 1: Add versioned lifecycle and resource types

**Files:**
- Modify: `test/helpers.go` near `DeploymentState` (currently around line 3130)
- Test: `test/helpers_test.go` beside the existing deployment-state tests

**Interfaces:**
- Produces `DeploymentStateSchemaVersion = 2`.
- Produces `DeploymentPhaseStatus` values `pending`, `running`, `succeeded`, and `failed`.
- Produces `DeploymentPhaseRecord` with JSON fields `phase`, `status`, `started_at`, `completed_at`, and `error`.
- Produces `DeploymentResource` with JSON fields `provider`, `type`, `name`, `namespace`, `resource_group`, and `status`.
- Extends `DeploymentState` with `SchemaVersion`, `Phase`, `PhaseStatus`, `PhaseHistory`, `LastError`, and `Resources`.

- [ ] **Step 1: Write failing serialization tests**

Add tests that marshal a state containing phase and resource data, assert the exact JSON keys, and assert that zero-value optional fields are omitted where the schema specifies `omitempty`.

- [ ] **Step 2: Run the focused tests to verify they fail**

Run: `GOCACHE=/tmp/capi-tests-go-cache go test ./test -run 'TestDeploymentState(Schema|Phase|Resource)' -count=1`

Expected: compile failures because the new fields and types do not exist.

- [ ] **Step 3: Add the types and fields**

Define the constants and structs next to `DeploymentState`. Use UTC RFC3339 timestamps as strings so the persisted schema remains stable across processes. Add a `resourceKey()` helper that joins provider, type, namespace, resource group, and name for deterministic deduplication.

- [ ] **Step 4: Run the focused tests to verify they pass**

Run: `GOCACHE=/tmp/capi-tests-go-cache go test ./test -run 'TestDeploymentState(Schema|Phase|Resource)' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit the type-only change**

```bash
git add test/helpers.go test/helpers_test.go
git commit -m "feat: add deployment state lifecycle schema" -m "Assisted-by: Codex GPT-5 <noreply@anthropic.com>"
```

### Task 2: Implement atomic state persistence and v1 migration

**Files:**
- Modify: `test/helpers.go` in `WriteDeploymentState`, `ReadDeploymentState`, `SaveMCEOriginalStates`, and the adjacent state helpers
- Test: `test/helpers_test.go` for persistence, migration, and atomicity cases

**Interfaces:**
- Produces `writeDeploymentState(state *DeploymentState) error` as the only internal deployment-state writer.
- `WriteDeploymentState(config *TestConfig) error` preserves existing lifecycle/history/resources while refreshing configuration identity.
- `ReadDeploymentState() (*DeploymentState, error)` returns version-2 in-memory state for valid v1 input and errors for malformed input.
- `SaveMCEOriginalStates(states map[string]bool) error` uses `writeDeploymentState` and never overwrites existing MCE entries.

- [ ] **Step 1: Write failing tests for atomic replacement and migration**

Add tests that:

1. write state into a temporary run-context directory and verify valid JSON, schema version 2, and mode `0600`;
2. inject an invalid destination directory or unwritable temporary path and verify the original state bytes remain unchanged;
3. read a version-1 state without lifecycle fields and verify it returns schema version 2 in memory with empty lifecycle/resource slices;
4. write the migrated state and verify `schema_version` is persisted as 2;
5. write malformed JSON and verify `ReadDeploymentState` returns an error; and
6. save MCE states and verify the result retains all prior fields and uses the same state writer.

- [ ] **Step 2: Run the focused tests to verify they fail**

Run: `GOCACHE=/tmp/capi-tests-go-cache go test ./test -run 'Test(Atomic|ReadDeploymentState|WriteDeploymentState|SaveMCE)' -count=1`

Expected: failures for direct writes, missing migration behavior, or missing schema fields.

- [ ] **Step 3: Implement the atomic writer**

Marshal the complete state, ensure the parent directory exists, create a same-directory temporary file with `os.OpenFile` and mode `0600`, write all bytes, call `Sync`, close it, rename it over the destination, and remove the temporary file on every failure path. Return contextual errors from each operation. Keep the destination untouched until rename succeeds.

- [ ] **Step 4: Make all state mutations use the writer**

Change `WriteDeploymentState` to fail on an existing-state read error instead of ignoring it. Preserve lifecycle/history/resources/MCE data from the existing state. Normalize missing/zero schema versions to v1 on read and set the current schema version before any successful write. Replace the `os.WriteFile` call in `SaveMCEOriginalStates` with the shared writer.

- [ ] **Step 5: Run the focused tests to verify they pass**

Run: `GOCACHE=/tmp/capi-tests-go-cache go test ./test -run 'Test(Atomic|ReadDeploymentState|WriteDeploymentState|SaveMCE)' -count=1`

Expected: PASS.

- [ ] **Step 6: Commit the persistence change**

```bash
git add test/helpers.go test/helpers_test.go
git commit -m "fix: atomically persist deployment state" -m "Assisted-by: Codex GPT-5 <noreply@anthropic.com>"
```

### Task 3: Add idempotent phase and resource mutation helpers

**Files:**
- Modify: `test/helpers.go` after the read/write/delete deployment-state helpers
- Test: `test/helpers_test.go`

**Interfaces:**
- `StartDeploymentPhase(phase string) error`
- `CompleteDeploymentPhase(phase string) error`
- `FailDeploymentPhase(phase string, cause error) error`
- `RecordDeploymentResource(resource DeploymentResource) error`

- [ ] **Step 1: Write failing lifecycle tests**

Cover a phase start, completion, failure, repeated start, and repeated completion. Assert that the current phase/status are correct, one phase-history entry is updated rather than duplicated, timestamps are non-empty and ordered, and `LastError` is set only on failure. Add resource tests that record the same key twice with changed status and assert one deduplicated record, then record a second resource and assert both remain.

- [ ] **Step 2: Run the focused tests to verify they fail**

Run: `GOCACHE=/tmp/capi-tests-go-cache go test ./test -run 'TestDeployment(StatePhase|Resource)' -count=1`

Expected: compile failures because the mutation helpers are not defined.

- [ ] **Step 3: Implement read-modify-write lifecycle helpers**

Load the state, initialize an empty version-2 state only when the file is absent, update the matching history record by phase name, and persist through `writeDeploymentState`. Reject an empty phase name. Completion clears `LastError` only when completing the current phase; failure stores `cause.Error()` and leaves the failed state available for recovery.

- [ ] **Step 4: Implement resource upsert**

Reject a resource without provider, type, or name. Find an existing record by `resourceKey()`, replace it with the new status when found, otherwise append it. Sort records by key before writing to make cross-process output deterministic.

- [ ] **Step 5: Run the focused tests to verify they pass**

Run: `GOCACHE=/tmp/capi-tests-go-cache go test ./test -run 'TestDeployment(StatePhase|Resource)' -count=1`

Expected: PASS.

- [ ] **Step 6: Commit the lifecycle helpers**

```bash
git add test/helpers.go test/helpers_test.go
git commit -m "feat: track deployment phases and resources" -m "Assisted-by: Codex GPT-5 <noreply@anthropic.com>"
```

### Task 4: Integrate lifecycle and ownership recording into phase boundaries

**Files:**
- Modify: `test/02_setup_test.go`, `test/03_cluster_test.go`, `test/04_generate_yamls_test.go`, `test/05_deploy_crs_test.go`, `test/06_verification_test.go`, `test/07_deletion_test.go`, and `test/08_cleanup_test.go`
- Test: existing phase tests plus focused helper tests where a failure path needs a unit seam

**Interfaces:**
- Consumes the four helpers from Task 3.
- Produces persisted phase records for setup, management-cluster, infrastructure, deployment, verification, deletion, and cleanup validation.

- [ ] **Step 1: Add phase-start calls at deterministic phase entry points**

At the first executable statement after each phase’s `NewTestConfig`, call `StartDeploymentPhase` with the stable phase name. Log a state-update error but preserve the existing test failure/skip behavior.

- [ ] **Step 2: Add success and failure recording at existing returns**

Call `CompleteDeploymentPhase` immediately before successful phase returns. For fatal errors, call `FailDeploymentPhase` before `t.Fatalf`/return. Do not convert existing warnings or intentional skips into deployment failures.

- [ ] **Step 3: Record known resources from existing configuration**

Record the management cluster and workload cluster from `TestConfig`, the workload namespace, and the provider resource group when the corresponding phase has enough information. Record generated YAML resource identities from the existing YAML extraction path where available. Preserve MCE original-state metadata as state data rather than inventing provider resources.

- [ ] **Step 4: Retain state until cleanup completion**

Ensure cleanup-phase success recording occurs before the existing final deletion call. If cleanup verification fails, record a failed cleanup phase and leave the state and run-context files intact. Update the cleanup documentation/test expectation to treat a remaining state file as recoverable evidence after failure.

- [ ] **Step 5: Run phase-focused tests without cloud execution**

Run: `GOCACHE=/tmp/capi-tests-go-cache go test ./test -run 'Test(DeploymentState|EnsureRunContext|Cleanup_VerifyDeploymentStateFile)' -count=1`

Expected: PASS or intentional skips for cloud-dependent tests, with no lifecycle-state regressions.

- [ ] **Step 6: Commit phase integration**

```bash
git add test/02_setup_test.go test/03_cluster_test.go test/04_generate_yamls_test.go test/05_deploy_crs_test.go test/06_verification_test.go test/07_deletion_test.go test/08_cleanup_test.go
git commit -m "feat: record deployment phase ownership" -m "Assisted-by: Codex GPT-5 <noreply@anthropic.com>"
```

### Task 5: Document the durable state contract

**Files:**
- Modify: `docs/ARCHITECTURE.md`, `docs/INTEGRATION.md`, and the deployment-state section of `README.md`
- Test: documentation inspection with `rg`

**Interfaces:**
- Documents schema version 2, lifecycle statuses, resource records, atomic-write guarantees, legacy migration, and retained state after interruption.

- [ ] **Step 1: Update architecture and integration documentation**

Describe the state file as the recovery record shared by phase processes, list the phase/status transitions, and state that malformed state fails closed rather than regenerating cleanup configuration.

- [ ] **Step 2: Update the README state-file reference**

Replace the configuration-only description with the schema-versioned, phase/resource-aware contract and explain when the file is removed.

- [ ] **Step 3: Validate documentation references**

Run: `rg -n "schema.version|phase_status|phase_history|managed|atomic|interrupted|deployment state" README.md docs/ARCHITECTURE.md docs/INTEGRATION.md`

Expected: all required concepts are documented without stale claims that state is written directly or contains configuration only.

- [ ] **Step 4: Commit documentation**

```bash
git add README.md docs/ARCHITECTURE.md docs/INTEGRATION.md
git commit -m "docs: document deployment state recovery contract" -m "Assisted-by: Codex GPT-5 <noreply@anthropic.com>"
```

### Task 6: Full verification and draft-PR handoff

**Files:**
- Modify: only files required by test failures found in Tasks 1–5

- [ ] **Step 1: Format the Go changes**

Run: `gofmt -w test/helpers.go test/helpers_test.go test/02_setup_test.go test/03_cluster_test.go test/04_generate_yamls_test.go test/05_deploy_crs_test.go test/06_verification_test.go test/07_deletion_test.go test/08_cleanup_test.go`

- [ ] **Step 2: Run the focused and repository checks**

Run:

```bash
GOCACHE=/tmp/capi-tests-go-cache go test ./test
make fmt
make lint
make test
git diff --check
```

Expected: all available checks pass. If the sandbox blocks the default Go cache, use the explicit `/tmp` cache shown above and report the limitation.

- [ ] **Step 3: Review the complete diff and commit history**

Run: `git diff fix/aro-29533-resource-group-validation...HEAD --stat`, `git diff fix/aro-29533-resource-group-validation...HEAD`, and `git log --oneline --decorate -8`. Confirm no unrelated files are included and every AI-assisted commit has the required trailer.

- [ ] **Step 4: Push the feature branch**

Run: `git push -u origin issue-24296-atomic-deployment-state`.

- [ ] **Step 5: Request final PR approval and create a draft PR**

Before `gh pr create`, show the exact target repository `stolostron/capi-tests`, base branch `fix/aro-29533-resource-group-validation`, head branch `issue-24296-atomic-deployment-state`, and draft status. Only after explicit approval run:

```bash
gh pr create --repo stolostron/capi-tests \
  --base fix/aro-29533-resource-group-validation \
  --head issue-24296-atomic-deployment-state \
  --draft \
  --title "feat: make deployment state atomic and phase-aware" \
  --body-file <prepared-pr-body>
```

The PR body must reference ARO-24296, summarize atomic persistence, lifecycle/resource tracking, compatibility behavior, and actual verification results.
