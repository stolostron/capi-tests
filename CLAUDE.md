# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Purpose

This is a Go-based test suite for validating Azure Red Hat OpenShift (ARO) deployments using Cluster API Provider Azure (CAPZ) and Azure Service Operator (ASO). The tests verify the complete deployment workflow from prerequisites to cluster verification.

**Important**: This is NOT a multi-cloud CAPI testing framework. It is specifically for ARO-CAPZ on Azure only.

## Test Architecture

### Test Execution Model

Tests are designed to run **sequentially** in a specific order, with each phase depending on the previous phase's success:

1. **Check Dependencies** (`01_check_dependencies_test.go`) - Tool availability and authentication
2. **Setup** (`02_setup_test.go`) - Repository cloning and validation
3. **Kind Cluster** (`03_cluster_test.go`) - Management cluster deployment
4. **Infrastructure** (`04_generate_yamls_test.go`) - YAML generation
5. **Deployment** (`05_deploy_crs_test.go`) - CR deployment monitoring
6. **Verification** (`06_verification_test.go`) - Final cluster validation
7. **Deletion** (`07_deletion_test.go`) - Workload cluster deletion
8. **Cleanup Validation** (`08_cleanup_test.go`) - Cleanup operations validation (standalone)

Tests are **idempotent** - they skip steps already completed, allowing re-runs.

### Idempotency Guarantees

All test phases are designed to be idempotent and safe to re-run:

| Phase | Skip Detection | Re-run Behavior |
|-------|----------------|-----------------|
| 01 Check Dependencies | N/A (stateless) | Always runs - no persistent state |
| 02 Setup | Directory exists | Validates git integrity, skips clone |
| 03 Kind Cluster | `kind get clusters` | Skips if cluster already exists |
| 04 Generate YAMLs | All YAML files exist | Skips if credentials.yaml, is.yaml, aro.yaml all present |
| 05 Deploy CRs | File existence | Uses `kubectl apply` (inherently idempotent) |
| 06 Verification | Kubeconfig + cluster state | Guards on required files and cluster phase |
| 07 Deletion | Resource existence | Skips if already deleted |
| 08 Cleanup Validation | N/A (stateless) | Always reports current cleanup status |

**Key idempotency patterns:**
- File-based detection (check if output exists before generating)
- Resource-based detection (check if Kubernetes resources exist)
- `kubectl apply` for CR deployment (creates or updates, never duplicates)
- Graceful skip with `t.Skipf()` when prerequisites aren't met

**To force regeneration:**
```bash
# Delete output directory to regenerate YAMLs
rm -rf /tmp/cluster-api-installer-aro/rcap-stage/

# Delete Kind cluster to redeploy
kind delete cluster --name capz-tests-stage
```

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
- `ValidateDomainPrefix(user, env)` - Validate domain prefix length (max 15 chars)
- `ValidateRFC1123Name(name, varName)` - Validate RFC 1123 subdomain naming compliance

Always use these helpers instead of reimplementing functionality.

### Test Patterns

All test functions follow this pattern:
```go
func TestPhase_Specific(t *testing.T) {
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

This repository follows proven Go testing best practices including table-driven tests, `t.Helper()`, and `t.Cleanup()`. For comprehensive guidelines, see `docs/TESTING_GUIDELINES.md`.

## Development Commands

### Running Tests

```bash
# Check dependencies tests only (fast, no Azure resources)
make test

# Full test suite (all phases sequentially)
make test-all

# Individual test phases (internal use - called by test-all)
make _check-dep      # Check dependencies
make _setup          # Repository setup
make _cluster        # Cluster deployment
make _generate-yamls # YAML generation
make _deploy-crs     # CR deployment
make _verify         # Cluster verification
make _delete         # Cluster deletion

# Run specific test function
go test -v ./test -run TestCheckDependencies_ToolAvailable
go test -v ./test -run TestInfrastructure

# With custom configuration
DEPLOYMENT_ENV=prod WORKLOAD_CLUSTER_NAME=my-cluster go test -v ./test -timeout 60m
```

### Repository Management

```bash
# Check prerequisites
make check-prereq

# Setup cluster-api-installer as submodule
make setup-submodule

