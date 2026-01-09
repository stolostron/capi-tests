# Orphaned Azure Resources Discovery and Cleanup

When deleting ARO HCP clusters or resource groups, some Azure resources may become orphaned - they survive the parent resource group deletion but remain in Azure. This document explains how to find and clean up these orphaned resources.

## Background

Orphaned resources can occur when:
- Resource group deletion doesn't fully complete
- Race conditions during deletion
- Resource locks prevent deletion
- Dependencies between resources cause partial failures

Common orphaned resources after ARO HCP cluster deletion:
- **Managed Identities** (control-plane and data-plane components)
- **Virtual Networks**
- **Network Security Groups**
- **DNS Zones**

## Prerequisites

- Azure CLI installed and authenticated (`az login`)
- Azure Resource Graph extension (`az extension add --name resource-graph`)
- `jq` for JSON processing (optional but recommended)

## Finding Orphaned Resources

### Step 1: Check if Resource Group Exists

```bash
az group show --name <RESOURCE_GROUP_NAME> -o table
```

If you get `ResourceGroupNotFound` error but suspect resources still exist, proceed to Step 2.

### Step 2: Query Resource Graph

Azure Resource Graph indexes all resources across subscriptions. Use it to find resources claiming to belong to a deleted resource group:

```bash
# List orphaned resources by name and type
az graph query -q "Resources | where resourceGroup =~ '<RESOURCE_GROUP_NAME>' | project name, type, id" -o table
```

Example with JSON output:
```bash
az graph query -q "Resources | where resourceGroup =~ 'rcap-stage-resgroup' | project name, type" \
  -o json | jq -r '.data[] | "\(.name) | \(.type)"'
```

### Step 3: Verify Resources Actually Exist

Resource Graph has indexing lag (up to 24-48 hours). Verify resources exist with direct API calls:

```bash
az resource show --ids "<RESOURCE_ID>"
```

If the resource exists, you'll see its details. If deleted, you'll get `ResourceNotFound`.

## Example Output

```bash
$ az group show --name rcap-stage-resgroup -o table
ERROR: (ResourceGroupNotFound) Resource group 'rcap-stage-resgroup' could not be found.

$ az graph query -q "Resources | where resourceGroup =~ 'rcap-stage-resgroup' | project name, type" \
    -o json | jq -r '.data[] | "\(.name) | \(.type)"'
rcap-<prefix>-cp-cloud-network-config-<hash> | microsoft.managedidentity/userassignedidentities
rcap-<prefix>-cp-cluster-api-azure-<hash> | microsoft.managedidentity/userassignedidentities
rcap-<prefix>-cp-control-plane-<hash> | microsoft.managedidentity/userassignedidentities
rcap-<prefix>-cp-disk-csi-driver-<hash> | microsoft.managedidentity/userassignedidentities
rcap-<prefix>-cp-file-csi-driver-<hash> | microsoft.managedidentity/userassignedidentities
rcap-<prefix>-cp-image-registry-<hash> | microsoft.managedidentity/userassignedidentities
rcap-<prefix>-cp-kms-<hash> | microsoft.managedidentity/userassignedidentities
rcap-<prefix>-dp-disk-csi-driver-<hash> | microsoft.managedidentity/userassignedidentities
rcap-<prefix>-dp-file-csi-driver-<hash> | microsoft.managedidentity/userassignedidentities
<prefix>-nsg | microsoft.network/networksecuritygroups
<prefix>-vnet | microsoft.network/virtualnetworks
```

## Deleting Orphaned Resources

### Batch Deletion

Delete all orphaned resources in a resource group:

```bash
az graph query -q "Resources | where resourceGroup =~ '<RESOURCE_GROUP_NAME>' | project id" -o json | \
  jq -r '.data[].id' | \
  while read id; do
    echo "Deleting: $(basename "$id")"
    az resource delete --ids "$id"
  done
```

### Deletion Order

Some resources have dependencies. If deletion fails, follow this order:

1. **Managed Identities** (no dependencies)
2. **Virtual Network** (may reference NSG via subnet associations)
3. **Network Security Group** (referenced by VNet subnets)

```bash
# Delete VNet first, then NSG
az resource delete --ids "/subscriptions/<SUB_ID>/resourceGroups/<RG>/providers/Microsoft.Network/virtualNetworks/<VNET_NAME>"
az resource delete --ids "/subscriptions/<SUB_ID>/resourceGroups/<RG>/providers/Microsoft.Network/networkSecurityGroups/<NSG_NAME>"
```

### Single Resource Deletion

```bash
az resource delete --ids "<FULL_RESOURCE_ID>" --verbose
```

## Verification

After deletion, verify using direct API calls (not Resource Graph due to lag):

```bash
az resource show --ids "<RESOURCE_ID>"
# Expected: ERROR: (ResourceNotFound) The Resource '...' was not found.
```

## Important Notes

1. **Resource Graph Indexing Lag**: Deleted resources may still appear in Resource Graph queries for up to 24-48 hours. Always use `az resource show` for verification.

2. **Soft-Deleted Resources**: Some resources (Key Vaults, Storage Accounts) support soft-delete. Check for soft-deleted resources:
   ```bash
   az keyvault list-deleted --query "[?contains(name, '<PREFIX>')]" -o table
   ```

3. **DNS Zones**: ARO HCP creates DNS zones in separate resource groups. Check for orphaned DNS zones:
   ```bash
   az graph query -q "Resources | where type == 'microsoft.network/dnszones' and name contains '<PREFIX>'" -o table
   ```

4. **Role Assignments**: Managed Identity role assignments may persist. Clean up orphaned assignments:
   ```bash
   az role assignment list --query "[?contains(principalName, '<PREFIX>')]" -o table
   ```

## Related Documentation

- [ARO HCP Domain Prefix Reservation](https://github.com/RadekCap/CAPZTests/issues/289) - Cluster name reuse limitations
- [Azure Resource Graph Query Language](https://learn.microsoft.com/en-us/azure/governance/resource-graph/concepts/query-language)
- [Azure CLI Resource Management](https://learn.microsoft.com/en-us/cli/azure/resource)

## Troubleshooting

### "ResourceNotFound" when deleting

The resource may have already been deleted but Resource Graph hasn't updated. Verify with:
```bash
az resource show --ids "<RESOURCE_ID>"
```

### "AuthorizationFailed" when deleting

You may lack permissions. Ensure you have:
- `Microsoft.Resources/subscriptions/resources/delete` permission
- Or `Contributor` role on the subscription

### Deletion hangs or times out

Some resources take time to delete. Add `--no-wait` for async deletion:
```bash
az resource delete --ids "<RESOURCE_ID>" --no-wait
```
