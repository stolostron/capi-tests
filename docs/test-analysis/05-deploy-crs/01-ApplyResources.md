# Test 1: TestDeployment_ApplyResources

**Location:** `test/05_deploy_crs_test.go:12-71`

**Purpose:** Apply all generated YAML files to the Kind cluster in sequence.

---

## Commands Executed

| Step | Command | Purpose |
|------|---------|---------|
| 1 | `kubectl --context kind-<name> apply -f credentials.yaml` | Apply credentials |
| 2 | `kubectl --context kind-<name> apply -f aro.yaml` | Apply ARO cluster |

---

## Detailed Flow

```
1. Check prerequisite:
   └─ DirExists(outputDir)?
      └─ No → SKIP: "Output directory does not exist"

2. Build kubectl context:
   └─ context = "kind-<ManagementClusterName>"

3. Change to output directory:
   └─ os.Chdir(outputDir)

4. For each file in [credentials.yaml, aro.yaml]:
   │
   ├─► FileExists(file)?
   │   └─ No → FAIL: "Cannot apply missing file"
   │
   └─► kubectl --context <ctx> apply -f <file>
       ├─ Success → Log "Successfully applied"
       └─ Failure → Check IsKubectlApplySuccess(output)
          ├─ True (unchanged) → Continue
          └─ False → FAIL
```

---

## Files Applied

```go
expectedFiles := []string{
    "credentials.yaml",
    "aro.yaml",
}
```

---

## Example Output

```
=== Applying Kubernetes resources ===
Applying resource file: credentials.yaml...
✅ Successfully applied credentials.yaml
Applying resource file: aro.yaml...
✅ Successfully applied aro.yaml

=== Resource application complete ===
```

---

## Error Handling

The test uses `IsKubectlApplySuccess()` helper to handle cases where:
- `kubectl apply` returns non-zero exit code
- But the output indicates success (e.g., "unchanged")

This prevents false failures when resources already exist.
