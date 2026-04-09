# Security Review

This document summarizes the security reviews conducted for the CAPI test suite.

## Review History

| Version | Date | Issue | Status |
|---------|------|-------|--------|
| V1 | 2026-01-23 | #393 | PASSED |
| V1.1 | 2026-02-08 | ACM-29882 | PASSED |

---

## V1.1 Security Review

This section covers the security review for v1.1 changes (ACM-29882), which introduced external cluster mode, MCE component management, and deployment state persistence.

### New Attack Surface in V1.1

| Area | New Code | Risk |
|------|----------|------|
| External kubeconfig handling | `ExtractCurrentContext()` | File path from env var passed to kubectl |
| `os.Setenv` side effect | `NewTestConfig()` | Global process environment mutation |
| MCE patching via jq | `SetMCEComponentState()`, `EnableMCEComponent()` | Component names interpolated into jq expressions |
| Deployment state file | `getWorkloadClusterNamespace()` | JSON file read from disk during config init |
| MCE component status query | `GetMCEComponentStatus()` | Component names interpolated into jsonpath |

### V1.1 Detailed Findings

#### 1. ExtractCurrentContext() - Command Injection via Kubeconfig Path

**Status**: PASSED

**Location**: `test/helpers.go:293`

**Analysis**: The `kubeconfigPath` parameter originates from the `USE_KUBECONFIG` environment variable (set by the test operator). It is passed to `exec.Command()` as a separate argument:

```go
func ExtractCurrentContext(kubeconfigPath string) string {
    output, err := exec.Command("kubectl", "config", "current-context",
        "--kubeconfig", kubeconfigPath).Output()
```

**Why this is safe**: `exec.Command()` passes each argument directly to the process without shell interpretation. Even if `kubeconfigPath` contains spaces, semicolons, backticks, or other shell metacharacters, they are treated as literal characters in the file path argument. No shell is invoked.

**Callers**:
- `config.go:325` - `GetKubeContext()` passes `c.UseKubeconfig` (from env var)
- `01_check_dependencies_test.go:83` - Same origin

**Risk assessment**: Low. The path comes from an environment variable set by the test operator (who already has full shell access). There is no privilege escalation vector.

#### 2. os.Setenv Side Effect in NewTestConfig()

**Status**: PASSED (acceptable trade-off)

**Location**: `test/config.go:164`

**Analysis**: When `USE_KUBECONFIG` is set and `USE_K8S` is not, `NewTestConfig()` sets `USE_K8S=true` to default controller namespaces to `multicluster-engine`:

```go
if useKubeconfig != "" && os.Getenv("USE_K8S") == "" {
    _ = os.Setenv("USE_K8S", "true")
}
```

**Security implications**:
- Mutates global process state, which could affect other code reading `USE_K8S`
- The `#nosec G104` suppression for the ignored error return is justified: `os.Setenv` with a fixed key and value cannot fail in any realistic scenario (it would only fail if the OS kernel rejected the syscall)
- The side effect is documented in CLAUDE.md and the code comment

**Risk assessment**: Low. This is a test framework configuration convenience, not a security boundary. The mutation is idempotent and deterministic.

#### 3. SetMCEComponentState() / EnableMCEComponent() - jq Command Injection

**Status**: PASSED

**Location**: `test/helpers.go:2790-2796` and `test/helpers.go:2839-2845`

**Analysis**: Both functions interpolate `componentName` into a jq expression using `fmt.Sprintf`:

```go
// SetMCEComponentState (line 2790)
jqExpr := fmt.Sprintf(
    `.spec.overrides.components | map(if .name == "%s" then .enabled = %t else . end)`,
    componentName, enabled)
jqCmd := exec.Command("jq", "-c", jqExpr)

// EnableMCEComponent (line 2839)
jqExpr := fmt.Sprintf(
    `.spec.overrides.components | map(if .name == "%s" then .enabled = true else . end)`,
    componentName)
jqCmd := exec.Command("jq", "-c", jqExpr)
```

**Why this is safe**:

