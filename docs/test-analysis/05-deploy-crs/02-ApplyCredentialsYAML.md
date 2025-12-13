# Test 2: TestDeployment_ApplyCredentialsYAML

**Location:** `test/05_deploy_crs_test.go:73-106`

**Purpose:** Apply `credentials.yaml` to the Kind cluster (individual file test).

---

## Command Executed

| Command | Purpose |
|---------|---------|
| `kubectl --context kind-<name> apply -f <path>/credentials.yaml` | Apply Azure credentials secret |

---

## Detailed Flow

```
1. Check prerequisite:
   └─ DirExists(outputDir)?
      └─ No → SKIP: "Output directory does not exist"

2. Check file exists:
   └─ FileExists(filePath)?
      └─ No → FAIL: "credentials.yaml not found"

3. Build kubectl context:
   └─ context = "kind-<ManagementClusterName>"

4. Apply file:
   └─ kubectl --context <ctx> apply -f <path>
      ├─ Success → Log "Successfully applied"
      └─ Failure → Check IsKubectlApplySuccess(output)
         ├─ True → Continue (resource unchanged)
         └─ False → FAIL
```

---

## Example Output

```
=== Applying credentials.yaml ===
✅ Successfully applied credentials.yaml
```

---

## What credentials.yaml Contains

The file creates a Kubernetes Secret with Azure credentials:
- `AZURE_SUBSCRIPTION_ID`
- `AZURE_TENANT_ID`
- `AZURE_CLIENT_ID`
- `AZURE_CLIENT_SECRET`

These are used by CAPZ and ASO to authenticate with Azure.
