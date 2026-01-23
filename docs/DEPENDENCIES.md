# Dependency Management

This document describes how dependencies are managed in the ARO-CAPZ test suite, including Go modules, external tools, and security scanning.

## Overview

The test suite has two categories of dependencies:

1. **Go Dependencies** - Managed via `go.mod` and `go.sum`
2. **External Tools** - CLI tools required to run tests
3. **cluster-api-installer** - External repository dependency

## Go Dependencies

### Current Dependencies

| Package | Version | Purpose |
|---------|---------|---------|
| `gopkg.in/yaml.v3` | v3.0.1 | YAML parsing for configuration |

### Go Version

The project requires **Go 1.22** or later, as specified in `go.mod`.

### Dependency Verification

```bash
# Verify module checksums
go mod verify

# Tidy dependencies (remove unused, add missing)
go mod tidy

# Check for differences
git diff go.mod go.sum  # Should be empty after tidy
```

### Adding New Dependencies

When adding new dependencies:

1. **Use specific versions** - Always pin to a specific version
2. **Verify source** - Only use dependencies from trusted sources
3. **Check security** - Run vulnerability scans before committing
4. **Update documentation** - Add to this document if significant

```bash
# Add a new dependency
go get github.com/example/package@v1.2.3

# Run security checks
go run golang.org/x/vuln/cmd/govulncheck@latest ./...

# Commit changes
git add go.mod go.sum
git commit -m "deps(go): add github.com/example/package v1.2.3"
```

## External Tools

### Required Tool Versions

| Tool | Minimum Version | CI Version | Purpose |
|------|----------------|------------|---------|
| **Go** | 1.22 | from go.mod | Running tests |
| **Docker/Podman** | 20.10+ | latest | Container runtime |
| **Kind** | 0.20.0 | 0.20.0 | Management cluster |
| **Azure CLI** | 2.50.0 | latest | Azure authentication |
| **kubectl** | 1.28+ | latest | Kubernetes CLI |
| **oc** | 4.14+ | latest | OpenShift CLI |
| **Helm** | 3.12+ | 3.16.0 | Kubernetes package manager |
| **Git** | 2.30+ | latest | Source control |
| **jq** | 1.6+ | latest | JSON processing |

### Version Pinning Strategy

| Tool | Strategy | Rationale |
|------|----------|-----------|
| Go | Pinned in go.mod | Consistent across all environments |
| Kind | Pinned | Cluster compatibility |
| Helm | Pinned | Chart compatibility |
| kubectl | Latest stable | Broad Kubernetes compatibility |
| Azure CLI | Latest | Azure feature support |
| oc | Latest | OpenShift feature support |

### Verifying Tool Versions

```bash
# Check all tool versions
go version
docker --version   # or: podman --version
kind version
az version
kubectl version --client
oc version --client
helm version
git --version
jq --version
```

## Vulnerability Scanning

### Automated Scanning

The repository includes multiple security scanning workflows:

| Scanner | Workflow | Schedule | Database |
|---------|----------|----------|----------|
| **govulncheck** | `security-govulncheck.yml` | Daily 2 AM UTC | Go Vulnerability DB |
| **nancy** | `security-nancy.yml` | Daily 3:30 AM UTC | Sonatype OSS Index |
| **Trivy** | `security-trivy.yml` | On push/PR | Multiple sources |
| **gosec** | `security-gosec.yml` | On push/PR | Static analysis |

### Running Scans Locally

```bash
# Official Go vulnerability checker
go run golang.org/x/vuln/cmd/govulncheck@latest ./...

# Nancy (Sonatype OSS Index) - requires API token for full access
go list -json -m all | go run github.com/sonatype-nexus-community/nancy@latest sleuth

# gosec static analysis
go run github.com/securego/gosec/v2/cmd/gosec@latest ./...
```

### Dependabot

Dependabot is configured in `.github/dependabot.yml` to:

- Check Go module updates weekly (Mondays at 09:00 UTC)
- Check GitHub Actions updates weekly
- Group minor/patch updates to reduce PR noise
- Auto-assign reviewers

## cluster-api-installer Dependency

See [INTEGRATION.md](INTEGRATION.md) for detailed documentation on:

- Integration approaches (submodule, dynamic clone, vendored)
- Branch/version pinning strategy
- Compatibility testing procedures
- Update and rollback procedures

### Quick Reference

```bash
# Update submodule to latest
make update-submodule

# Clone specific branch
export ARO_REPO_BRANCH=ARO-ASO
export ARO_REPO_DIR=/tmp/cluster-api-installer-aro
git clone -b $ARO_REPO_BRANCH https://github.com/RadekCap/cluster-api-installer.git $ARO_REPO_DIR
```

## Dependency Update Workflow

### Weekly Updates (Automated)

1. Dependabot creates PRs for Go and GitHub Actions updates
2. CI runs all tests and security scans
3. Review and merge approved PRs

### Manual Updates

```bash
# Check for outdated Go dependencies
go list -u -m all

# Update all dependencies (careful - test thoroughly)
go get -u ./...
go mod tidy

# Run tests
make test

# Run security scans
go run golang.org/x/vuln/cmd/govulncheck@latest ./...

# Commit if all passes
git add go.mod go.sum
git commit -m "deps(go): update all dependencies"
```

### Emergency Security Updates

For critical vulnerabilities:

1. Identify affected package from security alert
2. Update to patched version:
   ```bash
   go get -u <vulnerable-package>@<patched-version>
   go mod tidy
   ```
3. Run security scan to verify fix:
   ```bash
   go run golang.org/x/vuln/cmd/govulncheck@latest ./...
   ```
4. Run tests:
   ```bash
   make test
   ```
5. Create PR with `security` label

## Best Practices

### Do

- Pin dependencies to specific versions
- Run vulnerability scans before merging
- Keep dependencies minimal
- Document significant dependencies
- Test thoroughly after updates

### Don't

- Use `latest` tags in Go dependencies
- Skip security scans for "minor" updates
- Add unnecessary dependencies
- Ignore Dependabot alerts
- Update multiple major versions at once

## Troubleshooting

### go mod verify fails

```bash
# Clear module cache and re-download
go clean -modcache
go mod download
go mod verify
```

### Vulnerability scan finds issues

1. Check if vulnerability affects your usage
2. Look for patched version:
   ```bash
   go list -m -versions <package>
   ```
3. Update to patched version
4. If no patch available, consider:
   - Replacing the dependency
   - Adding to vulnerability ignore list (with justification)
   - Implementing workaround

### Dependabot PRs failing

1. Check CI logs for specific failure
2. May need manual intervention for breaking changes
3. Consider grouping with other updates
