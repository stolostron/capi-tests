# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Purpose

This is a Go-based test suite for validating Azure Red Hat OpenShift (ARO) deployments using Cluster API Provider Azure (CAPZ) and Azure Service Operator (ASO). The tests verify the complete deployment workflow from prerequisites to cluster verification.

**Important**: This is NOT a multi-cloud CAPI testing framework. It is specifically for ARO-CAPZ on Azure only.

## Test Architecture

### Test Execution Model

Tests are designed to run **sequentially** in a specific order, with each phase depending on the previous phase's success:

1. **Prerequisites** (`01_prerequisites_test.go`) - Tool availability and authentication
2. **Setup** (`02_setup_test.go`) - Repository cloning and validation
3. **Kind Cluster** (`kind_cluster_test.go`) - Management cluster deployment
4. **Infrastructure** (`infrastructure_test.go`) - Resource generation
5. **Deployment** (`deployment_test.go`) - Cluster provisioning monitoring
6. **Verification** (`verification_test.go`) - Final cluster validation

Tests are **idempotent** - they skip steps already completed, allowing re-runs.

### Configuration System

All test configuration is centralized in `test/config.go` via the `TestConfig` struct. Configuration follows this precedence:

1. Environment variables (highest priority)
2. Defaults in `NewTestConfig()`

Key configuration pattern:
```go
config := NewTestConfig()  // Creates config with env vars or defaults
```

Never hardcode values - always use `GetEnvOrDefault()` for new configuration.

### Helper Functions

`test/helpers.go` provides shared utilities used across all tests:

- `CommandExists(cmd)` - Check if CLI tool is available
- `RunCommand(t, name, args...)` - Execute shell commands with test context
- `SetEnvVar(t, key, value)` - Set env var with automatic cleanup
- `FileExists(path)` / `DirExists(path)` - Path validation
- `GetEnvOrDefault(key, default)` - Config value resolution

Always use these helpers instead of reimplementing functionality.

### Test Patterns

All test functions follow this pattern:
```go
func TestPhase_Specific(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping in short mode")
    }

    config := NewTestConfig()

    // Validate prerequisites
    if !prerequisitesMet {
        t.Skipf("Prerequisites not met")
    }

    // Perform test action
    // Use t.Logf() for progress
    // Use t.Errorf() for non-fatal errors
    // Use t.Fatalf() for fatal errors that prevent continuation
}
```

## Development Commands

### Running Tests

```bash
# Prerequisite tests only (fast, no Azure resources)
make test

# Full test suite (all phases sequentially)
make test-all

# Individual test phases
make test-prereq    # Prerequisites verification
make test-setup     # Repository setup
make test-kind      # Kind cluster deployment
make test-infra     # Infrastructure generation
make test-deploy    # Deployment monitoring
make test-verify    # Cluster verification

# Quick tests (uses Go's -short flag)
make test-short

# Run specific test function
go test -v ./test -run TestPrerequisites_ToolsAvailable
go test -v ./test -run TestInfrastructure

# With custom configuration
ENV=prod CLUSTER_NAME=my-cluster go test -v ./test -timeout 60m
```

### Repository Management

```bash
# Check prerequisites
make check-prereq

# Setup cluster-api-installer as submodule
make setup-submodule

# Update submodule
make update-submodule

# Clean up test resources
make clean
```

### Code Quality

```bash
# Format code
make fmt

# Run linters
make lint

# Download/update dependencies
make deps
```

## Integration with cluster-api-installer

