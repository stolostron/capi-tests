# Test 2: TestCheckDependencies_AzureCLILogin_IsLoggedIn

**Location:** `test/01_check_dependencies_test.go:40-56`

**Purpose:** Verify Azure CLI is logged in with valid credentials.

---

## Command Executed

| Command | Purpose |
|---------|---------|
| `az account show` | Check if Azure CLI has valid login session |

---

## Detailed Flow

```
1. Check CI environment:
   └─ CI=true OR GITHUB_ACTIONS=true?
      └─ Yes → SKIP test (Azure login not available in CI)
      └─ No  → Continue

2. Run: az account show
   └─ Success → Log "Azure CLI is logged in"
   └─ Failure → FAIL: "Azure CLI not logged in. Please run 'az login'"
```

---

## Environment Variables Checked

| Variable | Value | Effect |
|----------|-------|--------|
| `CI` | `true` | Skip test |
| `GITHUB_ACTIONS` | `true` | Skip test |

---

## Example Output

### Success
```
=== RUN   TestCheckDependencies_AzureCLILogin_IsLoggedIn
    01_check_dependencies_test.go:55: Azure CLI is logged in
--- PASS: TestCheckDependencies_AzureCLILogin_IsLoggedIn (0.50s)
```

### Skipped (CI Environment)
```
=== RUN   TestCheckDependencies_AzureCLILogin_IsLoggedIn
    01_check_dependencies_test.go:44: Skipping Azure CLI login check in CI environment
--- SKIP: TestCheckDependencies_AzureCLILogin_IsLoggedIn (0.00s)
```

### Failure
```
=== RUN   TestCheckDependencies_AzureCLILogin_IsLoggedIn
    01_check_dependencies_test.go:50: Azure CLI not logged in. Please run 'az login': exit status 1
--- FAIL: TestCheckDependencies_AzureCLILogin_IsLoggedIn (0.30s)
```

---

## Security Note

The test intentionally does NOT log the output of `az account show` as it contains sensitive information:
- Tenant ID
- Subscription ID
- User information
