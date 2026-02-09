# Test 14: TestCleanup_DryRunMode

**Location:** `test/08_cleanup_test.go:488-534`

**Purpose:** Verify the cleanup script dry-run mode works correctly without deleting anything.

---

## Commands Executed

| Command | Purpose |
|---------|---------|
| `bash ../scripts/cleanup-azure-resources.sh --prefix <user> --dry-run` | Run cleanup in dry-run mode |

---

## Detailed Flow

```
1. Prerequisites:
   ├── Script exists → Skip if not
   ├── Azure CLI available → Skip if not
   └── Azure authenticated → Skip if not

2. Run script with --dry-run:
   │
   └── bash cleanup-azure-resources.sh --prefix <prefix> --dry-run
       │
       ├── Output contains "DRY-RUN" → Dry-run mode confirmed
       │
       ├── Output contains "No cleanup needed" → Clean state
       │
       └── Other output → Log for review
```

---

## Key Notes

- Dry-run mode queries Azure but does **not** delete any resources
- Still requires Azure authentication since it queries for existing resources
- Uses `CAPZ_USER` as the prefix for resource discovery
