---
description: Clean up a git worktree after a GitHub issue PR has been merged
---

# Close Worktree

Clean up a git worktree that was created with `/prepare-worktree` after the associated pull request has been merged.

## Usage

```
/close-worktree <issue-number>
```

**Example**: `/close-worktree 263`

## Workflow

1. **Validate issue number argument**
   - If no issue number provided, prompt user: "Please provide an issue number: /close-worktree <number>"
   - If issue number is not a valid integer, show error and exit

2. **Find the worktree for this issue**
   ```bash
   git worktree list
   ```
   - Look for worktree path containing `issue-<number>` pattern
   - If no matching worktree found, show available worktrees and exit

3. **Verify changes have been accepted (PR merged)**

   This is a critical safety check to prevent losing work.

   ```bash
   # Check if there's a merged PR for this issue
   gh pr list --search "issue-<number> in:title" --state merged --json number,title,mergedAt
   ```

   **If PR is merged**:
   - Display: "✓ PR #<number> was merged on <date>"
   - Proceed with cleanup

   **If PR is NOT merged**:
   - Check if PR exists but is still open:
     ```bash
     gh pr list --search "issue-<number> in:title" --state open --json number,title,url
     ```
   - Display warning:
     ```
     ⚠️  Warning: Changes have NOT been accepted yet!

     Open PR: #<pr-number> - <title>
     URL: <pr-url>

     Deleting this worktree will NOT lose committed changes (they're in the branch),
     but you won't be able to continue working on them easily.
     ```
   - Ask user:
     - Option 1: "Cancel - I'll close the worktree after PR is merged"
     - Option 2: "Continue anyway - I understand the PR is not merged"

   **If NO PR found at all**:
   - Display warning:
     ```
     ⚠️  Warning: No PR found for issue #<number>!

     This could mean:
     - The work was never pushed/PR was never created
     - The PR was closed without merging
     - The PR title doesn't contain the issue number
     ```
   - Ask user:
     - Option 1: "Cancel - let me check the worktree first"
     - Option 2: "Continue anyway - I want to discard this work"

4. **Check for uncommitted changes in worktree**
   ```bash
   git -C <worktree-path> status --porcelain
   ```
   - If there are uncommitted changes:
     ```
     ⚠️  Warning: Worktree has uncommitted changes that will be LOST:

       M  file1.go
       ?? file2.go
     ```
   - Ask user:
     - Option 1: "Cancel - let me commit or stash these changes first"
     - Option 2: "Continue - discard uncommitted changes"

5. **Check for unpushed commits**
   ```bash
   git -C <worktree-path> log @{u}..HEAD --oneline 2>/dev/null
   ```
   - If there are unpushed commits:
     ```
     ⚠️  Warning: Branch has unpushed commits:

       abc1234 Fix edge case in validation
       def5678 Add additional test
     ```
   - Ask user:
     - Option 1: "Cancel - let me push these commits first"
     - Option 2: "Continue - these commits will remain in local branch"

6. **Remove the worktree**
   ```bash
   git worktree remove <worktree-path>
   ```
   - If removal fails due to dirty state, ask about force:
     ```bash
     git worktree remove --force <worktree-path>
     ```

7. **Delete the branch (if merged)**
   ```bash
   git branch -d <branch-name>
   ```
   - `-d` only succeeds if branch is fully merged
   - If deletion fails (branch not merged), keep the branch:
     ```
     Note: Branch '<branch-name>' kept (not fully merged to main).
     To delete later: git branch -D <branch-name>
     ```

8. **Prune stale worktree references**
   ```bash
   git worktree prune
   ```

9. **Display summary**
   ```
   ================================================
   Worktree closed successfully!
   ================================================

   ✓ Removed worktree: <worktree-path>
   ✓ Deleted branch:   <branch-name>
   ✓ PR #<number> was merged

   Remaining worktrees:
     /path/to/CAPZTests  main

   ================================================
   ```

## Safety Checks Summary

The skill performs these checks in order to prevent losing work:

| Check | What it prevents |
|-------|------------------|
| PR merged? | Closing before work is accepted |
| Uncommitted changes? | Losing local edits |
| Unpushed commits? | Losing commits not on remote |
| Branch merged? | Deleting branch with unmerged work |

## Examples

### Example 1: Clean closure after merged PR

```
User: /close-worktree 263

Claude: Finding worktree for issue #263...

Found:
  Path:   ../CAPZTests-issue-263-add-non-interactive-mode
  Branch: issue-263-add-non-interactive-mode

Checking if changes were accepted...
  ✓ PR #267 was merged on 2026-01-07

Checking for uncommitted changes...
  ✓ Working tree clean

Removing worktree...
Deleting branch...

================================================
Worktree closed successfully!
================================================

✓ Removed worktree: ../CAPZTests-issue-263-add-non-interactive-mode
✓ Deleted branch:   issue-263-add-non-interactive-mode
✓ PR #267 was merged

Remaining worktrees:
  /Users/radoslavcap/git/CAPZTests  main

================================================
```

### Example 2: PR not merged yet

```
User: /close-worktree 264

Claude: Finding worktree for issue #264...

Found:
  Path:   ../CAPZTests-issue-264-fix-tests
  Branch: issue-264-fix-tests

Checking if changes were accepted...

⚠️  Warning: Changes have NOT been accepted yet!

Open PR: #270 - Fix test timeout issue (fixes #264)
URL: https://github.com/RadekCap/CAPZTests/pull/270

Deleting this worktree will NOT lose committed changes (they're in the branch),
but you won't be able to continue working on them easily.

What would you like to do?
  1. Cancel - I'll close the worktree after PR is merged
  2. Continue anyway - I understand the PR is not merged

User: 1

Claude: Cancelled. The worktree remains at:
  ../CAPZTests-issue-264-fix-tests

Run /close-worktree 264 again after the PR is merged.
```

### Example 3: Uncommitted changes warning

```
User: /close-worktree 265

Claude: Finding worktree for issue #265...

Found:
  Path:   ../CAPZTests-issue-265-add-logging
  Branch: issue-265-add-logging

Checking if changes were accepted...
  ✓ PR #271 was merged on 2026-01-07

Checking for uncommitted changes...

⚠️  Warning: Worktree has uncommitted changes that will be LOST:

  M  test/helpers.go
  ?? test/debug_notes.txt

What would you like to do?
  1. Cancel - let me commit or stash these changes first
  2. Continue - discard uncommitted changes

User: 1

Claude: Cancelled. To review your changes:
  cd ../CAPZTests-issue-265-add-logging
  git status
  git diff
```

## Related Skills

- `/prepare-worktree` - Create a worktree for an issue
- `/implement-issue` - Implement the fix in the worktree
- `/cleanup` - Interactive cleanup of all test resources

## Complete Workflow

```bash
# Instance 1: Prepare worktree
/prepare-worktree 263
# Copies command to clipboard

# Instance 2: New terminal, paste command
cd /path/to/CAPZTests-issue-263-... && claude
/implement-issue 263
# Creates PR, gets merged

# Instance 1: Clean up after PR is merged
/close-worktree 263
# Verifies PR merged, then removes worktree and branch
```
