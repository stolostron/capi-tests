#!/usr/bin/env bash
# visualize-pipeline.sh — Generate an HTML pipeline visualization from Prow JUnit XML
#
# Usage:
#   ./scripts/visualize-pipeline.sh <junit_operator.xml>          # from local file
#   ./scripts/visualize-pipeline.sh <prow-build-url>              # from Prow job URL
#   ./scripts/visualize-pipeline.sh <build-id>                    # from build ID (uses PR 75733)
#
# Output: Opens pipeline.html in the default browser

set -euo pipefail

OUTPUT_FILE="${OUTPUT_FILE:-/tmp/prow-pipeline.html}"

# Default PR context for build ID lookups
DEFAULT_PR="75733"
DEFAULT_REPO="openshift_release"
DEFAULT_JOB="rehearse-${DEFAULT_PR}-pull-ci-stolostron-capi-tests-configure-prow-capz-e2e"
GCS_BASE="https://storage.googleapis.com/test-platform-results/pr-logs/pull"

usage() {
    echo "Usage: $0 <junit_operator.xml | prow-url | build-id>"
    echo ""
    echo "Examples:"
    echo "  $0 junit_operator.xml"
    echo "  $0 2032475992039100416"
    echo "  $0 https://prow.ci.openshift.org/view/gs/test-platform-results/pr-logs/pull/..."
    exit 1
}

# Resolve input to a local XML file
resolve_input() {
    local input="$1"
    local tmp_xml
    tmp_xml="$(mktemp /tmp/junit_operator_prow.XXXXXX.xml)"

    if [[ -f "$input" ]]; then
        echo "$input"
        return
    fi

    # Pure numeric = build ID
    if [[ "$input" =~ ^[0-9]+$ ]]; then
        local url="${GCS_BASE}/${DEFAULT_REPO}/${DEFAULT_PR}/${DEFAULT_JOB}/${input}/artifacts/junit_operator.xml"
        echo "Downloading from: $url" >&2
        curl -sf "$url" -o "$tmp_xml" || { echo "Error: Failed to download JUnit XML from build $input" >&2; exit 1; }
        echo "$tmp_xml"
        return
    fi

    # Prow URL — extract GCS path
    if [[ "$input" == *"prow.ci.openshift.org"* ]]; then
        local gcs_path
        gcs_path=$(echo "$input" | sed 's|.*view/gs/|gs://|')
        local url="https://storage.googleapis.com/${gcs_path#gs://}/artifacts/junit_operator.xml"
        echo "Downloading from: $url" >&2
        curl -sf "$url" -o "$tmp_xml" || { echo "Error: Failed to download JUnit XML from Prow URL" >&2; exit 1; }
        echo "$tmp_xml"
        return
    fi

    # Direct GCS/HTTP URL
    if [[ "$input" == http* ]]; then
        echo "Downloading from: $input" >&2
        curl -sf "$input" -o "$tmp_xml" || { echo "Error: Failed to download from URL" >&2; exit 1; }
        echo "$tmp_xml"
        return
    fi

    echo "Error: Cannot resolve input '$input'" >&2
    usage
}

