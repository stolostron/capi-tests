# Test 2: TestInfrastructure_VerifyCredentialsYAML

**Location:** `test/04_generate_yamls_test.go:99-128`

**Purpose:** Verify that `credentials.yaml` exists and contains valid YAML syntax.

---

## Checks Performed

| Check | Method |
|-------|--------|
| File exists | `FileExists(filePath)` |
| Valid YAML | `ValidateYAMLFile(filePath)` |
| File stats | `os.Stat(filePath)` |

---

## Detailed Flow

```
1. Build output directory path:
   └─ outputDir = <RepoDir>/<env>-<user>-<cluster>

2. Check prerequisite:
   └─ DirExists(outputDir)?
      └─ No → SKIP: "Output directory does not exist"

3. Check file exists:
   └─ FileExists(outputDir/credentials.yaml)?
      └─ No → FAIL: "credentials.yaml not found"

4. Validate YAML:
   └─ ValidateYAMLFile(filePath)?
      └─ Error → FAIL: "credentials.yaml validation failed"

5. Get file info:
   └─ os.Stat(filePath)
      └─ Log file size
```

---

## Example Output

```
=== RUN   TestInfrastructure_VerifyCredentialsYAML
    04_generate_yamls_test.go:101: Verifying credentials.yaml
    04_generate_yamls_test.go:127: credentials.yaml is valid YAML (size: 1234 bytes)
--- PASS: TestInfrastructure_VerifyCredentialsYAML (0.01s)
```

---

## File Contents (Example Structure)

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: azure-credentials
  namespace: default
type: Opaque
data:
  AZURE_SUBSCRIPTION_ID: <base64>
  AZURE_TENANT_ID: <base64>
  AZURE_CLIENT_ID: <base64>
  AZURE_CLIENT_SECRET: <base64>
```

---

## Dependency

This test depends on `TestInfrastructure_GenerateResources` completing successfully.
