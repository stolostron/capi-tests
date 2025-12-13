# Test 3: TestCheckDependencies_OpenShiftCLI_IsAvailable

**Location:** `test/01_check_dependencies_test.go:58-67`

**Purpose:** Verify OpenShift CLI (oc) is functional and get its version.

---

## Command Executed

| Command | Purpose |
|---------|---------|
| `oc version --client` | Get OpenShift CLI client version |

---

## Detailed Flow

```
1. Run: oc version --client
   │
   └─► Success?
       ├─ Yes → Log version output
       └─ No  → FAIL: "OpenShift CLI check failed"
```

---

## Example Output

```
=== RUN   TestCheckDependencies_OpenShiftCLI_IsAvailable
    01_check_dependencies_test.go:66: OpenShift CLI version:
Client Version: 4.14.0
Kustomize Version: v5.0.1
--- PASS: TestCheckDependencies_OpenShiftCLI_IsAvailable (0.05s)
```

---

## Notes

- Uses `--client` flag to avoid requiring cluster connection
- Version information is logged for debugging purposes
