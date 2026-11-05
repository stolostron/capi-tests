#!/usr/bin/env bash
# check-stale-resources.sh - Detect stale cloud resources left by CAPI test runs
#
# Scans Azure for resources that are older than a configurable threshold,
# indicating they were leaked by failed or interrupted test runs.
#
# Usage:
#   ./scripts/check-stale-resources.sh [OPTIONS]
#
# Options:
#   --max-age HOURS    Staleness threshold in hours (default: 24)
#   --azure            Check Azure resources only (default)
#   --json             Output results as JSON (for GHA workflow parsing)
#   --help             Show this help message
#
# Environment variables:
#   CAPI_USER              User prefix for resource matching (default: $USER)
#   AZURE_SUBSCRIPTION_ID  Azure subscription to scan
#
# Exit codes:
#   0 - No confirmed stale resources (informational findings may be present)
#   1 - Stale resources detected
#   2 - Error (missing prerequisites, auth failure, etc.)
#
# Examples:
#   ./scripts/check-stale-resources.sh                           # Check Azure
#   ./scripts/check-stale-resources.sh --azure --max-age 12      # Azure, 12h threshold
#   ./scripts/check-stale-resources.sh --json --max-age 48       # Azure, 48h, JSON

set -euo pipefail

# Defaults
MAX_AGE_HOURS=24
CHECK_AZURE=true
JSON_OUTPUT=false

# ANSI colors for output (disabled in JSON mode)
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_info() { [[ "$JSON_OUTPUT" == "true" ]] || echo -e "${BLUE}[INFO]${NC} $1"; }
print_success() { [[ "$JSON_OUTPUT" == "true" ]] || echo -e "${GREEN}[OK]${NC} $1"; }
print_warning() { [[ "$JSON_OUTPUT" == "true" ]] || echo -e "${YELLOW}[WARN]${NC} $1"; }
print_error() { echo -e "${RED}[ERROR]${NC} $1" >&2; }

usage() {
    local exit_code="${1:-0}"
    sed -n '2,/^$/p' "$0" | grep '^#' | sed 's/^# \?//'
    exit "$exit_code"
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --max-age)
            [[ $# -lt 2 ]] && { print_error "Missing value for --max-age"; exit 2; }
            MAX_AGE_HOURS="$2"
            if ! [[ "$MAX_AGE_HOURS" =~ ^[0-9]+$ ]] || [[ "$MAX_AGE_HOURS" -eq 0 ]]; then
                print_error "Invalid --max-age value '${MAX_AGE_HOURS}': must be a positive integer (hours)"
                exit 2
            fi
            shift 2
            ;;
        --azure)
            CHECK_AZURE=true
            shift
            ;;
        --json)
            JSON_OUTPUT=true
            shift
            ;;
        --help|-h)
            usage
            ;;
        *)
            print_error "Unknown option: $1"
            usage 2
            ;;
    esac
done

# Compute threshold epoch once
THRESHOLD_EPOCH=$(date -d "-${MAX_AGE_HOURS} hours" +%s 2>/dev/null) || {
    print_error "Failed to compute threshold time. Ensure GNU date is available."
    exit 2
}
THRESHOLD_HUMAN=$(date -u -d "-${MAX_AGE_HOURS} hours" "+%Y-%m-%d %H:%M UTC" 2>/dev/null)

# Accumulators for JSON output
AZURE_RGS_JSON="[]"
AZURE_APPS_JSON="[]"
AZURE_SPS_JSON="[]"
STALE_FOUND=false

# Keep errors separate from successfully queried empty collections. JSON mode
# always emits a report, including when Azure could only be partially inspected.
OBSERVED_AT=$(date -u +%Y-%m-%dT%H:%M:%SZ)
SUBSCRIPTION_ID="${AZURE_SUBSCRIPTION_ID:-}"
SCAN_ERRORS_JSON="[]"
AZURE_UNKNOWN_RGS_JSON="[]"
AZURE_DELETING_RGS_JSON="[]"
AZURE_DISAPPEARED_RGS_JSON="[]"
QUERY_OUTPUT=""
QUERY_ERROR=""
QUERY_STDERR=$(mktemp)
trap 'rm -f "$QUERY_STDERR"' EXIT

