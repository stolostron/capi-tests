#!/usr/bin/env bash
# check-stale-resources.sh - Detect stale cloud resources left by CAPI test runs
#
# Scans Azure and AWS environments for resources that are older than a configurable
# threshold, indicating they were leaked by failed or interrupted test runs.
#
# Usage:
#   ./scripts/check-stale-resources.sh [OPTIONS]
#
# Options:
#   --max-age HOURS    Staleness threshold in hours (default: 24)
#   --azure            Check Azure resources only
#   --aws              Check AWS resources only
#   --json             Output results as JSON (for GHA workflow parsing)
#   --help             Show this help message
#
# Environment variables:
#   CAPI_USER              User prefix for resource matching (default: $USER)
#   AZURE_SUBSCRIPTION_ID  Azure subscription to scan (for Azure mode)
#   AWS_REGION             AWS region to scan (for AWS mode)
#   AWS_ACCESS_KEY_ID      AWS credentials (for AWS mode)
#   AWS_SECRET_ACCESS_KEY  AWS credentials (for AWS mode)
#
# Exit codes:
#   0 - No stale resources found
#   1 - Stale resources detected
#   2 - Error (missing prerequisites, auth failure, etc.)
#
# Examples:
#   ./scripts/check-stale-resources.sh                           # Check both Azure and AWS
#   ./scripts/check-stale-resources.sh --azure --max-age 12      # Azure only, 12h threshold
#   ./scripts/check-stale-resources.sh --aws --json              # AWS only, JSON output
#   ./scripts/check-stale-resources.sh --json --max-age 48       # Both providers, 48h, JSON

set -euo pipefail

# Defaults
MAX_AGE_HOURS=24
CHECK_AZURE=false
CHECK_AWS=false
JSON_OUTPUT=false
EXPLICIT_PROVIDER=false

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
            EXPLICIT_PROVIDER=true
            shift
            ;;
        --aws)
            CHECK_AWS=true
            EXPLICIT_PROVIDER=true
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

# Default: check both providers when none specified explicitly
if [[ "$EXPLICIT_PROVIDER" == "false" ]]; then
    CHECK_AZURE=true
    CHECK_AWS=true
fi

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
AWS_VPCS_JSON="[]"
AWS_STACKS_JSON="[]"
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

    # 2. AD Applications with capi-test prefix patterns
    check_azure_ad_apps

    # 3. Service Principals with capi-test prefix patterns
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

# ─── AWS Detection ──────────────────────────────────────────────────────────────

check_aws_stale() {
    if ! command -v aws >/dev/null 2>&1; then
        print_warning "AWS CLI not installed — skipping AWS check"
        return 0
    fi

    if ! aws sts get-caller-identity >/dev/null 2>&1; then
        print_warning "Not authenticated to AWS — skipping AWS check"
        return 0
    fi

    local region="${AWS_REGION:-us-east-1}"
    print_info "Checking AWS (${region}) for stale resources (older than ${MAX_AGE_HOURS}h)..."

    check_aws_vpcs "$region"
    check_aws_cloudformation "$region"
}

