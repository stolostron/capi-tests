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
| 2 | [02-AzureCLILogin](02-AzureCLILogin.md) | Verify Azure CLI is logged in |
| 3 | [03-OpenShiftCLI](03-OpenShiftCLI.md) | Verify OpenShift CLI is functional |
| 4 | [04-Helm](04-Helm.md) | Verify Helm is installed |
| 5 | [05-Kind](05-Kind.md) | Verify Kind is installed |
| 6 | [06-DockerCredentialHelper](06-DockerCredentialHelper.md) | Check Docker credential helpers (macOS only) |

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
│  Test 2: AzureCLILogin                                           │
│  └── Run: az account show                                        │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 3: OpenShiftCLI                                            │
│  └── Run: oc version --client                                    │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 4: Helm                                                    │
│  └── Run: helm version --short                                   │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 5: Kind                                                    │
│  └── Run: kind version                                           │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Test 6: DockerCredentialHelper (macOS only)                     │
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