# Update submodule
make update-submodule

# Clean up test resources (interactive - prompts before deleting each resource)
make clean

# Clean up ALL test resources without prompting (non-interactive)
make clean-all
# OR use the FORCE variable:
FORCE=1 make clean

# Clean up all Azure resources (resource group + orphaned resources)
make clean-azure
```

The `make clean` command is interactive by default and will prompt you to confirm deletion of:
- Kind cluster
- Cluster-api-installer repository clone
- Kubeconfig files
- Results directory
- Azure resource group (`${CS_CLUSTER_NAME}-resgroup`)
- Orphaned Azure resources (resources with `${CAPZ_USER}` prefix that survive RG deletion)

This prevents accidental deletion and allows selective cleanup.

For non-interactive cleanup (useful for CI/CD, scripted workflows, or quick resets):
- Use `make clean-all` to delete all resources without prompts (includes Azure resources and orphaned resources)
- Or use `FORCE=1 make clean` to skip all confirmation prompts

**Azure Resource Cleanup (`make clean-azure`)**:

The unified `make clean-azure` command cleans up all Azure resources in one operation:
- Azure Resource Group (`${CS_CLUSTER_NAME}-resgroup`)
- Orphaned ARM resources (Managed Identities, VNets, NSGs, DNS Zones)
- Azure AD Applications (App Registrations)
- Service Principals

```bash
# Interactive cleanup of all Azure resources
make clean-azure

# Non-interactive cleanup
FORCE=1 make clean-azure

# Dry-run to see what would be deleted
./scripts/cleanup-azure-resources.sh --resource-group myapp-resgroup --prefix myapp --dry-run

