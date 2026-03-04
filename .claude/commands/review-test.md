Review test files in this repository for compliance with CAPI test suite patterns and guidelines.

## What to Review

Ask me which test file(s) to review, or review all test files if I don't specify.

## Review Checklist

For each test file, check and report on:

### 1. Configuration Management
- ✅ Uses `config := NewTestConfig()` instead of direct env var access
- ✅ No hardcoded values (cluster names, regions, paths, etc.)
- ✅ New config needs use `GetEnvOrDefault()` pattern
- ❌ Report any hardcoded values with file:line reference

### 2. Helper Function Usage
- ✅ Uses `CommandExists()` to check for required tools
- ✅ Uses `RunCommand()` for shell command execution
- ✅ Uses `FileExists()` / `DirExists()` for path validation
- ✅ Uses `SetEnvVar()` for temporary environment changes
- ❌ Report any direct `os.Stat()`, `exec.Command()`, or `os.Setenv()` calls

### 3. Test Structure
- ✅ Function names follow `Test<Phase>_<Functionality>` pattern
- ✅ Has prerequisite validation with `t.Skipf()`
- ✅ Uses `t.Run()` for subtests when appropriate
- ❌ Report any missing prerequisite validations

### 4. Error Handling
- ✅ Uses `t.Errorf()` for non-fatal errors (can continue)
- ✅ Uses `t.Fatalf()` for fatal errors (cannot continue)
- ✅ Uses `t.Logf()` for progress/informational messages
- ❌ Report improper error handling (e.g., using Fatal when Error would suffice)

### 5. Sequential Dependencies
- ✅ Tests check if previous phases completed
- ✅ Tests are idempotent (safe to re-run)
- ✅ Tests skip gracefully if prerequisites not met
- ❌ Report any tests that would break when re-run

### 6. Security Issues
- ❌ Check for command injection vulnerabilities
- ❌ Check for hardcoded secrets or credentials
- ❌ Check for unsafe file operations
- ❌ Flag the known issue in `06_verification_test.go:68` (base64 decode vulnerability)

### 7. CLAUDE.md Compliance
- ✅ Follows patterns documented in CLAUDE.md
- ✅ Aligns with test architecture (sequential, idempotent)
- ✅ Uses proper logging and error handling
- ❌ Report any deviations from documented patterns

## Output Format

Provide feedback as:

**File: `test/filename_test.go`**

✅ **Strengths:**
- Lists what's done correctly

❌ **Issues Found:**
- Issue description with `test/filename_test.go:123` reference
- Suggested fix aligned with CLAUDE.md guidelines

**Recommendations:**
- Specific improvements or refactorings

## Priority Levels

- 🔴 **Critical**: Security issues, breaking patterns, will cause failures
- 🟡 **Important**: Deviates from patterns, not following best practices
- 🟢 **Minor**: Style improvements, consistency tweaks