check_aws_vpcs() {
    local region="$1"
    print_info "Scanning AWS VPCs for stale CAPI test resources..."

    # Look for VPCs tagged with sigs.k8s.io/cluster-api-provider-aws/cluster/* keys
    # which are created by CAPA during ROSA cluster provisioning
    local vpcs_json
    vpcs_json=$(aws ec2 describe-vpcs \
        --region "$region" \
        --filters "Name=tag-key,Values=sigs.k8s.io/cluster-api-provider-aws/cluster/*" \
        --query "Vpcs[].{VpcId: VpcId, Tags: Tags, State: State}" \
        --output json 2>/dev/null) || {
        print_warning "Failed to query AWS VPCs"
        return 0
    }

    local total
    total=$(echo "$vpcs_json" | jq 'length')

    if [[ "$total" -eq 0 ]]; then
        print_success "No CAPA-tagged VPCs found"
        return 0
    fi

    local stale_vpcs="[]"

    while IFS= read -r vpc; do
        local vpc_id
        vpc_id=$(echo "$vpc" | jq -r '.VpcId')

        # Get VPC creation time from its flow logs or use the Name tag timestamp
        # VPCs don't have a native creation timestamp, so we check the creation time
        # of the associated CAPA cluster tag
        local name_tag
        name_tag=$(echo "$vpc" | jq -r '[.Tags[] | select(.Key == "Name")] | .[0].Value // "unknown"')

        # Use the VPC's earliest network interface creation as a proxy for VPC age
        local earliest_eni
        earliest_eni=$(aws ec2 describe-network-interfaces \
            --region "$region" \
            --filters "Name=vpc-id,Values=${vpc_id}" \
            --query "sort_by(NetworkInterfaces, &Attachment.AttachTime)[0].Attachment.AttachTime" \
            --output text 2>/dev/null) || earliest_eni=""

        if [[ -n "$earliest_eni" && "$earliest_eni" != "None" && "$earliest_eni" != "null" ]]; then
            local created_epoch
            created_epoch=$(date -d "$earliest_eni" +%s 2>/dev/null) || continue

            if [[ "$created_epoch" -lt "$THRESHOLD_EPOCH" ]]; then
                local enriched
                enriched=$(jq -n --arg id "$vpc_id" --arg name "$name_tag" --arg created "$earliest_eni" \
                    '{vpcId: $id, name: $name, createdAt: $created}')
                stale_vpcs=$(echo "$stale_vpcs" | jq --argjson vpc "$enriched" '. + [$vpc]')
            fi
        fi
    done < <(echo "$vpcs_json" | jq -c '.[]')

    local stale_count
    stale_count=$(echo "$stale_vpcs" | jq 'length')

    if [[ "$stale_count" -gt 0 ]]; then
        STALE_FOUND=true
        AWS_VPCS_JSON="$stale_vpcs"

        if [[ "$JSON_OUTPUT" != "true" ]]; then
            echo ""
            print_warning "Found ${stale_count} stale AWS VPC(s):"
            echo ""
            printf "%-25s | %-40s | %-25s\n" "VPC ID" "NAME" "CREATED AT"
            printf "%s\n" "$(printf '%.0s-' {1..95})"
            echo "$stale_vpcs" | jq -r '.[] | "\(.vpcId)|\(.name)|\(.createdAt)"' | while IFS='|' read -r id name created; do
                printf "%-25s | %-40s | %-25s\n" "$id" "${name:0:40}" "${created:0:25}"
            done
            echo ""
        fi
    else
        print_success "No stale VPCs (checked ${total} CAPA-tagged VPCs)"
    fi
}

check_aws_cloudformation() {
    local region="$1"
    print_info "Scanning AWS CloudFormation stacks for stale CAPI resources..."

    local stacks_json
    stacks_json=$(aws cloudformation list-stacks \
        --region "$region" \
        --stack-status-filter CREATE_COMPLETE UPDATE_COMPLETE ROLLBACK_COMPLETE \
        --query "StackSummaries[?contains(StackName, 'capa-') || contains(StackName, 'rosa-')].{StackName: StackName, CreationTime: CreationTime, StackStatus: StackStatus}" \
        --output json 2>/dev/null) || {
        print_warning "Failed to query AWS CloudFormation stacks"
        return 0
    }

    local total
    total=$(echo "$stacks_json" | jq 'length')

    if [[ "$total" -eq 0 ]]; then
        print_success "No CAPI-related CloudFormation stacks found"
        return 0
    fi

    local stale_stacks="[]"

    while IFS= read -r stack; do
        local created_at
        created_at=$(echo "$stack" | jq -r '.CreationTime')
        local created_epoch
        created_epoch=$(date -d "$created_at" +%s 2>/dev/null) || continue

        if [[ "$created_epoch" -lt "$THRESHOLD_EPOCH" ]]; then
            stale_stacks=$(echo "$stale_stacks" | jq --argjson stack "$stack" '. + [$stack]')
        fi
    done < <(echo "$stacks_json" | jq -c '.[]')

    local stale_count
    stale_count=$(echo "$stale_stacks" | jq 'length')

    if [[ "$stale_count" -gt 0 ]]; then
        STALE_FOUND=true
        AWS_STACKS_JSON="$stale_stacks"

        if [[ "$JSON_OUTPUT" != "true" ]]; then
            echo ""
            print_warning "Found ${stale_count} stale CloudFormation stack(s):"
            echo ""
            printf "%-50s | %-25s | %-20s\n" "STACK NAME" "CREATED AT" "STATUS"
            printf "%s\n" "$(printf '%.0s-' {1..100})"
            echo "$stale_stacks" | jq -r '.[] | "\(.StackName)|\(.CreationTime)|\(.StackStatus)"' | while IFS='|' read -r name created status; do
                printf "%-50s | %-25s | %-20s\n" "${name:0:50}" "${created:0:25}" "$status"
            done
            echo ""
        fi
    else
        print_success "No stale CloudFormation stacks (checked ${total} stacks)"
    fi
}

