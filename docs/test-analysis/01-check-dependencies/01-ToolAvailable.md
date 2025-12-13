# Test 1: TestCheckDependencies_ToolAvailable

**Location:** `test/01_check_dependencies_test.go:12-38`

**Purpose:** Verify all required CLI tools are installed and available in PATH.

---

## Commands Executed

For each tool, the test uses `CommandExists()` helper which internally runs:

| Tool | Check Method | Fallback |
|------|--------------|----------|
| `docker` | `which docker` | `podman` |
| `kind` | `which kind` | - |
| `az` | `which az` | - |
| `oc` | `which oc` | - |
| `helm` | `which helm` | - |
| `git` | `which git` | - |
| `kubectl` | `which kubectl` | - |
| `go` | `which go` | - |

---

## Detailed Flow

```
For each tool in [docker, kind, az, oc, helm, git, kubectl, go]:
│
├─► Run subtest: t.Run(tool, ...)
│
├─► CommandExists(tool)?
│   │
│   ├─ Yes → Log "Tool '<tool>' is available"
│   │
│   └─ No  → Is tool == "docker"?
│            │
│            ├─ Yes → CommandExists("podman")?
│            │        │
│            │        ├─ Yes → Log "docker not found, but podman is available"
│            │        │
│            │        └─ No  → FAIL: "Required tool 'docker' is not installed"
│            │
│            └─ No  → FAIL: "Required tool '<tool>' is not installed"
```

---

## Required Tools List

```go
requiredTools := []string{
    "docker",
    "kind",
    "az",
    "oc",
    "helm",
    "git",
    "kubectl",
    "go",
}
```

---

## Example Output

```
=== RUN   TestCheckDependencies_ToolAvailable
=== RUN   TestCheckDependencies_ToolAvailable/docker
    01_check_dependencies_test.go:34: Tool 'docker' is available
=== RUN   TestCheckDependencies_ToolAvailable/kind
    01_check_dependencies_test.go:34: Tool 'kind' is available
=== RUN   TestCheckDependencies_ToolAvailable/az
    01_check_dependencies_test.go:34: Tool 'az' is available
=== RUN   TestCheckDependencies_ToolAvailable/oc
    01_check_dependencies_test.go:34: Tool 'oc' is available
=== RUN   TestCheckDependencies_ToolAvailable/helm
    01_check_dependencies_test.go:34: Tool 'helm' is available
=== RUN   TestCheckDependencies_ToolAvailable/git
    01_check_dependencies_test.go:34: Tool 'git' is available
=== RUN   TestCheckDependencies_ToolAvailable/kubectl
    01_check_dependencies_test.go:34: Tool 'kubectl' is available
=== RUN   TestCheckDependencies_ToolAvailable/go
    01_check_dependencies_test.go:34: Tool 'go' is available
--- PASS: TestCheckDependencies_ToolAvailable (0.02s)
```

---

## Key Observations

- Uses Go's `t.Run()` for subtests, allowing individual tool checks to pass/fail independently
- Docker has a special fallback to podman for container runtime flexibility
- No version requirements are checked, only presence in PATH
