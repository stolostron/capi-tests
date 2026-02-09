# Test 8: TestDeployment_00_CreateNamespace

**Location:** `test/05_deploy_crs_test.go:16-65`

**Purpose:** Create the workload cluster namespace before deploying resources.

---

## Commands Executed

| Command | Purpose |
|---------|---------|
| `kubectl --context <ctx> get namespace <ns>` | Check if namespace exists |
| `kubectl --context <ctx> create namespace <ns>` | Create namespace |
| `kubectl --context <ctx> label namespace <ns> capz-test=true ...` | Add identification labels |

---

## Detailed Flow

```
1. Check if namespace already exists:
   ├── Exists → Skip (idempotent)
   └── Not found → Create

2. Create namespace:
   └── kubectl create namespace <ns>
       ├── Success → Add labels
       └── Failure → Fatal error

3. Add labels for identification:
   └── capz-test=true
   └── capz-test-prefix=<prefix>
```

---

## Namespace Naming

The namespace is unique per test run to allow parallel test runs:

```
WORKLOAD_CLUSTER_NAMESPACE = ${WORKLOAD_CLUSTER_NAMESPACE_PREFIX}-${TIMESTAMP}
Example: capz-test-20260202-135526
```

If `WORKLOAD_CLUSTER_NAMESPACE` is explicitly set, the exact value is used (for resume scenarios).

---

## Key Notes

- **Idempotent**: Skips if namespace already exists
- Labels enable easy discovery and cleanup of test namespaces
- Must run before any resource application tests
- This test was added to support namespace isolation per test run
