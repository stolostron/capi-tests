# Test 11: TestCheckDependencies_NamingConstraints

**Location:** `test/01_check_dependencies_test.go:434-467`

**Purpose:** Validate that cluster naming configuration is within Azure/ARO limits.

---

## Constraints Validated

| Constraint | Rule | Max Length |
|------------|------|------------|
| Domain Prefix | `${CAPZ_USER}-${DEPLOYMENT_ENV}` | 15 characters |
| ExternalAuth ID | `${CS_CLUSTER_NAME}-ea` | 15 characters |

---

## Detailed Flow

```
1. Check CI environment:
   - CI=true OR GITHUB_ACTIONS=true?
     - Yes -> SKIP
     - No  -> Continue

2. Load configuration:
   - config := NewTestConfig()

3. Validate Domain Prefix:
   - prefix = "${CAPZ_USER}-${DEPLOYMENT_ENV}"
   - len(prefix) <= 15?
     - Yes -> PASS with prefix details
     - No  -> FAIL

4. Validate ExternalAuth ID:
   - id = "${CS_CLUSTER_NAME}-ea"
   - len(id) <= 15?
     - Yes -> PASS with ID details
     - No  -> FAIL
```

---

## Why This Matters

The domain prefix is used in Azure DNS names for the cluster. Azure has a 15-character limit for this prefix. If exceeded:

1. Deployment will fail during CR reconciliation (Phase 5)
2. Error message may not clearly indicate the root cause
3. User has already waited 30+ minutes before seeing the failure

This test catches the issue in Phase 1 (seconds), not Phase 5 (45+ minutes).

---

## Environment Variables

| Variable | Default | Used In |
|----------|---------|---------|
| `CAPZ_USER` | `rcap` | Domain prefix |
| `DEPLOYMENT_ENV` | `stage` | Domain prefix |
| `CS_CLUSTER_NAME` | `${CAPZ_USER}-${DEPLOYMENT_ENV}` | ExternalAuth ID |

---

## Example Output

### Success
```
=== RUN   TestCheckDependencies_NamingConstraints
=== RUN   TestCheckDependencies_NamingConstraints/DomainPrefix
    01_check_dependencies_test.go:452: Domain prefix 'rcap-stage' (10 chars) is valid (max: 15)
=== RUN   TestCheckDependencies_NamingConstraints/ExternalAuthID
    01_check_dependencies_test.go:463: ExternalAuth ID 'rcap-stage-ea' (13 chars) is valid (max: 15)
--- PASS: TestCheckDependencies_NamingConstraints (0.00s)
```

### Failure (Domain Prefix Too Long)
```
=== RUN   TestCheckDependencies_NamingConstraints
=== RUN   TestCheckDependencies_NamingConstraints/DomainPrefix
    01_check_dependencies_test.go:449: Domain prefix validation failed:
Domain prefix 'longusername-production' is 23 characters, exceeds maximum of 15.

The domain prefix is derived from CAPZ_USER and DEPLOYMENT_ENV:
  Current: CAPZ_USER=longusername DEPLOYMENT_ENV=production

To fix, shorten these values so their combined length (with hyphen) is <= 15:
  export CAPZ_USER=<shorter-name>
  export DEPLOYMENT_ENV=<shorter-env>
--- FAIL: TestCheckDependencies_NamingConstraints (0.00s)
```

---

## Remediation

If domain prefix is too long:

```bash
# Check current values
echo "CAPZ_USER=$CAPZ_USER"
echo "DEPLOYMENT_ENV=$DEPLOYMENT_ENV"
echo "Combined: ${CAPZ_USER}-${DEPLOYMENT_ENV} ($(echo -n "${CAPZ_USER}-${DEPLOYMENT_ENV}" | wc -c) chars)"

# Fix by shortening
export CAPZ_USER="usr"
export DEPLOYMENT_ENV="stg"
```

---

## Related Configuration

See `test/helpers.go` for validation functions:
- `ValidateDomainPrefix()`
- `ValidateExternalAuthID()`
- `GetDomainPrefix()`
- `GetExternalAuthID()`
