Process all GitHub Copilot code review findings for a pull request. Analyze each finding, implement fixes or provide rationale for denial, reply to each comment, and mark as resolved.

## Instructions

1. Ask me for the PR number if not provided as argument (e.g., `/copilot-review 123`)

2. Fetch all GitHub Copilot code review comments:
   ```bash
   gh api repos/{owner}/{repo}/pulls/{pr_number}/comments
   ```

3. For EACH finding, perform this workflow:

   a. **Analyze the finding**:
      - Read the code context
      - Understand the suggestion
      - Evaluate if it aligns with repo patterns (CLAUDE.md)
      - Check if it improves code quality, security, or maintainability

   b. **Make a decision**:

      **Option 1: ACCEPT**
      - Implement the suggested fix
      - Ensure fix follows repo patterns
      - Test that code still works (if applicable)
      - Post individual reply to the specific comment using GitHub CLI:
        ```bash
        gh pr review {pr_number} --comment --body "$(cat <<'EOF'
✅ Implemented.

[Detailed description of what was changed and why]

Changes:
- [Specific change 1]
- [Specific change 2]
EOF
)"
        ```
      - Mark comment as resolved (if supported by GitHub CLI or done manually in UI)

      **Option 2: DENY**
      - Provide clear rationale (e.g., "This conflicts with our sequential test pattern", "This would break idempotency", etc.)
      - Post individual reply to the specific comment:
        ```bash
        gh pr review {pr_number} --comment --body "$(cat <<'EOF'
❌ Not implementing.

**Rationale**: [Detailed explanation]

[Additional context about why this doesn't fit the project]
EOF
)"
        ```
      - Mark comment as resolved

4. After processing all findings:
   - If any implementations were made, commit changes:
     ```
     git add .
     git commit -m "Address GitHub Copilot code review findings for PR #XXX

     - [List major changes]

     Generated with Claude Code
     Co-Authored-By: Claude <noreply@anthropic.com>"
     ```
   - Provide summary of actions taken

## Important Guidelines

- **Security findings**: ALWAYS implement security fixes unless there's a very strong reason not to
- **Pattern compliance**: Prioritize fixes that align with CLAUDE.md patterns
- **Test impact**: Consider if changes affect test behavior or idempotency
- **Be thorough**: Every finding gets a response and resolution
- **Be respectful**: Provide clear rationale for denials

## Response Format for Each Finding

**Finding #N: [Brief description]**
- **Location**: `file.go:line`
- **Copilot Suggestion**: [Summary of what Copilot suggested]
- **Decision**: ✅ ACCEPTED / ❌ DENIED
- **Action**: [What was implemented OR why it was denied]
- **Reply Posted**: Yes (via `gh pr review --comment`)
- **Status**: ✅ Resolved (or pending manual resolution in UI)

### Template for Individual Replies

**For Accepted Findings:**
```
✅ **Implemented** - Finding #{N}

**Location**: {file}:{line}

**Change Made**: {description of implementation}

**Details**:
- {specific change 1}
- {specific change 2}

{Any additional context or testing notes}
```

**For Denied Findings:**
```
❌ **Not Implementing** - Finding #{N}

**Location**: {file}:{line}

**Rationale**: {Clear explanation of why this doesn't fit}

**Reasoning**:
- {specific reason 1}
- {specific reason 2}

{Any additional context}
```

## Using GitHub CLI

### Reply to Individual Comments

For each finding, post a separate review comment:
```bash
gh pr review {pr_number} --comment --body "Response to Finding #N..."
```

This creates a general PR review comment. For more context, you can reference the specific file and line in your comment body.

### Alternative: Thread Replies (if needed)

To reply directly in a comment thread (creates a threaded reply):
```bash
gh api -X POST repos/{owner}/{repo}/pulls/comments/{comment_id}/replies \
  -f body="Your reply here"
```

Note: Thread replies may not always work when replying to review comments (as opposed to issue comments) due to GitHub API limitations. Use `gh pr review --comment` as the primary method.

### Mark as Resolved

There is no direct built-in `gh` CLI command to mark conversations as resolved, but you can resolve review threads using the GitHub GraphQL API via `gh api`. Options:

1. **Use GraphQL API to resolve threads**:
   ```bash
   gh api graphql -f query='
     mutation {
       resolveReviewThread(input: {threadId: "<thread_id>"}) {
         thread {
           isResolved
         }
       }
     }
   '
   ```
   Replace `<thread_id>` with the actual thread ID (you can fetch thread IDs using the GraphQL API).

2. **Note in your reply** that the finding is addressed (✅ or ❌ emoji helps)

3. **Manually resolve** in GitHub UI after posting replies

4. **Copilot may auto-resolve** when it detects implementation

## Summary Report

At the end, provide:

**GitHub Copilot Review Summary for PR #XXX**

- **Total Findings**: X
- **Accepted**: Y
- **Denied**: Z
- **Commits Made**: [Yes/No]

**Key Changes**:
- [List significant implementations]

**Denied Items**:
- [List denials with brief rationale]
