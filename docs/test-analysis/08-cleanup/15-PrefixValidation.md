# Test 15: TestCleanup_PrefixValidation

**Location:** `test/08_cleanup_test.go:537-579`

**Purpose:** Verify the cleanup script validates prefixes correctly, rejecting invalid ones.

---

## Commands Executed

| Command | Purpose |
|---------|---------|
| `bash cleanup-azure-resources.sh --prefix <invalid> --dry-run` | Test invalid prefix rejection |
| `bash cleanup-azure-resources.sh --prefix validprefix123 --dry-run` | Test valid prefix acceptance |

---

## Invalid Prefixes Tested

| Prefix | Reason |
|--------|--------|
| `UPPER` | Uppercase letters |
| `-start-hyphen` | Starting with hyphen |
| `with spaces` | Containing spaces |
| `special!chars` | Containing special characters |

---

## Detailed Flow

```
1. Test invalid prefixes:
   └── For each invalid prefix:
       └── bash cleanup-azure-resources.sh --prefix <invalid> --dry-run
           ├── Error + "Invalid prefix" → Correctly rejected
           └── Other → Log for review

2. Test valid prefix:
   └── bash cleanup-azure-resources.sh --prefix validprefix123 --dry-run
       ├── No "Invalid prefix" error → Correctly accepted
       └── "Invalid prefix" error → Unexpected rejection
```

---

## Key Notes

- Uses table-driven test pattern with invalid prefix test cases
- Validates that the script enforces RFC 1123-like naming rules
- All tests use `--dry-run` to avoid any actual resource deletion
