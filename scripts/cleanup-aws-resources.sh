#!/usr/bin/env bash
# cleanup-aws-resources.sh - Clean up AWS resources created during CAPI testing
#
# This script finds and deletes AWS resources tagged with capi-test-* ownership
# metadata. It cleans up:
#   - CloudFormation stacks (tagged with capi-test-created-at)
#   - VPCs and their dependencies (tagged with capi-test-created-at)
#
# Usage:
#   ./scripts/cleanup-aws-resources.sh [OPTIONS]
#
# Options:
#   --region REGION     AWS region to scan (default: AWS_REGION env var, else us-east-1)
#   --tag KEY=VALUE     Find resources by tag (e.g., 'capi-test-user=alice')
#   --my-resources      Find all resources tagged with capi-test-user=$CAPI_USER (or $USER fallback)
#   --dry-run           Show what would be deleted without actually deleting
#   --force             Skip confirmation prompts
#   --help              Show this help message
#
# Environment variables:
#   AWS_REGION             AWS region to scan (default: us-east-1)
#   AWS_ACCESS_KEY_ID      AWS credentials
#   AWS_SECRET_ACCESS_KEY  AWS credentials
#   CAPI_USER              User prefix for --my-resources (default: $USER)
#
# Examples:
#   ./scripts/cleanup-aws-resources.sh --dry-run
#   ./scripts/cleanup-aws-resources.sh --force
#   ./scripts/cleanup-aws-resources.sh --my-resources
#   ./scripts/cleanup-aws-resources.sh --tag capi-test-user=alice --force
#   ./scripts/cleanup-aws-resources.sh --region us-west-2 --force

set -euo pipefail

# Defaults
REGION="${AWS_REGION:-us-east-1}"
DRY_RUN=false
FORCE=false
TAG_FILTER=""
MY_RESOURCES=false

# ANSI colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
print_success() { echo -e "${GREEN}[OK]${NC} $1"; }
print_warning() { echo -e "${YELLOW}[WARN]${NC} $1"; }
print_error() { echo -e "${RED}[ERROR]${NC} $1" >&2; }

