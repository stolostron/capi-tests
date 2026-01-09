# Test 6: TestVerification_TestedVersionsSummary

**Location:** `test/06_verification_test.go:229-267`

**Purpose:** Display a summary of all tested infrastructure component versions.

---

## Components Checked

| Component | Namespace | Deployment |
|-----------|-----------|------------|
| CAPI Controller | `capi-system` | `capi-controller-manager` |
| CAPZ Controller | `capz-system` | `capz-controller-manager` |
| ASO Controller | `capz-system` | `azureserviceoperator-controller-manager` |

---

## Detailed Flow

```
1. Get management cluster context:
   - context = "kind-${MANAGEMENT_CLUSTER_NAME}"

2. For each component:
   - Query deployment for container image
   - Extract version from image tag
   - Store version info

3. Format version summary:
   - Table with component name, version, image

4. Display summary:
   - Print to TTY
   - Log to test output

5. Count results:
   - Track found vs not-found components
   - Log summary count
```

---

## Information Collected

| Field | Source | Example |
|-------|--------|---------|
| Name | Hardcoded | `CAPI Controller` |
| Version | Image tag | `v1.6.0` |
| Image | Container spec | `registry.k8s.io/cluster-api/cluster-api-controller:v1.6.0` |

---

## Example Output

### Success
```
=== RUN   TestVerification_TestedVersionsSummary

===================================================
            TESTED COMPONENT VERSIONS
===================================================

Management Cluster: kind-capz-tests-stage
OpenShift Version:  4.21

---------------------------------------------------
 Component          | Version   | Image
---------------------------------------------------
 CAPI Controller    | v1.6.0    | registry.k8s.io/cluster-api/cluster-api-controller:v1.6.0
 CAPZ Controller    | v1.13.0   | mcr.microsoft.com/oss/capi/capz-controller:v1.13.0
 ASO Controller     | v2.5.0    | mcr.microsoft.com/k8s/azureserviceoperator:v2.5.0
---------------------------------------------------

===================================================

    06_verification_test.go:263: Successfully retrieved version information for 3/3 components
--- PASS: TestVerification_TestedVersionsSummary (0.50s)
```

### Partial Success
```
=== RUN   TestVerification_TestedVersionsSummary

===================================================
            TESTED COMPONENT VERSIONS
===================================================

Management Cluster: kind-capz-tests-stage
OpenShift Version:  4.21

---------------------------------------------------
 Component          | Version   | Image
---------------------------------------------------
 CAPI Controller    | v1.6.0    | registry.k8s.io/cluster-api/cluster-api-controller:v1.6.0
 CAPZ Controller    | not found | -
 ASO Controller     | not found | -
---------------------------------------------------

===================================================

    06_verification_test.go:260: Warning: No component versions could be retrieved. Management cluster may not be running.
--- PASS: TestVerification_TestedVersionsSummary (0.50s)
```

---

## Why This Matters

This test provides valuable information for:

1. **Debugging** - Know exact versions when reporting issues
2. **Reproducibility** - Record versions used for successful deployments
3. **Compatibility** - Verify component version compatibility
4. **Documentation** - Include in test reports and logs

---

## Related Helpers

See `test/helpers.go` for:
- `GetComponentVersions()` - Fetches version info from cluster
- `FormatComponentVersions()` - Formats version table for display

---

## Notes

- This test does not fail even if versions cannot be retrieved
- Designed as an informational/summary test
- Runs at the end of the verification phase
