Process all GitHub Copilot code review findings for a pull request. Analyze each finding, implement fixes or provide rationale for denial, reply to each comment, and automatically resolve review threads via GitHub GraphQL API.

## Execution Mode (Autonomous Configuration)

This command runs in **autonomous mode** with the following behavior:

| Category | Behavior |
|----------|----------|
| 1. Repository Read Operations | ✅ Show progress ("Reading PR #189... Found 7 findings"), auto-execute |
| 2. Code Analysis & Decisions | ✅ Silent processing, show results at end only |
| 3. Code Modifications | ✅ Auto-approve and implement, no confirmation needed |
| 4. Git Operations (commit + push) | ✅ Auto-approve, silent |
| 5. GitHub API (comments + threads) | ✅ Auto-approve, silent |
| 6. External Tools (gh, jq, bash) | ✅ Auto-approve, silent |

**Output Format**: Minimal - show only:
- Initial: Repository and PR information
- Progress: Fetching data, found X findings
- Final: Summary of actions taken

**Comment Behavior**:
- Individual finding responses → Posted as **threaded replies** to each Copilot comment
- Final summary → Posted as **general PR comment** in main thread

**No interaction required** - command runs completely autonomously and reports results at the end.

## Instructions

1. Ask me for the PR number if not provided as argument (e.g., `/copilot-review 123`)

2. Use the pre-approved `.claude/scripts/copilot-review.sh` script to fetch review threads:
   ```bash
   .claude/scripts/copilot-review.sh {pr_number}
   ```
   This script:
   - Fetches all review threads via GraphQL
   - Filters for Copilot/GitHub Advanced Security comments
   - Saves results to `/tmp/copilot_threads_{pr_number}.json`
   - Outputs repository info and finding count

3. Load the fetched data:
   ```bash
   cat /tmp/copilot_threads_{pr_number}.json
   ```

4. Extract repository information and fetch all review threads with GraphQL (LEGACY - use script instead):
   ```bash
   # Get repository owner and name
   REPO_INFO=$(gh repo view --json owner,name)
   OWNER=$(echo "$REPO_INFO" | jq -r '.owner.login')
   REPO=$(echo "$REPO_INFO" | jq -r '.name')
   PR_NUMBER={pr_number}

   echo "Repository: $OWNER/$REPO"
   echo "Pull Request: #$PR_NUMBER"

   # Fetch all review threads including thread IDs
   THREADS_JSON=$(gh api graphql -f query='
     query($owner: String!, $repo: String!, $pr: Int!) {
       repository(owner: $owner, name: $repo) {
         pullRequest(number: $pr) {
           reviewThreads(first: 100) {
             nodes {
               id
               isResolved
               comments(first: 50) {
                 nodes {
                   id
                   databaseId
                   body
                   author { login }
                   path
                   line
                 }
                 pageInfo {
                   hasNextPage
                   endCursor
                 }
               }
             }
             pageInfo {
               hasNextPage
               endCursor
             }
           }
         }
       }
     }
   ' -F owner="$OWNER" -F repo="$REPO" -F pr="$PR_NUMBER")

   # Warn if pagination limits are exceeded for review threads
   THREADS_HAS_NEXT=$(echo "$THREADS_JSON" | jq -r '.data.repository.pullRequest.reviewThreads.pageInfo.hasNextPage')
   if [ "$THREADS_HAS_NEXT" = "true" ]; then
     echo "WARNING: More than 100 review threads exist. Only the first 100 were fetched. Some Copilot findings may be missing."
   fi

   # Warn if pagination limits are exceeded for comments in any thread
   COMMENTS_OVER_LIMIT=$(echo "$THREADS_JSON" | jq '[.data.repository.pullRequest.reviewThreads.nodes[] | select(.comments.pageInfo.hasNextPage == true)] | length')
   if [ "$COMMENTS_OVER_LIMIT" -gt 0 ]; then
     echo "WARNING: One or more review threads have more than 50 comments. Only the first 50 comments per thread were fetched. Some Copilot findings may be missing."
   fi

   # Filter for Copilot comments only
   COPILOT_THREADS=$(echo "$THREADS_JSON" | jq '[
     .data.repository.pullRequest.reviewThreads.nodes[] |
     select(.comments.nodes | length > 0) |
     select((.comments.nodes[0].author.login // "") | test("copilot|Copilot"; "i"))
   ]')

   TOTAL_FINDINGS=$(echo "$COPILOT_THREADS" | jq 'length')
   echo "Found $TOTAL_FINDINGS Copilot review findings"
   ```

