# Test 3: TestSetup_ScriptPermissions

**Location:** `test/02_setup_test.go:64-102`

**Purpose:** Ensure required scripts have executable permissions, and fix them if not.

---

## Scripts Checked

| Script Path | Purpose |
|-------------|---------|
| `scripts/deploy-charts-kind-capz.sh` | Kind cluster deployment |
| `doc/aro-hcp-scripts/aro-hcp-gen.sh` | YAML generation |

---

## Detailed Flow

```
1. Check prerequisite:
   └─ DirExists(config.RepoDir)?
      └─ No → SKIP: "Repository not cloned yet"

2. For each script:
   │
   ├─► FileExists(scriptPath)?
   │   └─ No → FAIL: "Script not found"
   │
   ├─► os.Stat(scriptPath) to get file mode
   │
   └─► mode & 0111 == 0? (no execute bits)
       ├─ Yes → os.Chmod(scriptPath, mode | 0111)
       │        └─ Log "making it executable"
       └─ No  → Log "has executable permissions"
```

---

## Permission Check Logic

```go
mode := info.Mode()
if mode&0111 == 0 {
    // No execute bits set - fix it
    os.Chmod(scriptPath, mode|0111)
}
```

The bitmask `0111` checks for any execute permission (user, group, or other).

---

## Example Output

### Already Executable
```
=== RUN   TestSetup_ScriptPermissions
    02_setup_test.go:99: Script scripts/deploy-charts-kind-capz.sh has executable permissions
    02_setup_test.go:99: Script doc/aro-hcp-scripts/aro-hcp-gen.sh has executable permissions
--- PASS: TestSetup_ScriptPermissions (0.01s)
```

### Fixed Permissions
```
=== RUN   TestSetup_ScriptPermissions
    02_setup_test.go:94: Script scripts/deploy-charts-kind-capz.sh is not executable, making it executable
    02_setup_test.go:99: Script doc/aro-hcp-scripts/aro-hcp-gen.sh has executable permissions
--- PASS: TestSetup_ScriptPermissions (0.01s)
```

---

## Self-Healing Behavior

This test is **self-healing**:
- If a script lacks execute permissions, it automatically adds them
- The test only fails if chmod itself fails
- Ensures scripts can be run in subsequent phases regardless of git clone behavior