# Parse JUnit XML and generate JSON-like data for the HTML template
parse_junit() {
    local xml_file="$1"

    # Extract testcases using xmllint
    if ! command -v xmllint &>/dev/null; then
        echo "Error: xmllint is required. Install with: brew install libxml2" >&2
        exit 1
    fi

    local count
    count=$(xmllint --xpath 'count(//testcase)' "$xml_file" 2>/dev/null)

    local steps_json="["
    local first=true

    for ((i = 1; i <= count; i++)); do
        local name time_val failed msg

        name=$(xmllint --xpath "string(//testcase[$i]/@name)" "$xml_file" 2>/dev/null || echo "")
        time_val=$(xmllint --xpath "string(//testcase[$i]/@time)" "$xml_file" 2>/dev/null || echo "0")

        # Check for failure element — check both @message attribute and element text content
        # Some JUnit producers put the error in @message, others in the element body
        msg=$(xmllint --xpath "string(//testcase[$i]/failure/@message)" "$xml_file" 2>/dev/null || echo "")
        if [[ -z "$msg" ]]; then
            msg=$(xmllint --xpath "string(//testcase[$i]/failure)" "$xml_file" 2>/dev/null || echo "")
        fi
        # Detect failure by checking if a <failure> element exists at all
        has_failure=$(xmllint --xpath "count(//testcase[$i]/failure)" "$xml_file" 2>/dev/null || echo "0")
        if [[ "$has_failure" -gt 0 ]]; then
            failed="true"
        else
            failed="false"
        fi

        # Skip non-step entries (build/image/release steps and phase summaries)
        if [[ "$name" != *"container test"* ]]; then
            continue
        fi

        # Extract step name: "Run multi-stage test capz-e2e - capz-e2e-<STEP> container test"
        local step_name
        step_name=$(echo "$name" | sed 's/.*capz-e2e-capz-e2e-//' | sed 's/.*capz-e2e-//' | sed 's/ container test//')

        if [[ "$first" != "true" ]]; then
            steps_json+=","
        fi
        first=false

        # Escape name and message for JSON
        local escaped_name
        escaped_name=$(printf '%s' "$step_name" | sed 's/\\/\\\\/g; s/"/\\"/g; s/\t/\\t/g' | tr '\n' ' ')
        local escaped_msg
        escaped_msg=$(echo "$msg" | head -20 | sed 's/\\/\\\\/g; s/"/\\"/g; s/\t/\\t/g' | tr '\n' ' ' | cut -c1-500)

        steps_json+="{\"name\":\"${escaped_name}\",\"time\":${time_val},\"failed\":${failed},\"message\":\"${escaped_msg}\"}"
    done

    steps_json+="]"
    echo "$steps_json"
}