azure_query() {
    QUERY_ERROR=""
    local status
    if QUERY_OUTPUT=$(az "$@" 2>"$QUERY_STDERR"); then
        return 0
    else
        status=$?
    fi
    QUERY_ERROR=$(cat "$QUERY_STDERR")
    QUERY_ERROR="${QUERY_ERROR:-Azure CLI exited with status $status}"
    return 1
}

record_scan_error() {
    SCAN_ERRORS_JSON=$(jq --arg context "$1" --arg message "$2" \
        '. + [{context: $context, message: $message}]' <<<"$SCAN_ERRORS_JSON")
    print_error "$1: $2"
}

# Only this specific Azure error confirms absence. An authorization, token,
# throttling, or transport error must never be interpreted as a missing group.
group_not_found() {
    [[ "$1" =~ \(ResourceGroupNotFound\) ]]
}

# Validate fields before projecting them: malformed responses must follow the
# incomplete-report path rather than escaping as an unstructured jq failure.
valid_group_json() {
    jq -e 'type == "object" and
        (.name | type == "string" and length > 0) and
        (.tags == null or (.tags | type == "object" and all(.[]; type == "string"))) and
        (.properties == null or (.properties | type == "object"))' >/dev/null 2>&1 <<<"$1"
}

valid_directory_json() {
    jq -e 'type == "array" and all(.[]; type == "object" and
        (.appId | type == "string") and (.displayName | type == "string") and
        (.tags | type == "array" and all(.[]; type == "string")))' >/dev/null 2>&1 <<<"$1"
}

check_azure_stale() {
    if ! command -v az >/dev/null 2>&1; then
        record_scan_error "Azure CLI" "az is not installed"
        return 0
    fi
    if [[ -n "$SUBSCRIPTION_ID" ]] && ! azure_query account set --subscription "$SUBSCRIPTION_ID"; then
        record_scan_error "Select subscription" "$QUERY_ERROR"
        return 0
    fi
    if ! azure_query account show -o json; then
        record_scan_error "Azure account" "$QUERY_ERROR"
        return 0
    fi
    local account_subscription_id
    if ! account_subscription_id=$(jq -er '.id | select(type == "string" and length > 0)' <<<"$QUERY_OUTPUT" 2>/dev/null); then
        record_scan_error "Azure account" "Invalid response or missing subscription ID"
        return 0
    fi
    SUBSCRIPTION_ID="$account_subscription_id"
    check_azure_resource_groups
    check_azure_ad_apps
    check_azure_service_principals
}

