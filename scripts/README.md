# Scripts Directory

This directory contains utility scripts used by the CAPI test suite.

## report-security-finding.sh

Reports a security scanner finding to Jira. It searches for an open issue for
the same scanner under `JIRA_PARENT_KEY`, adds a comment when one exists, and
creates a new ARO Task otherwise.

```bash
JIRA_EMAIL=... JIRA_API_TOKEN=... JIRA_PARENT_KEY=ARO-12345 \
  ./scripts/report-security-finding.sh \
    --scanner gosec \
    --summary '[Security][gosec] Security findings detected' \
    --body-file jira-report.md
```

The security workflows provide `JIRA_EMAIL` and `JIRA_API_TOKEN` from GitHub
Secrets and `JIRA_SECURITY_PARENT_KEY` from a repository variable. The helper
uses Jira v3, the ARO security/component defaults, the configured assignee,
and the `no-qe` label by default.

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
