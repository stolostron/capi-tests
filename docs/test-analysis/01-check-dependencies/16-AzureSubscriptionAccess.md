# Test 16: TestCheckDependencies_AzureSubscriptionAccess

**Location:** `test/01_check_dependencies_test.go:746-780`

**Purpose:** Validate that the Azure subscription is accessible and the current credentials have access.

---

## Commands Executed

| Command | Purpose |
|---------|---------|
| `az account show --query id -o tsv` | Extract subscription ID if not set |

---

## Detailed Flow

```
1. Skip in CI environments

2. Get subscription ID:
   ├── From AZURE_SUBSCRIPTION_ID env var
   └── Or extract from Azure CLI (az account show)
       └── Skip if not available

3. Validate access:
   └── ValidateAzureSubscriptionAccess(t, subscriptionID)
       ├── Error → "Subscription access validation failed"
       └── Success → Log masked subscription ID
```

---

## Key Notes

- Masks the subscription ID in output (shows first 8 and last 4 chars)
- Verifies the subscription exists and is enabled
- Prevents deployment failures due to expired or inaccessible subscriptions