1. **No shell invocation**: `exec.Command("jq", "-c", jqExpr)` passes the jq expression as a single argument directly to the jq process. There is no shell to interpret metacharacters.

2. **Controlled input**: All callers pass compile-time constants defined in `config.go`:
   - `MCEComponentCAPI = "cluster-api"` (line 35)
   - `MCEComponentCAPZ = "cluster-api-provider-azure-preview"` (line 36)
   - `ExpectedMCEComponents` in `03_cluster_test.go:50-68` uses hardcoded string literals

3. **jq expression context**: Even if a hypothetical attacker could control `componentName`, injecting into a jq string comparison (`.name == "..."`) would require unescaped double quotes to break out. The component names contain only lowercase alphanumeric characters and hyphens, which cannot break jq string syntax.

**Risk assessment**: Low. The `#nosec G204` annotations are justified. The component names are compile-time constants, and the jq binary is invoked without a shell.

#### 4. GetMCEComponentStatus() - jsonpath Injection

**Status**: PASSED

**Location**: `test/helpers.go:2749-2751`

**Analysis**: `componentName` is interpolated into a kubectl jsonpath expression:

```go
fmt.Sprintf("jsonpath={.spec.overrides.components[?(@.name=='%s')].enabled}", componentName)
```

**Why this is safe**: Same reasoning as finding #3 — all callers pass compile-time string constants. The jsonpath expression is passed as a single argument to `exec.Command` (no shell). The component names (`cluster-api`, `cluster-api-provider-azure-preview`) contain only alphanumeric characters and hyphens, which cannot break jsonpath string syntax.

**Risk assessment**: Low.

#### 5. Deployment State File Read in getWorkloadClusterNamespace()

**Status**: PASSED

**Location**: `test/config.go:88-99`

**Analysis**: The function reads `.deployment-state.json` from the repository directory:

```go
repoDir := getDefaultRepoDir()
stateFilePath := filepath.Join(repoDir, ".deployment-state.json")
// #nosec G304 - path constructed from repo directory and fixed filename
if data, err := os.ReadFile(stateFilePath); err == nil {
    var state struct {
        WorkloadClusterNamespace string `json:"workload_cluster_namespace"`
    }
    if err := json.Unmarshal(data, &state); err == nil && state.WorkloadClusterNamespace != "" {
        workloadClusterNamespace = state.WorkloadClusterNamespace
        return
    }
}
```

**Path traversal analysis**:
- `repoDir` comes from `ARO_REPO_DIR` env var or defaults to `os.TempDir() + "/cluster-api-installer-aro"`
- The filename `.deployment-state.json` is a hardcoded constant
- `filepath.Join()` normalizes the path (collapses `..` segments), but the base directory itself could be user-controlled via `ARO_REPO_DIR`
- However, this is by design — the test operator intentionally points `ARO_REPO_DIR` at their working directory

**Data integrity**: The file content is parsed as JSON and only the `workload_cluster_namespace` string field is extracted. A malformed JSON file results in a silent fallback to timestamp-based namespace generation. A malicious namespace value would fail RFC 1123 validation in the check dependencies phase.

**Risk assessment**: Low. The `#nosec G304` annotation is justified. The file path is constructed from operator-controlled configuration and a fixed filename.

#### 6. ReadDeploymentState() / WriteDeploymentState()

**Status**: PASSED

**Location**: `test/helpers.go:1571-1595`

**Analysis**: These functions read/write `.deployment-state.json` in the current working directory using the constant `DeploymentStateFile = ".deployment-state.json"`.

- `WriteDeploymentState` uses `0600` permissions (owner-only read/write)
- `ReadDeploymentState` parses JSON into a typed struct, preventing arbitrary data injection
- The fixed filename prevents path traversal

**Risk assessment**: Low.

### V1.1 #nosec Annotation Audit

All 7 current `#nosec` annotations were reviewed and verified as justified:

