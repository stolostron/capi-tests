---
description: Cleanup local git repository by updating main and removing all other branches
---

# Cleanup Local Repository

Clean up your local Git repository by checking out main, updating it to the latest version, and removing all other local branches.

## Workflow

1. **Check current git status**
   ```bash
   git status
   ```
   - Check for uncommitted changes on current branch
   - If there are uncommitted changes, ask the user:
     - "You have uncommitted changes. What would you like to do?"
       - Option 1: Commit changes first, then continue
       - Option 2: Stash changes and continue
       - Option 3: Discard changes (dangerous, confirm first)
       - Option 4: Cancel cleanup operation
   - Handle user's choice before proceeding

2. **Checkout main branch**
   ```bash
   git checkout main
   ```
   - If checkout fails, explain error and exit
   - Confirm switched to main branch

3. **Update main to latest version**
   ```bash
   git pull origin main
   ```
   - Use fast-forward only to ensure clean merge:
     ```bash
     git pull --ff-only origin main
     ```
   - If pull fails (diverged history), explain error and suggest:
     - `git reset --hard origin/main` (WARNING: discards local commits)
     - Manual merge resolution
   - Show summary of changes pulled (if any)
   - If already up to date, inform user

4. **Get list of all local branches (excluding main)**
   ```bash
   git branch | grep -v "^\\* main$" | grep -v "^  main$" | sed 's/^[ *]*//'
   ```
   - This lists all local branches except main
   - Count the number of branches to delete
   - If no other branches exist, inform user and skip deletion

5. **Show branches to be deleted**
   - Display the list of branches that will be deleted
   - Count total branches
   - Example output:
     ```
     The following local branches will be deleted:
     - feature-branch-1
     - fix-issue-123
     - experimental-work
     Total: 3 branches
     ```

6. **Ask for confirmation**
   - Use AskUserQuestion tool:
     - "Are you sure you want to delete these X branches?"
       - Option 1: Yes, delete all branches
       - Option 2: No, cancel cleanup (keep branches)
   - If user cancels, exit gracefully with message

7. **Delete all branches**
   ```bash
   git branch | grep -v "^\\* main$" | grep -v "^  main$" | sed 's/^[ *]*//' | xargs git branch -D
   ```
   - Use `-D` flag to force delete even if branches are not merged
   - Show progress for each branch deleted
   - Catch any errors (e.g., if branch is currently checked out)

8. **Verify cleanup**
   ```bash
   git branch
   ```
   - Should only show main branch
   - Confirm cleanup completed successfully

9. **Provide summary**
   - Confirm main branch is up to date
   - Confirm number of branches deleted
   - Show current git status
   - Remind user: "Your local repository is now clean with only the main branch"

## Important Notes

- **Safety First**: This command will DELETE all local branches except main
- **Uncommitted Work**: Always handle uncommitted changes before cleanup
- **Force Delete**: Uses `-D` flag which deletes branches even if not merged to main
- **Remote Branches**: This only deletes LOCAL branches, not remote branches
- **Stashed Changes**: If you stashed changes, they remain available via `git stash list`
- **Cannot Undo**: Once branches are deleted, you cannot recover them unless they exist on remote

## Safety Warnings

Before deleting branches, warn the user:

⚠️ **WARNING**: This will permanently delete all local branches except main.

- Branches that exist on remote can be recovered by checking them out again
- Branches that are LOCAL ONLY will be permanently lost
- Make sure any important work is either:
  - Committed and pushed to remote
  - Stashed for later use
  - Not in these branches

## Error Handling

### Uncommitted Changes
- Detect via `git status --porcelain`
- Offer stash, commit, or discard options
- Never proceed with uncommitted changes without user consent

### Pull Fails
- If `git pull --ff-only` fails, local main has diverged
- Suggest options:
  - `git reset --hard origin/main` (destroys local commits)
  - Manual merge resolution
  - Cancel operation

### Branch Delete Fails
- Some branches may fail to delete (e.g., currently checked out)
- Continue with remaining branches
- Report which branches failed to delete

## Examples

**Example 1: Clean repository with no uncommitted changes**
```
User: /cleanup
Claude: Checking for uncommitted changes... None found.
        Checking out main branch...
        Pulling latest changes from origin/main...
        Already up to date.

        Found 4 local branches to delete:
        - feature-add-logging
        - fix-issue-123
        - experimental-work
        - old-branch

        ⚠️ WARNING: This will permanently delete 4 local branches.
        Are you sure you want to continue?
        [User selects: Yes, delete all branches]

        Deleting branches...
        ✓ Deleted feature-add-logging
        ✓ Deleted fix-issue-123
        ✓ Deleted experimental-work
        ✓ Deleted old-branch

        Cleanup complete!
        - Main branch is up to date
        - 4 branches deleted
        - Your local repository is now clean
```

**Example 2: Cleanup with uncommitted changes**
```
User: /cleanup
Claude: You have uncommitted changes on branch 'feature-xyz'.
        What would you like to do?
        [User selects: Stash changes and continue]

        Stashing changes...
        Saved working directory and index state On feature-xyz: WIP

        Checking out main branch...
        Pulling latest changes...
        Updated main with 2 new commits.

        Found 3 local branches to delete:
        - feature-xyz
        - fix-bug
        - test-branch

        Note: Your stashed changes from 'feature-xyz' are saved.
        You can restore them with: git stash pop

        ⚠️ WARNING: This will permanently delete 3 local branches.
        Are you sure you want to continue?
        [User selects: Yes, delete all branches]

        Deleting branches...
        ✓ Deleted feature-xyz
        ✓ Deleted fix-bug
        ✓ Deleted test-branch

        Cleanup complete!
        Your stashed changes are preserved in the stash.
```

**Example 3: No branches to delete**
```
User: /cleanup
Claude: Checking for uncommitted changes... None found.
        Checking out main branch...
        Already on main branch.
        Pulling latest changes...
        Already up to date.

        No other local branches found.
        Your repository is already clean!

        Current branch: main
        Status: Up to date with origin/main
```

**Example 4: User cancels cleanup**
```
User: /cleanup
Claude: Checking for uncommitted changes... None found.
        Checking out main branch...
        Pulling latest changes...
        Updated main with 5 new commits.

        Found 2 local branches to delete:
        - important-work
        - feature-in-progress

        ⚠️ WARNING: This will permanently delete 2 local branches.
        Are you sure you want to continue?
        [User selects: No, cancel cleanup]

        Cleanup cancelled. No branches were deleted.
        Current branch: main (up to date)
```

## Post-Cleanup Checklist

After cleanup completes, remind the user:
- ✅ Main branch is up to date with origin/main
- ✅ All local branches deleted (list count)
- ✅ Repository is clean and ready for new work
- If stashed changes exist: How to view (`git stash list`) and restore (`git stash pop`)
- To create a new branch: `git checkout -b <branch-name>`
- To start fresh: Use `/sync-main` command to create a new feature branch

## Related Commands

- `/sync-main` - Sync main and optionally create a new feature branch
- Consider running `/cleanup` periodically to maintain a clean local repository
- Especially useful after PR merges when feature branches are no longer needed
