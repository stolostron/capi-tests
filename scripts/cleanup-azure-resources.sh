#!/usr/bin/env bash
# cleanup-azure-resources.sh - Clean up Azure resources created during CAPI testing
#
# This script finds and deletes Azure resources that match the naming patterns used
# during testing. It cleans up:
#   - Azure Resource Group (optional, via --resource-group)
#   - ARM resources (via Azure Resource Graph)
#   - Azure AD Applications (App Registrations)
#   - Service Principals
#
# These resources may not be tied to the resource group and can survive resource
# group deletion.
#
# Usage:
#   ./scripts/cleanup-azure-resources.sh [OPTIONS]
#
# Options:
#   --prefix PREFIX        Resource name prefix to search for (default: CS_CLUSTER_NAME env var, else CAPI_USER-DEPLOYMENT_ENV, else cate)
#                          Note: The Go test suite auto-generates CS_CLUSTER_NAME as CAPI_USER-<random>; this fallback is for standalone script usage.
#   --resource-group RG    Also delete this Azure resource group
#   --match-mode MODE      How to match resource names: 'startswith' (default, safer) or 'contains' (broader)
#   --my-resources         Find all resources tagged with capi-test-user=$CAPI_USER (or $USER fallback) (dry-run)
#   --tag KEY=VALUE        Find resources by Azure tag (e.g., 'capi-test-user=alice')
#   --dry-run              Show what would be deleted without actually deleting
#   --force                Skip confirmation prompts
#   --help                 Show this help message
#
# Environment variables:
#   CS_CLUSTER_NAME    Full cluster name prefix (e.g., 'cate-a1b2c') - preferred, most specific.
#                      The Go test suite auto-generates this as CAPI_USER-<5hex> for parallel runs.
#   CAPI_USER          User prefix for resource names (e.g., 'cate') - fallback if CS_CLUSTER_NAME not set
#   DEPLOYMENT_ENV     Deployment environment (default: 'stage') - combined with CAPI_USER as standalone fallback
#   AZURE_SUBSCRIPTION_ID  Azure subscription ID to search in
#
# Examples:
#   ./scripts/cleanup-azure-resources.sh --dry-run
#   ./scripts/cleanup-azure-resources.sh --prefix cate-stage --force
#   ./scripts/cleanup-azure-resources.sh --resource-group myapp-resgroup --prefix myapp
#   CS_CLUSTER_NAME=cate-stage ./scripts/cleanup-azure-resources.sh
#   ./scripts/cleanup-azure-resources.sh --prefix cate --match-mode contains  # broader search
#   ./scripts/cleanup-azure-resources.sh --my-resources                       # find all my test resources
#   ./scripts/cleanup-azure-resources.sh --tag capi-test-user=alice --force   # delete by tag

set -euo pipefail

# Default values
# Prefer CS_CLUSTER_NAME (e.g., cate-stage) for more specific matching.
# Fall back to CAPI_USER-DEPLOYMENT_ENV, then 'cate' as final fallback.
if [[ -n "${CS_CLUSTER_NAME:-}" ]]; then
    PREFIX="${CS_CLUSTER_NAME}"
elif [[ -n "${CAPI_USER:-}" ]]; then
    PREFIX="${CAPI_USER}-${DEPLOYMENT_ENV:-stage}"
else
    PREFIX="cate"
fi
RESOURCE_GROUP=""
MATCH_MODE="startswith"
TAG_FILTER=""
DRY_RUN=false
FORCE=false