generate_html() {
    local steps_json="$1"

    # Write HTML with a placeholder, then replace
    cat > "$OUTPUT_FILE" << 'HTMLEOF'
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Prow Pipeline Visualization</title>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    background: #0d1117;
    color: #c9d1d9;
    padding: 24px;
  }
  h1 {
    font-size: 20px;
    margin-bottom: 8px;
    color: #f0f6fc;
  }
  .subtitle {
    font-size: 13px;
    color: #8b949e;
    margin-bottom: 24px;
  }
  .summary {
    display: flex;
    gap: 16px;
    margin-bottom: 24px;
    flex-wrap: wrap;
  }
  .summary-card {
    background: #161b22;
    border: 1px solid #30363d;
    border-radius: 6px;
    padding: 12px 16px;
    min-width: 120px;
  }
  .summary-card .label { font-size: 11px; color: #8b949e; text-transform: uppercase; }
  .summary-card .value { font-size: 24px; font-weight: 600; margin-top: 4px; }
  .summary-card .value.green { color: #3fb950; }
  .summary-card .value.red { color: #f85149; }
  .summary-card .value.gray { color: #8b949e; }

  .phase {
    margin-bottom: 20px;
  }
  .phase-header {
    font-size: 13px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.5px;
    color: #8b949e;
    margin-bottom: 8px;
    padding-left: 4px;
  }
  .pipeline {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    align-items: stretch;
  }
  .step {
    background: #161b22;
    border: 1px solid #30363d;
    border-radius: 6px;
    padding: 10px 14px;
    min-width: 180px;
    max-width: 280px;
    flex: 0 0 auto;
    cursor: default;
    transition: border-color 0.15s;
    position: relative;
  }
  .step:hover { border-color: #58a6ff; }
  .step.passed { border-left: 3px solid #3fb950; }
  .step.failed { border-left: 3px solid #f85149; cursor: pointer; }
  .step.not-reached { border-left: 3px solid #484f58; opacity: 0.5; }
  .step.not-wired { border-left: 3px solid #d29922; opacity: 0.6; }
  .step.not-created { border-left: 3px solid #6e7681; opacity: 0.5; border-style: dashed; }

  .step-name {
    font-size: 13px;
    font-weight: 500;
    color: #f0f6fc;
    word-break: break-word;
  }
  .step-meta {
    font-size: 11px;
    color: #8b949e;
    margin-top: 4px;
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
  .step-status {
    font-size: 11px;
    font-weight: 600;
    padding: 1px 6px;
    border-radius: 3px;
  }
  .step-status.passed { background: #238636; color: #fff; }
  .step-status.failed { background: #da3633; color: #fff; }
  .step-status.not-reached { background: #30363d; color: #8b949e; }
  .step-status.not-wired { background: #4d2d00; color: #d29922; }
  .step-status.not-created { background: #21262d; color: #6e7681; }

  .connector {
    display: flex;
    align-items: center;
    color: #30363d;
    font-size: 18px;
    flex: 0 0 auto;
  }

  .error-tooltip {
    display: none;
    position: fixed;
    background: #1c2028;
    border: 1px solid #f85149;
    border-radius: 8px;
    padding: 12px 16px;
    max-width: 600px;
    max-height: 300px;
    overflow-y: auto;
    font-size: 12px;
    font-family: monospace;
    color: #f0f6fc;
    z-index: 1000;
    white-space: pre-wrap;
    word-break: break-word;
    box-shadow: 0 8px 24px rgba(0,0,0,0.4);
  }
  .error-tooltip.visible { display: block; }

  .legend {
    display: flex;
    gap: 16px;
    margin-top: 24px;
    flex-wrap: wrap;
  }
  .legend-item {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
    color: #8b949e;
  }
  .legend-dot {
    width: 10px;
    height: 10px;
    border-radius: 2px;
  }
  .legend-dot.passed { background: #3fb950; }
  .legend-dot.failed { background: #f85149; }
  .legend-dot.not-reached { background: #484f58; }
  .legend-dot.not-wired { background: #d29922; }
  .legend-dot.not-created { background: #6e7681; border: 1px dashed #6e7681; }
</style>
</head>
<body>

<h1>Prow Pipeline &mdash; CAPZ E2E</h1>
<div class="subtitle" id="subtitle"></div>

<div class="summary" id="summary"></div>

<div id="pipeline-container"></div>

<div class="legend">
  <div class="legend-item"><div class="legend-dot passed"></div> Passed</div>
  <div class="legend-item"><div class="legend-dot failed"></div> Failed</div>
  <div class="legend-item"><div class="legend-dot not-reached"></div> Not Reached</div>
  <div class="legend-item"><div class="legend-dot not-wired"></div> Not Wired</div>
  <div class="legend-item"><div class="legend-dot not-created"></div> Not Created</div>
</div>

<div class="error-tooltip" id="error-tooltip"></div>

<script>
const STEPS_DATA = STEPS_JSON_PLACEHOLDER;

// Step definitions with lifecycle phases
// grouped: true = collapse all steps into a single summary box (for IPI infra steps we don't own)
const PIPELINE_DEF = [
  { phase: "Pre \u2014 OpenShift Cluster", grouped: true, label: "IPI Azure Provisioning", steps: [
    "ipi-conf", "ipi-conf-azure", "ipi-conf-telemetry", "rhcos-conf-osstream",
    "ipi-azure-rbac", "azure-provision-service-principal-minimal-permission", "azure-provision-custom-role",
    "ipi-install-rbac", "ipi-install-install", "ipi-install-monitoringpvc",
    "ipi-install-hosted-loki", "ipi-install-times-collection",
    "openshift-cluster-bot-rbac", "multiarch-validate-nodes", "nodes-readiness"
  ]},
  { phase: "Pre \u2014 CAPZ Test Setup", steps: [
    "capz-test-check-dependencies", "capz-test-setup"
  ]},
  { phase: "Test \u2014 CAPZ Workload Cluster", steps: [
    "capz-test-management-cluster", "capz-test-generate-yamls",
    "capz-test-deploy-crs", "capz-test-verify-workload-cluster",
    "capz-test-delete-workload-cluster", "capz-test-validate-cleanup"
  ]},
  { phase: "Post \u2014 Cleanup", steps: [
    "capz-test-teardown"
  ]},
  { phase: "Post \u2014 IPI Deprovision", grouped: true, label: "IPI Azure Deprovision", steps: [
    "gather-must-gather", "gather-extra", "gather-audit-logs", "gather-azure-cli",
    "azure-deprovision-sp-and-custom-role", "ipi-deprovision-deprovision"
  ]}
];

const NOT_WIRED = new Set(["capz-test-deploy-crs", "capz-test-delete-workload-cluster"]);
const NOT_CREATED = new Set(["capz-test-verify-workload-cluster", "capz-test-validate-cleanup"]);

// Build lookup from parsed steps
const stepMap = {};
STEPS_DATA.forEach(function(s) { stepMap[s.name] = s; });

// Find if/where the pipeline broke
let pipelineBroken = false;

function formatDuration(seconds) {
  if (seconds < 1) return "<1s";
  if (seconds < 60) return Math.round(seconds) + "s";
  const m = Math.floor(seconds / 60);
  const s = Math.round(seconds % 60);
  if (m < 60) return m + "m " + s + "s";
  const h = Math.floor(m / 60);
  return h + "h " + (m % 60) + "m";
}

function getStepInfo(name) {
  if (NOT_CREATED.has(name)) return { status: "not-created", label: "NOT CREATED", time: 0, message: "" };
  if (NOT_WIRED.has(name)) return { status: "not-wired", label: "NOT WIRED", time: 0, message: "" };

  const data = stepMap[name];
  if (!data) {
    return { status: "not-reached", label: "NOT REACHED", time: 0, message: "" };
  }

  if (data.failed) {
    pipelineBroken = true;
    return { status: "failed", label: "FAILED", time: data.time, message: data.message };
  }

  return { status: "passed", label: "PASSED", time: data.time, message: "" };
}

// Render pipeline using safe DOM methods
const container = document.getElementById("pipeline-container");
let totalPassed = 0, totalFailed = 0, totalNotReached = 0, totalTime = 0;

PIPELINE_DEF.forEach(function(phase) {
  const phaseDiv = document.createElement("div");
  phaseDiv.className = "phase";

  const header = document.createElement("div");
  header.className = "phase-header";
  header.textContent = phase.phase;
  phaseDiv.appendChild(header);

  const pipeline = document.createElement("div");
  pipeline.className = "pipeline";

  if (phase.grouped) {
    // Render as a single consolidated box
    var groupTime = 0, groupFailed = false, groupMsg = "", groupPassed = 0, groupFailCount = 0;
    phase.steps.forEach(function(stepName) {
      var info = getStepInfo(stepName);
      groupTime += info.time;
      if (info.status === "passed") groupPassed++;
      if (info.status === "failed") { groupFailed = true; groupFailCount++; if (!groupMsg) groupMsg = info.message; }
    });
    var groupStatus = groupFailed ? "failed" : (groupPassed > 0 ? "passed" : "not-reached");
    var groupLabel = groupFailed ? "FAILED" : (groupPassed > 0 ? "PASSED" : "NOT REACHED");

    // Count toward totals as a single item
    if (groupStatus === "passed") totalPassed++;
    else if (groupStatus === "failed") totalFailed++;
    else totalNotReached++;
    totalTime += groupTime;

    var step = document.createElement("div");
    step.className = "step " + groupStatus;
    step.style.minWidth = "240px";

    var nameDiv = document.createElement("div");
    nameDiv.className = "step-name";
    nameDiv.textContent = phase.label || phase.phase;
    step.appendChild(nameDiv);

    var detailDiv = document.createElement("div");
    detailDiv.style.fontSize = "11px";
    detailDiv.style.color = "#8b949e";
    detailDiv.style.marginTop = "2px";
    detailDiv.textContent = phase.steps.length + " steps" + (groupFailed ? " (" + groupFailCount + " failed)" : "");
    step.appendChild(detailDiv);

    var metaDiv = document.createElement("div");
    metaDiv.className = "step-meta";

    var timeSpan = document.createElement("span");
    timeSpan.textContent = groupTime > 0 ? formatDuration(groupTime) : "\u2014";
    metaDiv.appendChild(timeSpan);

    var statusSpan = document.createElement("span");
    statusSpan.className = "step-status " + groupStatus;
    statusSpan.textContent = groupLabel;
    metaDiv.appendChild(statusSpan);

    step.appendChild(metaDiv);

    if (groupMsg) {
      step.addEventListener("click", function(e) {
        var tooltip = document.getElementById("error-tooltip");
        tooltip.textContent = groupMsg;
        tooltip.style.left = Math.min(e.clientX, window.innerWidth - 620) + "px";
        tooltip.style.top = Math.min(e.clientY + 10, window.innerHeight - 320) + "px";
        tooltip.classList.toggle("visible");
      });
    }

    pipeline.appendChild(step);
  } else {
    // Render individual steps
    phase.steps.forEach(function(stepName, idx) {
      if (idx > 0) {
        var conn = document.createElement("div");
        conn.className = "connector";
        conn.textContent = "\u2192";
        pipeline.appendChild(conn);
      }

      var info = getStepInfo(stepName);

      if (info.status === "passed") totalPassed++;
      else if (info.status === "failed") totalFailed++;
      else totalNotReached++;
      totalTime += info.time;

      var step = document.createElement("div");
      step.className = "step " + info.status;

      var nameDiv = document.createElement("div");
      nameDiv.className = "step-name";
      nameDiv.textContent = stepName;
      step.appendChild(nameDiv);

      var metaDiv = document.createElement("div");
      metaDiv.className = "step-meta";

      var timeSpan = document.createElement("span");
      timeSpan.textContent = info.time > 0 ? formatDuration(info.time) : "\u2014";
      metaDiv.appendChild(timeSpan);

      var statusSpan = document.createElement("span");
      statusSpan.className = "step-status " + info.status;
      statusSpan.textContent = info.label;
      metaDiv.appendChild(statusSpan);

      step.appendChild(metaDiv);

      if (info.message) {
        step.addEventListener("click", function(e) {
          var tooltip = document.getElementById("error-tooltip");
          tooltip.textContent = info.message;
          tooltip.style.left = Math.min(e.clientX, window.innerWidth - 620) + "px";
          tooltip.style.top = Math.min(e.clientY + 10, window.innerHeight - 320) + "px";
          tooltip.classList.toggle("visible");
        });
      }

      pipeline.appendChild(step);
    });
  }

  phaseDiv.appendChild(pipeline);
  container.appendChild(phaseDiv);
});

// Close tooltip on outside click
document.addEventListener("click", function(e) {
  if (!e.target.closest(".step.failed")) {
    document.getElementById("error-tooltip").classList.remove("visible");
  }
});

// Summary cards
const summaryDiv = document.getElementById("summary");
var cards = [
  { label: "Passed", value: totalPassed, cls: "green" },
  { label: "Failed", value: totalFailed, cls: "red" },
  { label: "Not Run", value: totalNotReached, cls: "gray" },
  { label: "Total Time", value: formatDuration(totalTime), cls: "" }
];
cards.forEach(function(card) {
  const cardDiv = document.createElement("div");
  cardDiv.className = "summary-card";
  const labelDiv = document.createElement("div");
  labelDiv.className = "label";
  labelDiv.textContent = card.label;
  const valueDiv = document.createElement("div");
  valueDiv.className = "value" + (card.cls ? " " + card.cls : "");
  valueDiv.textContent = card.value;
  cardDiv.appendChild(labelDiv);
  cardDiv.appendChild(valueDiv);
  summaryDiv.appendChild(cardDiv);
});

document.getElementById("subtitle").textContent = "Generated: " + new Date().toLocaleString();
</script>
</body>
</html>
HTMLEOF

    # Inject the actual steps JSON
    local escaped_json
    escaped_json=$(echo "$steps_json" | sed 's/[&/\]/\\&/g')
    if [[ "$(uname)" == "Darwin" ]]; then
        sed -i '' "s|STEPS_JSON_PLACEHOLDER|${escaped_json}|" "$OUTPUT_FILE"
    else
        sed -i "s|STEPS_JSON_PLACEHOLDER|${escaped_json}|" "$OUTPUT_FILE"
    fi
}

# Main
[[ $# -lt 1 ]] && usage

xml_file=$(resolve_input "$1")
echo "Parsing: $xml_file"

steps_json=$(parse_junit "$xml_file")
echo "Found $(echo "$steps_json" | tr ',' '\n' | grep -c '"name"') steps"

generate_html "$steps_json"
echo "Generated: $OUTPUT_FILE"

# Open in browser
if [[ "$(uname)" == "Darwin" ]]; then
    open "$OUTPUT_FILE"
elif command -v xdg-open &>/dev/null; then
    xdg-open "$OUTPUT_FILE"
else
    echo "Open $OUTPUT_FILE in your browser"
fi
