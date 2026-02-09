# Test 12: TestCleanup_ScriptExists

**Location:** `test/08_cleanup_test.go:429-457`

**Purpose:** Verify the cleanup script exists and is executable.

---

## Detailed Flow

```
1. Check script exists:
   └── FileExists("../scripts/cleanup-azure-resources.sh")
       ├── Not found → Fatal error
       └── Found → Check permissions

2. Check executable permission:
   └── info.Mode() & 0111
       ├── Not executable → Warning
       └── Executable → "Script is executable"
```

---

## Key Notes

- Script path is relative to test directory: `../scripts/cleanup-azure-resources.sh`
- This is a **fatal** test - if the script is missing, subsequent script tests will fail
- Checks Unix execute permission bits (owner, group, other)
