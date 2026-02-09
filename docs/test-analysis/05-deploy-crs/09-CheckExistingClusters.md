# Test 9: TestDeployment_01_CheckExistingClusters

**Location:** `test/05_deploy_crs_test.go:70-123`

**Purpose:** Check for existing Cluster CRs that don't match the current configuration, preventing deployment conflicts.

---

## Detailed Flow

```
1. Check for mismatched clusters:
   └── CheckForMismatchedClusters(context, namespace, prefix)
       │
       ├── Error → Warning (non-fatal), continue
       │
       └── Results:
           ├── Get all existing cluster names
           ├── Display which match current config
           └── Display which don't match

2. If mismatched clusters found:
   └── Fatal error with cleanup instructions
```

---

## What It Prevents

This fail-fast check prevents deploying new clusters alongside stale resources from previous configurations. For example, if `CAPZ_USER` was changed from `user1` to `user2` without cleaning up, there would be:
- `user1-stage` cluster (stale, from previous config)
- `user2-stage` cluster (new, from current config)

This situation causes resource conflicts and confusing errors.

---

## Key Notes

- Checks cluster name prefix against `CS_CLUSTER_NAME` (e.g., `rcap-stage`)
- Provides specific cleanup commands in the error message
- Non-fatal if the check itself fails (e.g., CAPI not installed yet)
- Uses `FormatMismatchedClustersError()` for clear, actionable error messages