# ANSI colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Print colored output
print_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
print_success() { echo -e "${GREEN}[OK]${NC} $1"; }
print_warning() { echo -e "${YELLOW}[WARN]${NC} $1"; }
print_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Show usage
usage() {
    sed -n '2,/^$/p' "$0" | grep '^#' | sed 's/^# \?//'
    exit 0
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --prefix)
            if [[ $# -lt 2 ]]; then
                print_error "Missing value for --prefix"
                exit 1
            fi
            PREFIX="$2"
            shift 2
            ;;
        --resource-group)
            if [[ $# -lt 2 ]]; then
                print_error "Missing value for --resource-group"
                exit 1
            fi
            RESOURCE_GROUP="$2"
            shift 2
            ;;
        --match-mode)
            if [[ $# -lt 2 ]]; then
                print_error "Missing value for --match-mode"
                exit 1
            fi
            MATCH_MODE="$2"
            if [[ "$MATCH_MODE" != "startswith" && "$MATCH_MODE" != "contains" ]]; then
                print_error "Invalid match mode '${MATCH_MODE}': must be 'startswith' or 'contains'"
                exit 1
            fi
            shift 2
            ;;
        --tag)
            if [[ $# -lt 2 ]]; then
                print_error "Missing value for --tag"
                exit 1
            fi
            TAG_FILTER="$2"
            if [[ ! "$TAG_FILTER" =~ ^[a-zA-Z0-9_-]+=[a-zA-Z0-9_.@:/-]+$ ]]; then
                print_error "Invalid tag format '${TAG_FILTER}': expected KEY=VALUE with alphanumeric, hyphens, dots, @, colons, slashes (e.g., 'capi-test-user=alice')"
                exit 1
            fi
            shift 2
            ;;
        --my-resources)
            # Use CAPI_USER to match the tag set by the Go test suite (config.go).
            # Fall back to USER (OS login) if CAPI_USER is not set.
            my_user="${CAPI_USER:-${USER:-}}"
            if [[ -z "$my_user" ]]; then
                print_error "Neither CAPI_USER nor USER environment variable is set"
                exit 1
            fi
            if [[ ! "$my_user" =~ ^[a-zA-Z0-9_.@/-]+$ ]]; then
                print_error "User identifier '$my_user' contains disallowed characters for tag value"
                exit 1
            fi
            TAG_FILTER="capi-test-user=${my_user}"
            DRY_RUN=true
            shift
            ;;
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        --force)
            FORCE=true
            shift
            ;;
        --help|-h)
            usage
            ;;
        *)
            print_error "Unknown option: $1"
            usage
            ;;
    esac
done

# Validate prefix to prevent OData filter injection (skip when using tag mode only)
# Must be lowercase alphanumeric with optional hyphens, starting with alphanumeric
if [[ -z "$TAG_FILTER" ]] && [[ ! "$PREFIX" =~ ^[a-z0-9][a-z0-9-]*$ ]]; then
    print_error "Invalid prefix '${PREFIX}': must be lowercase alphanumeric with hyphens, starting with alphanumeric"
    exit 1
fi

# Check prerequisites
check_prerequisites() {
    print_info "Checking prerequisites..."

    if ! command -v az >/dev/null 2>&1; then
        print_error "Azure CLI (az) is not installed"
        echo "Install from: https://docs.microsoft.com/en-us/cli/azure/install-azure-cli"
        exit 1
    fi

    if ! az account show >/dev/null 2>&1; then
        print_error "Not logged in to Azure CLI"
        echo "Run 'az login' to authenticate"
        exit 1
    fi

    # Check for resource-graph extension
    if ! az extension show --name resource-graph >/dev/null 2>&1; then
        print_warning "Azure Resource Graph extension not installed"
        print_info "Installing resource-graph extension..."
        az extension add --name resource-graph --yes
    fi

    print_success "Prerequisites check passed"
}

# Delete Azure resource group
delete_resource_group() {
    local rg_name="$1"

    # Check if resource group exists
    if ! az group show --name "$rg_name" >/dev/null 2>&1; then
        print_info "Resource group '${rg_name}' not found (already deleted or doesn't exist)"
        return 0
    fi

    echo ""
    print_warning "Found resource group '${rg_name}'"

    if [[ "$DRY_RUN" == "true" ]]; then
        print_warning "[DRY-RUN] Would delete resource group '${rg_name}'"
        return 0
    fi

    # Confirm deletion unless --force is specified
    if [[ "$FORCE" != "true" ]]; then
        echo ""
        echo "⚠️  Warning: This will delete ALL resources in the resource group!"
        read -p "Delete Azure resource group '${rg_name}'? [y/N] " -n 1 -r
        echo ""
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            print_info "Resource group deletion cancelled"
            return 0
        fi
    fi

    echo ""
    print_info "Deleting resource group '${rg_name}' (this may take several minutes)..."

    if az group delete --name "$rg_name" --yes 2>/dev/null; then
        print_success "Resource group deleted"
    else
        print_error "Failed to delete resource group '${rg_name}'"
        return 1
    fi
}

# Find resources matching the pattern
find_resources() {
    local prefix="$1"

    print_info "Searching for Azure resources with prefix '${prefix}' (mode: ${MATCH_MODE})..." >&2

    # Build the query based on match mode:
    # - startswith (default): safer, only matches resources whose names begin with the prefix
    # - contains: broader, matches resources whose names contain the prefix anywhere
    local query
    if [[ "$MATCH_MODE" == "startswith" ]]; then
        query="Resources | where name startswith '${prefix}' | project id, name, type, resourceGroup, subscriptionId | order by type asc, name asc"
    else
        query="Resources | where name contains '${prefix}' | project id, name, type, resourceGroup, subscriptionId | order by type asc, name asc"
    fi

    local resources_json
    resources_json=$(az graph query -q "$query" -o json 2>/dev/null)
    if [[ $? -ne 0 ]]; then
        print_error "Failed to search for ARM resources with prefix '${prefix}'" >&2
        echo '{"data": []}'
        return
    fi
    echo "$resources_json"
}

# Find Azure AD Applications matching the prefix
find_ad_applications() {
    local prefix="$1"

    print_info "Searching for Azure AD Applications with prefix '${prefix}'..." >&2

    # Use az ad app list with OData filter for displayName starting with prefix
    # Note: OData filters for az ad only support 'startswith', not 'contains',
    # so --match-mode does not apply here.
    local apps_json
    apps_json=$(az ad app list --filter "startswith(displayName, '${prefix}')" --query "[].{appId: appId, displayName: displayName}" -o json 2>/dev/null)
    if [[ $? -ne 0 ]]; then
        print_error "Failed to list Azure AD Applications with prefix '${prefix}'" >&2
        echo "[]"
        return
    fi
    echo "$apps_json"
}

# Find Service Principals matching the prefix
find_service_principals() {
    local prefix="$1"

    print_info "Searching for Service Principals with prefix '${prefix}'..." >&2

    # Use az ad sp list with OData filter for displayName starting with prefix
    # Note: OData filters for az ad only support 'startswith', not 'contains',
    # so --match-mode does not apply here.
    local sps_json
    sps_json=$(az ad sp list --filter "startswith(displayName, '${prefix}')" --query "[].{appId: appId, displayName: displayName, id: id}" -o json 2>/dev/null)
    if [[ $? -ne 0 ]]; then
        print_error "Failed to list Service Principals with prefix '${prefix}'" >&2
        echo "[]"
        return
    fi
    echo "$sps_json"
}

# Find resources by Azure tag
find_resources_by_tag() {
    local tag_filter="$1"
    local tag_key="${tag_filter%%=*}"
    local tag_value="${tag_filter#*=}"

    print_info "Searching for Azure resources with tag '${tag_key}=${tag_value}'..." >&2

    local query="Resources | where tags['${tag_key}'] == '${tag_value}' | project id, name, type, resourceGroup, subscriptionId | order by resourceGroup asc, type asc, name asc"

    local resources_json
    local az_stderr
    az_stderr=$(mktemp)
    if ! resources_json=$(az graph query -q "$query" -o json 2>"$az_stderr"); then
        print_error "Failed to search for resources with tag '${tag_filter}': $(cat "$az_stderr")" >&2
        rm -f "$az_stderr"
        return 1
    fi
    rm -f "$az_stderr"
    echo "$resources_json"
}

# Find resource groups by Azure tag
find_resource_groups_by_tag() {
    local tag_filter="$1"
    local tag_key="${tag_filter%%=*}"
    local tag_value="${tag_filter#*=}"

    print_info "Searching for resource groups with tag '${tag_key}=${tag_value}'..." >&2

    local rgs_json
    local az_stderr
    az_stderr=$(mktemp)
    if ! rgs_json=$(az group list --tag "${tag_key}=${tag_value}" --query "[].{name: name, location: location}" -o json 2>"$az_stderr"); then
        print_error "Failed to search for resource groups with tag '${tag_filter}': $(cat "$az_stderr")" >&2
        rm -f "$az_stderr"
        return 1
    fi
    rm -f "$az_stderr"
    echo "$rgs_json"
}

# Parse and display resources
display_resources() {
    local resources_json="$1"
    local count

    # Handle empty or invalid JSON
    if [[ -z "$resources_json" ]] || ! echo "$resources_json" | jq -e '.' >/dev/null 2>&1; then
        print_info "No resources found matching prefix '${PREFIX}'"
        return 1
    fi

    count=$(echo "$resources_json" | jq -r '.data | length // 0')

    if [[ "$count" -eq 0 ]]; then
        print_info "No resources found matching prefix '${PREFIX}'"
        return 1
    fi

    echo ""
    print_warning "Found ${count} resource(s) matching prefix '${PREFIX}':"
    echo ""

    # Print table header
    printf "%-60s | %-50s | %-30s\n" "NAME" "TYPE" "RESOURCE GROUP"
    printf "%s\n" "$(printf '%.0s-' {1..145})"

    # Print each resource
    echo "$resources_json" | jq -r '.data[] | "\(.name)|\(.type)|\(.resourceGroup)"' | while IFS='|' read -r name type rg; do
        # Truncate long names for display
        name_display="${name:0:60}"
        type_display="${type:0:50}"
        rg_display="${rg:0:30}"
        printf "%-60s | %-50s | %-30s\n" "$name_display" "$type_display" "$rg_display"
    done

    echo ""
    return 0
}

# Delete resources
delete_resources() {
    local resources_json="$1"
    local count
    local deleted=0
    local failed=0
    local skipped=0

    count=$(echo "$resources_json" | jq -r '.data | length')

    if [[ "$count" -eq 0 ]]; then
        return 0
    fi

    if [[ "$DRY_RUN" == "true" ]]; then
        print_warning "[DRY-RUN] Would delete ${count} resource(s)"
        return 0
    fi

    # Confirm deletion unless --force is specified
    if [[ "$FORCE" != "true" ]]; then
        echo ""
        read -p "Delete all ${count} resource(s)? [y/N] " -n 1 -r
        echo ""
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            print_info "Deletion cancelled"
            return 0
        fi
    fi

    echo ""
    print_info "Deleting resources..."
    echo ""

    # Get resource IDs and delete them
    # Sort by type to handle dependencies (identities first, then VNets, then NSGs)
    local resource_ids
    resource_ids=$(echo "$resources_json" | jq -r '.data | sort_by(.type) | reverse | .[].id')

    while IFS= read -r resource_id; do
        if [[ -z "$resource_id" ]]; then
            continue
        fi

        local resource_name
        resource_name=$(basename "$resource_id")

        echo -n "  Deleting: ${resource_name}... "

        # First verify the resource still exists (Resource Graph may have stale data)
        if ! az resource show --ids "$resource_id" >/dev/null 2>&1; then
            echo "SKIPPED (not found)"
            ((skipped++)) || true
            continue
        fi

        # Attempt deletion
        if az resource delete --ids "$resource_id" --no-wait 2>/dev/null; then
            echo "INITIATED"
            ((deleted++)) || true
        else
            echo "FAILED"
            ((failed++)) || true
        fi
    done <<< "$resource_ids"

    echo ""
    print_info "Deletion summary:"
    echo "  - Initiated: ${deleted}"
    echo "  - Failed: ${failed}"
    echo "  - Skipped (not found): ${skipped}"

    if [[ "$deleted" -gt 0 ]]; then
        print_warning "Note: Deletions run asynchronously. Resources may take a few minutes to be fully removed."
        print_info "Run this script again to verify cleanup is complete."
    fi
}

# Display Azure AD Applications
display_ad_applications() {
    local apps_json="$1"
    local count

    # Handle empty or invalid JSON
    if [[ -z "$apps_json" ]] || [[ "$apps_json" == "[]" ]] || ! echo "$apps_json" | jq -e '.' >/dev/null 2>&1; then
        print_info "No Azure AD Applications found matching prefix '${PREFIX}'"
        return 1
    fi

    count=$(echo "$apps_json" | jq -r 'length // 0')

    if [[ "$count" -eq 0 ]]; then
        print_info "No Azure AD Applications found matching prefix '${PREFIX}'"
        return 1
    fi

    echo ""
    print_warning "Found ${count} Azure AD Application(s) matching prefix '${PREFIX}':"
    echo ""

    # Print table header
    printf "%-50s | %-40s\n" "DISPLAY NAME" "APP ID"
    printf "%s\n" "$(printf '%.0s-' {1..95})"

    # Print each application
    echo "$apps_json" | jq -r '.[] | "\(.displayName)|\(.appId)"' | while IFS='|' read -r name appId; do
        name_display="${name:0:50}"
        printf "%-50s | %-40s\n" "$name_display" "$appId"
    done

    echo ""
    return 0
}

# Delete Azure AD Applications
delete_ad_applications() {
    local apps_json="$1"
    local count
    local deleted=0
    local failed=0

    count=$(echo "$apps_json" | jq -r 'length')

    if [[ "$count" -eq 0 ]]; then
        return 0
    fi

    if [[ "$DRY_RUN" == "true" ]]; then
        print_warning "[DRY-RUN] Would delete ${count} Azure AD Application(s)"
        return 0
    fi

    # Confirm deletion unless --force is specified
    if [[ "$FORCE" != "true" ]]; then
        echo ""
        read -p "Delete all ${count} Azure AD Application(s)? [y/N] " -n 1 -r
        echo ""
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            print_info "Azure AD Application deletion cancelled"
            return 0
        fi
    fi

    echo ""
    print_info "Deleting Azure AD Applications..."
    print_info "(Note: Deleting an App also deletes its associated Service Principal)"
    echo ""

    # Delete each application by appId
    # Use process substitution to avoid subshell (preserves counter variables)
    while IFS= read -r appId; do
        if [[ -z "$appId" ]]; then
            continue
        fi

        local app_name
        app_name=$(echo "$apps_json" | jq -r --arg id "$appId" '.[] | select(.appId == $id) | .displayName')

        echo -n "  Deleting: ${app_name} (${appId})... "

        # Attempt deletion
        if az ad app delete --id "$appId" 2>/dev/null; then
            echo "DELETED"
            ((deleted++)) || true
        else
            echo "FAILED"
            ((failed++)) || true
        fi
    done < <(echo "$apps_json" | jq -r '.[].appId')

    echo ""
    print_info "Azure AD Application deletion summary:"
    echo "  - Deleted: ${deleted}"
    echo "  - Failed: ${failed}"
}

# Display Service Principals
display_service_principals() {
    local sps_json="$1"
    local count

    # Handle empty or invalid JSON
    if [[ -z "$sps_json" ]] || [[ "$sps_json" == "[]" ]] || ! echo "$sps_json" | jq -e '.' >/dev/null 2>&1; then
        print_info "No Service Principals found matching prefix '${PREFIX}'"
        return 1
    fi

    count=$(echo "$sps_json" | jq -r 'length // 0')

    if [[ "$count" -eq 0 ]]; then
        print_info "No Service Principals found matching prefix '${PREFIX}'"
        return 1
    fi

    echo ""
    print_warning "Found ${count} Service Principal(s) matching prefix '${PREFIX}':"
    echo ""

    # Print table header
    printf "%-50s | %-40s\n" "DISPLAY NAME" "APP ID"
    printf "%s\n" "$(printf '%.0s-' {1..95})"

    # Print each service principal
    echo "$sps_json" | jq -r '.[] | "\(.displayName)|\(.appId)"' | while IFS='|' read -r name appId; do
        name_display="${name:0:50}"
        printf "%-50s | %-40s\n" "$name_display" "$appId"
    done

    echo ""
    return 0
}

# Delete Service Principals
delete_service_principals() {
    local sps_json="$1"
    local count
    local deleted=0
    local failed=0

    count=$(echo "$sps_json" | jq -r 'length')

    if [[ "$count" -eq 0 ]]; then
        return 0
    fi

    if [[ "$DRY_RUN" == "true" ]]; then
        print_warning "[DRY-RUN] Would delete ${count} Service Principal(s)"
        return 0
    fi

    # Confirm deletion unless --force is specified
    if [[ "$FORCE" != "true" ]]; then
        echo ""
        read -p "Delete all ${count} Service Principal(s)? [y/N] " -n 1 -r
        echo ""
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            print_info "Service Principal deletion cancelled"
            return 0
        fi
    fi

    echo ""
    print_info "Deleting Service Principals..."
    print_info "(Note: Some may already be deleted if their App was deleted above)"
    echo ""

    local skipped=0

    # Delete each service principal by id (object id)
    # Use process substitution to avoid subshell (preserves counter variables)
    while IFS= read -r spId; do
        if [[ -z "$spId" ]]; then
            continue
        fi

        local sp_name
        sp_name=$(echo "$sps_json" | jq -r --arg id "$spId" '.[] | select(.id == $id) | .displayName')

        echo -n "  Deleting: ${sp_name}... "

        # Attempt deletion (may fail if already deleted with its App)
        if az ad sp delete --id "$spId" 2>/dev/null; then
            echo "DELETED"
            ((deleted++)) || true
        else
            echo "SKIPPED (already deleted)"
            ((skipped++)) || true
        fi
    done < <(echo "$sps_json" | jq -r '.[].id')

    echo ""
    print_info "Service Principal deletion summary:"
    echo "  - Deleted: ${deleted}"
    echo "  - Skipped (already deleted): ${skipped}"
}

# Main function
main() {
    echo "========================================"
    echo "=== Azure Resource Cleanup ==="
    echo "========================================"
    echo ""

    if [[ -n "$TAG_FILTER" ]]; then
        print_info "Tag filter: ${TAG_FILTER}"
    else
        print_info "Resource prefix: ${PREFIX}"
        print_info "Match mode: ${MATCH_MODE}"
    fi
    if [[ -n "$RESOURCE_GROUP" ]]; then
        print_info "Resource group: ${RESOURCE_GROUP}"
    fi
    if [[ "$DRY_RUN" == "true" ]]; then
        print_warning "DRY-RUN mode enabled - no resources will be deleted"
    fi
    echo ""

    check_prerequisites
    echo ""

    local found_any=false

    # Tag-based cleanup mode
    if [[ -n "$TAG_FILTER" ]]; then
        # Find tagged resource groups
        local rgs_json
        if ! rgs_json=$(find_resource_groups_by_tag "$TAG_FILTER"); then
            print_warning "Skipping resource group cleanup due to query failure"
            rgs_json="[]"
        fi

        local rg_count
        rg_count=$(echo "$rgs_json" | jq -r 'length // 0')

        if [[ "$rg_count" -gt 0 ]]; then
            found_any=true
            echo ""
            print_warning "Found ${rg_count} resource group(s) with tag '${TAG_FILTER}':"
            echo ""
            echo "$rgs_json" | jq -r '.[] | "  \(.name) (\(.location))"'
            echo ""

            for rg_name in $(echo "$rgs_json" | jq -r '.[].name'); do
                delete_resource_group "$rg_name"
            done
        fi

        # Find tagged ARM resources (orphans that survive RG deletion)
        local resources_json
        if ! resources_json=$(find_resources_by_tag "$TAG_FILTER"); then
            print_warning "Skipping ARM resource cleanup due to query failure"
            resources_json='{"data": []}'
        fi

        if display_resources "$resources_json"; then
            found_any=true
            delete_resources "$resources_json"
        fi

        # Find Azure AD Applications and Service Principals with matching tags.
        # Microsoft Graph doesn't support server-side tag filtering via OData, so we use
        # displayName prefix filters (fast, server-side) derived from resource group names.
        # For capi-test-run-id tags, the value is the prefix directly.
        # For other tags (like capi-test-user), we extract prefixes from all matched RGs
        # to handle multiple test runs by the same user.
        local tag_key="${TAG_FILTER%%=*}"
        local tag_value="${TAG_FILTER#*=}"

        local ad_prefixes=()
        if [[ "$tag_key" == "capi-test-run-id" ]]; then
            ad_prefixes+=("${tag_value}")
        elif [[ "$rg_count" -gt 0 ]]; then
            # Extract distinct prefixes from all matched resource groups (strip -resgroup suffix)
            while IFS= read -r rg_prefix; do
                ad_prefixes+=("$rg_prefix")
            done < <(echo "$rgs_json" | jq -r '.[].name' | sed 's/-resgroup$//' | sort -u)
        fi

        if [[ ${#ad_prefixes[@]} -gt 0 ]]; then
            for ad_prefix in "${ad_prefixes[@]}"; do
                echo ""
                local apps_json
                apps_json=$(find_ad_applications "$ad_prefix")

                if display_ad_applications "$apps_json"; then
                    found_any=true
                    delete_ad_applications "$apps_json"
                fi

                echo ""
                local sps_json
                sps_json=$(find_service_principals "$ad_prefix")

                if display_service_principals "$sps_json"; then
                    found_any=true
                    delete_service_principals "$sps_json"
                fi
            done
        else
            echo ""
            print_info "No AD Application/Service Principal prefix available for tag '${TAG_FILTER}'"
            print_info "Tip: Use --tag capi-test-run-id=<prefix> to search AD objects by prefix"
        fi

        if [[ "$found_any" == "false" ]]; then
            print_success "No resources found with tag '${TAG_FILTER}'"
        fi

        echo ""
        echo "========================================"
        echo "=== Cleanup Complete ==="
        echo "========================================"
        return
    fi

    # Prefix-based cleanup mode (original behavior)

    # Delete resource group if specified (do this first)
    if [[ -n "$RESOURCE_GROUP" ]]; then
        if az group show --name "$RESOURCE_GROUP" >/dev/null 2>&1; then
            found_any=true
        fi
        delete_resource_group "$RESOURCE_GROUP"
    fi

    # Find and cleanup ARM resources
    local resources_json
    resources_json=$(find_resources "$PREFIX")

    # Display found resources
    if display_resources "$resources_json"; then
        found_any=true
        delete_resources "$resources_json"
    fi

    # Find and cleanup Azure AD Applications
    echo ""
    local apps_json
    apps_json=$(find_ad_applications "$PREFIX")

    if display_ad_applications "$apps_json"; then
        found_any=true
        delete_ad_applications "$apps_json"
    fi

    # Find and cleanup Service Principals
    echo ""
    local sps_json
    sps_json=$(find_service_principals "$PREFIX")

    if display_service_principals "$sps_json"; then
        found_any=true
        delete_service_principals "$sps_json"
    fi

    if [[ "$found_any" == "false" ]]; then
        print_success "No cleanup needed"
    fi

    echo ""
    echo "========================================"
    echo "=== Cleanup Complete ==="
    echo "========================================"
}

main "$@"
