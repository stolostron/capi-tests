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
```

The `make clean` command is interactive by default and will prompt you to confirm deletion of:
- Kind cluster
- Cluster-api-installer repository clone
- Kubeconfig files
- Results directory

This prevents accidental deletion and allows selective cleanup.

For non-interactive cleanup (useful for CI/CD, scripted workflows, or quick resets):
- Use `make clean-all` to delete all resources without prompts
- Or use `FORCE=1 make clean` to skip all confirmation prompts

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

These environment variables are validated in the Check Dependencies phase. If missing, tests will fail with clear remediation instructions.

- `AZURE_TENANT_ID` - Azure tenant ID (**required**)
  ```bash
  export AZURE_TENANT_ID=$(az account show --query tenantId -o tsv)
  ```
- `AZURE_SUBSCRIPTION_ID` or `AZURE_SUBSCRIPTION_NAME` - Azure subscription identifier (**one required**)
  ```bash
  export AZURE_SUBSCRIPTION_ID=$(az account show --query id -o tsv)
  # OR
  export AZURE_SUBSCRIPTION_NAME=$(az account show --query name -o tsv)
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
- `CS_CLUSTER_NAME` - Cluster name prefix used for YAML generation (default: `${CAPZ_USER}-${DEPLOYMENT_ENV}`). The Azure resource group will be named `${CS_CLUSTER_NAME}-resgroup`.
- `OPENSHIFT_VERSION` - OpenShift version (default: `4.21`)
- `REGION` - Azure region (default: `uksouth`)
- `DEPLOYMENT_ENV` - Deployment environment identifier (default: `stage`)
- `CAPZ_USER` - User identifier for domain prefix (default: `rcap`). Must be short enough that `${CAPZ_USER}-${DEPLOYMENT_ENV}` does not exceed 15 characters.

**RFC 1123 Naming Compliance**: The following variables must be RFC 1123 compliant (lowercase alphanumeric and hyphens only, must start/end with alphanumeric):
- `CAPZ_USER`
- `CS_CLUSTER_NAME`
- `DEPLOYMENT_ENV`

This is validated during Check Dependencies (phase 1) to prevent late deployment failures.

### Test Behavior
- `DEPLOYMENT_TIMEOUT` - Control plane deployment timeout (default: `45m`, format: Go duration like `1h`, `45m`)

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
- CI runs check dependencies tests automatically via GitHub Actions
- Use `make test` locally before pushing (runs fast check dependencies tests)

## Known Issues

- `go.mod` specifies invalid Go version 1.25.4 (should be 1.21 or 1.22)
- `test/start_test.go` contains trivial test with unreachable code
- Command injection vulnerability in `06_verification_test.go:68` (base64 decode)

These are tracked issues and should be fixed in separate PRs when addressed.

## Testing Azure Resources

**Warning**: Tests beyond prerequisites require:
- Azure CLI authenticated (`az login`)
- Valid Azure subscription
- Appropriate permissions for ARO deployment
- 30+ minutes for full deployment

Always run `make test` (check dependencies only) locally. Full tests should run in CI/CD or with explicit intent.

## Claude Code Slash Commands

This repository includes custom slash commands in `.claude/commands/` for common workflows. These commands are version-controlled and automatically available after cloning.

### Available Commands

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

#### `/copilot-review`
Process GitHub Copilot code review findings for a PR and automatically resolve review threads.

**Use when**: Responding to automated code review comments

**What it does**:
- Fetches all Copilot review threads via GraphQL API (includes thread IDs)
- Analyzes each finding against repo patterns (CLAUDE.md)
- Implements accepted fixes or provides denial rationale
- Posts individual replies to each finding
- **Automatically resolves review threads** using GraphQL `resolveReviewThread` mutation
- Commits changes if implementations made

**Example**: `/copilot-review 123`

**Technical details**:
- Uses GraphQL to fetch review threads (REST API doesn't include thread IDs)
- Thread resolution requires Repository > Contents or Pull Requests permissions
- Gracefully handles already-resolved threads and resolution failures
- Replies always post even if thread resolution fails (graceful degradation)

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

#### `/implement-issue`
Analyze a GitHub issue and automatically create a pull request with the implementation.

**Use when**: You want to quickly implement a fix for an open GitHub issue

**What it does**:
- Fetches issue details from GitHub
- Analyzes the issue to understand requirements
- Creates a feature branch with appropriate naming
- Implements the fix following repo patterns
- Adds tests for new functionality
- Runs tests to verify changes
- Commits with descriptive message
- Creates a pull request with comprehensive description
- Links PR to issue with "Fixes #<number>"

**Example**: `/implement-issue 72`

**Features**:
- Follows CLAUDE.md patterns and guidelines
- Uses TodoWrite to track implementation progress
- Handles git operations (branch creation, commits, push)
- Validates tests pass before committing
- Generates well-formatted commit messages and PR descriptions
- Automatically references issue in commit and PR

#### `/prepare-worktree`
Create a git worktree for implementing a GitHub issue in an isolated directory.

**Use when**: You want to work on an issue without affecting your current work (e.g., while tests are running)

**What it does**:
- Fetches issue details from GitHub
- Creates a git worktree with a branch named after the issue
- Worktree is created as a sibling directory (e.g., `../CAPZTests-issue-263-...`)
- Copies the `cd` command to clipboard (macOS)
- Prints clear next steps

**Example**: `/prepare-worktree 263`

**Workflow**:
1. Run `/prepare-worktree 263` in your main worktree
2. Open new terminal, paste command from clipboard
3. Run `/implement-issue 263` in the new Claude instance
4. After PR is merged, clean up with `git worktree remove <path>`

**Why use this**:
- Keep your main branch clean while working on issues
- Work on multiple issues in parallel
- Don't interrupt long-running tests or builds

#### `/close-worktree`
Clean up a git worktree after the associated PR has been merged.

**Use when**: After your PR is merged and you want to clean up the worktree

**What it does**:
- Finds the worktree for the given issue number
- **Verifies PR was merged** before proceeding (safety check)
- Warns about uncommitted changes or unpushed commits
- Removes the worktree directory
- Deletes the branch (if merged)
- Prunes stale worktree references

**Example**: `/close-worktree 263`

**Safety checks**:
- PR merged? → Prevents closing before work is accepted
- Uncommitted changes? → Prevents losing local edits
- Unpushed commits? → Warns about commits not on remote

**Complete worktree workflow**:
```bash
# Instance 1: Prepare
/prepare-worktree 263

# Instance 2: Implement (new terminal)
cd ../CAPZTests-issue-263-... && claude
/implement-issue 263

# Instance 1: Cleanup (after PR merged)
/close-worktree 263
```

### Using Slash Commands

Simply type the command in Claude Code:
```
/add-test-phase
```

Commands will prompt for any required information and guide you through the task.

## Documentation

- `README.md` - Repository overview and quick start
- `test/README.md` - Detailed test suite documentation
- `docs/INTEGRATION.md` - Integration patterns with cluster-api-installer
- `TEST_COVERAGE.md` - Test coverage analysis and metrics
