# Test 7: TestCleanup_AzureAuthentication

**Location:** `test/08_cleanup_test.go:227-248`

**Purpose:** Verify Azure authentication for cleanup operations.

---

## Commands Executed

| Command | Purpose |
|---------|---------|
| `az account show --output json` | Check Azure CLI authentication status |

---

## Detailed Flow

```
1. Check Azure CLI available:
   └── Skip if not

2. Check authentication:
   │
   └── az account show --output json
       │
       ├── Error → "Not logged in, run az login"
       │
       └── Success → Display account info
```

---

## Key Notes

- Skips if Azure CLI is not available (depends on test 6)
- Displays subscription and tenant info for verification
- Directs users to `az login` if not authenticated
