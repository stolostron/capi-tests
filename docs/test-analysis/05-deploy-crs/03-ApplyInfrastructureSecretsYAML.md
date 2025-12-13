# Test 3: TestDeployment_ApplyInfrastructureSecretsYAML

**Location:** `test/05_deploy_crs_test.go:108-141`

**Purpose:** Apply `is.yaml` (infrastructure secrets) to the Kind cluster.

---

## Command Executed

| Command | Purpose |
|---------|---------|
| `kubectl --context kind-<name> apply -f <path>/is.yaml` | Apply infrastructure secrets |

---

## Detailed Flow

```
1. Check prerequisite:
   └─ DirExists(outputDir)?
      └─ No → SKIP: "Output directory does not exist"

2. Check file exists:
   └─ FileExists(filePath)?
      └─ No → FAIL: "is.yaml not found"

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
=== Applying is.yaml (infrastructure secrets) ===
✅ Successfully applied is.yaml
```

---

## What is.yaml Contains

Infrastructure secrets may include:
- Pull secrets for container registries
- SSH keys for node access
- Network configuration secrets
- Service principal secrets for specific Azure operations
