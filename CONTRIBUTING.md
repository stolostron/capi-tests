# Contributing to ARO-CAPZ Test Suite

Thank you for your interest in contributing to the ARO-CAPZ Test Suite! This document provides guidelines for contributing to the project.

## Table of Contents

- [Getting Started](#getting-started)
- [Development Workflow](#development-workflow)
- [Making Changes](#making-changes)
- [Pull Request Process](#pull-request-process)
- [Test Execution Model](#test-execution-model)
- [Coding Guidelines](#coding-guidelines)
  - [Go Code](#go-code)
  - [Test Code](#test-code)
  - [Configuration](#configuration)
  - [Helper Functions](#helper-functions)
  - [Naming Conventions](#naming-conventions)
  - [Error Handling](#error-handling)
  - [Avoid Over-Engineering](#avoid-over-engineering)

## Getting Started

### Prerequisites

Before contributing, ensure you have:

- **Go 1.21 or later** (`go version`)
- **Docker or Podman** for container operations
- **Azure CLI** with valid subscription (`az login`)
- **Required CLI tools**: kind, helm, kubectl, oc, clusterctl

### Development Setup

1. Fork and clone the repository:
   ```bash
   git clone https://github.com/<your-username>/CAPZTests.git
   cd CAPZTests
   ```

2. Verify prerequisites:
   ```bash
   make check-prereq
   ```

3. Run fast tests to verify setup:
   ```bash
   make test
   ```

## Development Workflow

### Running Tests

| Command | Description | Duration |
|---------|-------------|----------|
| `make test` | Check dependencies tests only | ~30 seconds |
| `make test-all` | Full test suite (requires Azure) | 30+ minutes |
| `go test -v ./test -run TestName` | Run specific test | Varies |

### Code Quality

```bash
make fmt      # Format Go code
make lint     # Run linters
make deps     # Update dependencies
```

## Making Changes

### Branch Naming

Use descriptive branch names:

- `issue-<number>-brief-description` - For issue fixes
- `feature-<name>` - For new features
- `fix-<name>` - For bug fixes
- `docs-<name>` - For documentation updates

Example: `issue-72-add-cluster-validation`

### Commit Message Convention

Follow conventional commits format:

```
type: description (fixes #123)

Detailed explanation if needed.

Co-Authored-By: Your Name <email@example.com>
```

Types:
- `fix:` - Bug fixes
- `feat:` - New features
- `docs:` - Documentation changes
- `chore:` - Maintenance tasks
- `refactor:` - Code refactoring
- `test:` - Test additions/changes

### Adding New Test Phases

See [CLAUDE.md](CLAUDE.md#adding-a-new-test-phase) for the detailed pattern. Key steps:

1. Create `test/<phase>_test.go`
2. Follow the existing test pattern with `NewTestConfig()`
3. Add Makefile target
4. Update documentation

### Adding Configuration

1. Add field to `TestConfig` struct in `test/config.go`
2. Initialize using `GetEnvOrDefault()` - never hardcode values
3. Document in README.md and CLAUDE.md
4. Use in tests via `config.<FieldName>`

### Adding Helper Functions

1. Add to `test/helpers.go`
2. Use `t.Helper()` for test helper functions
3. Add cleanup logic with `t.Cleanup()` where applicable
4. Add corresponding tests in `test/helpers_test.go`

## Pull Request Process

1. **Create a branch** from `main`
2. **Make your changes** following the guidelines above
3. **Run tests locally**:
   ```bash
   make fmt
   make lint
   make test
   ```
4. **Commit with descriptive message** referencing issue number
5. **Push and create PR** with detailed description
6. **Address review feedback**
7. **Squash merge** when approved

### PR Checklist

- [ ] Tests pass locally (`make test`)
- [ ] Code is formatted (`make fmt`)
- [ ] Linter passes (`make lint`)
- [ ] Documentation updated if needed
- [ ] Commit message follows convention
- [ ] PR description explains changes

## Test Execution Model

Tests run **sequentially** in phases - each depends on the previous:

1. **Check Dependencies** - Tool availability and authentication
2. **Setup** - Repository cloning and validation
3. **Kind Cluster** - Management cluster deployment
4. **YAML Generation** - Infrastructure YAML generation
5. **CR Deployment** - Custom resource deployment
6. **Verification** - Final cluster validation

Tests are **idempotent** - they skip steps already completed.

### Why Sequential?

- Each phase depends on resources from the previous phase
- Tests interact with external state (Kind cluster, Azure resources)
- Designed for workflow validation, not unit testing

## Coding Guidelines

### Go Code

- Follow standard Go formatting (`gofmt`)
- Use meaningful variable names
- Add comments for non-obvious logic
- Handle errors explicitly - never ignore errors silently
- Prefer early returns to reduce nesting

### Test Code

All test functions must follow this pattern:

```go
func TestPhase_Specific(t *testing.T) {
    config := NewTestConfig()

    // Validate prerequisites
    if !prerequisitesMet {
        t.Skipf("Prerequisites not met: reason")
    }

    // Perform test action
    // Use t.Logf() for progress
    // Use t.Errorf() for non-fatal errors
    // Use t.Fatalf() for fatal errors that prevent continuation
}
```

Key practices:
- Use `t.Helper()` in helper functions
- Use `t.Logf()` for progress information
- Use `t.Errorf()` for non-fatal errors (test continues)
- Use `t.Fatalf()` for fatal errors (test stops)
- Use `t.Skipf()` when prerequisites aren't met

### Configuration

- Always use `GetEnvOrDefault()` for config values - never hardcode
- Document new environment variables in README.md and CLAUDE.md
- Provide sensible defaults
- Use the `TestConfig` struct from `test/config.go`

### Helper Functions

Use existing helpers from `test/helpers.go` instead of reimplementing:

| Helper | Purpose |
|--------|---------|
| `CommandExists(cmd)` | Check if CLI tool is available |
| `RunCommand(t, name, args...)` | Execute shell commands with test context |
| `SetEnvVar(t, key, value)` | Set env var with automatic cleanup |
| `FileExists(path)` / `DirExists(path)` | Path validation |
| `GetEnvOrDefault(key, default)` | Config value resolution |
| `ValidateDomainPrefix(user, env)` | Validate domain prefix length (max 15 chars) |
| `ValidateRFC1123Name(name, varName)` | Validate RFC 1123 subdomain naming |

### Naming Conventions

**RFC 1123 Compliance**: These variables must be RFC 1123 compliant (lowercase alphanumeric and hyphens only, must start/end with alphanumeric):
- `CAPZ_USER`
- `CS_CLUSTER_NAME`
- `DEPLOYMENT_ENV`
- `TEST_NAMESPACE`

### Error Handling

```go
// Good - explicit error handling with context
output, err := RunCommand(t, "kubectl", "get", "pods")
if err != nil {
    t.Fatalf("Failed to get pods: %v", err)
}

// Bad - ignoring errors
output, _ := RunCommand(t, "kubectl", "get", "pods")
```

### Avoid Over-Engineering

- Only make changes directly requested or clearly necessary
- Don't add features, refactor code, or make "improvements" beyond what was asked
- Don't add error handling for scenarios that can't happen
- Three similar lines of code is better than a premature abstraction

## Getting Help

- Check existing [documentation](docs/)
- Review [CLAUDE.md](CLAUDE.md) for detailed patterns
- Open an issue for bugs or questions
- Use `/troubleshoot` Claude command for debugging

## License

By contributing, you agree that your contributions will be licensed under the [Apache License 2.0](LICENSE).
