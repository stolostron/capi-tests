# Test 10: TestCheckDependencies_PythonVersion

**Location:** `test/01_check_dependencies_test.go:95-198`

**Purpose:** Validate Python version is supported for cluster-api-installer scripts.

---

## Command Executed

| Command | Purpose |
|---------|---------|
| `python3 --version` or `python --version` | Get installed Python version |

---

## Detailed Flow

```
1. Check platform:
   - macOS (darwin)?
     - Yes -> SKIP (see issue #330)
     - No  -> Continue

2. Find Python command:
   - python3 in PATH?
     - Yes -> Use python3
     - No  -> python in PATH?
       - Yes -> Use python
       - No  -> FAIL: Python not installed

3. Get version:
   - Run: <python-cmd> --version
   - Parse output: "Python X.Y.Z"

4. Validate version:
   - Major < 3 -> FAIL: Python 2.x not supported
   - Major == 3 AND Minor > 12 -> FAIL: Python 3.13+ not supported
   - Major == 3 AND Minor < 12 -> WARN: Python <3.12 not recommended
   - Major == 3 AND Minor == 12 -> PASS: Python 3.12.x supported
```

---

## Version Requirements

| Python Version | Status | Notes |
|----------------|--------|-------|
| 2.x | Not supported | Will fail |
| 3.0 - 3.11 | Warning | May work but not recommended |
| 3.12.x | Supported | Required for cluster-api-installer |
| 3.13+ | Not supported | Causes failures with cluster-api-installer scripts |

---

## Platform-Specific Behavior

| Platform | Behavior |
|----------|----------|
| macOS | Skipped (see issue #330) |
| Linux | Full validation |
| Windows | Full validation |

---

## Example Output

### Success (Python 3.12)
```
=== RUN   TestCheckDependencies_PythonVersion
    01_check_dependencies_test.go:135: Detected: Python 3.12.4
    01_check_dependencies_test.go:197: Python version 3.12 is supported
--- PASS: TestCheckDependencies_PythonVersion (0.05s)
```

### Skipped (macOS)
```
=== RUN   TestCheckDependencies_PythonVersion
    01_check_dependencies_test.go:105: Skipping Python version check on macOS (see issue #330)
--- SKIP: TestCheckDependencies_PythonVersion (0.00s)
```

### Failure (Python 3.13+)
```
=== RUN   TestCheckDependencies_PythonVersion
    01_check_dependencies_test.go:135: Detected: Python 3.13.0
    01_check_dependencies_test.go:177: Python 3.13 is not supported.

Detected: Python 3.13.0
Required: Python 3.12.x

Python 3.13+ causes failures with cluster-api-installer scripts.

To switch to Python 3.12:
  - Using pyenv: pyenv install 3.12 && pyenv global 3.12
  - Using alternatives (Fedora): sudo alternatives --set python3 /usr/bin/python3.12
  - Using update-alternatives (Debian/Ubuntu): sudo update-alternatives --set python3 /usr/bin/python3.12
--- FAIL: TestCheckDependencies_PythonVersion (0.05s)
```

---

## Remediation

If Python version is not supported:

### Using pyenv (recommended)
```bash
pyenv install 3.12
pyenv global 3.12
```

### Fedora
```bash
sudo dnf install python3.12
sudo alternatives --set python3 /usr/bin/python3.12
```

### Ubuntu/Debian
```bash
sudo apt install python3.12
sudo update-alternatives --set python3 /usr/bin/python3.12
```

---

## Related Issues

- Issue #330: Python version check on macOS
