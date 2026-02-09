# Test 5: TestDeletion_VerifyAzureResourcesDeletion

**Location:** `test/07_deletion_test.go:208-262`

**Purpose:** Verify Azure resources are cleaned up after cluster deletion by checking the resource group status.

---

## Commands Executed

| Command | Purpose |
|---------|---------|
| `az account show` | Verify Azure CLI authentication |
| `az group show --name <rg-name>` | Check if resource group exists |
| `az resource list --resource-group <rg-name> --output table` | List remaining resources |

---

## Detailed Flow

```
1. Prerequisites:
   ├── Check Azure CLI available → Skip if not
   └── Check Azure CLI authenticated → Skip if not

2. Derive resource group name:
   └── resourceGroup = "${CS_CLUSTER_NAME}-resgroup"

3. Check resource group:
   │
   └── az group show --name <rg-name>
       │
       ├── Error (not found) → "Resource group has been deleted"
       │
       ├── Error (other) → Warning: "Could not check RG status"
       │
       └── Exists → Warning + list remaining resources:
           └── az resource list --resource-group <rg-name> --output table
```

---

## Resource Group Naming

The Azure resource group name is derived from `CS_CLUSTER_NAME`:

```
CS_CLUSTER_NAME = ${CAPZ_USER}-${DEPLOYMENT_ENV}
Resource Group  = ${CS_CLUSTER_NAME}-resgroup

Example: rcap-stage-resgroup
```

---

## Key Notes

- Skips gracefully if Azure CLI is not available or not authenticated
- Resource group deletion may still be in progress when this test runs
- An empty resource group or one marked for deletion is considered acceptable

---

## Example Output

```
=== RUN   TestDeletion_VerifyAzureResourcesDeletion
    Checking Azure resource group 'rcap-stage-resgroup'...
    Resource group 'rcap-stage-resgroup' has been deleted
--- PASS: TestDeletion_VerifyAzureResourcesDeletion (1.23s)
```