3. For EACH finding (iterate through COPILOT_THREADS), perform this workflow:

   a. **Extract finding details** (silent processing):
      ```bash
      # For finding index $i (0-based)
      FINDING_DATA=$(echo "$COPILOT_THREADS" | jq ".[$i]")
      THREAD_ID=$(echo "$FINDING_DATA" | jq -r '.id')
      IS_RESOLVED=$(echo "$FINDING_DATA" | jq -r '.isResolved')
      COMMENT=$(echo "$FINDING_DATA" | jq '.comments.nodes[0]')
      COMMENT_ID=$(echo "$COMMENT" | jq -r '.databaseId')
      COMMENT_BODY=$(echo "$COMMENT" | jq -r '.body')
      FILE_PATH=$(echo "$COMMENT" | jq -r '.path')
      LINE=$(echo "$COMMENT" | jq -r '.line')

      # Skip if already resolved (no output)
      if [ "$IS_RESOLVED" = "true" ]; then
        continue
      fi
      ```

   b. **Analyze the finding** (silent processing):
      - Read the code context at $FILE_PATH:$LINE
      - Understand the suggestion in $COMMENT_BODY
      - Evaluate if it aligns with repo patterns (CLAUDE.md)
      - Check if it improves code quality, security, or maintainability
      - **No output during analysis**

   c. **Make a decision**:

      **Option 1: ACCEPT** (silent execution)
      - Implement the suggested fix
      - Ensure fix follows repo patterns
      - Test that code still works (if applicable)
      - Post threaded reply directly to the Copilot finding (silent):
        ```bash
        # Reply directly in the comment thread
        gh api -X POST repos/$OWNER/$REPO/pulls/comments/$COMMENT_ID/replies \
          -f body="✅ Implemented.

[Detailed description of what was changed and why]

Changes:
- [Specific change 1]
- [Specific change 2]" > /dev/null 2>&1
        ```
      - Resolve thread via GraphQL (silent):
        ```bash
        gh api graphql -f query='
          mutation($threadId: ID!) {
            resolveReviewThread(input: {threadId: $threadId}) {
              thread {
                id
                isResolved
              }
            }
          }
        ' -F threadId="$THREAD_ID" > /dev/null 2>&1

        # Small delay to avoid rate limiting
        sleep 0.5
        ```

      **Option 2: DENY** (silent execution)
      - Provide clear rationale (e.g., "This conflicts with our sequential test pattern", "This would break idempotency", etc.)
      - Post threaded reply directly to the Copilot finding (silent):
        ```bash
        # Reply directly in the comment thread
        gh api -X POST repos/$OWNER/$REPO/pulls/comments/$COMMENT_ID/replies \
          -f body="❌ Not implementing.

**Rationale**: [Detailed explanation]

[Additional context about why this doesn't fit the project]" > /dev/null 2>&1
        ```
      - Resolve thread via GraphQL (silent):
        ```bash
        gh api graphql -f query='
          mutation($threadId: ID!) {
            resolveReviewThread(input: {threadId: $threadId}) {
              thread {
                id
                isResolved
              }
            }
          }
        ' -F threadId="$THREAD_ID" > /dev/null 2>&1

        # Small delay to avoid rate limiting
        sleep 0.5
        ```

