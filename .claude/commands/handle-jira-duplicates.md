---
description: Handle duplicate Jira subtasks by linking, renaming, and closing the old ones.
---

# Handle Jira Duplicates

Process duplicate Jira subtasks: link them to their replacements, rename with "(duplicate)" prefix, and close as Duplicate.

## Input

The user provides pairs of Jira issue keys: old (duplicate) → new (replacement).

Example: `ACM-29887:ACM-29892 ACM-29888:ACM-29893`

Or described as a range: "ACM-29887 through ACM-29891 are duplicates of ACM-29892 through ACM-29896"

## Workflow

### Step 1: Verify the pairs

Before making any changes:
1. Fetch all issues (both old and new) from Jira
2. Display a table showing:
   - Old key, summary, status, parent
   - New key, summary, status, parent
3. **Ask the user to confirm** the pairs are correct before proceeding

### Step 2: Add "Duplicate" issue links

For each pair, create a Jira issue link:

```
POST /rest/api/2/issueLink
{
  "type": {"id": "12310000"},
  "outwardIssue": {"key": "<old_key>"},
  "inwardIssue": {"key": "<new_key>"}
}
```

This creates: `<old_key> duplicates <new_key>`

### Step 3: Rename old issues

Prepend "(duplicate)" to the summary of each old issue:

```
PUT /rest/api/2/issue/<old_key>
{"fields": {"summary": "(duplicate) <original summary>"}}
```

### Step 4: Close old issues

Transition each old issue to Closed with resolution Duplicate:

```
POST /rest/api/2/issue/<old_key>/transitions
{"transition": {"id": "61"}, "fields": {"resolution": {"name": "Duplicate"}}}
```

### Step 5: Report manual steps

If any of the old issues are subtasks, report:

> The following issues are Jira subtasks and cannot be converted to standalone tasks via the API.
> To fully disconnect them from their parent, manually convert each one in the Jira UI:
> **Actions > Convert to Issue > Task**
>
> Issues to convert: ACM-XXXXX, ACM-XXXXX, ...

### Step 6: Verify

Fetch all issues again and display a final summary table confirming:
- Duplicate links are in place
- Old issues are renamed and closed
- New issues are unchanged

## Important Rules

- **NEVER skip the confirmation step** (Step 1) — always show the pairs and wait for user approval
- **NEVER delete Jira issues** — only close them
- Use `curl -s -w "\n%{http_code}"` for all Jira write calls
- Read Jira token from `credentials.json`
- Use visibility settings matching the issue's security level
