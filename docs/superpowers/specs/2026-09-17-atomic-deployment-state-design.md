# Atomic, Phase-Aware Deployment State

## Context

ARO-24296 identifies deployment-state loss as a high-risk gap during partial
or interrupted deployments. The current state file records mostly static
configuration, is written directly to its final path, and is updated by
multiple helpers with inconsistent error handling. It does not tell cleanup
which phase was active, whether that phase failed, or which resources the run
owned.

This change is the implementation of recommendation 2 from ARO-24296. It is
stacked on PR #854 (`fix/aro-29533-resource-group-validation`), which provides
the stable run-context path used by all phase processes.

## Goals

1. Never replace a valid deployment state with a truncated or partially written
   file.
2. Record enough lifecycle information to distinguish a fresh run, an active
   phase, a successful phase, and a failed/interrupted phase.
3. Version the persisted JSON schema and retain compatibility with state files
   written before this change.
4. Persist resource ownership information that cleanup and recovery can use.
5. Keep the existing sequential phase workflow and public configuration
   behavior intact.

## Non-goals

- Compensating rollback for partially applied CRs; that is recommendation 1.
- Waiting for Azure resource-group deletion or improving orphan discovery; that
  is recommendation 3 and 5.
- Building a new external state store.
- Changing the meaning of existing environment variables.

## State model

`DeploymentState` remains the persisted deployment record, with a schema
version field added at the top level. Version 2 contains the existing identity
and configuration fields plus:

- `schema_version`: integer identifying the JSON schema. Missing or zero is
  interpreted as version 1.
- `phase`: the current or last phase name.
- `phase_status`: `pending`, `running`, `succeeded`, or `failed`.
- `phase_history`: ordered phase records containing phase name, status, start
  time, completion time, and an error summary when applicable.
- `last_error`: the most recent phase failure summary, if any.
- `resources`: deduplicated managed-resource records. Each record identifies a
  provider, resource type, name, optional namespace/resource group, and status.

The existing resource-group, management-cluster, workload-cluster,
namespace, prefix, region, user, environment, test-run ID, tags, and MCE
original-state fields remain authoritative for compatibility. Resource records
are additive and must not contain secrets.

Phase status is lifecycle metadata, not a claim that cloud cleanup succeeded.
State is considered active until cleanup explicitly records cleanup completion
and removes the state file through the existing cleanup path.

## Persistence and atomicity

All writes to deployment state, including MCE-state updates, go through one
internal writer. The writer:

1. marshals the complete state;
2. creates a temporary file in the same directory with owner-only permissions;
3. writes all bytes and synchronizes the file;
4. closes the file; and
5. renames the temporary file over the destination.

The temporary file is removed on failure. A failed write returns an error and
leaves the previous destination file untouched. The writer must not silently
discard errors from reading existing state; callers that update state must
either fail the update or deliberately initialize a new state record when the
destination does not exist.

The run-context file continues to have its existing lifecycle, but deployment
state migration and updates must use the run-context-derived state path. Legacy
state files remain readable through the existing fallback path.

## Phase and resource APIs

Add small helpers that update state through read-modify-write operations:

- `StartDeploymentPhase(phase string) error`
- `CompleteDeploymentPhase(phase string) error`
- `FailDeploymentPhase(phase string, cause error) error`
- `RecordDeploymentResource(resource DeploymentResource) error`

Each helper is idempotent for repeated calls from rerun-safe phases. Starting
the same phase again updates its active record rather than appending unlimited
duplicates. Resource records are keyed by provider, type, namespace,
resource-group, and name.

Use these helpers at the existing phase boundaries and for resources already
available without cloud-specific discovery: management cluster, workload
cluster, workload namespace, Azure resource group, generated infrastructure
resources, and MCE state restoration metadata. A phase failure must be recorded
before the test returns its failure. Existing test failure behavior remains
unchanged if state recording itself fails; the state-recording error is logged
as an additional diagnostic.

## Compatibility and malformed state

Reading a missing state file continues to return no state. Reading a valid
version-1 file populates the existing fields and exposes it as an in-memory
version-2 state with empty lifecycle/resource fields; the next successful
write persists the version-2 schema. The legacy `azure_resource_tags` spelling
continues to be accepted.

Malformed or unreadable state is an error. No caller may fall back to generated
configuration after a malformed state file, because doing so could direct
cleanup at the wrong resources. The original file remains untouched when a
migration or update cannot be persisted.

## Verification

Unit tests must prove:

- the resulting file is valid JSON with schema version 2 and mode `0600`;
- a forced write failure preserves the prior file contents;
- version-1 and legacy-tag files remain readable and migrate on write;
- phase start, completion, failure, and rerun behavior are deterministic;
- resource records are deduplicated and retain prior resources during updates;
- malformed state returns an error rather than falling back;
- MCE-state updates use the same atomic writer; and
- cleanup-state deletion remains the final action after a successful cleanup.

Documentation will describe the schema, phase statuses, compatibility rules,
and the fact that retained state is the recovery record after interruption.