Tests require access to the [cluster-api-installer](https://github.com/RadekCap/cluster-api-installer) repository. Three approaches are supported:

1. **Git Submodule** (recommended for development)
   ```bash
   make setup-submodule
   export ARO_REPO_DIR="$(pwd)/vendor/cluster-api-installer"
   ```

2. **Automatic Clone** (CI/CD, default)
   - Tests auto-clone to `/tmp/cluster-api-installer-aro`
   - No manual setup needed

3. **Existing Clone** (manual)
   ```bash
   export ARO_REPO_DIR="/path/to/cluster-api-installer"
   ```

See `docs/INTEGRATION.md` for detailed integration patterns.

## Environment Variables

### Repository Configuration
- `ARO_REPO_URL` - cluster-api-installer URL (default: RadekCap/cluster-api-installer)
- `ARO_REPO_BRANCH` - Branch to use (default: `ARO-ASO`)
- `ARO_REPO_DIR` - Local path (default: `/tmp/cluster-api-installer-aro`)

### Cluster Configuration
- `KIND_CLUSTER_NAME` - Management cluster name (default: `capz-stage`)
- `CLUSTER_NAME` - ARO cluster name (default: `test-cluster`)
- `RESOURCE_GROUP` - Azure resource group
- `OPENSHIFT_VERSION` - OpenShift version (default: `4.18`)
- `REGION` - Azure region (default: `eastus`)
- `AZURE_SUBSCRIPTION_NAME` - Azure subscription ID (required for deployment)
- `ENV` - Environment identifier (default: `stage`)
- `USER` - User identifier (default: current user)

### Test Behavior
- Use `-short` flag or `make test-short` to skip long-running tests
- All tests check `testing.Short()` before executing expensive operations
- `DEPLOYMENT_TIMEOUT` - Control plane deployment timeout (default: `30m`, format: Go duration like `1h`, `45m`)

## Key Architecture Decisions

### Why Tests Are Sequential

Unlike typical Go tests that run in parallel, these tests MUST run sequentially because:
- Each phase depends on resources created by the previous phase
- Tests interact with external state (Kind cluster, Azure resources)
- Tests are designed for workflow validation, not unit testing

### Why Configuration is Environment-Based

Configuration uses environment variables rather than config files because:
- Easier CI/CD integration (GitHub Actions, etc.)
- No sensitive data in repository
- Flexibility for different environments (dev, stage, prod)
- Follows 12-factor app principles

### Why Helper Functions Matter

Centralized helpers in `helpers.go` ensure:
- Consistent error handling across all tests
- Automatic cleanup via `t.Cleanup()`
- Uniform command execution and logging
- Reusable test utilities

## Common Tasks

### Adding a New Test Phase

1. Create `test/<phase>_test.go`
2. Follow the test pattern (see Test Patterns above)
3. Use `config := NewTestConfig()` for configuration
4. Check prerequisites with `t.Skip()` or `t.Skipf()`
5. Add Makefile target in `Makefile`:
   ```makefile
   test-<phase>: ## Run <phase> tests only
       go test -v ./test -run Test<Phase> -timeout 30m
   ```
6. Update `test-all` target to include new phase

### Adding Configuration

1. Add field to `TestConfig` struct in `test/config.go`
2. Initialize in `NewTestConfig()` using `GetEnvOrDefault()`
3. Document in README.md configuration section
4. Use in tests via `config.<FieldName>`

### Adding Helper Functions

1. Add to `test/helpers.go`
2. Use `t.Helper()` for functions that call `t.*` methods
3. Follow existing patterns for error handling
4. Add cleanup logic with `t.Cleanup()` where applicable

## Git Workflow

- Main branch for PRs: `readme`
- Tests run on: `main`, `readme`, and specific feature branches
- CI runs prerequisite tests automatically via GitHub Actions
- Use `make test` locally before pushing (runs fast prerequisite tests)

## Known Issues

- `go.mod` specifies invalid Go version 1.25.4 (should be 1.21 or 1.22)
- `test/start_test.go` contains trivial test with unreachable code
- Command injection vulnerability in `verification_test.go:61` (base64 decode)

These are tracked issues and should be fixed in separate PRs when addressed.

## Testing Azure Resources

**Warning**: Tests beyond prerequisites require:
- Azure CLI authenticated (`az login`)
- Valid Azure subscription
- Appropriate permissions for ARO deployment
- 30+ minutes for full deployment

Always run `make test` (prerequisites only) locally. Full tests should run in CI/CD or with explicit intent.

## Documentation

- `README.md` - Repository overview and quick start
- `test/README.md` - Detailed test suite documentation
- `docs/INTEGRATION.md` - Integration patterns with cluster-api-installer
- `TEST_COVERAGE.md` - Test coverage analysis and metrics
