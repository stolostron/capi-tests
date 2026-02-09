# Test 3: TestCleanup_VerifyClonedRepositoryRemoval

**Location:** `test/08_cleanup_test.go:111-142`

**Purpose:** Verify cloned repositories can be identified for cleanup.

---

## Detailed Flow

```
1. Determine repository path:
   ├── config.RepoDir (if set)
   └── Default: /tmp/cluster-api-installer-aro

2. Check if directory exists:
   ├── Not found → "Cloned repository not found (clean state)"
   └── Found:
       ├── Check if valid git repository (.git directory)
       └── Get current branch (git rev-parse --abbrev-ref HEAD)
```

---

## Key Notes

- Validates git repository integrity when the directory exists
- Reports the current branch for informational purposes
- Does not delete the repository - only identifies it
