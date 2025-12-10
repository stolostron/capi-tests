Create a new test phase file for the ARO-CAPZ test suite following the established patterns.

## Instructions

1. Ask me for:
   - The phase number (e.g., 04, 05)
   - A descriptive name for the phase (e.g., "infrastructure", "deployment")
   - A brief description of what this test phase will validate

2. Create the test file `test/XX_<name>_test.go` with:
   - Standard package declaration and imports (excluding testing.Short checks)
   - Use of helper functions from `test/helpers.go` (CommandExists, RunCommand, FileExists, etc.)
   - Clear logging with `t.Logf()` for progress
   - Prerequisite checks with `t.Skipf()` when conditions aren't met
   - Make tests idempotent

3. Add a Makefile target:
   - Pattern: `test-<name>: ## Run <name> tests only`
   - Use `go test -v ./test -run Test<PhaseName> -timeout 30m`
   - Add to the `test-all` target in proper sequence

4. Suggest documentation updates:
   - README.md (if high-level workflow changes)
   - CLAUDE.md (add to test architecture section)
   - test/README.md (detailed test documentation)

## Test File Template

Follow this structure:

```go
package test

import (
    "testing"
)

// Test<Phase>_<Functionality> describes what this test validates
func Test<Phase>_<Functionality>(t *testing.T) {

    config := NewTestConfig()

    // Check prerequisites
    if !CommandExists("required-tool") {
        t.Skipf("Required tool not found")
    }

    // Test logic here
    t.Logf("Starting <phase> validation...")

    // Use helpers
    output, err := RunCommand(t, "command", "args")
    if err != nil {
        t.Fatalf("Critical failure: %v", err)
    }

    t.Logf("Success: %s", output)
}
```

## Important Patterns

- All tests MUST be idempotent (safe to re-run)
- Each phase should check if previous phases completed successfully
- Use `GetEnvOrDefault()` for any new configuration needs
- Never hardcode values - use config or environment variables
- Follow sequential execution model - tests depend on previous phases
