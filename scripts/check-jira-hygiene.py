#!/usr/bin/env python3
"""
ARO HCP Jira Hygiene Checker for CAPZ tickets.

Fetches open CAPZ tickets from the Jira REST API and applies the official
ARO HCP governance rules (based on the aro-hcp-jira-hygiene skill by
Jasiyah Khalil). Outputs a Markdown report suitable for GitHub Step Summary.

Environment variables:
    JIRA_EMAIL        Jira account email
    JIRA_API_TOKEN    Jira API token
    JIRA_BASE_URL     Jira base URL (default: https://redhat.atlassian.net)
    JIRA_COMPONENT    Jira component filter (default: aro-hcp-capz)
    MAX_RESULTS       Max tickets to fetch per page (default: 100)
"""

import base64
import json
import os
import sys
import urllib.request
import urllib.parse
from datetime import datetime, timezone

JIRA_BASE_URL = os.environ.get("JIRA_BASE_URL", "https://redhat.atlassian.net").rstrip("/")
JIRA_EMAIL = os.environ.get("JIRA_EMAIL", "")
JIRA_API_TOKEN = os.environ.get("JIRA_API_TOKEN", "")
JIRA_COMPONENT = os.environ.get("JIRA_COMPONENT", "aro-hcp-capz")
MAX_RESULTS = int(os.environ.get("MAX_RESULTS", "100"))

# Staleness thresholds (days) per governance rules
STALE_IN_PROGRESS_WARN = 14
STALE_IN_PROGRESS_EVICT = 30
STALE_BACKLOG_WARN = 90
STALE_BACKLOG_CLOSE = 120

REQUIRED_DESCRIPTION_SECTIONS = [
    "overview",
    "acceptance criteria",
    "definition of done",
    "references",
]

IN_PROGRESS_STATUSES = {"In Progress"}
BACKLOG_STATUSES = {"Backlog", "To Do"}
# Statuses where we skip most hygiene checks (ticket not yet refined)
SKIP_CHECKS_STATUSES = {"New", "Refinement"}
# Statuses where story points should be set
POINTS_REQUIRED_STATUSES = {"To Do", "In Progress", "Review", "Backlog"}

ISSUE_TYPES_WITH_POINTS = {"Story", "Task", "Bug", "Spike", "Sub-task"}


def auth_header():
    token = base64.b64encode(f"{JIRA_EMAIL}:{JIRA_API_TOKEN}".encode()).decode()
    return f"Basic {token}"


def jira_get(path, params=None):
    url = f"{JIRA_BASE_URL}/rest/api/3{path}"
    if params:
        url += "?" + urllib.parse.urlencode(params)
    req = urllib.request.Request(url, headers={
        "Authorization": auth_header(),
        "Accept": "application/json",
    })
    with urllib.request.urlopen(req) as resp:
        return json.loads(resp.read().decode())


def extract_text_from_adf(node):
    """Recursively extract plain text from an Atlassian Document Format node."""
    if node is None:
        return ""
    if isinstance(node, str):
        return node
    if node.get("type") == "text":
        return node.get("text", "")
    parts = []
    for child in node.get("content", []):
        parts.append(extract_text_from_adf(child))
    return " ".join(parts)


def days_since(iso_timestamp):
    """Return the number of days since the given ISO 8601 timestamp."""
    dt = datetime.fromisoformat(iso_timestamp.replace("Z", "+00:00"))
    now = datetime.now(timezone.utc)
    return (now - dt).days


def check_ticket(issue):
    """
    Apply ARO HCP governance rules to a single Jira issue.
    Returns a list of (severity, message) tuples.
    """
    fields = issue["fields"]
    status = fields.get("status", {}).get("name", "")
    issue_type = fields.get("issuetype", {}).get("name", "")
    updated = fields.get("updated", "")
    components = [c["name"] for c in fields.get("components", [])]
    fix_versions = fields.get("fixVersions", [])
    parent = fields.get("parent")
    story_points = fields.get("customfield_10016")
    description_raw = fields.get("description")

    warnings = []

    # Skip heavy checks for tickets that are not yet refined
    if status in SKIP_CHECKS_STATUSES:
        return [("info", f"Status is '{status}' — skipped detailed hygiene checks")]

    # 1. Description presence
    if description_raw is None:
        warnings.append(("error", "Missing description — required for all statuses past New"))
    else:
        description_text = extract_text_from_adf(description_raw).lower()
        if status not in SKIP_CHECKS_STATUSES:
            missing_sections = [
                s for s in REQUIRED_DESCRIPTION_SECTIONS
                if s not in description_text
            ]
            if missing_sections:
                warnings.append((
                    "warn",
                    f"Description missing sections: {', '.join(missing_sections)}"
                    " (required: Overview, Acceptance Criteria, Definition of Done, References)",
                ))

    # 2. Components
    capz_components = {"aro-hcp-capz", "aro-hcp-1p", "aro-hcp-clusters-service-east",
                       "aro-hcp-clusters-service-west", "aro-hcp-service-lifecycle",
                       "aro-hcp-qe", "aro-hcp-ci"}
    if not any(c in capz_components for c in components):
        warnings.append(("error", f"No ARO-HCP component set (got: {components or 'none'})"))

    # 3. Fix version
    if not fix_versions:
        warnings.append(("warn", "No Fix Version set"))

    # 4. Parent epic
    if parent is None:
        warnings.append(("warn", "No parent Epic — orphan ticket invisible in Plan view"))

    # 5. Story points (for refined/active tickets)
    if (issue_type in ISSUE_TYPES_WITH_POINTS
            and status in POINTS_REQUIRED_STATUSES
            and story_points is None):
        warnings.append(("warn", "Story points not set — required before moving to In Progress"))

    # 6. Stale policy
    if updated:
        idle_days = days_since(updated)
        if status in IN_PROGRESS_STATUSES:
            if idle_days >= STALE_IN_PROGRESS_EVICT:
                warnings.append((
                    "error",
                    f"In Progress but idle {idle_days}d — policy: evict from sprint after 30d",
                ))
            elif idle_days >= STALE_IN_PROGRESS_WARN:
                warnings.append((
                    "warn",
                    f"In Progress but idle {idle_days}d — policy: ping assignee after 14d",
                ))
        elif status in BACKLOG_STATUSES:
            if idle_days >= STALE_BACKLOG_CLOSE:
                warnings.append((
                    "error",
                    f"In Backlog/To Do but idle {idle_days}d — policy: auto-close after 120d",
                ))
            elif idle_days >= STALE_BACKLOG_WARN:
                warnings.append((
                    "warn",
                    f"In Backlog/To Do but idle {idle_days}d — policy: stale warning after 90d",
                ))

    return warnings or [("ok", "Passes all governance checks")]


