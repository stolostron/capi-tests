# Test 2: TestCleanup_VerifyKubeconfigRemoval

**Location:** `test/08_cleanup_test.go:82-108`

**Purpose:** Verify kubeconfig files can be identified for cleanup.

---

## Detailed Flow

```
1. Search for kubeconfig files:
   └── filepath.Glob(os.TempDir() + "/*-kubeconfig.yaml")

2. Results:
   ├── No matches → "No kubeconfig files found (clean state)"
   └── Matches found → List all kubeconfig files
```

---

## Key Notes

- Uses cross-platform `os.TempDir()` for the search path
- Matches pattern `*-kubeconfig.yaml` (e.g., `capz-tests-cluster-kubeconfig.yaml`)
- Does not delete files - only identifies them for cleanup
