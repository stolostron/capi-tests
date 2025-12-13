# Test 1: TestSetup_CloneRepository

**Location:** `test/02_setup_test.go:9-38`

**Purpose:** Clone the cluster-api-installer repository or verify an existing clone is valid.

---

## Commands Executed

| Condition | Command | Purpose |
|-----------|---------|---------|
| Directory doesn't exist | `git clone -b <branch> <url> <dir>` | Clone repository |
| Directory exists | Check for `.git` subdirectory | Verify it's a git repo |

---

## Detailed Flow

```
1. Check if config.RepoDir exists:
   │
   ├─► Yes (directory exists):
   │   └─ Check if <RepoDir>/.git exists:
   │      ├─ Yes → Log "Using existing repository"
   │      └─ No  → FAIL: "Directory exists but is not a git repository"
   │
   └─► No (directory doesn't exist):
       └─ Run: git clone -b <branch> <url> <dir>
          ├─ Success → Log "Repository cloned successfully"
          └─ Failure → FAIL: "Failed to clone repository"
```

---

## Configuration Used

```go
config := NewTestConfig()
// Uses:
// - config.RepoDir    (default: /tmp/cluster-api-installer-aro)
// - config.RepoURL    (default: https://github.com/RadekCap/cluster-api-installer)
// - config.RepoBranch (default: ARO-ASO)
```

---

## Example Output

### Fresh Clone
```
=== RUN   TestSetup_CloneRepository
    02_setup_test.go:29: Cloning repository from https://github.com/RadekCap/cluster-api-installer (branch: ARO-ASO)
    02_setup_test.go:37: Repository cloned successfully to /tmp/cluster-api-installer-aro
--- PASS: TestSetup_CloneRepository (5.23s)
```

### Existing Repository
```
=== RUN   TestSetup_CloneRepository
    02_setup_test.go:15: Repository directory already exists at /tmp/cluster-api-installer-aro
    02_setup_test.go:24: Using existing repository
--- PASS: TestSetup_CloneRepository (0.01s)
```

---

## Idempotency

This test is **idempotent**:
- If the repository already exists and is valid, it skips cloning
- Allows re-running the test suite without re-cloning
