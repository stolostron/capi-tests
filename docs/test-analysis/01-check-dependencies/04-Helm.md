# Test 4: TestCheckDependencies_Helm_IsAvailable

**Location:** `test/01_check_dependencies_test.go:69-78`

**Purpose:** Verify Helm is installed and functional.

---

## Command Executed

| Command | Purpose |
|---------|---------|
| `helm version --short` | Get Helm version in short format |

---

## Detailed Flow

```
1. Run: helm version --short
   │
   └─► Success?
       ├─ Yes → Log version (e.g., "v3.14.0+g...")
       └─ No  → FAIL: "Helm version check failed"
```

---

## Example Output

```
=== RUN   TestCheckDependencies_Helm_IsAvailable
    01_check_dependencies_test.go:77: Helm version: v3.14.0+gc309b6f
--- PASS: TestCheckDependencies_Helm_IsAvailable (0.03s)
```

---

## Notes

- Uses `--short` flag for concise output
- No minimum version requirement is enforced
