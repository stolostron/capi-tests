# Test 10: TestCleanup_VerifyADApplications

**Location:** `test/08_cleanup_test.go:343-381`

**Purpose:** Check for Azure AD Applications (App Registrations) matching the user prefix.

---

## Commands Executed

| Command | Purpose |
|---------|---------|
| `az ad app list --filter "startswith(displayName, '<prefix>')" --query "[].{displayName: displayName, appId: appId}" --output table` | Search for AD apps |

---

## Detailed Flow

```
1. Prerequisites:
   ├── Azure CLI available → Skip if not
   └── Azure authenticated → Skip if not

2. Search for AD apps with prefix:
   │
   └── az ad app list --filter "startswith(displayName, '<prefix>')"
       │
       ├── No apps found → "No AD Applications found"
       │
       └── Apps found → List display names and app IDs
```

---

## Key Notes

- Uses `startswith` OData filter for precise matching
- AD Applications are created during cluster deployment for service authentication
- These are separate from the resource group and must be cleaned up independently
