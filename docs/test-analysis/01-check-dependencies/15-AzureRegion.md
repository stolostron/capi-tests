# Test 15: TestCheckDependencies_AzureRegion

**Location:** `test/01_check_dependencies_test.go:728-742`

**Purpose:** Validate that the configured Azure region is valid before deployment begins.

---

## Detailed Flow

```
1. Skip in CI environments

2. Validate region:
   └── ValidateAzureRegion(t, config.Region)
       ├── Error → "Azure region validation failed"
       └── Success → Log "Azure region is valid"
```

---

## Key Notes

- Catches invalid region configurations early (Phase 1) rather than waiting for deployment failure (Phase 5)
- Default region: `uksouth`
- Configured via `REGION` environment variable
- Skipped in CI environments where Azure may not be available
