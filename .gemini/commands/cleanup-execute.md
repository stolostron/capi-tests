---
description: Update main and delete all other local git branches
---

# Cleanup Execute

This command cleans your local repository. It will fail safely if you have uncommitted changes.

**WARNING:** This is a destructive action. It will permanently delete all local branches except for `main`. This action cannot be undone for local-only branches.

## Workflow

1.  **Safety Check**
    - The command first runs `git status --porcelain`.
    - If there is *any* output (meaning you have uncommitted or untracked files), the command will stop immediately and will not make any changes.

2.  **Switch to and Update Main**
    - Checks out the `main` branch.
    - Pulls the latest changes from `origin/main` using a fast-forward-only strategy to avoid merge conflicts.

3.  **Delete Local Branches**
    - Deletes all local branches other than `main`.
    - It uses the `-D` flag, which means branches will be deleted even if they haven't been merged.

4.  **Provide a Final Summary**
    - Confirms that the `main` branch is up to date.
    - Lists the number of local branches that were deleted.
    - Shows the final clean status of the repository.

## Pre-computation

Before running, this command will automatically perform the safety check. If the check fails, the command will not proceed.

## Example Run

```
/cleanup-execute
> Switched to branch 'main'.
> Your branch is up to date with 'origin/main'.
> Pulling latest changes...
> Successfully updated main.
> Deleting local branches...
> - Deleted branch feature-branch-1 (was abc1234).
> - Deleted branch fix-issue-123 (was def5678).
> Cleanup complete. 2 local branches have been deleted.
```
