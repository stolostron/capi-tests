# Test 18: TestCleanup_Summary

**Location:** `test/08_cleanup_test.go:673-769`

**Purpose:** Provide a comprehensive summary of the cleanup status across all resource types.

---

## Commands Executed

| Command | Purpose |
|---------|---------|
| `kind get clusters` | Check Kind cluster status |
| `az group show --name <rg-name>` | Check resource group status |
| `az ad app list --filter ...` | Check AD applications |

---

## Summary Output Format

```
=== Cleanup Status Summary ===

--- Local Resources ---
  Kind Cluster:     CLEAN | <name> EXISTS
  Kubeconfig:       CLEAN | N file(s) found
  Cloned Repo:      CLEAN | EXISTS at <path>
  Results Dir:      CLEAN | EXISTS
  Deploy State:     CLEAN | EXISTS

--- Azure Resources ---
  Resource Group:   CLEAN | EXISTS (<rg-name>)
  AD Apps:          CLEAN | Some exist with prefix '<prefix>'

=== Cleanup Commands ===
  make clean       - Interactive cleanup (prompts for each)
  make clean-all   - Non-interactive (delete everything)
  make clean-azure - Azure resources only
  FORCE=1 make clean - Skip all prompts
```

---

## Resources Checked

### Local Resources

| Resource | Check Method | Clean State |
|----------|-------------|-------------|
| Kind cluster | `kind get clusters` | No clusters or management cluster absent |
| Kubeconfig | `filepath.Glob(*-kubeconfig.yaml)` | No matching files |
| Cloned repo | `DirExists(repoDir)` | Directory absent |
| Results dir | `DirExists("results")` | Directory absent |
| Deploy state | `FileExists(".deployment-state.json")` | File absent |

### Azure Resources

| Resource | Check Method | Clean State |
|----------|-------------|-------------|
| Resource group | `az group show` | Group not found |
| AD apps | `az ad app list --filter` | No matching apps |

---

## Key Notes

- This test is **informational** - it always passes
- Provides a single consolidated view of all cleanup status
- Includes actionable cleanup commands at the end
- Cross-platform: uses `os.TempDir()` for kubeconfig search
