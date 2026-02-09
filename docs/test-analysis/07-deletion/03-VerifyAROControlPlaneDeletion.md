# Test 3: TestDeletion_VerifyAROControlPlaneDeletion

**Location:** `test/07_deletion_test.go:139-170`

**Purpose:** Verify the AROControlPlane resource is deleted after cluster deletion.

---

## Commands Executed

| Command | Purpose |
|---------|---------|
| `kubectl --context <ctx> -n <ns> get arocontrolplane --ignore-not-found` | Check for remaining AROControlPlane resources |

---

## Detailed Flow

```
1. Check for remaining AROControlPlane resources:
   │
   └── kubectl get arocontrolplane --ignore-not-found
       │
       ├── Error → Log warning, return (non-fatal)
       │
       ├── Empty output → "No AROControlPlane resources found"
       │
       └── Non-empty → Warning: "AROControlPlane resources still exist"
```

---

## Key Notes

- Uses `--ignore-not-found` to avoid errors when the CRD itself may be absent
- This test is **informational** - it logs warnings but does not fail the test
- AROControlPlane should be deleted as part of the cascading deletion triggered by deleting the Cluster resource

---

## Example Output

```
=== RUN   TestDeletion_VerifyAROControlPlaneDeletion
    No AROControlPlane resources found (deleted successfully)
--- PASS: TestDeletion_VerifyAROControlPlaneDeletion (0.15s)
```