| # | Location | Rule | Justification | Verdict |
|---|----------|------|---------------|---------|
| 1 | `config.go:91` | G304 | Path from repo dir + fixed filename `.deployment-state.json` | Justified |
| 2 | `config.go:164` | G104 | `os.Setenv("USE_K8S", "true")` cannot fail in practice | Justified |
| 3 | `helpers.go:414` | G304 | `ExtractClusterNameFromYAML` - path from test config | Justified |
| 4 | `helpers.go:1494` | G304 | `ValidateYAMLFile` - path validated via `os.Stat`, from test config | Justified |
| 5 | `helpers.go:1517` | G304 | `ExtractNamespaceFromYAML` - path from test config | Justified |
| 6 | `helpers.go:2795` | G204 | `SetMCEComponentState` - jq with compile-time constant component names | Justified |
| 7 | `helpers.go:2844` | G204 | `EnableMCEComponent` - jq with compile-time constant component names | Justified |

**Change from V1**: Annotations #1 and #2 are new in v1.1 (config.go). Annotation #5 (`ExtractNamespaceFromYAML`) is also new. The total increased from 5 to 7, all properly documented with inline justifications.

### V1.1 Security Scan Results

```
gosec ./...  : 0 issues (7 #nosec annotations)
govulncheck  : 0 code vulnerabilities
               1 module-level vulnerability (GO-2026-4337 in stdlib, not called by code)
```

### V1.1 Summary

| Area | Status | Notes |
|------|--------|-------|
| ExtractCurrentContext (kubeconfig path) | PASSED | exec.Command prevents shell injection; env var from operator |
| os.Setenv side effect | PASSED | Fixed key/value, idempotent, documented |
| MCE jq command construction | PASSED | Compile-time constant component names, no shell invocation |
| MCE jsonpath query | PASSED | Same constants, no shell invocation |
| Deployment state file read | PASSED | Fixed filename, JSON-parsed into typed struct, RFC 1123 validated downstream |
| #nosec annotations (7 total) | PASSED | All reviewed and justified |

---

## V1 Security Review

