---
description: Analyze a GitHub issue and create a pull request that implements the fix
---

# Implement Issue

Automatically analyze a GitHub issue, implement the required changes, and create a pull request with the fix.

## Usage

```
/implement-issue <issue-number>
```

**Example**: `/implement-issue 72`

## Workflow

1. **Validate issue number argument**
   - If no issue number provided, prompt user: "Please provide an issue number: /implement-issue <number>"
   - If issue number is not a valid integer, show error and exit

2. **Fetch issue details from GitHub**
   ```bash
   gh issue view <issue-number>
   ```
   - If issue doesn't exist, show error and exit
   - If issue is closed, ask user if they still want to proceed
   - Display issue title, description, and labels for context

3. **Analyze the issue**
   - Read the issue description carefully
   - Identify what type of change is needed:
     - Bug fix
     - New feature
     - Test addition
     - Documentation update
     - Workflow/CI fix
     - Refactoring
   - Determine affected files by:
     - Reading issue description for file/path mentions
     - Searching codebase for relevant code patterns
     - Using Grep/Glob tools to find related files
   - Check CLAUDE.md for repository-specific patterns and guidelines
   - Create a mental implementation plan

4. **Check current git status**
   ```bash
   git status
   ```
   - If there are uncommitted changes, ask user:
     - "You have uncommitted changes. What would you like to do?"
       - Option 1: Stash changes and continue
       - Option 2: Commit changes first
       - Option 3: Cancel operation
   - Handle user's choice before proceeding

5. **Ensure main branch is up to date**
   ```bash
   git checkout main
   git pull origin main
   ```
   - If pull fails, explain error and exit

6. **Create feature branch**
   - Generate branch name from issue:
     - Format: `fix-issue-<number>-<brief-description>`
     - Example: `fix-issue-72-add-logging-function`
     - Keep description under 50 chars, use kebab-case
   - Create and checkout branch:
     ```bash
     git checkout -b <branch-name>
     ```

7. **Use TodoWrite tool to create implementation plan**
   - Break down the implementation into specific tasks
   - Examples:
     - "Read current implementation of X"
     - "Create new function Y in file Z"
     - "Add tests for feature X"
     - "Update documentation"
     - "Run tests to verify changes"
     - "Commit changes"
     - "Create pull request"
   - Mark first task as in_progress

8. **Implement the fix**
   - Follow repository patterns from CLAUDE.md
   - Read existing code before making changes
   - Implement changes step-by-step, updating TodoWrite as you progress
   - For code changes:
     - Use Read tool to understand existing code
     - Use Edit/Write tools to make changes
     - Follow existing code style and patterns
     - Add comments where logic isn't self-evident
   - For test changes:
     - Follow existing test patterns (see helpers_test.go)
     - Add comprehensive test coverage
     - Ensure tests are idempotent and can run in sequence

9. **Run relevant tests**
   - Determine which tests to run based on changes:
     - If config changed: `go test -v ./test -run TestConfig`
     - If helpers changed: `go test -v ./test -run TestHelpers`
     - For general changes: `make test`
   - If tests fail:
     - Analyze failure
     - Fix implementation
     - Re-run tests
     - Repeat until tests pass

10. **Format code**
    ```bash
    go fmt ./...
    ```

11. **Commit changes**
    - Create descriptive commit message following this format:
      ```
      <Brief summary> (fixes #<issue-number>)

      <Detailed description of what changed and why>

      **Changes**:
      - <Change 1>
      - <Change 2>

      **Testing**:
      - <Test 1 passed>
      - <Test 2 passed>

      Fixes #<issue-number>

      🤖 Generated with [Claude Code](https://claude.com/claude-code)

      Co-Authored-By: Claude <noreply@anthropic.com>
      ```
    - Commit using:
      ```bash
      git add .
      git commit -m "$(cat <<'EOF'
      <commit message here>
      EOF
      )"
      ```

12. **Push branch to remote**
    ```bash
    git push -u origin <branch-name>
    ```

13. **Create pull request**
    - Use `gh pr create` with detailed PR description
    - PR title: `<Brief summary> (fixes #<issue-number>)`
    - PR body should include:
      - ## Summary
      - ## Problem (reference original issue)
      - ## Solution
      - ## Changes
      - ## Testing
      - Fixes #<issue-number>
      - 🤖 Generated with Claude Code
    - Example:
      ```bash
      gh pr create --title "Add logging function (fixes #72)" --body "$(cat <<'EOF'
      ## Summary
      Implements logging function as requested in #72

      ## Problem
      <Describe the problem from the issue>

      ## Solution
      <Describe how you fixed it>

      ## Changes
      - <Change 1>
      - <Change 2>

      ## Testing
      - [x] Tests pass
      - [x] Code formatted

      Fixes #72

      🤖 Generated with [Claude Code](https://claude.com/claude-code)
      EOF
      )"
      ```

14. **Provide summary to user**
    - Display PR URL
    - List files changed
    - Show test results
    - Remind user that CI will run automatically

## Important Guidelines

### Code Quality
- **Read before writing**: Always use Read tool to understand existing code before making changes
- **Follow patterns**: Adhere to CLAUDE.md guidelines and existing code patterns
- **Test coverage**: Add tests for new functionality
- **No over-engineering**: Only implement what's requested in the issue
- **Security**: Check for common vulnerabilities (SQL injection, XSS, command injection, etc.)

