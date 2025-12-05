# cluster-api-installer Integration Guide

This document provides recommendations for integrating the cluster-api-installer repository with this test suite.

## Integration Approaches

### Approach 1: Git Submodule (Recommended for Development)

**Pros:**
- Version controlled integration
- Easy to update to specific commits/tags
- Works well with CI/CD
- No manual setup required

**Cons:**
- Requires git submodule knowledge
- Extra step for repository cloning

**Implementation:**

```bash
# Add as submodule
git submodule add -b ARO-ASO https://github.com/RadekCap/cluster-api-installer.git vendor/cluster-api-installer

# Initialize and update
git submodule update --init --recursive

# In tests, use:
export ARO_REPO_DIR="$(pwd)/vendor/cluster-api-installer"
```

**Makefile integration:**

```makefile
.PHONY: setup-installer
setup-installer:
	git submodule update --init --recursive vendor/cluster-api-installer
```

### Approach 2: Dynamic Clone (Recommended for CI/CD)

**Pros:**
- No submodule complexity
- Always gets latest version from specified branch
- Simpler for new users

**Cons:**
- Network dependency
- Less control over version
- Slower test startup

**Implementation:**

The test suite already supports this - tests will automatically clone to `/tmp/cluster-api-installer-aro` if not present.

```bash
# Just run tests
go test -v ./test
```

**CI/CD configuration:**

```yaml
# .github/workflows/test.yml
- name: Clone cluster-api-installer
  run: |
    git clone -b ARO-ASO \
      https://github.com/RadekCap/cluster-api-installer.git \
      /tmp/cluster-api-installer-aro
```

### Approach 3: Vendored Scripts (Recommended for Production)

**Pros:**
- No external dependencies
- Fast test execution
- Complete control
- Works offline

**Cons:**
- Need to manually sync updates
- Duplicate code
- More maintenance

**Implementation:**

```bash
# Create vendored directory
mkdir -p vendor/installer/{scripts,doc/aro-hcp-scripts}

# Copy essential files
cp -r /path/to/cluster-api-installer/scripts/ vendor/installer/scripts/
cp -r /path/to/cluster-api-installer/doc/aro-hcp-scripts/ vendor/installer/doc/aro-hcp-scripts/
cp /path/to/cluster-api-installer/doc/ARO-capz.md vendor/installer/doc/

# Update test configuration
export ARO_REPO_DIR="$(pwd)/vendor/installer"
```

### Approach 4: Go Module Dependency (Future Enhancement)

If cluster-api-installer becomes a Go module, it could be integrated as a dependency:

```bash
# In go.mod
require github.com/RadekCap/cluster-api-installer v0.1.0
```

This would require cluster-api-installer to:
1. Have a `go.mod` file
2. Export Go packages/functions
3. Follow semantic versioning

## Recommended Setup by Use Case

### Local Development

**Best Choice: Git Submodule**

```bash
# Initial setup
make setup-submodule
export ARO_REPO_DIR="$(pwd)/vendor/cluster-api-installer"

# Run tests
make test
```

### CI/CD Pipeline

**Best Choice: Dynamic Clone**

```yaml
name: ARO-CAPZ Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Install Dependencies
        run: |
          # Install kind
          curl -Lo ./kind https://kind.sigs.k8s.io/dl/latest/kind-linux-amd64
          chmod +x ./kind
          sudo mv ./kind /usr/local/bin/kind

          # Install kubectl
          curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
          chmod +x kubectl
          sudo mv kubectl /usr/local/bin/

          # Install OpenShift CLI
          curl -LO https://mirror.openshift.com/pub/openshift-v4/clients/ocp/latest/openshift-client-linux.tar.gz
          tar xzf openshift-client-linux.tar.gz
          sudo mv oc /usr/local/bin/

          # Install Helm
          curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash

      - name: Clone cluster-api-installer
        run: |
          git clone -b ARO-ASO \
            https://github.com/RadekCap/cluster-api-installer.git \
            /tmp/cluster-api-installer-aro

      - name: Run Check Dependencies Tests
        run: go test -v ./test -run TestCheckDependencies

      - name: Run Setup Tests
        run: |
          export ARO_REPO_DIR=/tmp/cluster-api-installer-aro
          go test -v ./test -run TestSetup

      # Note: Full cluster tests require Azure credentials
      # which should be configured as repository secrets
```

### Production Deployments

**Best Choice: Vendored Scripts**

- Full control over versions
- No external dependencies
- Audit trail for changes

## Configuration Management

### Environment Variable Precedence

1. Explicit environment variables
2. `.env` file (if using dotenv)
3. Test defaults from `config.go`

### Recommended .env File

Create a `.env` file for local development:

```bash
# Repository Configuration
ARO_REPO_URL=https://github.com/RadekCap/cluster-api-installer.git
ARO_REPO_BRANCH=ARO-ASO
ARO_REPO_DIR=/tmp/cluster-api-installer-aro

# Cluster Configuration
KIND_CLUSTER_NAME=capz-tests-stage
CLUSTER_NAME=capz-tests-cluster
RESOURCE_GROUP=capz-tests-rg
OPENSHIFT_VERSION=4.18
REGION=uksouth
ENV=stage

# Azure Configuration (set your actual values)
# AZURE_SUBSCRIPTION_NAME=your-subscription-id
```

**Add to .gitignore:**

```gitignore
.env
.env.local
```

## Version Pinning

### Using Submodule with Specific Commit

```bash
cd vendor/cluster-api-installer
git checkout <specific-commit-hash>
cd ../..
git add vendor/cluster-api-installer
git commit -m "Pin cluster-api-installer to specific version"
```

### Using Tags (when available)

```bash
cd vendor/cluster-api-installer
git checkout v1.0.0
cd ../..
git add vendor/cluster-api-installer
git commit -m "Update cluster-api-installer to v1.0.0"
```

## Updating the Integration

### Update Submodule

```bash
make update-submodule
# or manually:
git submodule update --remote vendor/cluster-api-installer
```

### Update Vendored Scripts

```bash
# Re-copy files from source
cp -r /path/to/cluster-api-installer/scripts/ vendor/installer/scripts/
git add vendor/installer
git commit -m "Update vendored installer scripts"
```

## Testing the Integration

```bash
# Verify repository is accessible
make test-setup

# Run full test suite
make test

# Or step by step
make test
make test-setup
make test-kind
make test-infra
make test-deploy
make test-verify
```

## Troubleshooting

### Submodule Not Initialized

```bash
git submodule update --init --recursive
```

### Wrong Branch Checked Out

```bash
cd vendor/cluster-api-installer
git checkout ARO-ASO
git pull origin ARO-ASO
```

### Permission Issues with Scripts

```bash
find vendor/cluster-api-installer -name "*.sh" -exec chmod +x {} \;
```

## Recommendations Summary

| Use Case | Recommended Approach | Reason |
|----------|---------------------|---------|
| Local Development | Git Submodule | Version control, easy updates |
| CI/CD | Dynamic Clone | Simple, no submodule complexity |
| Production | Vendored Scripts | No external dependencies |
| Quick Testing | Dynamic Clone | Zero setup |
| Long-term Maintenance | Git Submodule | Trackable versions |

## Future Enhancements

1. **Go Module Integration**: If cluster-api-installer exports Go packages
2. **OCI Artifacts**: Package scripts as OCI artifacts
3. **Helm Chart**: Bundle installer as Helm chart
4. **Binary Distribution**: Pre-built binaries for faster setup
