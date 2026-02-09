# Test 5: TestCleanup_VerifyDeploymentStateFile

**Location:** `test/08_cleanup_test.go:175-194`

**Purpose:** Verify deployment state file can be identified for cleanup.

---

## Detailed Flow

```
1. Check if .deployment-state.json exists:
   ├── Not found → "Deployment state file not found (clean state)"
   └── Found:
       ├── Read and display contents
       └── Note: "automatically removed by make clean"
```

---

## Key Notes

- The `.deployment-state.json` file records what was deployed (cluster name, namespace, etc.)
- Created during Phase 3 (Kind cluster deployment) by `WriteDeploymentState()`
- Used by cleanup scripts to know what resources to clean up
