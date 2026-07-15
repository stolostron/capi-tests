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
#   0 - No stale resources found
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
THRESHOLD_HUMAN=$(date -d "-${MAX_AGE_HOURS} hours" "+%Y-%m-%d %H:%M UTC" 2>/dev/null)

# Accumulators for JSON output
AZURE_RGS_JSON="[]"
AZURE_APPS_JSON="[]"
AZURE_SPS_JSON="[]"
STALE_FOUND=false

# ─── Azure Detection ────────────────────────────────────────────────────────────

check_azure_stale() {
    if ! command -v az >/dev/null 2>&1; then
        print_warning "Azure CLI (az) not installed — skipping Azure check"
        return 0
    fi

    if ! az account show >/dev/null 2>&1; then
        print_warning "Not logged in to Azure CLI — skipping Azure check"
        return 0
    fi

    if [[ -n "${AZURE_SUBSCRIPTION_ID:-}" ]]; then
        az account set --subscription "$AZURE_SUBSCRIPTION_ID" >/dev/null 2>&1 || {
            print_error "Failed to select Azure subscription '${AZURE_SUBSCRIPTION_ID}'"
            exit 2
        }
    fi

    if ! az extension show --name resource-graph >/dev/null 2>&1; then
        print_info "Installing Azure Resource Graph extension..."
        az extension add --name resource-graph --yes 2>/dev/null
    fi

    print_info "Checking Azure for stale resources (older than ${MAX_AGE_HOURS}h, before ${THRESHOLD_HUMAN})..."

    # 1. Resource groups with capi-test-created-at tag
    check_azure_resource_groups

    # 2. Resource groups by naming convention (catches untagged orphans from Prow CI, failed runs)
    check_azure_resource_groups_by_convention

    # 3. AD Applications with capi-test prefix patterns
    check_azure_ad_apps

    # 4. Service Principals with capi-test prefix patterns
    check_azure_service_principals
}

check_azure_resource_groups() {
    print_info "Scanning Azure resource groups with capi-test-created-at tag..."

    local rgs_json
    rgs_json=$(az group list --query "[?tags.\"capi-test-created-at\" != null].{name: name, location: location, createdAt: tags.\"capi-test-created-at\", user: tags.\"capi-test-user\", env: tags.\"capi-test-env\", runId: tags.\"capi-test-run-id\"}" -o json 2>/dev/null) || {
        print_warning "Failed to list Azure resource groups"
        return 0
    }

    local stale_rgs="[]"
    local total
    total=$(echo "$rgs_json" | jq 'length')

    if [[ "$total" -eq 0 ]]; then
        print_success "No tagged resource groups found"
        return 0
    fi

    # Filter by age
    while IFS= read -r rg; do
        local created_at
        created_at=$(echo "$rg" | jq -r '.createdAt')
        local created_epoch
        created_epoch=$(date -d "$created_at" +%s 2>/dev/null) || continue

        if [[ "$created_epoch" -lt "$THRESHOLD_EPOCH" ]]; then
            stale_rgs=$(echo "$stale_rgs" | jq --argjson rg "$rg" '. + [$rg]')
        fi
    done < <(echo "$rgs_json" | jq -c '.[]')

    local stale_count
    stale_count=$(echo "$stale_rgs" | jq 'length')

    if [[ "$stale_count" -gt 0 ]]; then
        STALE_FOUND=true
        AZURE_RGS_JSON="$stale_rgs"

        if [[ "$JSON_OUTPUT" != "true" ]]; then
            echo ""
            print_warning "Found ${stale_count} stale Azure resource group(s):"
            echo ""
            printf "%-35s | %-15s | %-25s | %-10s | %-8s\n" "NAME" "LOCATION" "CREATED AT" "USER" "ENV"
            printf "%s\n" "$(printf '%.0s-' {1..100})"
            echo "$stale_rgs" | jq -r '.[] | "\(.name)|\(.location)|\(.createdAt)|\(.user // "-")|\(.env // "-")"' | while IFS='|' read -r name loc created user env; do
                printf "%-35s | %-15s | %-25s | %-10s | %-8s\n" "${name:0:35}" "${loc:0:15}" "${created:0:25}" "${user:0:10}" "${env:0:8}"
            done
            echo ""
        fi
    else
        print_success "No stale resource groups (checked ${total} tagged groups)"
    fi
}

