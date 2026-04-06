---
description: Review all open PRs — analyze, present findings with fixes, post approved ones as GitHub suggestion comments
---

# Review All Open Pull Requests

Review all open PRs in the repository. For each PR: analyze the code, generate findings with fixes,
present them to the user for approval, and post approved findings as inline GitHub review comments
with suggestion blocks.

## Usage

```
/review-open-prs
```

No arguments needed. Reviews all open non-draft PRs in the current repository (up to 10 most recently updated).

## Step 0: Determine repository context

```bash
REPO_INFO=$(gh repo view --json owner,name)
OWNER=$(echo "$REPO_INFO" | jq -r '.owner.login')
REPO=$(echo "$REPO_INFO" | jq -r '.name')
```

Use `$OWNER/$REPO` in all subsequent `gh` and `gh api` commands.

## Phase 1: Discover and Summarize

### Step 1: Fetch all open PRs

```bash
gh pr list --repo $OWNER/$REPO --state open \
  --json number,title,author,createdAt,updatedAt,isDraft,headRefName,baseRefName,mergeable,reviewDecision,statusCheckRollup,labels,additions,deletions,changedFiles \
  --jq 'sort_by(.updatedAt) | reverse | .[:10]'
```

This fetches the 10 most recently updated PRs. If fewer than 10 are open, all are returned.

If there are no open PRs, print "No open pull requests found." and stop.

### Step 2: Print compact status table

Print a short overview table (this is the only "report" output — keep it brief):

```markdown
# Open PRs — <date>

| # | Title | CI | Mergeable | Size |
|---|-------|----|-----------|------|
| <number> | <title> | pass/fail | yes/no | +N/-N |

Skipped: #<numbers> (draft), #<numbers> (auto/* branch), #<numbers> (self-authored)
```

## Phase 2: Analyze Each PR

For each **non-draft** open PR (skip draft PRs, skip branches starting with `auto/`, skip PRs authored by the current user):

### Step 3: Fetch diff and existing reviews

```bash
# Get the diff
gh pr diff <number> --repo $OWNER/$REPO

# Check existing review comments to avoid duplicates
gh api repos/$OWNER/$REPO/pulls/<number>/comments \
  --jq '[.[] | {path: .path, line: .line, body: .body}]'
```

### Step 4: Run code review

Use the Agent tool to launch the `pr-review-toolkit:code-reviewer` agent (with `subagent_type: "pr-review-toolkit:code-reviewer"`).

Pass the PR number, full diff, and repository context. Instruct the agent to return findings in this structured format:

For each finding, collect:
- **file**: exact file path (relative to repo root)
- **line**: the line number in the **new version** of the file where the comment should appear
- **severity**: critical / important / minor
- **reasoning**: 2-4 sentences explaining the problem
- **start_line** (optional): if the suggestion spans multiple lines, the starting line number

The agent identifies problems but does **not** generate fixes — that happens in Step 5.

### Step 5: Generate fix for each finding

For each finding from Step 4:

1. Read the relevant file section to understand full context
2. Write the concrete fix (the exact replacement code for the GitHub suggestion block)
3. Ensure the fix is minimal and correct — change only what's needed
4. If a finding cannot be fixed mechanically (needs design decisions), mark it as `manual_only: true`

## Phase 3: User Approval

### Step 6: Present findings one by one

For each PR, present its findings to the user sequentially. For each finding, print:

```
────────────────────────────────────────
PR #<number> — Finding <N>/<total> [<severity>]
File: <path>:<line>
────────────────────────────────────────

<reasoning — 2-4 sentences explaining the problem>

Suggested change:
  <show the current code and the proposed replacement as a diff>

────────────────────────────────────────
```

Then ask: **"Accept, skip, or stop reviewing this PR? (a/s/stop)"**

- **accept** — Add this finding to the list of approved findings
- **skip** — Do not include this finding
- **stop** — Stop reviewing this PR, move to next PR

For findings marked `manual_only: true`, present the reasoning but note:
"(No auto-fix — requires manual implementation. Accept to post as a comment without suggestion block.)"

### Step 7: Confirm before posting

After reviewing all findings for a PR, show a summary:

```
PR #<number>: <accepted>/<total> findings accepted

Accepted:
  1. [critical] <path>:<line> — <one-line summary>
  3. [important] <path>:<line> — <one-line summary>

Skipped:
  2. [minor] <path>:<line> — <one-line summary>

Post these as a GitHub review? (yes/no)
```

Wait for user confirmation before posting.

## Phase 4: Post to GitHub

### Step 8: Submit batch review

For each PR with approved findings, submit a **single batch review** using the GitHub API.

Each approved finding becomes an inline review comment with a suggestion block:

Build the full JSON payload (see below) and post via `--input` to avoid shell escaping issues:

The JSON body for the review:

```json
{
  "event": "COMMENT",
  "body": "Automated code review — <N> suggestions. Apply individually or batch-apply in the Files changed tab.",
  "comments": [
    {
      "path": "<file-path>",
      "line": <line-number>,
      "side": "RIGHT",
      "body": "**<severity>**: <reasoning>\n\n```suggestion\n<replacement-code>\n```"
    }
  ]
}
```

For multi-line suggestions, include `start_line` and `start_side`:

```json
{
  "path": "<file-path>",
  "line": <end-line>,
  "start_line": <start-line>,
  "side": "RIGHT",
  "start_side": "RIGHT",
  "body": "**<severity>**: <reasoning>\n\n```suggestion\n<replacement-code>\n```"
}
```

For `manual_only` findings (no auto-fix), omit the suggestion block:

```json
{
  "path": "<file-path>",
  "line": <line-number>,
  "side": "RIGHT",
  "body": "**<severity>**: <reasoning>\n\n_No auto-fix available — requires manual implementation._"
}
```

**Important**: Write the JSON to a temp file and pass via `--input` to avoid shell escaping issues:

```bash
REVIEW_FILE=$(mktemp)
cat > "$REVIEW_FILE" << 'REVIEW_JSON'
<json content>
REVIEW_JSON
gh api repos/$OWNER/$REPO/pulls/<number>/reviews \
  --method POST --input "$REVIEW_FILE"
rm -f "$REVIEW_FILE"
```

### Step 9: Final summary

After posting all reviews, print:

```
Done. Posted reviews:
  PR #<number>: <N> suggestions posted
  PR #<number>: <N> suggestions posted
  PR #<number>: no findings — clean

PRs skipped (draft): #<numbers>
```

## Important Guidelines

- **Local analysis, remote output**: All code analysis happens locally. GitHub is only used to fetch diffs and post reviews.
- **User approval required**: Never post review comments without explicit user approval for each finding.
- **Skip auto/* branches**: Exclude PRs from branches starting with `auto/` (automated PRs).
- **Skip self-authored PRs**: Exclude PRs authored by the current GitHub user (determined via `gh api user --jq '.login'`). Posting automated review comments on your own PRs is confusing.
- **Skip duplicates**: Check existing review comments (Step 3) and skip findings that target the same file path and line number (within ±3 lines) as an already-posted comment. When in doubt, skip — duplicate comments are worse than missing one.
- **Rate limiting**: If there are more than 10 open PRs, process the 10 most recently updated first.
- **Suggestion accuracy**: The `line` number in the review comment must reference the line in the PR diff (the new file version), not the old file. Use `side: "RIGHT"` always.
- **Minimal fixes**: Suggestion blocks should change only the lines needed. Do not reformat or refactor surrounding code.
- **One review per PR**: Submit all approved findings for a PR as a single batch review, not individual comments.
