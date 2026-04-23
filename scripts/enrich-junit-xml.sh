#!/usr/bin/env bash
#
# enrich-junit-xml.sh - Enrich JUnit XML files with metadata properties and generate a combined report
#
# This script performs two Sippy integration enhancements:
# 1. Injects <properties> metadata into each JUnit XML file for Sippy filtering
# 2. Generates a combined junit-combined.xml merging all per-phase reports
#
# Usage:
#   ./scripts/enrich-junit-xml.sh <results-directory>
#
# Environment variables used for properties (all optional, with sensible defaults):
#   INFRA_PROVIDER            - Infrastructure provider (default: "aro")
#   DEPLOYMENT_ENV            - Deployment environment (default: "stage")
#   REGION                    - Cloud region (default: "uksouth")
#   CAPI_USER                 - User identifier (default: "cate")
#   MANAGEMENT_CLUSTER_NAME   - Management cluster name
#   WORKLOAD_CLUSTER_NAMESPACE - Workload cluster namespace
#   OCP_VERSION               - OpenShift version
#   BUILD_ID                  - Prow build ID (set by Prow automatically)
#   JOB_NAME                  - Prow job name (set by Prow automatically)
#
# Example:
#   INFRA_PROVIDER=aro REGION=uksouth ./scripts/enrich-junit-xml.sh results/20260106_212030

set -eo pipefail

# Function to print usage
usage() {
    echo "Usage: $0 <results-directory>"
    echo ""
    echo "Enrich JUnit XML files with metadata properties and generate a combined report."
    echo ""
    echo "Arguments:"
    echo "  results-directory    Directory containing JUnit XML files (junit-*.xml)"
    echo ""
    echo "Example:"
    echo "  $0 results/20260106_212030"
    exit 1
}

