Process all GitHub Copilot code review findings for a pull request. Analyze each finding, implement fixes or provide rationale for denial, reply to each comment, and automatically resolve review threads via GitHub GraphQL API.

## Instructions

1. Ask me for the PR number if not provided as argument (e.g., `/copilot-review 123`)

2. Extract repository information and fetch all review threads with GraphQL:
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

   a. **Extract finding details**:
      ```bash
      # For finding index $i (0-based)
      FINDING_DATA=$(echo "$COPILOT_THREADS" | jq ".[$i]")
      THREAD_ID=$(echo "$FINDING_DATA" | jq -r '.id')
      IS_RESOLVED=$(echo "$FINDING_DATA" | jq -r '.isResolved')
      COMMENT=$(echo "$FINDING_DATA" | jq '.comments.nodes[0]')
      COMMENT_BODY=$(echo "$COMMENT" | jq -r '.body')
      FILE_PATH=$(echo "$COMMENT" | jq -r '.path')
      LINE=$(echo "$COMMENT" | jq -r '.line')

      echo "Finding #$((i+1))/$TOTAL_FINDINGS"
      echo "Thread ID: $THREAD_ID"
      echo "File: $FILE_PATH:$LINE"

      # Skip if already resolved
      if [ "$IS_RESOLVED" = "true" ]; then
        echo "ℹ️ Thread already resolved, skipping"
        continue
      fi
      ```

   b. **Analyze the finding**:
      - Read the code context at $FILE_PATH:$LINE
      - Understand the suggestion in $COMMENT_BODY
      - Evaluate if it aligns with repo patterns (CLAUDE.md)
      - Check if it improves code quality, security, or maintainability

   c. **Make a decision**:

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
      - Resolve thread via GraphQL:
        ```bash
        echo "Resolving thread $THREAD_ID..."
        RESOLVE_RESULT=$(gh api graphql -f query='
          mutation($threadId: ID!) {
            resolveReviewThread(input: {threadId: $threadId}) {
              thread {
                id
                isResolved
              }
            }
          }
        ' -F threadId="$THREAD_ID" 2>&1)

        # Verify success
        if echo "$RESOLVE_RESULT" | jq -e '.data.resolveReviewThread.thread.isResolved == true' > /dev/null 2>&1; then
          echo "✅ Thread $THREAD_ID resolved successfully"
        else
          echo "⚠️ Warning: Failed to resolve thread $THREAD_ID"
          echo "Error: $RESOLVE_RESULT"
          echo "You can resolve manually in GitHub UI if needed"
        fi

        # Small delay to avoid rate limiting
        sleep 0.5
        ```

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
      - Resolve thread via GraphQL (same as ACCEPT):
        ```bash
        echo "Resolving thread $THREAD_ID..."
        RESOLVE_RESULT=$(gh api graphql -f query='
          mutation($threadId: ID!) {
            resolveReviewThread(input: {threadId: $threadId}) {
              thread {
                id
                isResolved
              }
            }
          }
        ' -F threadId="$THREAD_ID" 2>&1)

        # Verify success
        if echo "$RESOLVE_RESULT" | jq -e '.data.resolveReviewThread.thread.isResolved == true' > /dev/null 2>&1; then
          echo "✅ Thread $THREAD_ID resolved successfully"
        else
          echo "⚠️ Warning: Failed to resolve thread $THREAD_ID"
          echo "Error: $RESOLVE_RESULT"
          echo "You can resolve manually in GitHub UI if needed"
        fi

        # Small delay to avoid rate limiting
        sleep 0.5
        ```

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
- **Thread ID**: `PRRT_...`
- **Copilot Suggestion**: [Summary of what Copilot suggested]
- **Decision**: ✅ ACCEPTED / ❌ DENIED
- **Action**: [What was implemented OR why it was denied]
- **Reply Posted**: ✅ Yes
- **Thread Resolved**: ✅ Yes / ⚠️ Failed (manual resolution needed)

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
