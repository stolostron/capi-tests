# Phase 1: Check Dependencies

**Make target:** `make _check-dep`
**Test file:** `test/01_check_dependencies_test.go`
**Timeout:** Default (2 minutes)

---

## Purpose

Verify all required tools are installed and properly configured before running the test suite. This phase runs quickly and doesn't require Azure resources.

---

## Test Summary

| # | Test | Purpose |
|---|------|---------|
| 1 | [01-ToolAvailable](01-ToolAvailable.md) | Check all required CLI tools are in PATH |
| 2 | [13-OptionalTools](13-OptionalTools.md) | Check optional tools (jq for MCE) |
| 3 | [14-ExternalKubeconfig](14-ExternalKubeconfig.md) | Validate external kubeconfig connectivity |
| 4 | [02-DockerDaemonRunning](02-DockerDaemonRunning.md) | Verify Docker daemon is running and accessible |
| 5 | [10-PythonVersion](10-PythonVersion.md) | Validate Python version compatibility |
| 6 | [03-AzureCLILogin](03-AzureCLILogin.md) | Verify Azure authentication (SP or CLI) |
| 7 | [04-AzureEnvironment](04-AzureEnvironment.md) | Validate and auto-extract Azure environment variables |
| 8 | [05-OpenShiftCLI](05-OpenShiftCLI.md) | Verify OpenShift CLI is functional |
| 9 | [06-Helm](06-Helm.md) | Verify Helm is installed |
| 10 | [07-Kind](07-Kind.md) | Verify Kind is installed |
| 11 | [08-Clusterctl](08-Clusterctl.md) | Check if clusterctl is available (platform-specific) |
| 12 | [11-NamingConstraints](11-NamingConstraints.md) | Validate domain prefix and ExternalAuth ID lengths |
| 13 | [09-DockerCredentialHelper](09-DockerCredentialHelper.md) | Check Docker credential helpers (macOS only) |
| 14 | [12-NamingCompliance](12-NamingCompliance.md) | Validate RFC 1123 naming compliance |
| 15 | [15-AzureRegion](15-AzureRegion.md) | Validate configured Azure region |
| 16 | [16-AzureSubscriptionAccess](16-AzureSubscriptionAccess.md) | Validate Azure subscription access |
| 17 | [17-TimeoutConfiguration](17-TimeoutConfiguration.md) | Validate timeout configurations |
| 18 | [18-ComprehensiveValidation](18-ComprehensiveValidation.md) | Comprehensive configuration validation summary |

---

## Flow Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                    make _check-dep                               │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 1: ToolAvailable                                           │
│  └── Check: docker, kind, az, oc, helm, git, kubectl, go        │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 2: OptionalTools                                           │
│  └── Check: jq (for MCE auto-enablement)                        │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 3: ExternalKubeconfig (only when USE_KUBECONFIG set)       │
│  └── Validate file, context, and cluster connectivity            │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 4: DockerDaemonRunning                                     │
│  └── Run: docker info --format {{.ServerVersion}}               │
│  └── Skip if: using podman or in CI environment                 │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 5: PythonVersion                                           │
│  └── Check: python3/python version compatibility                 │
│  └── Fail: Python 3.14.0 (az cli incompatibility)              │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 6: AzureAuthentication                                     │
│  └── Check: Service principal OR Azure CLI login                │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 7: AzureEnvironment                                        │
│  └── Check AZURE_TENANT_ID (auto-extract from az if missing)    │
│  └── Check AZURE_SUBSCRIPTION_ID/NAME (auto-extract if missing) │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Tests 8-11: Tool Version Checks                                 │
│  ├── oc version --client                                         │
│  ├── helm version --short                                        │
│  ├── kind version                                                │
│  └── clusterctl version (platform-specific behavior)             │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Tests 12-14: Naming Validations                                 │
│  ├── Domain prefix + ExternalAuth ID length constraints          │
│  ├── Docker credential helper availability (macOS)               │
│  └── RFC 1123 compliance for CAPZ_USER, DEPLOYMENT_ENV, etc.    │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Tests 15-17: Azure & Configuration Validations                  │
│  ├── Azure region validity                                       │
│  ├── Azure subscription accessibility                            │
│  └── Timeout configuration reasonableness                        │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 18: ComprehensiveValidation                                │
│  └── Run all validations and display summary table               │
│  └── Fail if any critical errors found                           │
└─────────────────────────────────────────────────────────────────┘
```

---

## Required Tools

| Tool | Purpose | Alternative |
|------|---------|-------------|
| `docker` | Container runtime | `podman` |
| `kind` | Kubernetes in Docker | - |
| `az` | Azure CLI | - |
| `oc` | OpenShift CLI | - |
| `helm` | Kubernetes package manager | - |
| `git` | Version control | - |
| `kubectl` | Kubernetes CLI | - |
| `go` | Go runtime | - |
| `python3` | Python runtime | `python` |
| `clusterctl` | Cluster API CLI (optional) | Provided by cluster-api-installer |
| `jq` | JSON processor (optional) | Required for MCE auto-enablement |

---

## Fail-Fast Validations

These tests catch configuration errors early (Phase 1) that would otherwise cause cryptic failures in later phases:

| Test | What It Prevents |
|------|------------------|
| PythonVersion | az cli incompatibility with Python 3.14.0 |
| NamingConstraints | Azure DNS name length violations |
| NamingCompliance | Kubernetes resource name validation errors |
| AzureRegion | Invalid region causing deployment failure |
| AzureSubscriptionAccess | Expired or inaccessible subscription |
| TimeoutConfiguration | Unreasonably short deployment timeouts |
| ComprehensiveValidation | Summary of all critical configuration issues |
