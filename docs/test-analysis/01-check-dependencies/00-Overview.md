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
| 2 | [02-DockerDaemonRunning](02-DockerDaemonRunning.md) | Verify Docker daemon is running and accessible |
| 3 | [03-AzureCLILogin](03-AzureCLILogin.md) | Verify Azure authentication (SP or CLI) |
| 4 | [04-AzureEnvironment](04-AzureEnvironment.md) | Validate and auto-extract Azure environment variables |
| 5 | [05-OpenShiftCLI](05-OpenShiftCLI.md) | Verify OpenShift CLI is functional |
| 6 | [06-Helm](06-Helm.md) | Verify Helm is installed |
| 7 | [07-Kind](07-Kind.md) | Verify Kind is installed |
| 8 | [08-Clusterctl](08-Clusterctl.md) | Check if clusterctl is available (informational) |
| 9 | [09-DockerCredentialHelper](09-DockerCredentialHelper.md) | Check Docker credential helpers (macOS only) |
| 10 | [10-PythonVersion](10-PythonVersion.md) | Validate Python version is supported (3.12.x required) |
| 11 | [11-NamingConstraints](11-NamingConstraints.md) | Validate domain prefix and ExternalAuth ID lengths |
| 12 | [12-NamingCompliance](12-NamingCompliance.md) | Validate RFC 1123 naming compliance |

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
│  Test 2: DockerDaemonRunning                                     │
│  └── Run: docker info --format {{.ServerVersion}}               │
│  └── Skip if: using podman or in CI environment                 │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 3: PythonVersion                                           │
│  └── Check: python3/python version is 3.12.x                    │
│  └── Skip if: macOS (see issue #330)                            │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 4: AzureAuthentication                                     │
│  └── Check: Service principal OR Azure CLI login                │
│  └── Supports both auth methods                                  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 5: AzureEnvironment                                        │
│  └── Check AZURE_TENANT_ID (auto-extract from az if missing)    │
│  └── Check AZURE_SUBSCRIPTION_ID/NAME (auto-extract if missing) │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 6: OpenShiftCLI                                            │
│  └── Run: oc version --client                                    │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 7: Helm                                                    │
│  └── Run: helm version --short                                   │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 8: Kind                                                    │
│  └── Run: kind version                                           │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 9: Clusterctl                                              │
│  └── Check: clusterctl in PATH                                   │
│  └── Informational only (not required for Phase 1)              │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 10: NamingConstraints                                      │
│  └── Validate: domain prefix <= 15 chars                        │
│  └── Validate: ExternalAuth ID <= 15 chars                      │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 11: DockerCredentialHelper (macOS only)                    │
│  └── Parse ~/.docker/config.json and verify helpers exist       │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 12: NamingCompliance                                       │
│  └── Validate: CAPZ_USER, DEPLOYMENT_ENV, CS_CLUSTER_NAME       │
│  └── Validate: RFC 1123 subdomain naming compliance             │
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
| `python3` | Python runtime (3.12.x) | `python` |
| `clusterctl` | Cluster API CLI (optional) | Provided by cluster-api-installer |

---

## Fail-Fast Validations

These tests catch configuration errors early (Phase 1) that would otherwise cause cryptic failures in later phases (Phase 5):

| Test | What It Prevents |
|------|------------------|
| PythonVersion | Script failures with Python 3.13+ |
| NamingConstraints | Azure DNS name length violations |
| NamingCompliance | Kubernetes resource name validation errors |