check_azure_resource_groups() {
    if ! azure_query group list --subscription "$SUBSCRIPTION_ID" -o json; then
        record_scan_error "List resource groups" "$QUERY_ERROR"
        return 0
    fi
    if ! jq -e 'type == "array"' >/dev/null 2>&1 <<<"$QUERY_OUTPUT"; then
        record_scan_error "List resource groups" "Invalid JSON array"
        return 0
    fi
    local listed_rg
    while IFS= read -r listed_rg; do
        if ! valid_group_json "$listed_rg"; then
            record_scan_error "List resource groups" "Invalid resource-group fields"
            return 0
        fi
    done < <(jq -c '.[]' <<<"$QUERY_OUTPUT")
    local candidates
    candidates=$(jq '[.[] | select(
        .tags."capi-test-created-at" != null or
        (.name | test("^cap[az]-tests(-[a-f0-9]+)?-resgroup$|^capz_node_.*_rg$"))
    )]' <<<"$QUERY_OUTPUT")

    local rg rg_name created_at resources_json resource_count lookup_error detection
    local current_rg state enriched created_epoch
    while IFS= read -r rg; do
        rg_name=$(jq -r '.name' <<<"$rg")
        created_at=$(jq -r '.tags."capi-test-created-at" // empty' <<<"$rg")
        state=$(jq -r '.properties.provisioningState // empty' <<<"$rg")
        resource_count=null
        lookup_error=""
        detection=tag

        if [[ -z "$created_at" && "${state,,}" != "deleting" ]]; then
            detection=convention
            if azure_query resource list --subscription "$SUBSCRIPTION_ID" --resource-group "$rg_name" -o json; then
                resources_json="$QUERY_OUTPUT"
                if jq -e 'type == "array" and all(.[]; type == "object" and
                    (.createdTime == null or (.createdTime | type == "string")))' >/dev/null 2>&1 <<<"$resources_json"; then
                    resource_count=$(jq 'length' <<<"$resources_json")
                    created_at=$(jq -r '[.[].createdTime | select(. != null and . != "")] | sort | .[0] // empty' <<<"$resources_json")
                else
                    lookup_error="Resource lookup returned an invalid JSON array"
                fi
            else
                lookup_error="$QUERY_ERROR"
            fi
            if [[ -z "$created_at" && -z "$lookup_error" ]]; then
                if azure_query monitor activity-log list --subscription "$SUBSCRIPTION_ID" --resource-group "$rg_name" --offset 90d \
                    --query "[?operationName.value=='Microsoft.Resources/subscriptions/resourceGroups/write'] | sort_by(@, &eventTimestamp) | [0].eventTimestamp" -o json; then
                    if jq -e '. == null or type == "string"' >/dev/null 2>&1 <<<"$QUERY_OUTPUT"; then
                        created_at=$(jq -r '. // empty' <<<"$QUERY_OUTPUT")
                    else
                        lookup_error="Activity lookup returned an invalid timestamp response"
                    fi
                else
                    lookup_error="$QUERY_ERROR"
                fi
            fi
            if [[ -n "$created_at" ]] && ! date -d "$created_at" +%s >/dev/null 2>&1; then
                lookup_error="Azure lookup returned an invalid creation timestamp"
            fi
        fi

        # Revalidate after the age/resource lookups, for tagged and untagged
        # candidates alike. A list snapshot alone is never a final finding.
        if ! azure_query group show --subscription "$SUBSCRIPTION_ID" --name "$rg_name" -o json; then
            if group_not_found "$QUERY_ERROR"; then
                AZURE_DISAPPEARED_RGS_JSON=$(jq --arg name "$rg_name" \
                    '. + [{name: $name, detection: "disappeared during scan"}]' <<<"$AZURE_DISAPPEARED_RGS_JSON")
                # A separate non-404 lookup failure still makes the scan incomplete.
                if [[ -n "$lookup_error" ]] && ! group_not_found "$lookup_error"; then
                    record_scan_error "Inspect $rg_name" "$lookup_error"
                fi
            else
                record_scan_error "Revalidate $rg_name" "$QUERY_ERROR"
            fi
            continue
        fi
        current_rg="$QUERY_OUTPUT"
        if ! valid_group_json "$current_rg" || [[ "$(jq -r '.name' <<<"$current_rg")" != "$rg_name" ]]; then
            record_scan_error "Revalidate $rg_name" "Invalid resource-group fields"
            continue
        fi
        if ! state=$(jq -er '.properties.provisioningState | select(type == "string" and length > 0)' <<<"$current_rg" 2>/dev/null); then
            record_scan_error "Revalidate $rg_name" "Invalid response or missing provisioning state"
            continue
        fi
        if [[ -n "$lookup_error" ]]; then
            record_scan_error "Inspect $rg_name" "$lookup_error"
            continue
        fi
        # Prefer the current ownership timestamp if tags changed during the scan.
        created_at=$(jq -r --arg fallback "$created_at" '.tags."capi-test-created-at" // $fallback' <<<"$current_rg")
        enriched=$(jq --arg createdAt "${created_at:-unknown}" --arg detection "$detection" \
            --argjson resourceCount "$resource_count" \
            '{name, location, createdAt: $createdAt, user: .tags."capi-test-user",
              env: .tags."capi-test-env", runId: .tags."capi-test-run-id",
              provisioningState: .properties.provisioningState, detection: $detection,
              resourceCount: $resourceCount}' <<<"$current_rg")
        if [[ "${state,,}" == "deleting" ]]; then
            AZURE_DELETING_RGS_JSON=$(jq --argjson rg "$enriched" '. + [$rg]' <<<"$AZURE_DELETING_RGS_JSON")
        elif [[ -z "$created_at" || "$created_at" == "None" || "$created_at" == "null" ]] || \
            ! created_epoch=$(date -d "$created_at" +%s 2>/dev/null); then
            enriched=$(jq '.createdAt = "unknown"' <<<"$enriched")
            AZURE_UNKNOWN_RGS_JSON=$(jq --argjson rg "$enriched" '. + [$rg]' <<<"$AZURE_UNKNOWN_RGS_JSON")
        elif [[ "$created_epoch" -lt "$THRESHOLD_EPOCH" ]]; then
            STALE_FOUND=true
            AZURE_RGS_JSON=$(jq --argjson rg "$enriched" '. + [$rg]' <<<"$AZURE_RGS_JSON")
            # Preserve existing retroactive tagging for confirmed convention findings.
            if [[ "$detection" == "convention" ]]; then
                if ! az tag update --resource-id "/subscriptions/${SUBSCRIPTION_ID}/resourceGroups/${rg_name}" \
                    --operation Merge --tags "capi-test-created-at=$created_at" \
                    "capi-test-detection=convention-retroactive" >/dev/null 2>&1; then
                    print_warning "Could not retroactively tag $rg_name (non-fatal)"
                fi
            fi
        fi
    done < <(jq -c '.[]' <<<"$candidates")
}

