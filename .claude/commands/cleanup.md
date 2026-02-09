---
description: Cleanup local git repository by updating dev and removing all other branches (except main)
---

# Cleanup Local Repository

Clean up your local Git repository by checking out dev, updating it to the latest version, and removing all other local branches (keeping main and dev).

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

2. **Checkout dev branch**
   ```bash
   git checkout dev
   ```
   - If checkout fails, explain error and exit
   - Confirm switched to dev branch

3. **Update dev to latest version**
   ```bash
   git pull --ff-only origin dev
   ```
   - If pull fails (diverged history), explain error and suggest:
     - `git reset --hard origin/dev` (WARNING: discards local commits)
     - Manual merge resolution
   - Show summary of changes pulled (if any)
   - If already up to date, inform user

4. **Update main to latest version (without switching to it)**
   ```bash
   git fetch origin main:main
   ```
   - This updates the local main branch to match origin/main without checking it out
   - If it fails (e.g., local main has diverged), warn but continue with cleanup

5. **Get list of all local branches (excluding main and dev)**
   ```bash
   git branch | grep -v '^\* dev$' | grep -v '^  dev$' | grep -v '^  main$' | sed 's/^[ *]*//'
   ```
   - This lists all local branches except main and dev
   - Count the number of branches to delete
   - If no other branches exist, inform user and skip deletion

6. **Show branches to be deleted**
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

7. **Ask for confirmation**
   - Use AskUserQuestion tool:
     - "Are you sure you want to delete these X branches?"
       - Option 1: Yes, delete all branches
       - Option 2: No, cancel cleanup (keep branches)
   - If user cancels, exit gracefully with message

8. **Delete all branches**
   ```bash
   git branch | grep -v '^\* dev$' | grep -v '^  dev$' | grep -v '^  main$' | sed 's/^[ *]*//' | xargs git branch -D
   ```
   - Use `-D` flag to force delete even if branches are not merged
   - Show progress for each branch deleted
   - Catch any errors (e.g., if branch is currently checked out)

9. **Verify cleanup**
   ```bash
   git branch
   ```
   - Should only show main and dev branches
   - Confirm cleanup completed successfully

10. **Provide summary**
    - Confirm dev branch is up to date
    - Confirm main branch is up to date
    - Confirm number of branches deleted
    - Show current git status
    - Remind user: "Your local repository is now clean with main and dev branches"

## Important Notes

- **Working Branch**: This repository uses `dev` as the working branch, not `main`
- **Kept Branches**: Both `main` and `dev` are preserved during cleanup
- **Safety First**: This command will DELETE all local branches except main and dev
- **Uncommitted Work**: Always handle uncommitted changes before cleanup
- **Force Delete**: Uses `-D` flag which deletes branches even if not merged
- **Remote Branches**: This only deletes LOCAL branches, not remote branches
- **Stashed Changes**: If you stashed changes, they remain available via `git stash list`
- **Cannot Undo**: Once branches are deleted, you cannot recover them unless they exist on remote

## Safety Warnings

Before deleting branches, warn the user:

WARNING: This will permanently delete all local branches except main and dev.

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
- If `git pull --ff-only` fails, local dev has diverged
- Suggest options:
  - `git reset --hard origin/dev` (destroys local commits)
  - Manual merge resolution
  - Cancel operation

### Branch Delete Fails
- Some branches may fail to delete (e.g., currently checked out)
- Continue with remaining branches
- Report which branches failed to delete

## Post-Cleanup Checklist

After cleanup completes, remind the user:
- Dev branch is up to date with origin/dev
- Main branch is up to date with origin/main
- All other local branches deleted (list count)
- Repository is clean and ready for new work
- If stashed changes exist: How to view (`git stash list`) and restore (`git stash pop`)
- To create a new feature branch from dev: `git checkout -b <branch-name>`

## Related Commands

- `/sync-main` - Sync main and optionally create a new feature branch
- Consider running `/cleanup` periodically to maintain a clean local repository
- Especially useful after PR merges when feature branches are no longer needed
