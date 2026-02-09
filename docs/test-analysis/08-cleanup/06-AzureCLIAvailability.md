# Test 6: TestCleanup_AzureCLIAvailability

**Location:** `test/08_cleanup_test.go:201-224`

**Purpose:** Verify Azure CLI is available for cleanup operations.

---

## Commands Executed

| Command | Purpose |
|---------|---------|
| `az version --output json` | Check Azure CLI version |

---

## Detailed Flow

```
1. Check if az command exists:
   ├── Not found → "Azure CLI not installed, cleanup will be skipped"
   └── Found:
       └── az version --output json
           ├── Success → Display version info
           └── Error → Log warning
```

---

## Key Notes

- Azure CLI is optional - Azure cleanup tests will be skipped if not available
- Displays full version information (azure-cli version, extensions, etc.)
