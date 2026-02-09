# Test 13: TestCheckDependencies_OptionalTools

**Location:** `test/01_check_dependencies_test.go:44-64`

**Purpose:** Check for optional tools that enhance functionality but are not required for basic operation.

---

## Optional Tools Checked

| Tool | Description | Required For |
|------|-------------|--------------|
| `jq` | JSON processor for MCE component patching | MCE auto-enablement (`MCE_AUTO_ENABLE=true`) |

---

## Detailed Flow

```
1. For each optional tool:
   │
   └── CommandExists(tool.name)
       │
       ├── Not found → Log informational message (not an error)
       │   └── Include description and what feature requires it
       │
       └── Found → Log "Optional tool is available"
```

---

## Key Notes

- Uses table-driven test pattern with subtests for each tool
- **Never fails** - optional tools are informational only
- `jq` is needed when using external MCE clusters with auto-enablement
