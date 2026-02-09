# Test 8: TestCleanup_VerifyResourceGroupStatus

**Location:** `test/08_cleanup_test.go:251-290`

**Purpose:** Check the Azure resource group status and list any remaining resources.

---

## Commands Executed

| Command | Purpose |
|---------|---------|
| `az group show --name <rg-name> --output json` | Check if resource group exists |
| `az resource list --resource-group <rg-name> --output table` | List resources in the group |

---

## Detailed Flow

```
1. Prerequisites:
   ├── Azure CLI available → Skip if not
   └── Azure authenticated → Skip if not

2. Derive resource group name:
   └── resourceGroup = "${CS_CLUSTER_NAME}-resgroup"

3. Check resource group:
   │
   └── az group show --name <rg-name>
       │
       ├── Error → "Resource group does not exist (clean state)"
       │
       └── Exists:
           ├── Display resource group info
           └── List resources in group
```

---

## Key Notes

- Resource group name: `${CAPZ_USER}-${DEPLOYMENT_ENV}-resgroup`
- Directs users to `make clean-azure` for cleanup
- Lists individual resources if the group still exists
