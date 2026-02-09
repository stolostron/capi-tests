# Test 11: TestCleanup_VerifyServicePrincipals

**Location:** `test/08_cleanup_test.go:384-422`

**Purpose:** Check for Service Principals matching the user prefix.

---

## Commands Executed

| Command | Purpose |
|---------|---------|
| `az ad sp list --filter "startswith(displayName, '<prefix>')" --query "[].{displayName: displayName, appId: appId}" --output table` | Search for service principals |

---

## Detailed Flow

```
1. Prerequisites:
   ├── Azure CLI available → Skip if not
   └── Azure authenticated → Skip if not

2. Search for service principals with prefix:
   │
   └── az ad sp list --filter "startswith(displayName, '<prefix>')"
       │
       ├── No SPs found → "No Service Principals found"
       │
       └── SPs found → List display names and app IDs
```

---

## Key Notes

- Service Principals are the runtime identity associated with AD Applications
- Uses `startswith` OData filter for precise matching
- Must be cleaned up along with their associated AD Applications