check_azure_resource_groups_by_convention() {
    print_info "Scanning Azure resource groups by naming convention (untagged orphan detection)..."

    local tagged_rg_names
    tagged_rg_names=$(echo "$AZURE_RGS_JSON" | jq -r '.[].name' 2>/dev/null || echo "")

    # Cache subscription ID for retroactive tagging (computed once, used per RG)
    local subscription_id
    subscription_id=$(az account show --query id -o tsv 2>/dev/null || echo "")

    local all_rgs_json
    all_rgs_json=$(az group list \
        --query "[].{name: name, location: location, tags: tags}" \
        -o json 2>/dev/null) || {
        print_warning "Failed to list Azure resource groups for convention check"
        return 0
    }

    # Match resource groups created by our test suite:
    #   capz-tests-resgroup, capz-tests-abc12-resgroup, capa-tests-d3a0f-resgroup
    #   capz_node_*_rg (Azure-managed node resource groups)
    # Exclude RGs that already have capi-test-created-at tag (handled by tagged detection)
    local convention_rgs
    convention_rgs=$(echo "$all_rgs_json" | jq '[
        .[] | select(
            (.name | test("^cap[az]-tests(-[a-f0-9]+)?-resgroup$"))
            or
            (.name | test("^capz_node_.*_rg$"))
        )
        | select(.tags == null or .tags."capi-test-created-at" == null)
    ]')

    local total
    total=$(echo "$convention_rgs" | jq 'length')

    if [[ "$total" -eq 0 ]]; then
        print_success "No untagged resource groups matching naming convention"
        return 0
    fi

    print_info "Found ${total} untagged resource group(s) matching naming convention — checking age..."

    local stale_convention_rgs="[]"

    while IFS= read -r rg; do
        local rg_name rg_location rg_user rg_env
        rg_name=$(echo "$rg" | jq -r '.name')
        rg_location=$(echo "$rg" | jq -r '.location')
        rg_user=$(echo "$rg" | jq -r '.tags."capi-test-user" // "-"')
        rg_env=$(echo "$rg" | jq -r '.tags."capi-test-env" // "-"')

        # Skip if already found by tagged detection
        if echo "$tagged_rg_names" | grep -qx "$rg_name" 2>/dev/null; then
            continue
        fi

        local created_at=""
        local resource_count
        local resources_json
        resources_json=$(az resource list --resource-group "$rg_name" \
            --query "[].{createdTime: createdTime}" -o json 2>/dev/null) || resources_json="[]"
        resource_count=$(echo "$resources_json" | jq 'length')

        if [[ "$resource_count" -gt 0 ]]; then
            created_at=$(echo "$resources_json" | jq -r '[.[] | .createdTime | select(. != null and . != "")] | sort | .[0] // empty')
        fi

        if [[ -z "$created_at" || "$created_at" == "null" ]]; then
            created_at=$(az monitor activity-log list \
                --resource-group "$rg_name" \
                --offset "90d" \
                --query "[?operationName.value=='Microsoft.Resources/subscriptions/resourceGroups/write'] | sort_by(@, &eventTimestamp) | [0].eventTimestamp" \
                -o tsv 2>/dev/null) || created_at=""
        fi

        if [[ -z "$created_at" || "$created_at" == "null" || "$created_at" == "None" ]]; then
            if [[ "$resource_count" -gt 0 ]]; then
                continue
            fi
            local enriched
            enriched=$(jq -n \
                --arg name "$rg_name" \
                --arg location "$rg_location" \
                --arg createdAt "unknown" \
                --arg user "$rg_user" \
                --arg env "$rg_env" \
                --arg detection "convention (unknown age)" \
                --argjson resourceCount "$resource_count" \
                '{name: $name, location: $location, createdAt: $createdAt, user: $user, env: $env, detection: $detection, resourceCount: $resourceCount}')
            stale_convention_rgs=$(echo "$stale_convention_rgs" | jq --argjson rg "$enriched" '. + [$rg]')
            continue
        fi

        local created_epoch
        created_epoch=$(date -d "$created_at" +%s 2>/dev/null) || continue

        if [[ "$created_epoch" -lt "$THRESHOLD_EPOCH" ]]; then
            local enriched
            enriched=$(jq -n \
                --arg name "$rg_name" \
                --arg location "$rg_location" \
                --arg createdAt "$created_at" \
                --arg user "$rg_user" \
                --arg env "$rg_env" \
                --arg detection "convention" \
                --argjson resourceCount "$resource_count" \
                '{name: $name, location: $location, createdAt: $createdAt, user: $user, env: $env, detection: $detection, resourceCount: $resourceCount}')
            stale_convention_rgs=$(echo "$stale_convention_rgs" | jq --argjson rg "$enriched" '. + [$rg]')

            # Retroactively tag the RG so future scans can find it via capi-test-created-at.
            # Phase 04 defers tagging when ASO hasn't created the RG yet; if the run is killed
            # before Phase 05 retries, the RG ends up permanently untagged.
            # az tag update --operation Merge adds tags without removing existing ones.
            if [[ -n "$subscription_id" ]]; then
                local resource_id="/subscriptions/${subscription_id}/resourceGroups/${rg_name}"
                local tag_json
                tag_json=$(jq -n --arg ts "$created_at" '{"capi-test-created-at": $ts, "capi-test-detection": "convention-retroactive"}')
                if az tag update --resource-id "$resource_id" --operation Merge --tags "$tag_json" \
                        >/dev/null 2>&1; then
                    print_info "Retroactively tagged ${rg_name} with capi-test-created-at=${created_at}"
                else
                    print_warning "Could not retroactively tag ${rg_name} (non-fatal)"
                fi
            fi
        fi
    done < <(echo "$convention_rgs" | jq -c '.[]')

    local stale_count
    stale_count=$(echo "$stale_convention_rgs" | jq 'length')

    if [[ "$stale_count" -gt 0 ]]; then
        STALE_FOUND=true
        AZURE_RGS_JSON=$(echo "$AZURE_RGS_JSON" | jq --argjson convention "$stale_convention_rgs" '. + $convention')

        if [[ "$JSON_OUTPUT" != "true" ]]; then
            echo ""
            print_warning "Found ${stale_count} stale untagged Azure resource group(s) (by naming convention):"
            echo ""
            printf "%-45s | %-15s | %-25s | %-10s | %-5s | %-25s\n" "NAME" "LOCATION" "CREATED AT" "USER" "RES#" "DETECTION"
            printf "%s\n" "$(printf '%.0s-' {1..135})"
            echo "$stale_convention_rgs" | jq -r '.[] | "\(.name)|\(.location)|\(.createdAt)|\(.user // "-")|\(.resourceCount)|\(.detection)"' | while IFS='|' read -r name loc created user rescount detection; do
                printf "%-45s | %-15s | %-25s | %-10s | %-5s | %-25s\n" "${name:0:45}" "${loc:0:15}" "${created:0:25}" "${user:0:10}" "${rescount:0:5}" "${detection:0:25}"
            done
            echo ""
        fi
    else
        print_success "No stale untagged resource groups (checked ${total} by naming convention)"
    fi
}

