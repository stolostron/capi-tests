# Test 5: TestCheckDependencies_Kind_IsAvailable

**Location:** `test/01_check_dependencies_test.go:80-89`

**Purpose:** Verify Kind (Kubernetes in Docker) is installed.

---

## Command Executed

| Command | Purpose |
|---------|---------|
| `kind version` | Get Kind version |

---

## Detailed Flow

```
1. Run: kind version
   │
   └─► Success?
       ├─ Yes → Log version
       └─ No  → FAIL: "Kind version check failed"
```

---

## Example Output

```
=== RUN   TestCheckDependencies_Kind_IsAvailable
    01_check_dependencies_test.go:88: Kind version: kind v0.22.0 go1.21.7 darwin/arm64
--- PASS: TestCheckDependencies_Kind_IsAvailable (0.02s)
```

---

## Notes

- Kind is essential for creating the management cluster
- Version output includes Go version and platform information
