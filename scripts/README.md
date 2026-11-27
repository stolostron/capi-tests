# Scripts Directory

This directory contains utility scripts used by the CAPI test suite.

## check-stale-resources.sh

Detect Azure resources left by test runs:

```bash
./scripts/check-stale-resources.sh --azure --max-age 24
./scripts/check-stale-resources.sh --azure --max-age 24 --json
```

The report records the UTC observation time (scan start) and subscription ID.
Resource-group discovery is a snapshot: each candidate is checked with
`az group show` after its age lookups, using the selected subscription explicitly.
The report describes what was observed then; another process can delete a group
after verification and before a later manual check.

Groups are classified separately:

- `azure.resource_groups`: verified groups older than the threshold and not being
  deleted. Only these groups contribute to `stale_found` and `total_count`, along
  with confirmed stale AD applications and service principals.
- `azure.unknown_age_resource_groups`: verified groups whose age could not be
  determined, including empty groups. No age above the threshold is inferred.
- `azure.deleting_resource_groups`: verified groups with provisioning state
  `Deleting`. These are informational because cleanup is already underway.
- `azure.disappeared_resource_groups`: groups for which direct revalidation
  returned `ResourceGroupNotFound` after discovery. These are informational.

Exit status is **0** when no confirmed stale resources are found, **1** when
confirmed stale resources are found, and **2** when the scan is incomplete.
Unknown-age, deleting, and disappeared groups do not fail the check. Authentication,
authorization, throttling, transport, and malformed-response errors are preserved
in `errors`, set `scan_complete` to `false`, and take precedence over exit status 1.
An unsuccessful lookup is never treated as a successfully queried empty group.

The scheduled workflow publishes JSON artifacts and a summary for informational
findings and incomplete scans as well as stale resources. Its stale-resource
failure and Jira actions apply only to confirmed stale findings; lookup errors
fail the scan job separately. Existing retroactive ownership tagging of confirmed
convention matches is retained. The detector does not delete resources.

Run regression checks without Azure access:

```bash
go test -mod=mod ./test -run '^TestStaleResource' -count=1
shellcheck scripts/check-stale-resources.sh
```

## monitor-cluster-json.sh

**Purpose**: Provides comprehensive JSON output for CAPI cluster status monitoring.

**Usage**:
```bash
./scripts/monitor-cluster-json.sh <namespace> <cluster-name>
```

**Example**:
```bash
./scripts/monitor-cluster-json.sh capz-test-20260305-223538 mv-stage | jq .
```

**Output**: JSON containing:
- Cluster status (phase, readiness flags, conditions)
- Infrastructure cluster (AROCluster, ROSACluster, etc.)
- Control plane (AROControlPlane, ROSAControlPlane, etc.)
- Machine pools with infrastructure details
- Workload cluster nodes (if accessible via kubeconfig)
- Summary with counts and readiness

**Provider Support**: Works with any CAPI cluster:
- ARO (Azure Red Hat OpenShift)
- ROSA (Red Hat OpenShift Service on AWS)
- Any other CAPI-compatible provider

**Go Integration**: See `test/cluster_monitor.go` for Go functions that call this script and parse the JSON output.

## Source

This script is maintained in the [cluster-api-installer](https://github.com/RadekCap/cluster-api-installer) repository and copied here for local use by the test suite.