# Validate arguments
if [[ $# -lt 1 ]]; then
    usage
fi

RESULTS_DIR="$1"

if [[ ! -d "$RESULTS_DIR" ]]; then
    echo "Error: Directory '$RESULTS_DIR' does not exist" >&2
    exit 1
fi

# Check for xmllint availability
if ! command -v xmllint &> /dev/null; then
    echo "Error: xmllint is required but not installed" >&2
    echo "Install with: sudo apt-get install libxml2-utils (Debian/Ubuntu)" >&2
    echo "           or: sudo dnf install libxml2 (Fedora)" >&2
    exit 1
fi

# Collect metadata properties from environment variables
INFRA_PROVIDER="${INFRA_PROVIDER:-aro}"
DEPLOYMENT_ENV="${DEPLOYMENT_ENV:-stage}"
CAPI_USER="${CAPI_USER:-cate}"
OCP_VERSION="${OCP_VERSION:-}"
BUILD_ID="${BUILD_ID:-}"
JOB_NAME="${JOB_NAME:-}"

# Region depends on provider
if [[ "$INFRA_PROVIDER" == "rosa" ]]; then
    REGION="${AWS_REGION:-${REGION:-us-east-1}}"
else
    REGION="${REGION:-uksouth}"
fi

# Management cluster name depends on provider
if [[ -z "${MANAGEMENT_CLUSTER_NAME:-}" ]]; then
    if [[ "$INFRA_PROVIDER" == "rosa" ]]; then
        MANAGEMENT_CLUSTER_NAME="capa-tests-stage"
    else
        MANAGEMENT_CLUSTER_NAME="capz-tests-stage"
    fi
fi

WORKLOAD_CLUSTER_NAMESPACE="${WORKLOAD_CLUSTER_NAMESPACE:-}"

# Write the CI properties to a temp file for injection into JUnit XML.
# Uses a file-based approach instead of awk -v to avoid macOS awk issues
# with multi-line strings.
PROPS_FILE=$(mktemp)
trap 'rm -f "$PROPS_FILE"' EXIT

xml_escape() {
    printf '%s' "$1" | sed \
        -e 's/&/\&amp;/g' \
        -e 's/"/\&quot;/g' \
        -e "s/'/\&apos;/g" \
        -e 's/</\&lt;/g' \
        -e 's/>/\&gt;/g'
}

write_properties_file() {
    local indent="$1"
    local out="$2"

    echo "${indent}<property name=\"ci.infra_provider\" value=\"$(xml_escape "${INFRA_PROVIDER}")\"/>" > "$out"
    echo "${indent}<property name=\"ci.deployment_env\" value=\"$(xml_escape "${DEPLOYMENT_ENV}")\"/>" >> "$out"
    echo "${indent}<property name=\"ci.region\" value=\"$(xml_escape "${REGION}")\"/>" >> "$out"
    echo "${indent}<property name=\"ci.capi_user\" value=\"$(xml_escape "${CAPI_USER}")\"/>" >> "$out"
    echo "${indent}<property name=\"ci.management_cluster\" value=\"$(xml_escape "${MANAGEMENT_CLUSTER_NAME}")\"/>" >> "$out"

    if [[ -n "$WORKLOAD_CLUSTER_NAMESPACE" ]]; then
        echo "${indent}<property name=\"ci.workload_cluster_namespace\" value=\"$(xml_escape "${WORKLOAD_CLUSTER_NAMESPACE}")\"/>" >> "$out"
    fi
    if [[ -n "$OCP_VERSION" ]]; then
        echo "${indent}<property name=\"ci.ocp_version\" value=\"$(xml_escape "${OCP_VERSION}")\"/>" >> "$out"
    fi
    if [[ -n "$BUILD_ID" ]]; then
        echo "${indent}<property name=\"ci.build_id\" value=\"$(xml_escape "${BUILD_ID}")\"/>" >> "$out"
    fi
    if [[ -n "$JOB_NAME" ]]; then
        echo "${indent}<property name=\"ci.job_name\" value=\"$(xml_escape "${JOB_NAME}")\"/>" >> "$out"
    fi
}

# Write properties with testsuite-level indentation (2 tabs + 1 tab for property)
write_properties_file "			" "$PROPS_FILE"

# Phase order for combined report (matches generate-summary.sh)
PHASE_ORDER=(
    "junit-check-dep"
    "junit-setup"
    "junit-cluster"
    "junit-generate-yamls"
    "junit-deploy-apply"
    "junit-deploy-monitor"
    "junit-deploy-crs"
    "junit-verify"
    "junit-delete"
    "junit-cleanup"
    "junit-mce-teardown"
)

# ============================================================================
# Enhancement 1: Inject properties into each JUnit XML file
# ============================================================================

inject_properties() {
    local file="$1"

    if [[ ! -f "$file" ]]; then
        return
    fi

    # Skip the combined file if it exists from a previous run
    if [[ "$(basename "$file")" == "junit-combined.xml" ]]; then
        return 1
    fi

    # Skip files already enriched (idempotency guard)
    if grep -q 'ci\.infra_provider' "$file"; then
        echo "  Skipping (already enriched): $(basename "$file")"
        return 1
    fi

    echo "  Enriching: $(basename "$file")"

    # gotestsum generates JUnit XML with a <properties> block containing go.version.
    # We inject our CI properties into that existing block, or create a new one.
    # Uses sed with 'r' (read file) command for cross-platform compatibility.

    local temp_file
    temp_file=$(mktemp)

    if grep -q '</properties>' "$file"; then
        # Existing <properties> block — insert CI properties before </properties>
        awk '
            /<\/properties>/ {
                while ((getline line < "'"$PROPS_FILE"'") > 0) print line
            }
            { print }
        ' "$file" > "$temp_file"
    else
        # No <properties> block — wrap in <properties> tags after <testsuite ...>
        awk '
            /<testsuite / && !done {
                print
                print "\t\t<properties>"
                while ((getline line < "'"$PROPS_FILE"'") > 0) print line
                print "\t\t</properties>"
                done = 1
                next
            }
            { print }
        ' "$file" > "$temp_file"
    fi

    mv "$temp_file" "$file"
}

echo "=== Enriching JUnit XML files with CI properties ==="
echo ""
echo "Properties:"
echo "  infra_provider: $INFRA_PROVIDER"
echo "  deployment_env: $DEPLOYMENT_ENV"
echo "  region:         $REGION"
echo "  capi_user:      $CAPI_USER"
echo "  mgmt_cluster:   $MANAGEMENT_CLUSTER_NAME"
[[ -n "$WORKLOAD_CLUSTER_NAMESPACE" ]] && echo "  wl_namespace:   $WORKLOAD_CLUSTER_NAMESPACE"
[[ -n "$OCP_VERSION" ]] && echo "  ocp_version:    $OCP_VERSION"
[[ -n "$BUILD_ID" ]] && echo "  build_id:       $BUILD_ID"
[[ -n "$JOB_NAME" ]] && echo "  job_name:       $JOB_NAME"
echo ""

# Process each JUnit XML file
xml_count=0
xml_found=0
for xml_file in "$RESULTS_DIR"/junit-*.xml; do
    if [[ -f "$xml_file" ]]; then
        xml_found=$((xml_found + 1))
        if inject_properties "$xml_file"; then
            xml_count=$((xml_count + 1))
        fi
    fi
done

if [[ $xml_found -eq 0 ]]; then
    echo "Warning: No junit-*.xml files found in $RESULTS_DIR" >&2
    exit 0
fi

echo ""
echo "Enriched $xml_count JUnit XML file(s) with CI properties."

# ============================================================================
# Enhancement 3: Generate combined JUnit report
# ============================================================================

echo ""
echo "=== Generating combined JUnit report ==="

COMBINED_FILE="$RESULTS_DIR/junit-combined.xml"

# Collect all testsuite elements and aggregate totals
total_tests=0
total_failures=0
total_errors=0
total_time=0
testsuite_blocks=""

for phase_key in "${PHASE_ORDER[@]}"; do
    file="$RESULTS_DIR/${phase_key}.xml"
    if [[ ! -f "$file" ]]; then
        continue
    fi

    # Extract top-level testsuites attributes
    tests=$(xmllint --xpath 'string(//testsuites/@tests)' "$file" 2>/dev/null || echo "0")
    failures=$(xmllint --xpath 'string(//testsuites/@failures)' "$file" 2>/dev/null || echo "0")
    errors=$(xmllint --xpath 'string(//testsuites/@errors)' "$file" 2>/dev/null || echo "0")
    time_val=$(xmllint --xpath 'string(//testsuites/@time)' "$file" 2>/dev/null || echo "0")

    tests=${tests:-0}
    failures=${failures:-0}
    errors=${errors:-0}
    time_val=${time_val:-0}

    total_tests=$((total_tests + tests))
    total_failures=$((total_failures + failures))
    total_errors=$((total_errors + errors))
    total_time=$(echo "$total_time + $time_val" | bc)

    # Extract all <testsuite> elements (the inner content, not the <testsuites> wrapper)
    # xmllint --xpath can extract the testsuite elements directly
    suite_xml=$(xmllint --xpath '//testsuite' "$file" 2>/dev/null || true)
    if [[ -n "$suite_xml" ]]; then
        testsuite_blocks+="$suite_xml"$'\n'
    fi

    echo "  Added: $(basename "$file") ($tests tests)"
done

# Write the combined XML file
cat > "$COMBINED_FILE" << EOF
<?xml version="1.0" encoding="UTF-8"?>
<testsuites tests="$total_tests" failures="$total_failures" errors="$total_errors" time="$total_time">
$testsuite_blocks</testsuites>
EOF

echo ""
echo "Combined report: $COMBINED_FILE ($total_tests tests, $total_failures failures, $total_errors errors)"
echo ""
echo "=== JUnit enrichment complete ==="
