# Go Testing Guidelines for CAPI Test Suite

This document outlines the testing guidelines and best practices used in this repository, based on proven Go testing patterns from official Go documentation and industry standards.

## Table of Contents

- [Core Principles](#core-principles)
- [Table-Driven Tests](#table-driven-tests)
- [Test Helpers](#test-helpers)
- [Test Lifecycle Management](#test-lifecycle-management)
- [Error Handling in Tests](#error-handling-in-tests)
- [Test Organization](#test-organization)
- [Parallel vs Sequential Tests](#parallel-vs-sequential-tests)
- [Best Practices Checklist](#best-practices-checklist)
- [References](#references)

## Core Principles

### Test Behavior, Not Implementation

Focus tests on verifying the public API behavior of your code, not internal implementation details. This makes tests more resilient to refactoring and easier to maintain.

### Keep Tests Simple and Focused

Each test should verify one specific behavior or scenario. This makes failures easier to diagnose and tests easier to understand.

### Tests Are Documentation

Well-written tests serve as living documentation of how your code should behave. Test names and assertions should clearly communicate intent.

## Table-Driven Tests

Table-driven testing is the **idiomatic Go approach** for testing multiple scenarios with the same logic. This repository uses table-driven tests extensively in `helpers_test.go` and `config_test.go`.

### Why Table-Driven Tests?

- **DRY Principle**: Write test logic once and reuse it across multiple test cases
- **Easy to Add Cases**: Adding new scenarios is as simple as adding a row to the table
- **Readable**: Clear input/output pairs make it easy to understand what's being tested
- **Maintainable**: Changes to test logic apply to all cases automatically

### Standard Pattern

```go
func TestValidateDomainPrefix(t *testing.T) {
    tests := []struct {
        name        string      // Descriptive test case name
        user        string      // Input 1
        environment string      // Input 2
        expectError bool        // Expected outcome
        errorMsgs   []string    // Additional validation (error message substrings)
    }{
        // Valid cases
        {
            name:        "exactly 15 chars",
            user:        "user1234567",
            environment: "dev",
            expectError: false,
        },
        // Invalid cases
        {
            name:        "16 chars - just over limit",
            user:        "radoslavcap",
            environment: "test",
            expectError: true,
            errorMsgs:   []string{"exceeds maximum length", "16 chars"},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateDomainPrefix(tt.user, tt.environment)

            if tt.expectError {
                if err == nil {
                    t.Errorf("ValidateDomainPrefix(%q, %q) expected error, got nil",
                        tt.user, tt.environment)
                    return
                }
                // Validate error message content
                for _, msg := range tt.errorMsgs {
                    if !strings.Contains(err.Error(), msg) {
                        t.Errorf("error = %q, expected to contain %q", err.Error(), msg)
                    }
                }
            } else {
                if err != nil {
                    t.Errorf("ValidateDomainPrefix(%q, %q) unexpected error: %v",
                        tt.user, tt.environment, err)
                }
            }
        })
    }
}
```

### Key Elements

1. **Slice of anonymous structs**: Each struct represents a complete test case
2. **Descriptive names**: The `name` field becomes the subtest name in output
3. **`t.Run()` subtests**: Enables selective test execution and clear output
4. **Detailed error messages**: Always include actual vs expected values

### Using Maps for Test Independence

For tests where order independence is important, use a `map[string]struct{}` instead:

```go
tests := map[string]struct {
    input    string
    expected string
}{
    "empty string":  {input: "", expected: ""},
    "single char":   {input: "x", expected: "x"},
}

for name, tc := range tests {
    t.Run(name, func(t *testing.T) {
        // test logic
    })
}
```

Maps provide automatic test names from keys and undefined iteration order ensures test independence.

## Test Helpers

Test helpers are functions that reduce boilerplate and make tests more readable. This repository centralizes helpers in `test/helpers.go`.

### Using t.Helper()

**Always** call `t.Helper()` at the beginning of helper functions. This ensures that when a test fails, the error is reported at the test call site, not inside the helper.

```go
// Good - with t.Helper()
func RunCommand(t *testing.T, name string, args ...string) (string, error) {
    t.Helper()  // Error line numbers point to caller
    cmd := exec.Command(name, args...)
    output, err := cmd.CombinedOutput()
    return strings.TrimSpace(string(output)), err
}

// Bad - without t.Helper()
func RunCommand(t *testing.T, name string, args ...string) (string, error) {
    // Error line numbers point here instead of caller
    cmd := exec.Command(name, args...)
    output, err := cmd.CombinedOutput()
    return strings.TrimSpace(string(output)), err
}
```

### Using testing.TB Interface

For helpers that work with both tests and benchmarks, use `testing.TB`:

```go
func requireEqual(tb testing.TB, got, want int) {
    tb.Helper()
    if got != want {
        tb.Fatalf("got %d, want %d", got, want)
    }
}
```

### Available Helpers in This Repository

| Helper | Purpose |
|--------|---------|
| `CommandExists(cmd)` | Check if CLI tool is available |
| `RunCommand(t, name, args...)` | Execute shell commands with test context |
| `RunCommandQuiet(t, name, args...)` | Execute commands without TTY output |
| `RunCommandWithStreaming(t, name, args...)` | Execute with real-time output streaming |
| `SetEnvVar(t, key, value)` | Set env var with automatic cleanup |
| `FileExists(path)` / `DirExists(path)` | Path validation |
| `GetEnvOrDefault(key, default)` | Config value resolution |
| `ValidateDomainPrefix(user, env)` | Validate domain prefix length |
| `ValidateRFC1123Name(name, varName)` | Validate Kubernetes naming |
| `PrintTestHeader(t, name, description)` | Print clear test identification |
| `ReportProgress(t, iteration, elapsed, remaining, timeout)` | Progress reporting |

## Test Lifecycle Management

### Using t.Cleanup()

`t.Cleanup()` registers a function to run when the test completes, regardless of success or failure. This is preferred over `defer` for test cleanup because:

1. Cleanup runs **after** the test function returns
2. Multiple cleanup functions run in LIFO order
3. Cleanup runs even if test is skipped or calls `t.FailNow()`

```go
func SetEnvVar(t *testing.T, key, value string) {
    t.Helper()
    oldValue := os.Getenv(key)
    os.Setenv(key, value)

    t.Cleanup(func() {
        if oldValue == "" {
            os.Unsetenv(key)
        } else {
            os.Setenv(key, oldValue)
        }
    })
}
```

### Using t.TempDir()

For tests that need temporary directories:

```go
func TestExtractClusterNameFromYAML(t *testing.T) {
    tmpDir := t.TempDir()  // Automatically cleaned up after test

    path := filepath.Join(tmpDir, "test.yaml")
    // ... use path
}
```

## Error Handling in Tests

### t.Errorf() vs t.Fatalf()

- **`t.Errorf()`**: Non-fatal error, test continues executing
- **`t.Fatalf()`**: Fatal error, test stops immediately

Choose based on whether subsequent assertions are meaningful:

```go
// Use t.Errorf() when test can continue meaningfully
output, err := RunCommand(t, "kubectl", "get", "pods")
if err != nil {
    t.Errorf("Failed to get pods: %v", err)
    // Continue to check other things
}

// Use t.Fatalf() when continuation is pointless
config := NewTestConfig()
if config == nil {
    t.Fatalf("NewTestConfig() returned nil - cannot continue")
    // No point checking anything else
}
```

### t.Skipf() for Prerequisites

Skip tests when prerequisites aren't met:

```go
func TestKindCluster_Deploy(t *testing.T) {
    if os.Getenv("CI") == "true" {
        t.Skip("Skipping in CI environment")
    }

    if !DirExists(config.RepoDir) {
        t.Skipf("Repository not cloned yet at %s", config.RepoDir)
    }

    // Continue with test...
}
```

### Descriptive Error Messages

Always include actual and expected values in error messages:

```go
// Good - includes context
if result != tt.expected {
    t.Errorf("ValidateDomainPrefix(%q, %q) = %v, expected %v",
        tt.user, tt.environment, result, tt.expected)
}

// Bad - missing context
if result != tt.expected {
    t.Error("validation failed")
}
```

## Test Organization

### Test File Naming

This repository follows a sequential phase numbering scheme:

```
test/
├── 01_check_dependencies_test.go   # Phase 1: Prerequisites
├── 02_setup_test.go                # Phase 2: Repository setup
├── 03_cluster_test.go              # Phase 3: Kind cluster
├── 04_generate_yamls_test.go       # Phase 4: YAML generation
├── 05_deploy_crs_test.go           # Phase 5: CR deployment
├── 06_verification_test.go         # Phase 6: Final verification
├── config.go                       # Configuration management
├── config_test.go                  # Config tests
├── helpers.go                      # Shared helpers
└── helpers_test.go                 # Helper tests
```

### Test Function Naming

Follow the pattern `TestPhase_Specific`:

```go
func TestCheckDependencies_ToolAvailable(t *testing.T) {}
func TestCheckDependencies_AzureAuthentication(t *testing.T) {}
func TestKindCluster_KindClusterReady(t *testing.T) {}
func TestKindCluster_CAPIControllerReady(t *testing.T) {}
```

### Standard Test Pattern

All test functions should follow this structure:

```go
func TestPhase_Specific(t *testing.T) {
    config := NewTestConfig()

    // Check prerequisites and skip if not met
    if !prerequisitesMet {
        t.Skipf("Prerequisites not met: reason")
    }

    // Perform test action
    result, err := SomeOperation()

    // Validate results
    if err != nil {
        t.Errorf("SomeOperation() failed: %v", err)
        return
    }

    // Log progress for visibility
    t.Logf("Operation completed: %v", result)
}
```

## Parallel vs Sequential Tests

### This Repository Uses Sequential Tests

Unlike typical Go tests that benefit from parallel execution, this test suite runs **sequentially by design**:

- Each phase depends on resources created by the previous phase
- Tests interact with external state (Kind cluster, Azure resources)
- Tests are designed for **workflow validation**, not unit testing

**Do NOT add `t.Parallel()`** to phase tests in this repository.

### When Parallel Testing Applies

If you're adding unit tests for helper functions that don't depend on external state, parallel execution can be beneficial:

```go
func TestHelperFunction(t *testing.T) {
    t.Parallel()  // Only for independent unit tests

    tests := []struct {
        name string
        // ...
    }{
        // test cases
    }

    for _, tt := range tests {
        tt := tt  // Capture range variable (not needed in Go 1.22+)
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()  // Subtests can also run in parallel
            // test logic
        })
    }
}
```

### Go 1.22+ Loop Variable Fix

Prior to Go 1.22, loop variables needed to be explicitly captured for parallel subtests:

```go
for _, tt := range tests {
    tt := tt  // Required before Go 1.22
    t.Run(tt.name, func(t *testing.T) {
        t.Parallel()
        // use tt safely
    })
}
```

Go 1.22 fixed this issue, making the capture unnecessary for new code targeting Go 1.22+.

## Best Practices Checklist

### Writing New Tests

- [ ] Use table-driven tests for multiple scenarios
- [ ] Include descriptive `name` field in test cases
- [ ] Use `t.Run()` for subtests
- [ ] Include actual vs expected values in error messages
- [ ] Use `t.Skipf()` when prerequisites aren't met
- [ ] Follow the `TestPhase_Specific` naming convention

### Writing Test Helpers

- [ ] Call `t.Helper()` at the start of every helper function
- [ ] Use `t.Cleanup()` for cleanup actions
- [ ] Add helper to `test/helpers.go` (don't duplicate)
- [ ] Add tests in `test/helpers_test.go`
- [ ] Consider using `testing.TB` for broader compatibility

### Configuration

- [ ] Use `config := NewTestConfig()` for configuration
- [ ] Use `GetEnvOrDefault()` for new config values - never hardcode
- [ ] Document new environment variables

### Error Handling

- [ ] Use `t.Errorf()` for non-fatal errors (test continues)
- [ ] Use `t.Fatalf()` for fatal errors (test stops)
- [ ] Never silently ignore errors

## References

These guidelines are based on proven testing practices from:

- [Go Wiki: Table-Driven Tests](https://go.dev/wiki/TableDrivenTests) - Official Go wiki on table-driven testing patterns
- [Go Testing Package Documentation](https://pkg.go.dev/testing) - Official testing package documentation
- [Dave Cheney: Prefer Table-Driven Tests](https://dave.cheney.net/2019/05/07/prefer-table-driven-tests) - Industry best practices for table-driven tests
- [Go Gopher Guides: Table-Driven Testing](https://www.gopherguides.com/articles/table-driven-testing-in-parallel) - Parallel execution patterns
- [JetBrains Go Testing Guide](https://blog.jetbrains.com/go/2022/11/22/comprehensive-guide-to-testing-in-go/) - Comprehensive testing guide
