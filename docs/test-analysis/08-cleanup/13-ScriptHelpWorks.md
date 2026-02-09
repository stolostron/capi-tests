# Test 13: TestCleanup_ScriptHelpWorks

**Location:** `test/08_cleanup_test.go:460-485`

**Purpose:** Verify the cleanup script --help option works correctly.

---

## Commands Executed

| Command | Purpose |
|---------|---------|
| `bash ../scripts/cleanup-azure-resources.sh --help` | Display help text |

---

## Detailed Flow

```
1. Skip if script not found

2. Run script with --help:
   └── Check output contains expected keywords:
       ├── "Usage" → Valid help output
       ├── "--dry-run" → Valid help output
       └── "--prefix" → Valid help output
```

---

## Key Notes

- Validates the script's user interface
- Checks that help output documents key options (`--dry-run`, `--prefix`)
