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
| 3 | [03-AzureCLILogin](03-AzureCLILogin.md) | Verify Azure CLI is logged in |
| 4 | [04-AzureEnvironment](04-AzureEnvironment.md) | Validate and auto-extract Azure environment variables |
| 5 | [05-OpenShiftCLI](05-OpenShiftCLI.md) | Verify OpenShift CLI is functional |
| 6 | [06-Helm](06-Helm.md) | Verify Helm is installed |
| 7 | [07-Kind](07-Kind.md) | Verify Kind is installed |
| 8 | [08-Clusterctl](08-Clusterctl.md) | Check if clusterctl is available (informational) |
| 9 | [09-DockerCredentialHelper](09-DockerCredentialHelper.md) | Check Docker credential helpers (macOS only) |

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
│  Test 3: AzureCLILogin                                           │
│  └── Run: az account show                                        │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 4: AzureEnvironment                                        │
│  └── Check AZURE_TENANT_ID (auto-extract from az if missing)    │
│  └── Check AZURE_SUBSCRIPTION_ID/NAME (auto-extract if missing) │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 5: OpenShiftCLI                                            │
│  └── Run: oc version --client                                    │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 6: Helm                                                    │
│  └── Run: helm version --short                                   │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 7: Kind                                                    │
│  └── Run: kind version                                           │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 8: Clusterctl                                              │
│  └── Check: clusterctl in PATH                                   │
│  └── Informational only (not required for Phase 1)              │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 9: DockerCredentialHelper (macOS only)                     │
│  └── Parse ~/.docker/config.json and verify helpers exist       │
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
| `clusterctl` | Cluster API CLI (optional) | Provided by cluster-api-installer |