usage() {
    local exit_code="${1:-0}"
    sed -n '2,/^$/p' "$0" | grep '^#' | sed 's/^# \?//'
    exit "$exit_code"
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --region)
            [[ $# -lt 2 ]] && { print_error "Missing value for --region"; exit 2; }
            REGION="$2"
            shift 2
            ;;
        --tag)
            [[ $# -lt 2 ]] && { print_error "Missing value for --tag"; exit 2; }
            TAG_FILTER="$2"
            shift 2
            ;;
        --my-resources)
            MY_RESOURCES=true
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
            usage 2
            ;;
    esac
done

# Handle --my-resources
if [[ "$MY_RESOURCES" == "true" ]]; then
    CAPI_USER="${CAPI_USER:-${USER:-cate}}"
    TAG_FILTER="capi-test-user=${CAPI_USER}"
    DRY_RUN=true
    print_info "Listing resources tagged with capi-test-user=${CAPI_USER} (dry-run)"
fi

# Validate prerequisites
if ! command -v aws >/dev/null 2>&1; then
    print_error "AWS CLI (aws) not installed"
    exit 2
fi

if ! aws sts get-caller-identity >/dev/null 2>&1; then
    print_error "Not authenticated to AWS. Run: aws configure"
    exit 2
fi

echo "========================================"
echo "=== AWS Resource Cleanup ==="
echo "========================================"
echo ""
print_info "Region: ${REGION}"
[[ -n "$TAG_FILTER" ]] && print_info "Tag filter: ${TAG_FILTER}"
[[ "$DRY_RUN" == "true" ]] && print_info "Mode: DRY RUN (no deletions)"
echo ""

# ─── CloudFormation Stacks ──────────────────────────────────────────────────────

cleanup_cloudformation_stacks() {
    print_info "Scanning CloudFormation stacks..."

    local stacks_json
    stacks_json=$(aws cloudformation describe-stacks \
        --region "$REGION" \
        --query "Stacks[?StackStatus=='CREATE_COMPLETE' || StackStatus=='UPDATE_COMPLETE'].{StackName: StackName, Tags: Tags, CreationTime: CreationTime}" \
        --output json 2>/dev/null) || {
        print_warning "Failed to query CloudFormation stacks"
        return 0
    }

    # Filter to stacks with capi-test-created-at tag
    local tagged_stacks
    tagged_stacks=$(echo "$stacks_json" | jq '[.[] | select(.Tags != null) | select([.Tags[] | select(.Key == "capi-test-created-at")] | length > 0)]')

    # Apply tag filter if specified
    if [[ -n "$TAG_FILTER" ]]; then
        local filter_key filter_value
        filter_key="${TAG_FILTER%%=*}"
        filter_value="${TAG_FILTER#*=}"
        tagged_stacks=$(echo "$tagged_stacks" | jq --arg key "$filter_key" --arg val "$filter_value" \
            '[.[] | select([.Tags[] | select(.Key == $key and .Value == $val)] | length > 0)]')
    fi

    local count
    count=$(echo "$tagged_stacks" | jq 'length')

    if [[ "$count" -eq 0 ]]; then
        print_success "No matching CloudFormation stacks found"
        return 0
    fi

    echo ""
    print_warning "Found ${count} CloudFormation stack(s) to delete:"
    echo ""
    echo "$tagged_stacks" | jq -r '.[] | "  \(.StackName) (created: \(.CreationTime))"'
    echo ""

    if [[ "$DRY_RUN" == "true" ]]; then
        print_info "Dry run — skipping deletion"
        return 0
    fi

    if [[ "$FORCE" != "true" ]]; then
        read -r -p "Delete these CloudFormation stacks? [y/N] " confirm
        if [[ "$confirm" != "y" && "$confirm" != "Y" ]]; then
            print_info "Skipped CloudFormation stack deletion"
            return 0
        fi
    fi

    while IFS= read -r stack_name; do
        print_info "Deleting stack: ${stack_name}..."
        if aws cloudformation delete-stack --stack-name "$stack_name" --region "$REGION" 2>/dev/null; then
            print_success "Initiated deletion of ${stack_name}"
        else
            print_warning "Failed to delete ${stack_name}"
        fi
    done < <(echo "$tagged_stacks" | jq -r '.[].StackName')
}

# ─── VPCs ───────────────────────────────────────────────────────────────────────

cleanup_vpcs() {
    print_info "Scanning VPCs..."

    local vpcs_json
    vpcs_json=$(aws ec2 describe-vpcs \
        --region "$REGION" \
        --filters "Name=tag-key,Values=capi-test-created-at" \
        --query "Vpcs[].{VpcId: VpcId, Tags: Tags}" \
        --output json 2>/dev/null) || {
        print_warning "Failed to query VPCs"
        return 0
    }

    # Apply tag filter if specified
    if [[ -n "$TAG_FILTER" ]]; then
        local filter_key filter_value
        filter_key="${TAG_FILTER%%=*}"
        filter_value="${TAG_FILTER#*=}"
        vpcs_json=$(echo "$vpcs_json" | jq --arg key "$filter_key" --arg val "$filter_value" \
            '[.[] | select([.Tags[] | select(.Key == $key and .Value == $val)] | length > 0)]')
    fi

    local count
    count=$(echo "$vpcs_json" | jq 'length')

    if [[ "$count" -eq 0 ]]; then
        print_success "No matching VPCs found"
        return 0
    fi

    echo ""
    print_warning "Found ${count} VPC(s) to delete:"
    echo ""
    echo "$vpcs_json" | jq -r '.[] | "  \(.VpcId) (\([.Tags[] | select(.Key == "Name")] | .[0].Value // "unnamed"))"'
    echo ""

    if [[ "$DRY_RUN" == "true" ]]; then
        print_info "Dry run — skipping deletion"
        return 0
    fi

    if [[ "$FORCE" != "true" ]]; then
        read -r -p "Delete these VPCs and their dependencies? [y/N] " confirm
        if [[ "$confirm" != "y" && "$confirm" != "Y" ]]; then
            print_info "Skipped VPC deletion"
            return 0
        fi
    fi

    while IFS= read -r vpc_id; do
        delete_vpc "$vpc_id"
    done < <(echo "$vpcs_json" | jq -r '.[].VpcId')
}

delete_vpc() {
    local vpc_id="$1"
    print_info "Deleting VPC ${vpc_id} and dependencies..."

    # Delete internet gateways
    local igws
    igws=$(aws ec2 describe-internet-gateways \
        --region "$REGION" \
        --filters "Name=attachment.vpc-id,Values=${vpc_id}" \
        --query "InternetGateways[].InternetGatewayId" \
        --output text 2>/dev/null) || igws=""
    for igw in $igws; do
        aws ec2 detach-internet-gateway --internet-gateway-id "$igw" --vpc-id "$vpc_id" --region "$REGION" 2>/dev/null || true
        aws ec2 delete-internet-gateway --internet-gateway-id "$igw" --region "$REGION" 2>/dev/null || true
    done

    # Delete subnets
    local subnets
    subnets=$(aws ec2 describe-subnets \
        --region "$REGION" \
        --filters "Name=vpc-id,Values=${vpc_id}" \
        --query "Subnets[].SubnetId" \
        --output text 2>/dev/null) || subnets=""
    for subnet in $subnets; do
        aws ec2 delete-subnet --subnet-id "$subnet" --region "$REGION" 2>/dev/null || true
    done

    # Delete route tables (skip main)
    local route_tables
    route_tables=$(aws ec2 describe-route-tables \
        --region "$REGION" \
        --filters "Name=vpc-id,Values=${vpc_id}" \
        --query "RouteTables[?Associations[0].Main != \`true\`].RouteTableId" \
        --output text 2>/dev/null) || route_tables=""
    for rt in $route_tables; do
        aws ec2 delete-route-table --route-table-id "$rt" --region "$REGION" 2>/dev/null || true
    done

    # Delete security groups (skip default)
    local sgs
    sgs=$(aws ec2 describe-security-groups \
        --region "$REGION" \
        --filters "Name=vpc-id,Values=${vpc_id}" \
        --query "SecurityGroups[?GroupName != 'default'].GroupId" \
        --output text 2>/dev/null) || sgs=""
    for sg in $sgs; do
        aws ec2 delete-security-group --group-id "$sg" --region "$REGION" 2>/dev/null || true
    done

    # Delete NAT gateways
    local nat_gws
    nat_gws=$(aws ec2 describe-nat-gateways \
        --region "$REGION" \
        --filter "Name=vpc-id,Values=${vpc_id}" "Name=state,Values=available" \
        --query "NatGateways[].NatGatewayId" \
        --output text 2>/dev/null) || nat_gws=""
    for nat in $nat_gws; do
        aws ec2 delete-nat-gateway --nat-gateway-id "$nat" --region "$REGION" 2>/dev/null || true
    done

    # Delete the VPC
    if aws ec2 delete-vpc --vpc-id "$vpc_id" --region "$REGION" 2>/dev/null; then
        print_success "Deleted VPC ${vpc_id}"
    else
        print_warning "Failed to delete VPC ${vpc_id} (may have remaining dependencies)"
    fi
}

# ─── Main ───────────────────────────────────────────────────────────────────────

cleanup_cloudformation_stacks
echo ""
cleanup_vpcs
echo ""

echo "========================================"
if [[ "$DRY_RUN" == "true" ]]; then
    print_info "Dry run complete — no resources were deleted"
else
    print_success "AWS cleanup complete"
fi
echo "========================================"
