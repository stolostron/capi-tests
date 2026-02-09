# Test 14: TestCheckDependencies_ExternalKubeconfig

**Location:** `test/01_check_dependencies_test.go:68-98`

**Purpose:** Validate the external kubeconfig when `USE_KUBECONFIG` is set, catching connectivity issues early.

---

## Commands Executed

| Command | Purpose |
|---------|---------|
| `kubectl --context <ctx> get nodes --no-headers` | Validate cluster connectivity |

---

## Detailed Flow

```
1. Check if external cluster mode:
   └── USE_KUBECONFIG not set → Skip

2. Validate kubeconfig file exists:
   └── FileExists(config.UseKubeconfig)
       └── Not found → Fatal error

3. Extract and validate current-context:
   └── ExtractCurrentContext(kubeconfigPath)
       └── Empty → Fatal error

4. Test connectivity:
   └── kubectl --context <ctx> get nodes --no-headers
       ├── Error → Fatal: "Cannot connect to external cluster"
       └── Success → Log node count
```

---

## Key Notes

- Only runs when `USE_KUBECONFIG` is set (external cluster mode)
- Provides fail-fast validation before Phase 3 external cluster tests
- Reports the number of nodes found on the external cluster
- Uses `ExtractCurrentContext()` helper to parse kubeconfig YAML