check_azure_ad_apps() {
    print_info "Scanning Azure AD Applications for stale entries..."

    # Query all AD apps that have capi-test tags in their tag array
    local apps_json
    if azure_query ad app list --all --query "[?tags[?starts_with(@, 'capi-test-created-at:')]].{appId: appId, displayName: displayName, tags: tags}" -o json; then
        apps_json="$QUERY_OUTPUT"
    else
        record_scan_error "List Azure AD Applications" "$QUERY_ERROR"
        return 0
    fi
    if ! valid_directory_json "$apps_json"; then
        record_scan_error "List Azure AD Applications" "Invalid JSON array or application fields"
        return 0
    fi

    local total
    total=$(echo "$apps_json" | jq 'length')

    if [[ "$total" -eq 0 ]]; then
        print_success "No tagged AD Applications found"
        return 0
    fi

    local stale_apps="[]"

    while IFS= read -r app; do
        local created_at
        created_at=$(echo "$app" | jq -r '[.tags[] | select(startswith("capi-test-created-at:"))] | .[0] // empty' | sed 's/^capi-test-created-at://')
        [[ -z "$created_at" ]] && continue

        local created_epoch
        created_epoch=$(date -d "$created_at" +%s 2>/dev/null) || continue

        if [[ "$created_epoch" -lt "$THRESHOLD_EPOCH" ]]; then
            local enriched
            enriched=$(echo "$app" | jq --arg created "$created_at" '{appId: .appId, displayName: .displayName, createdAt: $created}')
            stale_apps=$(echo "$stale_apps" | jq --argjson app "$enriched" '. + [$app]')
        fi
    done < <(echo "$apps_json" | jq -c '.[]')

    local stale_count
    stale_count=$(echo "$stale_apps" | jq 'length')

    if [[ "$stale_count" -gt 0 ]]; then
        STALE_FOUND=true
        AZURE_APPS_JSON="$stale_apps"

        if [[ "$JSON_OUTPUT" != "true" ]]; then
            echo ""
            print_warning "Found ${stale_count} stale Azure AD Application(s):"
            echo ""
            printf "%-40s | %-38s | %-25s\n" "DISPLAY NAME" "APP ID" "CREATED AT"
            printf "%s\n" "$(printf '%.0s-' {1..108})"
            echo "$stale_apps" | jq -r '.[] | "\(.displayName)|\(.appId)|\(.createdAt)"' | while IFS='|' read -r name appId created; do
                printf "%-40s | %-38s | %-25s\n" "${name:0:40}" "${appId:0:38}" "${created:0:25}"
            done
            echo ""
        fi
    else
        print_success "No stale AD Applications (checked ${total} tagged apps)"
    fi
}