# Clean with custom prefix
CAPZ_USER=myprefix make clean-azure
```

Notes:
- The resource group name is derived from `${CAPZ_USER}-${DEPLOYMENT_ENV}-resgroup` (default: `rcap-stage-resgroup`)
- Uses `az group delete --yes --no-wait` for non-blocking deletion
- Gracefully skips Azure cleanup if Azure CLI is not installed or not authenticated

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

### Azure Authentication (Required)

The test suite supports two authentication methods. Choose the one that best fits your workflow:

#### Option 1: Service Principal (Recommended for CI/Automation)

Set these environment variables to authenticate using an existing service principal:

- `AZURE_CLIENT_ID` - Service principal application (client) ID (**required for SP auth**)
- `AZURE_CLIENT_SECRET` - Service principal secret (**required for SP auth**)
- `AZURE_TENANT_ID` - Azure tenant ID (**required**)
- `AZURE_SUBSCRIPTION_ID` - Azure subscription ID (**required**)

```bash
export AZURE_CLIENT_ID=<your-client-id>
export AZURE_CLIENT_SECRET=<your-client-secret>
export AZURE_TENANT_ID=<your-tenant-id>
export AZURE_SUBSCRIPTION_ID=<your-subscription-id>
```

To create a new service principal:
```bash
az ad sp create-for-rbac --name <name> --role Contributor --scopes /subscriptions/<subscription-id>
```

#### Option 2: Azure CLI (Convenient for Development)

Simply login with Azure CLI and the test suite will auto-extract required credentials:

```bash
az login
```

The following environment variables can be auto-extracted from Azure CLI if not set:
- `AZURE_TENANT_ID` - Azure tenant ID (auto-extracted via `az account show`)
- `AZURE_SUBSCRIPTION_ID` or `AZURE_SUBSCRIPTION_NAME` - Azure subscription identifier (auto-extracted)

Manual export if needed:
```bash
export AZURE_TENANT_ID=$(az account show --query tenantId -o tsv)
export AZURE_SUBSCRIPTION_ID=$(az account show --query id -o tsv)
```

### Repository Configuration
- `ARO_REPO_URL` - cluster-api-installer URL (default: RadekCap/cluster-api-installer)
- `ARO_REPO_BRANCH` - Branch to use (default: `ARO-ASO`)
- `ARO_REPO_DIR` - Local path (default: `/tmp/cluster-api-installer-aro`)

### Cluster Configuration
- `MANAGEMENT_CLUSTER_NAME` - Management cluster name (default: `capz-tests-stage`)
  - **Note**: Tests automatically translate this to `KIND_CLUSTER_NAME` for the deployment script
  - Use this variable for configuring tests; `KIND_CLUSTER_NAME` is set internally
- `WORKLOAD_CLUSTER_NAME` - ARO workload cluster name (default: `capz-tests-cluster`)
- `CS_CLUSTER_NAME` - **C**luster **S**ervice cluster name prefix used for YAML generation and Azure resource naming (default: `${CAPZ_USER}-${DEPLOYMENT_ENV}`). The Azure resource group will be named `${CS_CLUSTER_NAME}-resgroup`. This prefix is also used for the ExternalAuth resource ID.
- `OPENSHIFT_VERSION` - OpenShift version (default: `4.21`)
- `REGION` - Azure region (default: `uksouth`)
- `DEPLOYMENT_ENV` - Deployment environment identifier (default: `stage`)
- `CAPZ_USER` - User identifier for domain prefix (default: `rcap`). Must be short enough that `${CAPZ_USER}-${DEPLOYMENT_ENV}` does not exceed 15 characters.
- `TEST_NAMESPACE` - Kubernetes namespace for testing resources (default: `default`). All resource checks will be scoped to this namespace instead of using `-A` (all namespaces).

### External Cluster Mode
- `USE_KUBECONFIG` - Path to an external kubeconfig file. When set, the test suite runs in "external cluster mode":
  - Skips Kind cluster creation (Phase 03)
  - Skips repository cloning (Phase 02) - controllers are pre-installed
  - Validates pre-installed CAPI/CAPZ/ASO controllers
  - Uses the `current-context` from the specified kubeconfig file
  - Automatically sets `USE_K8S=true` for MCE namespace defaults (`multicluster-engine`)

**RFC 1123 Naming Compliance**: The following variables must be RFC 1123 compliant (lowercase alphanumeric and hyphens only, must start/end with alphanumeric):
- `CAPZ_USER`
- `CS_CLUSTER_NAME`
- `DEPLOYMENT_ENV`
- `TEST_NAMESPACE`

This is validated during Check Dependencies (phase 1) to prevent late deployment failures.

### Test Behavior
- `DEPLOYMENT_TIMEOUT` - Control plane deployment timeout (default: `45m`, format: Go duration like `1h`, `45m`)

### MCE Component Management
- `MCE_AUTO_ENABLE` - Auto-enable MCE CAPI/CAPZ components if not found on external cluster (default: `true` when `USE_KUBECONFIG` is set)
- `MCE_ENABLEMENT_TIMEOUT` - Timeout for waiting after MCE component enablement (default: `15m`, format: Go duration)

When using an external MCE cluster (`USE_KUBECONFIG`), the test suite will:
1. Detect if the cluster is an MCE installation
2. Check if CAPI (`cluster-api`) and CAPZ (`cluster-api-provider-azure-preview`) components are enabled
3. If disabled and `MCE_AUTO_ENABLE=true`, automatically enable them via MCE patching
4. Wait for controllers to become available before proceeding

**Note**: MCE auto-enablement requires `jq` to be installed for JSON transformation.

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

### Branching Strategy
- Main branch for PRs: `main`
- Tests run on: `main` and feature branches
- CI runs check dependencies tests automatically via GitHub Actions
- Use `make test` locally before pushing (runs fast check dependencies tests)

### Rebase, Not Merge

**Important**: This repository uses rebase instead of merge to maintain a clean, linear commit history.

**Updating your feature branch:**
```bash
git fetch origin main
git rebase origin/main
# If already pushed, force push with lease:
git push --force-with-lease
```

**Why rebase?**
- Creates clean, linear history (no merge commits)
- Makes git log and bisect easier to use
- Each PR's commits are clearly visible

**Never do this:**
```bash
git merge main  # Creates merge commits - avoid this
```

**Using the `/sync-main` command:**
The `/sync-main` Claude Code command helps keep your branch updated with proper rebase workflow. It handles fetching, rebasing, and force pushing safely.

## Known Issues

- `test/start_test.go` contains trivial test with unreachable code

These are tracked issues and should be fixed in separate PRs when addressed.

## Testing Azure Resources

**Warning**: Tests beyond prerequisites require:
- Azure CLI authenticated (`az login`)
- Valid Azure subscription
- Appropriate permissions for ARO deployment
- 30+ minutes for full deployment

Always run `make test` (check dependencies only) locally. Full tests should run in CI/CD or with explicit intent.

## Claude Code Slash Commands

This repository uses two sources for slash commands:

1. **Global commands** from [RadekCap/claude-commands](https://github.com/RadekCap/claude-commands) - Generic commands that work across all repos (symlinked to `~/.claude/commands/`)
2. **Repo-specific commands** in `.claude/commands/` - Commands specific to this test suite

### Global Commands (from claude-commands repo)

These commands are available in all repos via the shared `~/.claude/commands/` symlink:

| Command | Description |
|---------|-------------|
| `/implement-issue <number>` | Analyze GitHub issue and create PR with implementation |
| `/prepare-worktree <number>` | Create isolated git worktree for an issue |
| `/close-worktree <number>` | Clean up worktree after PR is merged |
| `/sync-main` | Sync main branch and optionally create feature branch |
| `/copilot-review <pr>` | Process GitHub Copilot review findings |
| `/context` | Show current session context (dir, branch, todos) |

See the [claude-commands README](https://github.com/RadekCap/claude-commands) for setup instructions and full documentation.

### Repo-Specific Commands

#### `/add-test-phase`
Scaffold a new test phase file following established patterns.

**Use when**: Adding a new sequential test phase to the suite

**What it does**:
- Prompts for phase number and description
- Creates properly structured test file
- Adds Makefile target
- Suggests documentation updates

**Example**: `/add-test-phase`

#### `/review-test`
Review test files for compliance with repo patterns.

**Use when**: Checking if test code follows CLAUDE.md guidelines

**What it does**:
- Validates configuration usage (NewTestConfig)
- Checks helper function usage
- Verifies error handling patterns
- Reports issues with file:line references

**Example**: `/review-test test/03_cluster_test.go`

#### `/update-docs`
Update documentation after code changes.

**Use when**: After adding test phases, config vars, or helper functions

**What it does**:
- Identifies affected documentation files
- Updates README.md, CLAUDE.md, test/README.md as needed
- Ensures consistency across all docs
- Validates cross-references and examples

**Example**: `/update-docs`

#### `/troubleshoot`
Systematically debug test failures.

**Use when**: A test phase is failing

**What it does**:
- Guides through diagnostic workflow
- Checks prerequisites, auth, configuration
- Validates phase dependencies
- Suggests specific fixes with commands
- Provides prevention tips

**Example**: `/troubleshoot`

### Using Slash Commands

Simply type the command in Claude Code:
```
/add-test-phase
```

Commands will prompt for any required information and guide you through the task.

## Destructive Actions

- Never delete Azure resources without explicit confirmation
- When asked to "list", "check", or "show" resources, only report findings - do not take action
- Always ask before: deleting, force-deleting, removing, or cleaning up resources
- When listing resources that might need cleanup, present findings and wait for user instruction

## Documentation

- `README.md` - Repository overview and quick start
- `test/README.md` - Detailed test suite documentation
- `docs/INTEGRATION.md` - Integration patterns with cluster-api-installer
- `docs/DEPENDENCIES.md` - Dependency management, security scanning, and updates
- `docs/TESTING_GUIDELINES.md` - Go testing best practices and guidelines
- `docs/CROSS_PLATFORM.md` - Cross-platform compatibility guide (OS support, shell compatibility, installation)
- `docs/API_REVIEW.md` - V1 API/Interface contract review
- `docs/PERFORMANCE_REVIEW.md` - V1 Performance review and optimization analysis
- `docs/SECURITY_REVIEW.md` - V1 Security review and vulnerability assessment
- `TEST_COVERAGE.md` - Test coverage analysis and metrics

### Community Health Files

- `CONTRIBUTING.md` - Contribution guidelines and development workflow
- `SECURITY.md` - Security policy and vulnerability reporting
- `LICENSE` - Apache License 2.0
- `.github/ISSUE_TEMPLATE/` - Issue templates for bugs and features
- `.github/PULL_REQUEST_TEMPLATE.md` - PR template with checklists