# ─── Output ─────────────────────────────────────────────────────────────────────

generate_summary() {
    local parts=()

    local azure_rg_count azure_app_count azure_sp_count aws_vpc_count aws_stack_count
    azure_rg_count=$(echo "$AZURE_RGS_JSON" | jq 'length')
    azure_app_count=$(echo "$AZURE_APPS_JSON" | jq 'length')
    azure_sp_count=$(echo "$AZURE_SPS_JSON" | jq 'length')
    aws_vpc_count=$(echo "$AWS_VPCS_JSON" | jq 'length')
    aws_stack_count=$(echo "$AWS_STACKS_JSON" | jq 'length')

    [[ "$azure_rg_count" -gt 0 ]] && parts+=("${azure_rg_count} Azure resource group(s)")
    [[ "$azure_app_count" -gt 0 ]] && parts+=("${azure_app_count} Azure AD app(s)")
    [[ "$azure_sp_count" -gt 0 ]] && parts+=("${azure_sp_count} Azure service principal(s)")
    [[ "$aws_vpc_count" -gt 0 ]] && parts+=("${aws_vpc_count} AWS VPC(s)")
    [[ "$aws_stack_count" -gt 0 ]] && parts+=("${aws_stack_count} AWS CloudFormation stack(s)")

    local total=$((azure_rg_count + azure_app_count + azure_sp_count + aws_vpc_count + aws_stack_count))

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
              $(echo "$AZURE_SPS_JSON" | jq 'length') + $(echo "$AWS_VPCS_JSON" | jq 'length') + \
              $(echo "$AWS_STACKS_JSON" | jq 'length') ))

    jq -n \
        --argjson stale_found "$( [[ "$STALE_FOUND" == "true" ]] && echo "true" || echo "false" )" \
        --argjson total_count "$total" \
        --arg max_age_hours "$MAX_AGE_HOURS" \
        --arg threshold "$THRESHOLD_HUMAN" \
        --arg summary "$summary" \
        --argjson azure_rgs "$AZURE_RGS_JSON" \
        --argjson azure_apps "$AZURE_APPS_JSON" \
        --argjson azure_sps "$AZURE_SPS_JSON" \
        --argjson aws_vpcs "$AWS_VPCS_JSON" \
        --argjson aws_stacks "$AWS_STACKS_JSON" \
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
            aws: {
                vpcs: $aws_vpcs,
                cloudformation_stacks: $aws_stacks
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
        print_info "Providers: $( [[ "$CHECK_AZURE" == "true" ]] && echo -n "Azure " )$( [[ "$CHECK_AWS" == "true" ]] && echo -n "AWS" )"
        echo ""
    fi

    if [[ "$CHECK_AZURE" == "true" ]]; then
        check_azure_stale
        [[ "$JSON_OUTPUT" != "true" ]] && echo ""
    fi

    if [[ "$CHECK_AWS" == "true" ]]; then
        check_aws_stale
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
