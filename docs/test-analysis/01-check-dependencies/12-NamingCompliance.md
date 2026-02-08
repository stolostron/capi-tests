# Test 12: TestCheckDependencies_NamingCompliance

**Location:** `test/01_check_dependencies_test.go:555-622`

**Purpose:** Validate that configuration values comply with RFC 1123 subdomain naming rules.

---

## RFC 1123 Requirements

Kubernetes resource names must follow RFC 1123 subdomain naming:

| Rule | Requirement |
|------|-------------|
| Characters | Only lowercase alphanumeric and hyphens |
| Start | Must start with alphanumeric character |
| End | Must end with alphanumeric character |
| Length | Varies by resource type |

---

## Variables Validated

| Variable | Default | Purpose |
|----------|---------|---------|
| `CAPZ_USER` | `rcap` | User identifier for domain prefix |
| `DEPLOYMENT_ENV` | `stage` | Environment identifier |
| `CS_CLUSTER_NAME` | `${CAPZ_USER}-${DEPLOYMENT_ENV}` | Cluster name prefix |
| `WORKLOAD_CLUSTER_NAMESPACE` | _(auto-generated)_ | Namespace for workload cluster resources |

---

## Detailed Flow

```
1. Load configuration:
   - config := NewTestConfig()

2. Validate CAPZ_USER:
   - ValidateRFC1123Name(config.CAPZUser, "CAPZ_USER")
   - Pass/Fail with details

3. Validate DEPLOYMENT_ENV:
   - ValidateRFC1123Name(config.Environment, "DEPLOYMENT_ENV")
   - Pass/Fail with details

4. Validate CS_CLUSTER_NAME:
   - ValidateRFC1123Name(config.ClusterNamePrefix, "CS_CLUSTER_NAME")
   - Pass/Fail with details

5. Validate WORKLOAD_CLUSTER_NAMESPACE:
   - ValidateRFC1123Name(config.WorkloadClusterNamespace, "WORKLOAD_CLUSTER_NAMESPACE")
   - Pass/Fail with details

6. Cleanup:
   - If any failures, print summary to TTY
```

---

## Why This Matters

Invalid names cause cryptic errors during deployment:

```
Error: admission webhook "validation.azuremachine.cluster.x-k8s.io" denied the request:
spec.name: Invalid value: "My-Cluster_01": a lowercase RFC 1123 subdomain must consist of
lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character
```

This test catches naming issues in Phase 1 (seconds) instead of Phase 5 (45+ minutes).

---

## Common Invalid Patterns

| Pattern | Issue | Fix |
|---------|-------|-----|
| `MyUser` | Uppercase letters | `myuser` |
| `user_name` | Underscore | `user-name` |
| `-user` | Starts with hyphen | `user` |
| `user-` | Ends with hyphen | `user` |
| `user.name` | Contains period | `user-name` |

---

## Example Output

### Success
```
=== RUN   TestCheckDependencies_NamingCompliance
=== RUN   TestCheckDependencies_NamingCompliance/CAPZ_USER
    01_check_dependencies_test.go:577: CAPZ_USER 'rcap' is RFC 1123 compliant
=== RUN   TestCheckDependencies_NamingCompliance/DEPLOYMENT_ENV
    01_check_dependencies_test.go:584: DEPLOYMENT_ENV 'stage' is RFC 1123 compliant
=== RUN   TestCheckDependencies_NamingCompliance/CS_CLUSTER_NAME
    01_check_dependencies_test.go:591: CS_CLUSTER_NAME 'rcap-stage' is RFC 1123 compliant
=== RUN   TestCheckDependencies_NamingCompliance/WORKLOAD_CLUSTER_NAMESPACE
    01_check_dependencies_test.go:607: WORKLOAD_CLUSTER_NAMESPACE 'capz-test-20260203-140812' is RFC 1123 compliant
--- PASS: TestCheckDependencies_NamingCompliance (0.00s)
```

### Failure (Uppercase)
```
=== RUN   TestCheckDependencies_NamingCompliance
=== RUN   TestCheckDependencies_NamingCompliance/CAPZ_USER
    01_check_dependencies_test.go:574: CAPZ_USER 'MyUser' contains invalid characters.

RFC 1123 subdomain naming requires:
  - Only lowercase alphanumeric characters and hyphens
  - Must start and end with an alphanumeric character

Current value: 'MyUser'
Suggested fix: 'myuser'

To fix:
  export CAPZ_USER=myuser
--- FAIL: TestCheckDependencies_NamingCompliance (0.00s)
```

---

## Remediation

```bash
# Check current values
echo "CAPZ_USER=$CAPZ_USER"
echo "DEPLOYMENT_ENV=$DEPLOYMENT_ENV"
echo "CS_CLUSTER_NAME=$CS_CLUSTER_NAME"
echo "WORKLOAD_CLUSTER_NAMESPACE=$WORKLOAD_CLUSTER_NAMESPACE"

# Fix invalid values (convert to lowercase, replace invalid chars)
export CAPZ_USER=$(echo "$CAPZ_USER" | tr '[:upper:]' '[:lower:]' | tr '_' '-')
export DEPLOYMENT_ENV=$(echo "$DEPLOYMENT_ENV" | tr '[:upper:]' '[:lower:]' | tr '_' '-')
```

---

## Related Configuration

See `test/helpers.go` for validation function:
- `ValidateRFC1123Name(name, varName)`