check_azure_ad_apps() {
    print_info "Scanning Azure AD Applications for stale entries..."

    # Query all AD apps that have capi-test tags in their tag array
    local apps_json
    apps_json=$(az ad app list --all --query "[?tags[?starts_with(@, 'capi-test-created-at:')]].{appId: appId, displayName: displayName, tags: tags}" -o json 2>/dev/null) || {
        print_warning "Failed to list Azure AD Applications"
        return 0
    }

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
    sps_json=$(az ad sp list --all --query "[?tags[?starts_with(@, 'capi-test-created-at:')]].{id: id, appId: appId, displayName: displayName, tags: tags}" -o json 2>/dev/null) || {
        print_warning "Failed to list Azure Service Principals"
        return 0
    }

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

    if [[ ${#parts[@]} -eq 0 ]]; then
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
        --argjson stale_found "$( [[ "$STALE_FOUND" == "true" ]] && echo "true" || echo "false" )" \
        --argjson total_count "$total" \
        --arg max_age_hours "$MAX_AGE_HOURS" \
        --arg threshold "$THRESHOLD_HUMAN" \
        --arg summary "$summary" \
        --argjson azure_rgs "$AZURE_RGS_JSON" \
        --argjson azure_apps "$AZURE_APPS_JSON" \
        --argjson azure_sps "$AZURE_SPS_JSON" \
        '{
            stale_found: $stale_found,
            total_count: $total_count,
            max_age_hours: $max_age_hours,
            threshold: $threshold,
            azure: {
                resource_groups: $azure_rgs,
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
        local summary
        summary=$(generate_summary)
        if [[ "$STALE_FOUND" == "true" ]]; then
            print_error "$summary"
        else
            print_success "$summary"
        fi
        echo "========================================"
    fi

    if [[ "$STALE_FOUND" == "true" ]]; then
        exit 1
    fi
    exit 0
}

main "$@"
