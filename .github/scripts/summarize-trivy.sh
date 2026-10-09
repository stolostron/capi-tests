#!/usr/bin/env bash
set -euo pipefail

branch="${BRANCH:?BRANCH is required}"
scan_outcome="${TRIVY_JSON_OUTCOME:-failure}"
outcome=error
error=""
total=0
counts='{}'

if [[ "$scan_outcome" != "success" ]]; then
  error="Trivy JSON scan step ended with '$scan_outcome'."
elif [[ ! -s trivy-results.json ]] || ! jq -e 'type == "object" and (.Results | type == "array")' trivy-results.json >/dev/null; then
  error="Trivy did not produce a valid JSON results file."
else
  findings=$(jq -c '[.Results[]? | (.Vulnerabilities[]?, .Misconfigurations[]?, .Secrets[]?, .Licenses[]?)]' trivy-results.json)
  total=$(jq 'length' <<<"$findings")
  counts=$(jq -c '[.[] | (.Severity // "UNKNOWN")] | group_by(.) | map({(.[0]): length}) | add // {}' <<<"$findings")
  if (( total == 0 )); then
    outcome=clean
  else
    outcome=vulnerable
  fi
fi

jq -n \
  --arg branch "$branch" \
  --arg outcome "$outcome" \
  --arg error "$error" \
  --argjson total "$total" \
  --argjson counts "$counts" \
  '{branch: $branch, outcome: $outcome, total: $total, counts: $counts, error: $error}' \
  > scan-summary.json

{
  echo "# Trivy security scan (`$branch`)"
  echo
  case "$outcome" in
    clean)
      echo "## ✅ No findings"
      ;;
    vulnerable)
      echo "## ❌ $total findings"
      echo
      echo "Severity counts: `$counts`. See the uploaded Trivy JSON artifact and Security tab for details."
      ;;
    *)
      echo "## ⚠️ Scan did not complete"
      echo
      echo "$error"
      ;;
  esac
} >> "$GITHUB_STEP_SUMMARY"

echo "outcome=$outcome" >> "$GITHUB_OUTPUT"
