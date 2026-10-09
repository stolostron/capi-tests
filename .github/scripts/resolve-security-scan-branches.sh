#!/usr/bin/env bash
set -euo pipefail

if [[ "${EVENT_NAME:-}" == "schedule" ]]; then
  branches=$(jq -ce '
    if type == "array" and length > 0
      and all(.[]; type == "string" and length > 0)
      and length == (unique | length)
    then .
    else error("security scan branch list must contain unique, non-empty branch names")
    end
  ' .github/security-scan-branches.json)
else
  branch="${REF_NAME:-}"
  if [[ "$EVENT_NAME" == "pull_request" ]]; then
    branch="${BASE_REF:-}"
  fi
  if [[ -z "$branch" ]]; then
    echo "::error::Could not resolve a branch for event '$EVENT_NAME'." >&2
    exit 1
  fi
  branches=$(jq -cn --arg branch "$branch" '[$branch]')
fi

echo "branches=$branches" >> "$GITHUB_OUTPUT"
