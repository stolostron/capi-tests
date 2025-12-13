# Test 6: TestCheckDependencies_DockerCredentialHelper

**Location:** `test/01_check_dependencies_test.go:91-175`

**Purpose:** Check that Docker credential helpers configured in `~/.docker/config.json` are available in PATH. Only runs on macOS.

---

## Commands/Checks Executed

| Step | Action | Purpose |
|------|--------|---------|
| 1 | Check `runtime.GOOS` | Skip if not macOS |
| 2 | Check `docker` exists | Skip if using podman |
| 3 | Read `~/.docker/config.json` | Parse Docker configuration |
| 4 | Check `docker-credential-<helper>` | Verify each helper binary exists |

---

## Detailed Flow

```
1. Check platform:
   └─ runtime.GOOS != "darwin"?
      └─ Yes → SKIP: "not macOS"

2. Check Docker availability:
   └─ !CommandExists("docker")?
      ├─ CommandExists("podman") → SKIP: "Using podman"
      └─ else → SKIP: "Docker not installed"

3. Determine config path:
   └─ $DOCKER_CONFIG set?
      ├─ Yes → Use $DOCKER_CONFIG/config.json
      └─ No  → Use $HOME/.docker/config.json

4. Read and parse config.json:
   └─ File not found or parse error?
      └─ Yes → Log and return (OK)

5. Check credsStore:
   └─ config.CredsStore set?
      └─ Yes → t.Run("credsStore", ...)
               └─ CommandExists("docker-credential-<credsStore>")?
                  ├─ Yes → Log "available"
                  └─ No  → FAIL with fix instructions

6. Check credHelpers (per registry):
   └─ For each registry in config.CredHelpers:
      └─ t.Run(registry, ...)
         └─ CommandExists("docker-credential-<helper>")?
            ├─ Yes → Log "available"
            └─ No  → FAIL with fix instructions
```

---

## Docker Config Structure

```json
{
  "credsStore": "desktop",
  "credHelpers": {
    "gcr.io": "gcloud",
    "*.azurecr.io": "acr-env"
  }
}
```

---

## Example Output

### Success
```
=== RUN   TestCheckDependencies_DockerCredentialHelper
=== RUN   TestCheckDependencies_DockerCredentialHelper/credsStore
    01_check_dependencies_test.go:154: Docker credential helper 'docker-credential-desktop' is available
--- PASS: TestCheckDependencies_DockerCredentialHelper (0.01s)
```

### Failure
```
=== RUN   TestCheckDependencies_DockerCredentialHelper
=== RUN   TestCheckDependencies_DockerCredentialHelper/credsStore
    01_check_dependencies_test.go:146: Docker is configured to use credential helper 'desktop' but it's not in PATH
    This will cause 'docker pull' commands to fail with:
      error getting credentials - err: exec: "docker-credential-desktop": executable file not found in $PATH

    To fix this issue, run:
      make fix-docker-config
--- FAIL: TestCheckDependencies_DockerCredentialHelper (0.01s)
```

### Skipped (Linux)
```
=== RUN   TestCheckDependencies_DockerCredentialHelper
    01_check_dependencies_test.go:97: Skipping Docker credential helper check (not macOS)
--- SKIP: TestCheckDependencies_DockerCredentialHelper (0.00s)
```

---

## Why macOS Only?

This is a common issue on macOS when:
1. Docker Desktop was previously installed (sets `credsStore: desktop`)
2. User switches to alternative tools (Colima, Rancher Desktop, etc.)
3. The `docker-credential-desktop` helper is no longer available
4. Docker commands fail with cryptic credential errors

The fix is provided via `make fix-docker-config` which removes the credential helper configuration.
