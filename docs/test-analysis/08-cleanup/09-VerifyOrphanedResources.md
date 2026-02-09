# Test 9: TestCleanup_VerifyOrphanedResources

**Location:** `test/08_cleanup_test.go:293-340`

**Purpose:** Check for orphaned Azure resources that survive resource group deletion.

---

## Commands Executed

| Command | Purpose |
|---------|---------|
| `az extension show --name resource-graph` | Check if Resource Graph extension is installed |
| `az graph query -q "Resources \| where name contains '<prefix>' \| project name, type, resourceGroup \| limit 10"` | Search for orphaned resources |

---

## Detailed Flow

```
1. Prerequisites:
   ├── Azure CLI available → Skip if not
   ├── Azure authenticated → Skip if not
   └── Resource Graph extension installed → Skip if not

2. Search for resources with prefix:
   │
   └── az graph query -q "Resources | where name contains '<prefix>' ..."
       │
       ├── No matches → "No orphaned resources found"
       │
       └── Matches found → List resources
```

---

## Key Notes

- Uses Azure Resource Graph for cross-resource-group search
- Requires the `resource-graph` extension (`az extension add --name resource-graph`)
- Searches by CAPZ_USER prefix using `contains` (more permissive than `startswith`)
- Limited to 10 results for efficiency
- Some resources (Managed Identities, VNets, NSGs) can survive resource group deletion
