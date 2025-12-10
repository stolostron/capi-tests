#!/bin/bash
# Autonomous GitHub Copilot code review processor for pull requests
# This script is auto-approved in .claude/settings.local.json for autonomous execution
set -euo pipefail

PR_NUMBER="$1"

# Get repository information
REPO_INFO=$(gh repo view --json owner,name)
OWNER=$(echo "$REPO_INFO" | jq -r '.owner.login')
REPO=$(echo "$REPO_INFO" | jq -r '.name')

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
  select((.comments.nodes[0].author.login // "") | test("copilot|github-advanced-security"; "i"))
]')

TOTAL_FINDINGS=$(echo "$COPILOT_THREADS" | jq 'length')
echo "Found $TOTAL_FINDINGS Copilot review findings"

# Save for processing by Claude
echo "$COPILOT_THREADS" > "/tmp/copilot_threads_${PR_NUMBER}.json"
echo "$TOTAL_FINDINGS" > "/tmp/copilot_total_findings_${PR_NUMBER}.txt"

exit 0