### Repository-Specific Patterns
- **Configuration**: Use `config := NewTestConfig()` pattern
- **Helpers**: Reuse functions from `helpers.go`
- **Tests**: Follow sequential test pattern with proper prerequisite validation
- **Error handling**: Use `t.Errorf()` for non-fatal, `t.Fatalf()` for fatal errors
- **Environment**: Use `GetEnvOrDefault()` for config values

### Git Best Practices
- **Branch naming**: `fix-issue-<number>-<brief-description>` or `feature-issue-<number>-<brief-description>`
- **Commit messages**: Descriptive, reference issue number
- **One issue per PR**: Don't mix multiple unrelated changes

### Testing Requirements
- Run tests before committing
- Ensure all existing tests still pass
- Add new tests for new functionality
- Tests must be idempotent and sequential-safe

## Error Handling

### Issue Not Found
```
Error: Issue #<number> not found
Please check the issue number and try again
```

### Tests Fail
- Show test output
- Ask user: "Tests are failing. Would you like to:"
  - Option 1: Let me fix the issue
  - Option 2: Skip tests and commit anyway (not recommended)
  - Option 3: Cancel operation

### Git Conflicts
- Explain conflict
- Provide resolution commands
- Ask user how to proceed

### Uncommitted Changes
- Detect uncommitted changes before starting
- Offer to stash, commit, or cancel
- Never proceed without handling changes

## Examples

### Example 1: Simple bug fix

```
User: /implement-issue 72
Claude: Fetching issue #72...

Issue #72: Fix typo in README.md
Description: The word "teh" should be "the" in line 45 of README.md

Analyzing issue... This is a documentation fix.
Creating branch fix-issue-72-readme-typo...

[Implements fix]

Tests passed ✅
Committed changes ✅
Pushed to remote ✅
Created PR #73 ✅

Pull Request: https://github.com/RadekCap/CAPZTests/pull/73
```

### Example 2: New feature with tests

```
User: /implement-issue 75
Claude: Fetching issue #75...

Issue #75: Add helper function to validate cluster names
Description: Create a ValidateClusterName() function that checks if a cluster name follows Kubernetes naming conventions...

Analyzing issue... This requires:
- New helper function in helpers.go
- Tests in helpers_test.go
- Update to CLAUDE.md documentation

Creating branch feature-issue-75-validate-cluster-name...

[Creates implementation plan with TodoWrite]
[Implements helper function]
[Adds comprehensive tests]
[Updates documentation]

Tests passed ✅ (added 12 new test cases)
Committed changes ✅
Pushed to remote ✅
Created PR #76 ✅

Files changed:
- test/helpers.go (+25 lines)
- test/helpers_test.go (+150 lines)
- CLAUDE.md (+10 lines)

Pull Request: https://github.com/RadekCap/CAPZTests/pull/76
```

### Example 3: CI/Workflow fix

```
User: /implement-issue 77
Claude: Fetching issue #77...

Issue #77: Setup workflow failing on Mac runners
Description: The test-setup workflow fails on macOS runners due to...

Analyzing issue... This is a CI/workflow fix affecting .github/workflows/test-setup.yml

Creating branch fix-issue-77-macos-workflow...

[Analyzes workflow file]
[Identifies incompatible commands]
[Implements fix for macOS compatibility]

Tests cannot be run locally (requires CI environment)
Committed changes ✅
Pushed to remote ✅
Created PR #78 ✅

Note: CI will validate this fix when the PR runs

Pull Request: https://github.com/RadekCap/CAPZTests/pull/78
```

## Post-Implementation Checklist

After completing implementation, verify:
- [ ] Issue requirements fully addressed
- [ ] Code follows repository patterns (CLAUDE.md)
- [ ] Tests added for new functionality
- [ ] All tests pass (or explanation why they can't run locally)
- [ ] Code formatted (`go fmt ./...`)
- [ ] Commit message references issue number
- [ ] PR description includes "Fixes #<issue-number>"
- [ ] PR description is comprehensive and clear
- [ ] Branch pushed to remote
- [ ] PR created successfully

## Tips for Success

1. **Read the issue carefully**: Understand exactly what's being asked before starting
2. **Check for related issues**: Look for similar issues or PRs that might provide context
3. **Follow existing patterns**: Consistency is key in this codebase
4. **Test thoroughly**: Don't skip tests, they catch problems early
5. **Ask for clarification**: If issue is ambiguous, use AskUserQuestion to clarify with user
6. **Keep it focused**: Only implement what the issue requests, nothing more
7. **Document as you go**: Update CLAUDE.md or other docs if your changes affect usage

## Advanced Usage

### Complex Issues
For complex issues that require multiple steps or architectural decisions:
1. Use AskUserQuestion to clarify approach
2. Break down into smaller sub-tasks in TodoWrite
3. Implement incrementally, testing each step
4. Consider using EnterPlanMode for very complex changes

### Multiple Files
When changes span multiple files:
1. Read all affected files first
2. Make changes in logical order (dependencies first)
3. Test after each significant change
4. Commit atomically (all related changes together)

### Breaking Changes
If implementation requires breaking changes:
1. Flag this to the user immediately
2. Ask for confirmation before proceeding
3. Document breaking changes clearly in PR
4. Consider adding migration guide
