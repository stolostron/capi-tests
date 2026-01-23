# Cross-Platform Compatibility Guide

This document provides a comprehensive cross-platform compatibility review of the ARO-CAPZ test suite, addressing issue #402.

## Supported Platforms

| Operating System | Architecture | Shell | Status |
|------------------|--------------|-------|--------|
| Linux (Ubuntu/Debian) | x86_64 | bash | Fully Supported |
| Linux (RHEL/Fedora) | x86_64 | bash | Fully Supported |
| macOS 13+ | Intel (x86_64) | zsh/bash | Supported |
| macOS 14+ | Apple Silicon (arm64) | zsh/bash | Supported |
| Windows (WSL2) | x86_64 | bash | Supported (via WSL2) |

**Note:** Native Windows (without WSL2) is not supported due to Unix-specific dependencies.

## Shell Compatibility

### Scripts Overview

| Script | Shebang | Shell Required |
|--------|---------|----------------|
| `scripts/cleanup-azure-resources.sh` | `#!/usr/bin/env bash` | Bash 4.0+ |
| `scripts/generate-summary.sh` | `#!/usr/bin/env bash` | Bash 4.0+ |

### Bash Features Used

The shell scripts use the following Bash-specific features:

- **Double brackets `[[ ]]`**: Used for string comparisons and regex matching
- **Regex matching `=~`**: For input validation
- **Arrays**: For storing lists of items
- **Process substitution `< <(...)`**: For reading command output in loops
- **Brace expansion `{1..N}`**: For generating sequences
- **Local variables**: `local` keyword for function-scoped variables
- **`read -p`**: Interactive prompts with `-n 1 -r` flags

### macOS Compatibility Notes

macOS ships with Bash 3.2 (due to GPLv3 licensing). Both shell scripts require Bash 4.0+ due to process substitution patterns (`< <(...)`). Users must install newer Bash via Homebrew for compatibility:

```bash
# Check your current Bash version
bash --version
# macOS default is typically 3.2.x - you need 4.0+ for the shell scripts

# Install newer Bash on macOS
brew install bash
# Add to /etc/shells if needed (path differs: /opt/homebrew/bin/bash on Apple Silicon, /usr/local/bin/bash on Intel)
echo '/opt/homebrew/bin/bash' | sudo tee -a /etc/shells
```

### Zsh Compatibility

While macOS defaults to zsh, the scripts explicitly require bash via their shebang lines. Users running commands interactively can use zsh without issues.

## Path Handling

### Temporary Directory

The test suite uses `os.TempDir()` in Go code, which returns:

| Platform | Default Path |
|----------|--------------|
| Linux | `/tmp` |
| macOS | `/var/folders/...` (user-specific) |
| Windows (WSL2) | `/tmp` |

**Key files:**

- `test/config.go`: The `getDefaultRepoDir()` function uses `os.TempDir()` for the default repository directory
- `test/helpers.go`: The `GetResultsDir()` function falls back to `os.TempDir()` when a results directory cannot be created
- `test/06_verification_test.go`: The `getKubeconfigPath()` function uses `filepath.Join(os.TempDir(), ...)` for kubeconfig paths

### Hardcoded Paths

The following paths are documented in examples but properly use environment variables or `os.TempDir()` in code:

| Documentation Reference | Actual Code Behavior |
|------------------------|---------------------|
| `/tmp/cluster-api-installer-aro` | Uses `os.TempDir()` + suffix |
| `/tmp/*-kubeconfig.yaml` | Uses `filepath.Join(os.TempDir(), ...)` |

### TTY Handling

The `openTTY()` function in `test/helpers.go` handles cross-platform differences:

```go
func openTTY() (*os.File, bool) {
    tty, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
    if err != nil {
        // Fallback to stderr if /dev/tty unavailable (Windows, CI, etc.)
        return os.Stderr, false
    }
    return tty, true
}
```