def fetch_capz_tickets():
    """Fetch all open CAPZ tickets using cursor-based JQL search pagination."""
    jql = (
        f'project = ARO'
        f' AND component = "{JIRA_COMPONENT}"'
        f' AND status NOT IN (Closed, Done)'
        f' AND issuetype IN (Task, Story, Bug, Spike, "Sub-task")'
    )
    fields = "summary,status,issuetype,description,components,fixVersions,parent,customfield_10016,updated,assignee"

    tickets = []
    params = {"jql": jql, "fields": fields, "maxResults": MAX_RESULTS}
    while True:
        data = jira_get("/search/jql", params)
        tickets.extend(data["issues"])
        if data.get("isLast", True):
            break
        next_token = data.get("nextPageToken")
        if not next_token:
            break
        params = {"jql": jql, "fields": fields, "maxResults": MAX_RESULTS, "nextPageToken": next_token}
    return tickets


def severity_icon(severity):
    return {"error": "🔴", "warn": "🟡", "ok": "🟢", "info": "ℹ️"}.get(severity, "")


def print_report(tickets, results):
    total = len(tickets)
    errors = sum(1 for r in results.values() if any(s == "error" for s, _ in r))
    warnings = sum(1 for r in results.values() if any(s == "warn" for s, _ in r))
    clean = sum(1 for r in results.values() if all(s in ("ok", "info") for s, _ in r))

    now = datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M UTC")
    print(f"# CAPZ Jira Hygiene Report")
    print(f"\nGenerated: {now}  |  Component: `{JIRA_COMPONENT}`\n")
    print(f"| Tickets scanned | 🔴 Issues | 🟡 Warnings | 🟢 Clean |")
    print(f"|:---:|:---:|:---:|:---:|")
    print(f"| {total} | {errors} | {warnings} | {clean} |\n")

    if errors == 0 and warnings == 0:
        print("All tickets pass governance checks. ✅\n")
        return

    print("## Findings\n")
    print("| Ticket | Type | Status | Idle | Severity | Finding |")
    print("|--------|------|--------|------|----------|---------|")

    for issue in tickets:
        key = issue["key"]
        fields = issue["fields"]
        issue_type = fields.get("issuetype", {}).get("name", "")
        status = fields.get("status", {}).get("name", "")
        updated = fields.get("updated", "")
        idle = f"{days_since(updated)}d" if updated else "?"
        url = f"{JIRA_BASE_URL}/browse/{key}"

        checks = results[key]
        if all(s in ("ok", "info") for s, _ in checks):
            continue

        for severity, message in checks:
            if severity in ("ok", "info"):
                continue
            icon = severity_icon(severity)
            print(f"| [{key}]({url}) | {issue_type} | {status} | {idle} | {icon} | {message} |")

    print()


def main():
    if not JIRA_EMAIL or not JIRA_API_TOKEN:
        print("::error::Missing JIRA_EMAIL or JIRA_API_TOKEN environment variables")
        sys.exit(1)

    print("Fetching open CAPZ tickets from Jira...", file=sys.stderr)
    try:
        tickets = fetch_capz_tickets()
    except Exception as e:
        print(f"::error::Failed to fetch tickets: {e}")
        sys.exit(1)

    print(f"Fetched {len(tickets)} tickets. Running hygiene checks...", file=sys.stderr)
    results = {}
    for issue in tickets:
        results[issue["key"]] = check_ticket(issue)

    print_report(tickets, results)

    issues_found = any(
        any(s in ("error", "warn") for s, _ in checks)
        for checks in results.values()
    )
    sys.exit(0 if not issues_found else 1)


if __name__ == "__main__":
    main()
