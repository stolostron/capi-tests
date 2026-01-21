#!/usr/bin/env bash
# cleanup-azure-resources.sh - Clean up Azure resources created during ARO-CAPZ testing
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
#   --prefix PREFIX        Resource name prefix to search for (default: from CAPZ_USER env var or 'rcap')
#   --resource-group RG    Also delete this Azure resource group
#   --dry-run              Show what would be deleted without actually deleting
#   --force                Skip confirmation prompts
#   --help                 Show this help message
#
# Environment variables:
#   CAPZ_USER          Default prefix for resource names (e.g., 'rcap')
#   AZURE_SUBSCRIPTION_ID  Azure subscription ID to search in
#
# Examples:
#   ./scripts/cleanup-azure-resources.sh --dry-run
#   ./scripts/cleanup-azure-resources.sh --prefix rcapd --force
#   ./scripts/cleanup-azure-resources.sh --resource-group myapp-resgroup --prefix myapp
#   CAPZ_USER=myuser ./scripts/cleanup-azure-resources.sh

set -euo pipefail

# Default values
PREFIX="${CAPZ_USER:-rcap}"
RESOURCE_GROUP=""
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
    head -35 "$0" | grep '^#' | sed 's/^# \?//'
    exit 0
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --prefix)
            PREFIX="$2"
            shift 2
            ;;
        --resource-group)
            RESOURCE_GROUP="$2"
            shift 2
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

# Validate prefix to prevent OData filter injection
# Must be lowercase alphanumeric with optional hyphens, starting with alphanumeric
if [[ ! "$PREFIX" =~ ^[a-z0-9][a-z0-9-]*$ ]]; then
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
    print_info "Deleting resource group '${rg_name}' (running in background)..."

    if az group delete --name "$rg_name" --yes --no-wait 2>/dev/null; then
        print_success "Resource group deletion initiated"
        print_warning "Note: Resource group deletion runs asynchronously and may take several minutes."
    else
        print_error "Failed to delete resource group '${rg_name}'"
        return 1
    fi
}

# Find resources matching the pattern
find_resources() {
    local prefix="$1"

    print_info "Searching for Azure resources with prefix '${prefix}'..." >&2

    # Query Azure Resource Graph for resources matching the pattern
    # We search for:
    # 1. Resources with names starting with the prefix (e.g., rcapa, rcapb, rcapc, etc.)
    # 2. Resources with names containing the prefix pattern

    # Build the query to find resources with the naming pattern
    # The pattern is: prefix followed by optional suffix (e.g., rcapa, rcapb, rcap-stage, etc.)
    local query="Resources | where name contains '${prefix}' | project id, name, type, resourceGroup, subscriptionId | order by type asc, name asc"

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
    # Note: The filter is case-insensitive
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
    local sps_json
    sps_json=$(az ad sp list --filter "startswith(displayName, '${prefix}')" --query "[].{appId: appId, displayName: displayName, id: id}" -o json 2>/dev/null)
    if [[ $? -ne 0 ]]; then
        print_error "Failed to list Service Principals with prefix '${prefix}'" >&2
        echo "[]"
        return
    fi
    echo "$sps_json"
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
            ((skipped++))
            continue
        fi

        # Attempt deletion
        if az resource delete --ids "$resource_id" --no-wait 2>/dev/null; then
            echo "INITIATED"
            ((deleted++))
        else
            echo "FAILED"
            ((failed++))
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
            ((deleted++))
        else
            echo "FAILED"
            ((failed++))
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
            ((deleted++))
        else
            echo "SKIPPED (already deleted)"
            ((skipped++))
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
    print_info "Resource prefix: ${PREFIX}"
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
