# Test 3: TestInfrastructure_VerifyInfrastructureSecretsYAML

**Location:** `test/04_generate_yamls_test.go:130-159`

**Purpose:** Verify that `is.yaml` (infrastructure secrets) exists and contains valid YAML syntax.

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
   └─ FileExists(outputDir/is.yaml)?
      └─ No → FAIL: "is.yaml not found"

4. Validate YAML:
   └─ ValidateYAMLFile(filePath)?
      └─ Error → FAIL: "is.yaml validation failed"

5. Get file info:
   └─ os.Stat(filePath)
      └─ Log file size
```

---

## Example Output

```
=== RUN   TestInfrastructure_VerifyInfrastructureSecretsYAML
    04_generate_yamls_test.go:132: Verifying is.yaml (infrastructure secrets)
    04_generate_yamls_test.go:158: is.yaml is valid YAML (size: 5678 bytes)
--- PASS: TestInfrastructure_VerifyInfrastructureSecretsYAML (0.01s)
```

---

## What is is.yaml?

The `is.yaml` file contains infrastructure secrets needed for ARO deployment:
- Azure resource identifiers
- Network configuration references
- Service principal information

---

## Dependency

This test depends on `TestInfrastructure_GenerateResources` completing successfully.