check_azure_service_principals() {
    print_info "Scanning Azure Service Principals for stale entries..."

    local sps_json
    if azure_query ad sp list --all --query "[?tags[?starts_with(@, 'capi-test-created-at:')]].{id: id, appId: appId, displayName: displayName, tags: tags}" -o json; then
        sps_json="$QUERY_OUTPUT"
    else
        record_scan_error "List Azure Service Principals" "$QUERY_ERROR"
        return 0
    fi
    if ! valid_directory_json "$sps_json"; then
        record_scan_error "List Azure Service Principals" "Invalid JSON array or service-principal fields"
        return 0
    fi

    local total
    total=$(echo "$sps_json" | jq 'length')

    if [[ "$total" -eq 0 ]]; then
        print_success "No tagged Service Principals found"
        return 0
    fi

    local stale_sps="[]"

    while IFS= read -r sp; do
        local created_at
        created_at=$(echo "$sp" | jq -r '[.tags[] | select(startswith("capi-test-created-at:"))] | .[0] // empty' | sed 's/^capi-test-created-at://')
        [[ -z "$created_at" ]] && continue

        local created_epoch
        created_epoch=$(date -d "$created_at" +%s 2>/dev/null) || continue

        if [[ "$created_epoch" -lt "$THRESHOLD_EPOCH" ]]; then
            local enriched
            enriched=$(echo "$sp" | jq --arg created "$created_at" '{id: .id, appId: .appId, displayName: .displayName, createdAt: $created}')
            stale_sps=$(echo "$stale_sps" | jq --argjson sp "$enriched" '. + [$sp]')
        fi
    done < <(echo "$sps_json" | jq -c '.[]')

    local stale_count
    stale_count=$(echo "$stale_sps" | jq 'length')

    if [[ "$stale_count" -gt 0 ]]; then
        STALE_FOUND=true
        AZURE_SPS_JSON="$stale_sps"

        if [[ "$JSON_OUTPUT" != "true" ]]; then
            echo ""
            print_warning "Found ${stale_count} stale Service Principal(s):"
            echo ""
            printf "%-40s | %-38s | %-25s\n" "DISPLAY NAME" "APP ID" "CREATED AT"
            printf "%s\n" "$(printf '%.0s-' {1..108})"
            echo "$stale_sps" | jq -r '.[] | "\(.displayName)|\(.appId)|\(.createdAt)"' | while IFS='|' read -r name appId created; do
                printf "%-40s | %-38s | %-25s\n" "${name:0:40}" "${appId:0:38}" "${created:0:25}"
            done
            echo ""
        fi
    else
        print_success "No stale Service Principals (checked ${total} tagged SPs)"
    fi
}

# ─── Output ─────────────────────────────────────────────────────────────────────

