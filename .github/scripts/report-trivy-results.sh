#!/usr/bin/env bash
set -euo pipefail
shopt -s nullglob

expected_json="${EXPECTED_BRANCHES:?EXPECTED_BRANCHES is required}"
repo="${GH_REPO:?GH_REPO is required}"
server_url="${SERVER_URL:?SERVER_URL is required}"
run_id="${RUN_ID:?RUN_ID is required}"
declare -A expected=()
declare -A seen=()
mapfile -t expected_branches < <(jq -r '.[]' <<<"$expected_json")
for branch in "${expected_branches[@]}"; do
  expected["$branch"]=1
done

report_failed=0
processed=0
fail() {
  echo "::error::$*"
  report_failed=1
}

ensure_label() {
  local label="$1" colour="$2" description="$3" labels
  if ! labels=$(gh label list --repo "$repo" --limit 1000 --json name --jq '.[].name'); then
    fail "Could not check whether the '$label' label exists."
    return 1
  fi
  if grep -Fxq -- "$label" <<<"$labels"; then
    return 0
  fi
  echo "Label '$label' is missing; creating it."
  if ! gh label create "$label" --repo "$repo" --color "$colour" --description "$description"; then
    fail "Could not create the '$label' label, so findings cannot be filed."
    return 1
  fi
}

summary_files=()
if [[ -d trivy-artifacts ]]; then
  mapfile -d '' -t summary_files < <(find trivy-artifacts -type f -name scan-summary.json -print0)
fi

for summary_file in "${summary_files[@]}"; do
  processed=$((processed + 1))
  results_dir="${summary_file%/scan-summary.json}"
  if [[ ! -s "$summary_file" ]]; then
    fail "Trivy summary is empty: $summary_file."
    continue
  fi
  if ! jq -e 'type == "object" and (.branch | type == "string" and length > 0) and (.outcome | type == "string") and (.total | type == "number" and floor == .) and (.counts | type == "object")' "$summary_file" >/dev/null; then
    fail "Malformed Trivy summary in $summary_file."
    continue
  fi

  branch=$(jq -r '.branch // empty' "$summary_file")
  outcome=$(jq -r '.outcome // empty' "$summary_file")
  total=$(jq -r '.total // empty' "$summary_file")
  error=$(jq -r '.error // empty' "$summary_file")
  if [[ -z "${expected[$branch]+present}" ]]; then
    fail "Unexpected Trivy result artifact for branch '$branch'."
    continue
  fi
  if [[ "$outcome" != clean && "$outcome" != vulnerable && "$outcome" != error ]]; then
    fail "Unknown Trivy outcome '$outcome' for branch '$branch'."
    continue
  fi
  seen["$branch"]=1

  existing_issue=$(gh issue list --repo "$repo" --state open --label security --label trivy --label "$branch" --limit 1 --json number --jq '.[0].number // empty') || {
    echo "::warning::Could not check for an existing Trivy issue on '$branch'; a duplicate may be created."
    existing_issue=""
  }

  if [[ "$outcome" == clean ]]; then
    if [[ -n "$existing_issue" ]]; then
      gh issue comment "$existing_issue" --repo "$repo" --body \
        "✅ Trivy is now clean on `$branch` as of [run $run_id]($server_url/$repo/actions/runs/$run_id). No findings remain. Safe to close once reviewed." \
        || fail "Could not comment on resolved Trivy issue #$existing_issue for '$branch'."
    fi
    continue
  fi

  ensure_label security d93f0b "Security-related issue" || continue
  ensure_label trivy d93f0b "Reported by the Trivy scanner" || continue
  ensure_label "$branch" ededed "Affects the $branch branch" || continue

  if [[ "$outcome" == vulnerable ]]; then
    title="🚨 [Trivy] $total findings on $branch"
    heading="## 🚨 $total Trivy findings"
    context="Trivy reported findings for this branch. Review the severity summary, workflow run, and SARIF results in GitHub's Security tab."
  else
    title="⚠️ [Trivy] scan failed on $branch"
    heading="## ⚠️ Trivy scan did not complete"
    context="The scan did not produce a trustworthy result. This is a scan failure, not a clean scan. $error"
  fi

  {
    echo "$heading"
    echo
    echo "$context"
    echo
    echo "| Field | Value |"
    echo "|-------|-------|"
    echo "| **Scanner** | Trivy |"
    echo "| **Branch** | `$branch` |"
    echo "| **Findings** | $total |"
    echo "| **Workflow run** | [${run_id}](${server_url}/${repo}/actions/runs/${run_id}) |"
    echo "| **Date** | $(date -u +"%Y-%m-%d %H:%M:%S UTC") |"
    echo
    echo "### Severity counts"
    echo
    jq -r '.counts | to_entries | if length == 0 then "No severity counts available." else .[] | "- **\(.key):** \(.value)" end' "$summary_file"
    echo
    echo "See the [Trivy workflow](${server_url}/${repo}/actions/workflows/security-trivy.yml) and GitHub Security tab for details."
    if [[ -s "$results_dir/trivy-results.json" ]]; then
      echo
      echo "<details>"
      echo "<summary><b>Trivy JSON output (first 12 KB)</b></summary>"
      echo
      echo '```json'
      head -c 12000 "$results_dir/trivy-results.json"
      echo
      echo '```'
      echo "</details>"
    fi
    echo
    echo "*Automatically reported by the Trivy security workflow. When a later scan is clean, this issue will be updated for human review and can then be closed.*"
  } > issue_body.md

  if [[ -n "$existing_issue" ]]; then
    gh issue edit "$existing_issue" --repo "$repo" --title "$title" \
      || fail "Could not update the title on Trivy issue #$existing_issue for '$branch'."
    gh issue comment "$existing_issue" --repo "$repo" --body-file issue_body.md \
      || fail "Could not update Trivy issue #$existing_issue for '$branch'."
  else
    gh issue create --repo "$repo" --title "$title" --label security --label trivy --label "$branch" --body-file issue_body.md \
      || fail "Could not file the Trivy issue for '$branch'."
  fi
done

for branch in "${expected_branches[@]}"; do
  if [[ -z "${seen[$branch]+present}" ]]; then
    fail "No Trivy result artifact was found for expected branch '$branch'."
  fi
done

if (( processed == 0 )); then
  fail "No Trivy scan artifacts were found; this run proves nothing."
fi
if (( report_failed != 0 )); then
  echo "::error::Trivy results could not all be reported. See the errors above."
  exit 1
fi