4. After processing all findings (silent git operations):
   - If any implementations were made, commit changes (silent):
     ```bash
     git add . > /dev/null 2>&1
     git commit -m "Address GitHub Copilot code review findings for PR #XXX

     - [List major changes]

     Generated with Claude Code
     Co-Authored-By: Claude <noreply@anthropic.com>" > /dev/null 2>&1
     git push > /dev/null 2>&1
     ```
   - Post final summary as a general PR comment:
     ```bash
     gh pr review {pr_number} --comment --body "✅ Copilot Review Complete

Summary:
- Total findings: X
- Accepted: Y
- Denied: Z
- Files modified: [list files with +/- lines]
- Committed: [commit hash]
- Posted X threaded replies to findings
- Resolved X threads

See individual findings above for implementation details."
     ```
   - Display summary in console output (see Summary Report section)

## Important Guidelines

- **Security findings**: ALWAYS implement security fixes unless there's a very strong reason not to
- **Pattern compliance**: Prioritize fixes that align with CLAUDE.md patterns
- **Test impact**: Consider if changes affect test behavior or idempotency
- **Be thorough**: Every finding gets a response and resolution
- **Be respectful**: Provide clear rationale for denials

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

### Automatic Thread Resolution

This command automatically resolves GitHub review threads after posting replies using the GraphQL `resolveReviewThread` mutation.

**How it works:**

1. **Fetch threads upfront** (step 2):
   - Single GraphQL query retrieves all review threads for the PR
   - Each thread has a unique ID (format: `PRRT_kwDOQY85bs5kMu6k`)
   - Threads contain comments, including Copilot's original finding
   - Filter for threads where first comment author is "copilot" or "Copilot"

2. **Process each finding** (step 3):
   - Extract thread ID from thread data
   - Check if already resolved (skip if true)
   - Post reply comment
   - Call `resolveReviewThread` GraphQL mutation
   - Verify resolution succeeded
   - Log any errors

3. **GraphQL mutation**:
   ```bash
   gh api graphql -f query='
     mutation($threadId: ID!) {
       resolveReviewThread(input: {threadId: $threadId}) {
         thread {
           id
           isResolved
         }
       }
     }
   ' -F threadId="$THREAD_ID"
   ```

**Key concepts:**
- **Thread**: A conversation containing multiple comments
- **Thread ID**: GraphQL node ID (format: `PRRT_...`)
- **Resolution**: Marks the entire conversation as resolved in GitHub UI
- **Graceful degradation**: Replies still post even if resolution fails

**Error handling:**
- Already resolved threads: Automatically skipped
- Resolution failures: Logged with warning, doesn't stop processing
- Manual fallback: Users can resolve in GitHub UI if needed

**Permissions required:**
- Repository > Contents: Read and Write, OR
- Repository > Pull Requests: Read and Write

**References:**
- [GitHub GraphQL API - Mutations](https://docs.github.com/en/graphql/reference/mutations)
- [Resolve PR conversations](https://stackoverflow.com/questions/71421045/)

## Summary Report (Minimal Output Format)

At the end of execution, display this minimal summary in console AND post as general PR comment:

**Console Output**:
```
✅ Copilot Review Complete

Summary:
- Total findings: X
- Accepted: Y
- Denied: Z
- Files modified: [list files with +/- lines]
- Committed: [commit hash]
- Posted X threaded replies to findings
- Resolved X threads

See PR #XXX for details.
```

**Posted as PR Comment** (same content):
- Allows reviewers to see summary in GitHub UI
- Posted in main comment thread (not threaded)
- Individual finding responses remain threaded under each Copilot comment

**Example**:
```
✅ Copilot Review Complete

Summary:
- Total findings: 7
- Accepted: 6
- Denied: 1
- Files modified: test/helpers.go (+56, -20)
- Committed: aad86a9
- Posted 7 review comments
- Resolved 7 threads

See PR #189 for details.
```
