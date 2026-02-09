# Test 4: TestCleanup_VerifyResultsDirectoryRemoval

**Location:** `test/08_cleanup_test.go:145-172`

**Purpose:** Verify results directory can be identified for cleanup.

---

## Detailed Flow

```
1. Check if results/ directory exists:
   ├── Not found → "Results directory not found (clean state)"
   └── Found:
       ├── List directory contents
       └── Report entry count and sizes
```

---

## Key Notes

- The results directory contains controller logs saved during verification (Phase 6)
- Files include `capi-controller.log`, `capz-controller.log`, `aso-controller.log`
- Does not delete the directory - only identifies it
