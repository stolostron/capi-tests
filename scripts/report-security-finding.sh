#!/usr/bin/env bash
# Report a security scanner finding to Jira, updating the scanner's existing
# open issue when possible and creating one otherwise.

set -euo pipefail

JIRA_API_BASE="${JIRA_API_BASE:-https://redhat.atlassian.net/rest/api/3}"
JIRA_ISSUE_TYPE="${JIRA_ISSUE_TYPE:-Task}"
JIRA_ASSIGNEE_ACCOUNT_ID="${JIRA_ASSIGNEE_ACCOUNT_ID:-5fabb5fdecdae600685b01d6}"
JIRA_SECURITY_LABEL="${JIRA_SECURITY_LABEL:-no-qe}"
JIRA_SECURITY_GROUP="${JIRA_SECURITY_GROUP:-Red Hat Employee}"

usage() {
    cat <<'USAGE'
Usage: report-security-finding.sh --scanner NAME --summary SUMMARY --body-file FILE

Required environment variables:
  JIRA_EMAIL       Jira account email
  JIRA_API_TOKEN   Jira API token
  JIRA_PARENT_KEY  Parent Jira issue key
USAGE
}

SCANNER=""
SUMMARY=""
BODY_FILE=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --scanner)
            [[ $# -ge 2 ]] || { usage >&2; exit 2; }
            SCANNER="$2"
            shift 2
            ;;
        --summary)
            [[ $# -ge 2 ]] || { usage >&2; exit 2; }
            SUMMARY="$2"
            shift 2
            ;;
        --body-file)
            [[ $# -ge 2 ]] || { usage >&2; exit 2; }
            BODY_FILE="$2"
            shift 2
            ;;
        --help|-h)
            usage
            exit 0
            ;;
        *)
            echo "Unknown option: $1" >&2
            usage >&2
            exit 2
            ;;
    esac
done

if [[ ! "$SCANNER" =~ ^[a-z0-9_-]+$ ]]; then
    echo "Invalid scanner name: $SCANNER" >&2
    exit 2
fi
if [[ ! "${JIRA_PARENT_KEY:-}" =~ ^[A-Z]+-[0-9]+$ ]]; then
    echo "Invalid or missing JIRA_PARENT_KEY" >&2
    exit 2
fi
if [[ -z "$SUMMARY" || ! -f "$BODY_FILE" ]]; then
    echo "A non-empty summary and an existing body file are required" >&2
    exit 2
fi

if [[ -z "${JIRA_EMAIL:-}" || -z "${JIRA_API_TOKEN:-}" ]]; then
    echo "Jira credentials are not configured; skipping Jira update"
    exit 0
fi

echo "::add-mask::${JIRA_API_TOKEN}"

request() {
    local response curl_exit
    set +e
    response=$(curl -sS -L -w $'\n%{http_code}' "$@" 2>&1)
    curl_exit=$?
    set -e
    if [[ $curl_exit -ne 0 ]]; then
        REQUEST_BODY="$response"
        REQUEST_STATUS=0
        return 0
    fi
    REQUEST_STATUS="${response##*$'\n'}"
    REQUEST_BODY="${response%$'\n'*}"
}

JQL="project = ARO AND parent = ${JIRA_PARENT_KEY} AND summary ~ \"\\\"[Security][${SCANNER}]\\\"\" AND status != Closed"
request \
    -u "${JIRA_EMAIL}:${JIRA_API_TOKEN}" \
    -G "${JIRA_API_BASE}/search/jql" \
    --data-urlencode "jql=${JQL}" \
    --data-urlencode 'fields=key,summary,status' \
    -H 'Content-Type: application/json'

if [[ "$REQUEST_STATUS" -lt 200 || "$REQUEST_STATUS" -ge 300 ]]; then
    echo "Warning: Jira search failed (HTTP ${REQUEST_STATUS}); skipping Jira update"
    exit 0
fi

EXISTING_KEY=$(jq -r '.issues[0].key // empty' <<< "$REQUEST_BODY" 2>/dev/null || true)
if [[ -n "$EXISTING_KEY" && ! "$EXISTING_KEY" =~ ^[A-Z]+-[0-9]+$ ]]; then
    echo "Warning: unexpected Jira issue key; skipping Jira update"
    exit 0
fi

DESCRIPTION=$(jq -n --rawfile text "$BODY_FILE" \
    '{type:"doc",version:1,content:[{type:"paragraph",content:[{type:"text",text:$text}]}]}')

if [[ -n "$EXISTING_KEY" ]]; then
    COMMENT_PAYLOAD=$(jq -n \
        --argjson body "$DESCRIPTION" \
        --arg group "$JIRA_SECURITY_GROUP" \
        '{body:$body,visibility:{type:"group",value:$group}}')

    request \
        -u "${JIRA_EMAIL}:${JIRA_API_TOKEN}" \
        -X POST "${JIRA_API_BASE}/issue/${EXISTING_KEY}/comment" \
        -H 'Content-Type: application/json' \
        -d "$COMMENT_PAYLOAD"
    if [[ "$REQUEST_STATUS" -ge 200 && "$REQUEST_STATUS" -lt 300 ]]; then
        echo "Updated Jira issue: ${EXISTING_KEY}"
    else
        echo "Warning: Jira comment failed (HTTP ${REQUEST_STATUS}); continuing"
    fi
    exit 0
fi

ISSUE_PAYLOAD=$(jq -n \
    --arg parent "$JIRA_PARENT_KEY" \
    --arg summary "$SUMMARY" \
    --argjson description "$DESCRIPTION" \
    --arg issue_type "$JIRA_ISSUE_TYPE" \
    --arg label "$JIRA_SECURITY_LABEL" \
    --arg assignee "$JIRA_ASSIGNEE_ACCOUNT_ID" \
    '{
      fields: {
        project: {key:"ARO"},
        parent: {key:$parent},
        issuetype: {name:$issue_type},
        summary: $summary,
        description: $description,
        labels: [$label],
        components: [{id:"37592"}],
        security: {id:"10034"},
        assignee: {accountId:$assignee},
        customfield_10464: {id:"10608"},
        fixVersions: [{id:"105810"}]
      }
    }')

request \
    -u "${JIRA_EMAIL}:${JIRA_API_TOKEN}" \
    -X POST "${JIRA_API_BASE}/issue" \
    -H 'Content-Type: application/json' \
    -d "$ISSUE_PAYLOAD"
if [[ "$REQUEST_STATUS" -eq 201 ]]; then
    ISSUE_KEY=$(jq -r '.key // empty' <<< "$REQUEST_BODY" 2>/dev/null || true)
    echo "Created Jira issue: ${ISSUE_KEY:-unknown}"
    echo "https://redhat.atlassian.net/browse/${ISSUE_KEY:-unknown}"
else
    echo "Warning: Jira issue creation failed (HTTP ${REQUEST_STATUS}); continuing"
fi