This section contains the original V1 security review conducted as part of the V1 final review (issue #393).

### Review Date

2026-01-23

### Overall Status: PASSED

| Area | Status | Notes |
|------|--------|-------|
| Command Injection | PASS | No vulnerabilities found |
| Secrets Management | PASS | Proper .gitignore, no secrets in logs |
| Azure Security | PASS | Credentials masked, least-privilege documented |
| Dependencies | PASS | No CVEs found, minimal dependencies |
| File Operations | PASS | Path traversal mitigated, proper file permissions |
| CI/CD Security | PASS | Actions pinned to SHA, secrets properly handled |

### V1 Detailed Findings

#### 1. Command Injection

**Status**: PASSED

**Previously Known Issue**: The issue checklist mentioned a command injection vulnerability at `06_verification_test.go:68` related to base64 decoding. This has been **fixed**.

**Current Implementation**: The code now uses Go's native `base64.StdEncoding.DecodeString()` (line 90) instead of shelling out to decode base64 data:

```go
// Decode base64 using Go's encoding/base64 package (safe from command injection)
decoded, err := base64.StdEncoding.DecodeString(output)
```

**RunCommand() Analysis**: The helper function uses `exec.Command(name, args...)` which passes arguments directly to the process without shell interpretation, making it safe from command injection.

**Script Security**: The `cleanup-azure-resources.sh` script validates input prefixes to prevent OData filter injection:
```bash
if [[ ! "$PREFIX" =~ ^[a-z0-9][a-z0-9-]*$ ]]; then
    print_error "Invalid prefix '${PREFIX}': must be lowercase alphanumeric..."
    exit 1
fi
```

#### 2. Secrets Management

**Status**: PASSED

**Findings**:
- `.gitignore` properly excludes sensitive files (credentials.json, kubeconfig files, .env files)
- Enhanced `.gitignore` to cover additional patterns (*.pem, *.key, service-principal.json)
- Azure subscription/tenant IDs are masked in logs (showing only first 8 and last 4 chars)
- No secrets hardcoded in source code
- Environment variables used for all sensitive configuration

**Log Masking Example** (from `01_check_dependencies_test.go:290`):
```go
t.Logf("AZURE_TENANT_ID auto-extracted from Azure CLI: %s...%s", tenantID[:8], tenantID[len(tenantID)-4:])
```

#### 3. Azure Security

**Status**: PASSED

**Findings**:
- Service principal credentials are properly handled via environment variables
- Documentation recommends least-privilege (Contributor role scoped to subscription)
- Resource cleanup scripts properly handle orphaned resources
- Azure CLI authentication is validated before operations

#### 4. Dependencies

**Status**: PASSED

**go.mod Analysis**:
- Minimal dependency footprint (only `gopkg.in/yaml.v3`)
- Version pinned
- Go version: 1.24

**Security Scans** (at time of V1 review):
- `gosec ./...`: 0 issues (5 #nosec annotations with justifications)
- `govulncheck ./...`: No vulnerabilities found

#### 5. File Operations

**Status**: PASSED

**Findings**:
- File read operations use paths from test configuration, not user input
- G304 gosec findings properly suppressed with justification
- Kubeconfig files created with secure permissions (0600):
```go
if err := os.WriteFile(kubeconfigPath, decoded, 0600); err != nil {
```
- Temporary files handled appropriately

#### 6. CI/CD Security

**Status**: PASSED

**GitHub Actions Security**:
- All third-party actions pinned to SHA (not version tags):
  - `actions/checkout@8e8c483db84b4bee98b60c0593521ed34d9990e8`
  - `actions/setup-go@7a3fe6cf4cb3a834922a1244abfce67bcef6a0c5`
  - `actions/upload-artifact@b7c566a772e6b6bfb58ed0dc250532a479d7789f`
  - `golangci/golangci-lint-action@55c2c1448f86e01eaae002a5a3a9624417608d84`
  - And more...

**Permissions**:
- Minimal permissions declared (`contents: read`, `issues: write` where needed)
- No elevated permissions like `write-all`

**Secrets Handling**:
- `GITHUB_TOKEN` used only for issue creation, not exposed
- No custom secrets in workflow files

**Security Workflows**:
| Workflow | Schedule | Purpose |
|----------|----------|---------|
| security-gosec.yml | Daily 2:00 AM UTC | Static code analysis |
| security-govulncheck.yml | Daily 2:30 AM UTC | Go vulnerability database |
| security-nancy.yml | Daily 3:30 AM UTC | Sonatype OSS Index |
| security-trivy.yml | Daily 3:00 AM UTC | Comprehensive scanning |

### V1 Changes Made During Review

1. **Updated CLAUDE.md** - Removed outdated "Known Issue" about command injection in `06_verification_test.go:68` (already fixed)
2. **Enhanced .gitignore** - Added sensitive file patterns: `credentials.yaml`, `*.pem`, `*.key`, `*.crt`, `*.p12`, `*.pfx`, `kubeconfig`, `*-kubeconfig.yaml`, `.env`, `.env.*` (with `!.env.example` exception), `azure.json`, `service-principal.json`
3. **Created this documentation** - `docs/SECURITY_REVIEW.md`

---

## Recommendations

### Already Implemented
1. Automated security scanning (4 scanners with daily schedules)
2. GitHub Actions pinned to SHA
3. Minimal permissions in CI/CD
4. Comprehensive .gitignore
5. RFC 1123 validation for naming
6. `exec.Command` usage throughout (no shell invocation)
7. Compile-time constants for MCE component names
8. `0600` permissions for sensitive files (kubeconfig, deployment state)

### Future Considerations
1. Consider enabling GitHub Dependabot for automated dependency updates
2. Consider enabling GitHub secret scanning if not already active
3. Periodically review #nosec annotations to ensure they remain valid (currently 7)
4. Update Go toolchain to pick up stdlib fix for GO-2026-4337 when available

## Verification Commands

```bash
# Run gosec locally
go install github.com/securego/gosec/v2/cmd/gosec@v2.25.0
gosec ./...

# Run govulncheck locally
go install golang.org/x/vuln/cmd/govulncheck@v1.1.4
govulncheck ./...

# Check for secrets (requires git-secrets)
git secrets --scan

# Run all tests including check dependencies
make test
```
