---
description: Sync local main branch with remote and optionally create a new feature branch
---

# Sync Main Branch

Synchronize your local main branch with the remote repository and optionally create a new feature branch for development.

## Workflow

1. **Fetch latest changes from remote**
   ```bash
   git fetch origin main
   ```

2. **Check current branch and status**
   ```bash
   git status
   git branch --show-current
   ```

3. **Check for uncommitted changes**
   - Check `git status` output for any uncommitted changes (staged or unstaged)
   - If there are uncommitted changes, ask the user:
     - "You have uncommitted changes. What would you like to do?"
       - Option 1: Stash changes and continue
       - Option 2: Commit changes first
       - Option 3: Discard changes (dangerous, confirm first)
       - Option 4: Cancel sync operation
   - Handle user's choice before proceeding

4. **Check if local main is behind remote**
   ```bash
   git rev-list --count main..origin/main
   ```
   - If count is 0, main is up to date (inform user)
   - If count > 0, main is behind (proceed with sync)

5. **Switch to main branch (if not already on it)**
   ```bash
   git checkout main
   ```

6. **Capture current commit for comparison**
   ```bash
   BEFORE_PULL=$(git rev-parse HEAD)
   ```

7. **Pull latest changes with fast-forward only**
   ```bash
   git pull --ff-only origin main
   ```
   - If this fails (diverged history), explain error and suggest resolution
   - Only fast-forward merges allowed (no merge commits)

8. **Show summary of updates**
   - Display number of commits pulled
   - Show brief commit log of new changes
   ```bash
   git log --oneline $BEFORE_PULL..HEAD
   ```
   - If no new commits, confirm main was already up to date

9. **Ask if user wants to create a new feature branch**
   - Use AskUserQuestion tool with options:
     - "Would you like to create a new feature branch?"
       - Option 1: Yes, create new branch (prompt for branch name)
       - Option 2: No, stay on main

10. **If creating new branch**
    - Ask for branch name using AskUserQuestion
    - Validate branch name follows Git naming conventions
    - Create and checkout the new branch:
      ```bash
      git checkout -b <branch-name>
      ```
    - Remind user to use `git push -u origin <branch-name>` on first push to set up tracking

11. **Provide summary**
    - Confirm current branch
    - Show git status
    - Remind user they're ready to start working

## Important Notes

- **Safety First**: Always check for uncommitted changes before switching branches (on ANY branch, not just feature branches)
- **Clean Merges**: Only fast-forward pulls on main using `--ff-only` flag (no merge commits)
- **Branch Naming**: Validate branch name follows Git naming conventions (e.g., no spaces or control characters, cannot start with `-`, `.`, or `/`, cannot contain `..`, `@{`, `\`, or end with `.lock`, cannot end with `/` or `.`)
- **Remote Tracking**: New branches are created locally. Set up tracking on first push with: `git push -u origin <branch-name>`

## Error Handling

If any step fails:
- Explain what went wrong
- Provide recovery commands
- Ask user how to proceed

Common errors:
- **Pull fails with --ff-only**: Local main has diverged. User needs to either reset to origin/main or merge manually
- **Invalid branch name**: Git will reject. Prompt for new name
- **Uncommitted changes on main**: Handle via step 3 options

## Examples

**Example 1: Simple sync, no new branch**
```
User: /sync-main
Claude: Fetching latest changes...
        No uncommitted changes detected.
        Local main is behind by 3 commits. Updating...
        Successfully pulled 3 new commits:
        - abc1234 Fix infrastructure test
        - def5678 Add new slash command
        - ghi9012 Update documentation

        Would you like to create a new feature branch?
        [User selects: No, stay on main]
        You're now on main branch, up to date with origin.
```

**Example 2: Sync with uncommitted changes**
```
User: /sync-main
Claude: You have uncommitted changes on branch 'feature-xyz'.
        What would you like to do?
        [User selects: Stash changes and continue]
        Stashing changes...
        Switching to main...
        Pulling latest changes...
        Successfully updated main with 2 new commits.

        Note: Your stashed changes from 'feature-xyz' are saved.
        You can restore them later with: git stash pop

        Would you like to create a new feature branch?
        [User selects: No, stay on main]
        You're now on main branch, up to date with origin.
```

**Example 3: Sync and create new branch**
```
User: /sync-main
Claude: Fetching latest changes...
        No uncommitted changes detected.
        Local main is up to date with origin.
        Would you like to create a new feature branch?
        [User selects: Yes, create new branch]
        What should the new branch be called?
        [User enters: add-logging-feature]
        Creating branch 'add-logging-feature'...
        Switched to a new branch 'add-logging-feature'

        Tip: When ready to push, use: git push -u origin add-logging-feature

        Ready to start working!
```

## Post-Sync Checklist

After sync completes, remind the user:
- Current branch name
- Git status (clean/uncommitted changes)
- Number of commits ahead/behind origin (if applicable)
- Suggestion: Run tests if significant changes were pulled
- If stashed changes: Remind how to restore them (`git stash list`, `git stash pop`)