- **Linux/macOS**: Uses `/dev/tty` for unbuffered output
- **Windows/CI**: Falls back to `os.Stderr`

## Tool Availability

### Required Tools

| Tool | Purpose | Linux | macOS | Windows (WSL2) |
|------|---------|-------|-------|----------------|
| `docker` or `podman` | Container runtime | Package manager | Docker Desktop / `brew install docker` | Docker Desktop |
| `kind` | Local Kubernetes | `go install sigs.k8s.io/kind@latest` | Same | Same |
| `kubectl` | Kubernetes CLI | Package manager | `brew install kubernetes-cli` | Package manager |
| `az` | Azure CLI | [Install guide](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli-linux) | `brew install azure-cli` | [Install guide](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli-linux) |
| `oc` | OpenShift CLI | [Download](https://mirror.openshift.com/pub/openshift-v4/clients/ocp/latest/) | Same | Same |
| `helm` | Kubernetes package manager | Package manager | `brew install helm` | Package manager |
| `git` | Version control | Package manager | Xcode CLT or `brew install git` | Package manager |
| `go` | Go runtime | [golang.org](https://golang.org/dl/) | `brew install go` | [golang.org](https://golang.org/dl/) |
| `jq` | JSON processor | Package manager | `brew install jq` | Package manager |
| `xmllint` | XML parser | `libxml2-utils` (Debian) or `libxml2` (RHEL) | Pre-installed | `libxml2-utils` |
| `bc` | Calculator | Pre-installed | Pre-installed | Pre-installed |

### Platform-Specific Installation

#### Ubuntu/Debian

```bash
# Core tools
sudo apt-get update
sudo apt-get install -y docker.io git curl jq libxml2-utils bc

# Kind
go install sigs.k8s.io/kind@latest

# Azure CLI
curl -sL https://aka.ms/InstallAzureCLIDeb | sudo bash

# OpenShift CLI
wget -q https://mirror.openshift.com/pub/openshift-v4/clients/ocp/latest/openshift-client-linux.tar.gz
tar -xzf openshift-client-linux.tar.gz
sudo mv oc kubectl /usr/local/bin/

# Helm
curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
```

#### RHEL/Fedora

```bash
# Core tools
sudo dnf install -y podman git curl jq libxml2 bc

# Kind
go install sigs.k8s.io/kind@latest

# Azure CLI
sudo rpm --import https://packages.microsoft.com/keys/microsoft.asc
sudo dnf install -y https://packages.microsoft.com/config/rhel/9/packages-microsoft-prod.rpm
sudo dnf install -y azure-cli

# OpenShift CLI
wget -q https://mirror.openshift.com/pub/openshift-v4/clients/ocp/latest/openshift-client-linux.tar.gz
tar -xzf openshift-client-linux.tar.gz
sudo mv oc kubectl /usr/local/bin/

# Helm
curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
```

#### macOS (Intel & Apple Silicon)

```bash
# Using Homebrew
brew install docker kind kubernetes-cli azure-cli helm git go jq

# OpenShift CLI
brew install openshift-cli
# Or download directly:
# Intel: openshift-client-mac.tar.gz
# Apple Silicon: openshift-client-mac-arm64.tar.gz
```

#### Windows (WSL2)

1. Install WSL2 with Ubuntu:
   ```powershell
   wsl --install -d Ubuntu
   ```

2. Follow Ubuntu/Debian installation instructions above

3. Install Docker Desktop for Windows with WSL2 backend

## Environment Differences

### Date/Time Format

The test suite uses Go's `time` package which handles date/time consistently across platforms. Shell scripts that output timestamps use ISO 8601 format:

```bash
date +%Y%m%d_%H%M%S
```

This format is portable across all supported platforms.

### Locale Handling

The test suite does not depend on locale-specific behavior. All string comparisons are case-insensitive where appropriate, and no locale-dependent sorting is used.

### Line Endings

All files in the repository use Unix line endings (LF). Git is configured to handle line ending conversion:

```bash
# Ensure consistent line endings
git config --global core.autocrlf input  # On Linux/macOS
git config --global core.autocrlf true   # On Windows
```

### File Permissions

The test suite respects Unix file permissions. On Windows (WSL2), file permissions work as expected within the WSL2 environment.

## CI/CD Environments

### GitHub Actions

The test suite runs on GitHub Actions using `ubuntu-latest` runners. All CI workflows are designed for this environment:

| Workflow | Runner | Purpose |
|----------|--------|---------|
| `ci.yml` | `ubuntu-latest` | Lint and basic tests |
| `check-dependencies.yml` | `ubuntu-latest` | Dependency validation |
| `test-setup.yml` | `ubuntu-latest` | Repository setup tests |
| `test-kind-cluster.yml` | `ubuntu-latest` | Kind cluster tests |
| `full-test-suite.yml` | `ubuntu-latest` | Complete test run |

### Local Development

For local development on macOS or Linux, the test suite supports the same functionality as CI. Key differences:

- **TTY output**: Real-time output via `/dev/tty` on local development
- **Interactive prompts**: `make clean` prompts for confirmation locally
- **Colors**: ANSI colors enabled when outputting to a terminal

### Container Environments

The test suite can run in container environments with the following requirements:

- Bash 4.0+ available (for shell scripts)
- All required tools installed
- Docker-in-Docker or podman support for Kind

## Known Limitations

### Windows Native (Not Supported)

Native Windows (without WSL2) is not supported due to:

- `/dev/tty` not available
- Unix-specific shell scripts
- Path separator differences (`\` vs `/`)
- Different temporary directory handling

**Recommendation:** Use WSL2 for Windows development.

### macOS Bash Version

macOS ships with Bash 3.2. The shell scripts require Bash 4.0+ due to process substitution patterns. Install newer Bash via Homebrew (`brew install bash`) for compatibility.

### ARM64 Linux

ARM64 Linux (e.g., Raspberry Pi, AWS Graviton) is not officially tested but should work if all tools are available for ARM64.

## Testing Matrix

The following matrix defines the tested configurations:

| OS | Shell | Go Version | Docker/Podman | Status |
|----|-------|------------|---------------|--------|
| Ubuntu 22.04 | bash 5.1 | 1.24 | Docker 24.x | CI Tested |
| Ubuntu 24.04 | bash 5.2 | 1.24 | Docker 26.x | CI Tested |
| macOS 13 (Ventura) | zsh 5.9 / bash 3.2 | 1.24 | Docker Desktop | Manual Tested |
| macOS 14 (Sonoma) | zsh 5.9 / bash 3.2 | 1.24 | Docker Desktop | Manual Tested |
| Fedora 40 | bash 5.2 | 1.24 | Podman 5.x | Manual Tested |
| Windows 11 + WSL2 | bash 5.1 | 1.24 | Docker Desktop | Manual Tested |

## Troubleshooting

### Script Permission Denied

```bash
chmod +x scripts/*.sh
```

### Command Not Found

Ensure all required tools are in your `PATH`:

```bash
# Check tool availability
make check-prereq
```

### TTY Errors in CI

The test suite automatically falls back to stderr when `/dev/tty` is unavailable. This is expected behavior in CI environments.

### Temp Directory Issues

If you encounter issues with the default temporary directory:

```bash
# Override repository directory
export ARO_REPO_DIR=/custom/path/to/repo
```

### bc: command not found

Install bc (basic calculator):

```bash
# Ubuntu/Debian
sudo apt-get install bc

# RHEL/Fedora
sudo dnf install bc

# macOS (pre-installed, but if missing)
brew install bc
```

### xmllint: command not found

Install libxml2 utilities:

```bash
# Ubuntu/Debian
sudo apt-get install libxml2-utils

# RHEL/Fedora
sudo dnf install libxml2

# macOS (pre-installed)
```
