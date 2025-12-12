---
description: Check which local git branches will be deleted without making changes
---

# Cleanup Check (Dry Run)

This command performs a safe, read-only check to show you what the `/cleanup-execute` command will do. It does not make any changes to your repository.

## Workflow

1.  **Check for Uncommitted Changes**
    - Run `git status --porcelain`.
    - If uncommitted changes are found, the command will report them and advise you to commit or stash them before running `/cleanup-execute`.

2.  **Check Main Branch Status**
    - Run `git fetch origin main`.
    - Check how many commits your local `main` branch is behind the remote `origin/main`.

3.  **List Branches for Deletion**
    - Get a list of all local branches that are not `main`.

4.  **Provide a Summary Report**
    - Display the status of the `main` branch.
    - List all the local branches that are marked for deletion.
    - Confirm that no changes have been made.
    - Instruct you to run `/cleanup-execute` to perform the actual cleanup.

## Example Output

**If the repository is clean:**

```
/cleanup-check
> Your local `main` branch is 2 commits behind `origin/main`.
> The following 3 local branches will be deleted:
> - feature-branch-1
> - fix-issue-123
> - old-branch
>
> This is a dry run. No changes have been made.
> To proceed with the cleanup, run `/cleanup-execute`.
```

**If there are uncommitted changes:**

```
/cleanup-check
> Error: Cannot perform cleanup. You have uncommitted changes.
> Please commit or stash them before running `/cleanup-execute`.
>
> Uncommitted changes found in:
> - M README.md
> - ?? new-file.txt
```
