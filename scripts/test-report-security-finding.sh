#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
WORK_DIR=$(mktemp -d)
trap 'rm -rf "$WORK_DIR"' EXIT

cat > "$WORK_DIR/curl" <<'MOCK_CURL'
#!/usr/bin/env bash

set -euo pipefail

printf '%s\n' "$*" >> "${MOCK_CURL_LOG}"

case "${MOCK_CURL_MODE}" in
    create)
        if [[ "$*" == *"/search/jql"* ]]; then
            printf '%s\n200\n' '{"issues":[]}'
        else
            printf '%s\n201\n' '{"key":"ARO-99999"}'
        fi
        ;;
    existing)
        if [[ "$*" == *"/search/jql"* ]]; then
            printf '%s\n200\n' '{"issues":[{"key":"ARO-99998"}]}'
        else
            printf '%s\n201\n' '{}'
        fi
        ;;
    search-failure)
        if [[ "$*" == *"/search/jql"* ]]; then
            printf '%s\n503\n' '{"error":"unavailable"}'
        else
            printf '%s\n201\n' '{"key":"ARO-99997"}'
        fi
        ;;
    *)
        printf 'unexpected mock mode\n' >&2
        exit 1
        ;;
esac
MOCK_CURL
chmod +x "$WORK_DIR/curl"

printf '%s\n' 'security findings detected' > "$WORK_DIR/report.md"

export PATH="$WORK_DIR:$PATH"
export MOCK_CURL_LOG="$WORK_DIR/curl.log"
export JIRA_EMAIL='test@example.com'
export JIRA_API_TOKEN='token'
export JIRA_PARENT_KEY='ARO-27965'
export JIRA_ASSIGNEE_ACCOUNT_ID='account-id'

assert_contains() {
    local expected=$1
    grep -F -- "$expected" "$MOCK_CURL_LOG" >/dev/null
}

: > "$MOCK_CURL_LOG"
export MOCK_CURL_MODE=create
"$SCRIPT_DIR/report-security-finding.sh" \
    --scanner gosec \
    --summary '[Security][gosec] Security findings detected' \
    --body-file "$WORK_DIR/report.md" > "$WORK_DIR/create.out"
grep -F 'Created Jira issue: ARO-99999' "$WORK_DIR/create.out" >/dev/null
assert_contains '/search/jql'
assert_contains '/issue'

: > "$MOCK_CURL_LOG"
export MOCK_CURL_MODE=existing
"$SCRIPT_DIR/report-security-finding.sh" \
    --scanner gosec \
    --summary '[Security][gosec] Security findings detected' \
    --body-file "$WORK_DIR/report.md" > "$WORK_DIR/existing.out"
grep -F 'Updated Jira issue: ARO-99998' "$WORK_DIR/existing.out" >/dev/null
assert_contains '/issue/ARO-99998/comment'

: > "$MOCK_CURL_LOG"
export MOCK_CURL_MODE=search-failure
"$SCRIPT_DIR/report-security-finding.sh" \
    --scanner gosec \
    --summary '[Security][gosec] Security findings detected' \
    --body-file "$WORK_DIR/report.md" > "$WORK_DIR/failure.out"
grep -F 'skipping Jira update' "$WORK_DIR/failure.out" >/dev/null
if grep -F '/issue' "$MOCK_CURL_LOG" | grep -v '/search/jql' >/dev/null; then
    echo 'search failure must not create or update an issue' >&2
    exit 1
fi

echo 'report-security-finding tests passed'