generate_summary() {
    local parts=()

    local azure_rg_count azure_app_count azure_sp_count
    azure_rg_count=$(echo "$AZURE_RGS_JSON" | jq 'length')
    azure_app_count=$(echo "$AZURE_APPS_JSON" | jq 'length')
    azure_sp_count=$(echo "$AZURE_SPS_JSON" | jq 'length')

    [[ "$azure_rg_count" -gt 0 ]] && parts+=("${azure_rg_count} Azure resource group(s)")
    [[ "$azure_app_count" -gt 0 ]] && parts+=("${azure_app_count} Azure AD app(s)")
    [[ "$azure_sp_count" -gt 0 ]] && parts+=("${azure_sp_count} Azure service principal(s)")

    local total=$((azure_rg_count + azure_app_count + azure_sp_count))

    if [[ "$SCAN_ERRORS_JSON" != "[]" ]]; then
        echo "Scan incomplete: $(jq length <<<"$SCAN_ERRORS_JSON") Azure lookup error(s); confirmed stale resources: $total"
    elif [[ ${#parts[@]} -eq 0 ]]; then
        echo "No stale resources found (threshold: ${MAX_AGE_HOURS}h)"
    else
        local joined
        joined=$(IFS=', '; echo "${parts[*]}")
        echo "Found ${total} stale resource(s) older than ${MAX_AGE_HOURS}h: ${joined}"
    fi
}

output_json() {
    local summary
    summary=$(generate_summary)

    local total
    total=$(( $(echo "$AZURE_RGS_JSON" | jq 'length') + $(echo "$AZURE_APPS_JSON" | jq 'length') + \
              $(echo "$AZURE_SPS_JSON" | jq 'length') ))

    jq -n \
        --arg observed_at "$OBSERVED_AT" \
        --arg subscription_id "$SUBSCRIPTION_ID" \
        --argjson errors "$SCAN_ERRORS_JSON" \
        --argjson unknown_rgs "$AZURE_UNKNOWN_RGS_JSON" \
        --argjson deleting_rgs "$AZURE_DELETING_RGS_JSON" \
        --argjson disappeared_rgs "$AZURE_DISAPPEARED_RGS_JSON" \
        --argjson stale_found "$( [[ "$STALE_FOUND" == "true" ]] && echo "true" || echo "false" )" \
        --argjson total_count "$total" \
        --arg max_age_hours "$MAX_AGE_HOURS" \
        --arg threshold "$THRESHOLD_HUMAN" \
        --arg summary "$summary" \
        --argjson azure_rgs "$AZURE_RGS_JSON" \
        --argjson azure_apps "$AZURE_APPS_JSON" \
        --argjson azure_sps "$AZURE_SPS_JSON" \
        '{
            observed_at: $observed_at,
            subscription_id: $subscription_id,
            scan_complete: ($errors | length == 0),
            errors: $errors,
            stale_found: $stale_found,
            total_count: $total_count,
            max_age_hours: $max_age_hours,
            threshold: $threshold,
            azure: {
                resource_groups: $azure_rgs,
                unknown_age_resource_groups: $unknown_rgs,
                deleting_resource_groups: $deleting_rgs,
                disappeared_resource_groups: $disappeared_rgs,
                ad_applications: $azure_apps,
                service_principals: $azure_sps
            },
            summary: $summary
        }'
}

# ─── Main ────────────────────────────────────────────────────────────────────────

main() {
    if [[ "$JSON_OUTPUT" != "true" ]]; then
        echo "========================================"
        echo "=== Stale Resource Detection ==="
        echo "========================================"
        echo ""
        print_info "Staleness threshold: ${MAX_AGE_HOURS} hours (before ${THRESHOLD_HUMAN})"
        print_info "Provider: Azure"
        print_info "Observed at: $OBSERVED_AT"
        echo ""
    fi

    if [[ "$CHECK_AZURE" == "true" ]]; then
        check_azure_stale
        [[ "$JSON_OUTPUT" != "true" ]] && echo ""
    fi

    if [[ "$JSON_OUTPUT" == "true" ]]; then
        output_json
    else
        echo "========================================"
        print_info "Subscription: ${SUBSCRIPTION_ID:-unknown}"
        local summary
        summary=$(generate_summary)
        local category groups
        for category in stale unknown-age deleting disappeared; do
            case "$category" in
                stale) groups="$AZURE_RGS_JSON" ;;
                unknown-age) groups="$AZURE_UNKNOWN_RGS_JSON" ;;
                deleting) groups="$AZURE_DELETING_RGS_JSON" ;;
                disappeared) groups="$AZURE_DISAPPEARED_RGS_JSON" ;;
            esac
            jq -r --arg category "$category" '.[] | "\($category): \(.name) (state: \(.provisioningState // "absent"), created: \(.createdAt // "unknown"))"' <<<"$groups"
        done
        if [[ "$SCAN_ERRORS_JSON" != "[]" ]]; then
            print_warning "$summary"
        elif [[ "$STALE_FOUND" == "true" ]]; then
            print_warning "$summary"
        else
            print_success "$summary"
        fi
        echo "========================================"
    fi

    if [[ "$SCAN_ERRORS_JSON" != "[]" ]]; then
        exit 2
    fi
    if [[ "$STALE_FOUND" == "true" ]]; then
        exit 1
    fi
    exit 0
}

main "$@"
