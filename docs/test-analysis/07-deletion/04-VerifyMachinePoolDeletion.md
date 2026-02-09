# Test 4: TestDeletion_VerifyMachinePoolDeletion

**Location:** `test/07_deletion_test.go:173-204`

**Purpose:** Verify machine pool resources are deleted after cluster deletion.

---

## Commands Executed

| Command | Purpose |
|---------|---------|
| `kubectl --context <ctx> -n <ns> get machinepool --ignore-not-found` | Check for remaining MachinePool resources |

---

## Detailed Flow

```
1. Check for remaining MachinePool resources:
   │
   └── kubectl get machinepool --ignore-not-found
       │
       ├── Error → Log warning, return (non-fatal)
       │
       ├── Empty output → "No MachinePool resources found"
       │
       └── Non-empty → Warning: "MachinePool resources still exist"
```

---

## Key Notes

- Uses `--ignore-not-found` to avoid errors when the CRD itself may be absent
- This test is **informational** - it logs warnings but does not fail the test
- MachinePool resources represent Azure worker nodes and should be deleted as part of the cascading deletion

---

## Example Output

```
=== RUN   TestDeletion_VerifyMachinePoolDeletion
    No MachinePool resources found (deleted successfully)
--- PASS: TestDeletion_VerifyMachinePoolDeletion (0.14s)
```
