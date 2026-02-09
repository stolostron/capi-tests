# Test 17: TestCleanup_ResourceDiscoveryPrefixMatching

**Location:** `test/08_cleanup_test.go:616-666`

**Purpose:** Verify resource discovery prefix matching is accurate, comparing different filter strategies.

---

## Commands Executed

| Command | Purpose |
|---------|---------|
| `az ad app list --filter "startswith(displayName, '<prefix>')" --query "[].displayName" -o json` | AD apps with exact prefix match |
| `az graph query -q "Resources \| where name contains '<prefix>' \| project name \| limit 5"` | Resources with broader match |

---

## Detailed Flow

```
1. Prerequisites:
   ├── Azure CLI available → Skip if not
   └── Azure authenticated → Skip if not

2. Test AD Apps with startswith filter:
   └── More precise - only matches resources starting with prefix

3. Test Resource Graph with contains filter:
   └── More permissive - matches prefix anywhere in name
   └── Requires resource-graph extension
```

---

## Key Notes

- Highlights the difference between `startswith` (AD apps) and `contains` (Resource Graph)
- `startswith` is more precise but not available for all Azure resource types
- `contains` may return false positives if the prefix is common
- This test validates the accuracy of both approaches for the given CAPZ_USER prefix
