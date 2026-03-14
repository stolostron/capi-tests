#!/bin/bash
set -o pipefail

# Usage: ./monitor-cluster-json.sh [--context <context>] <namespace> <cluster-name>
# Example: ./monitor-cluster-json.sh --context kind-capz-tests-stage mv9 mv9-stage
# Example: ./monitor-cluster-json.sh mv9 mv9-stage

# Parse optional --context parameter
KUBECTL_CONTEXT=""
if [[ "$1" == "--context" ]]; then
    KUBECTL_CONTEXT="$2"
    shift 2
fi

NAMESPACE="${1:-}"
CLUSTER_NAME="${2:-}"

if [[ -z "$NAMESPACE" ]] || [[ -z "$CLUSTER_NAME" ]]; then
    echo '{"error": "Usage: '"$0"' [--context <context>] <namespace> <cluster-name>"}' | jq .
    exit 1
fi

# Build kubectl command with optional context (use array to prevent shell injection)
KUBECTL_CMD=(kubectl)
if [[ -n "$KUBECTL_CONTEXT" ]]; then
    KUBECTL_CMD=(kubectl --context "$KUBECTL_CONTEXT")
fi

# Get Cluster
# Capture both stdout and stderr separately to distinguish errors
KUBECTL_STDERR=$(mktemp)
CLUSTER_JSON=$("${KUBECTL_CMD[@]}" get cluster "$CLUSTER_NAME" -n "$NAMESPACE" -o json 2>"$KUBECTL_STDERR")
KUBECTL_EXIT_CODE=$?
KUBECTL_ERROR=$(cat "$KUBECTL_STDERR")
rm -f "$KUBECTL_STDERR"

if [[ $KUBECTL_EXIT_CODE -ne 0 ]]; then
    # kubectl failed - emit error details in JSON
    jq -n \
        --arg namespace "$NAMESPACE" \
        --arg name "$CLUSTER_NAME" \
        --arg error "$KUBECTL_ERROR" \
        --argjson exitCode "$KUBECTL_EXIT_CODE" \
        '{
            error: "kubectl get cluster failed",
            namespace: $namespace,
            name: $name,
            details: $error,
            exitCode: $exitCode
        }'
    exit 1
fi

# Initialize output structure
OUTPUT=$(jq -n '{
    metadata: {
        timestamp: now | todate,
        namespace: $namespace,
        clusterName: $clusterName
    },
    cluster: {},
    infrastructure: {},
    controlPlane: {},
    machinePools: [],
    nodes: [],
    summary: {}
}' \
    --arg namespace "$NAMESPACE" \
    --arg clusterName "$CLUSTER_NAME")

# Process Cluster
CLUSTER_PHASE=$(echo "$CLUSTER_JSON" | jq -r '.status.phase // "Unknown"')
CLUSTER_CONDITIONS=$(echo "$CLUSTER_JSON" | jq '.status.conditions // []')

# Try v1beta1 fields first, fall back to v1beta2 conditions
CLUSTER_INFRA_READY=$(echo "$CLUSTER_JSON" | jq -r '.status.infrastructureReady // null')
if [[ "$CLUSTER_INFRA_READY" == "null" ]]; then
    # v1beta2: derive from InfrastructureReady condition
    CLUSTER_INFRA_READY=$(echo "$CLUSTER_CONDITIONS" | jq -r '.[] | select(.type == "InfrastructureReady") | .status == "True"')
    [[ -z "$CLUSTER_INFRA_READY" ]] && CLUSTER_INFRA_READY="null"
fi

CLUSTER_CONTROL_PLANE_READY=$(echo "$CLUSTER_JSON" | jq -r '.status.controlPlaneReady // null')
if [[ "$CLUSTER_CONTROL_PLANE_READY" == "null" ]]; then
    # v1beta2: derive from ControlPlaneAvailable condition
    CLUSTER_CONTROL_PLANE_READY=$(echo "$CLUSTER_CONDITIONS" | jq -r '.[] | select(.type == "ControlPlaneAvailable") | .status == "True"')
    [[ -z "$CLUSTER_CONTROL_PLANE_READY" ]] && CLUSTER_CONTROL_PLANE_READY="null"
fi

# Extract initialization.infrastructureProvisioned (CAPI cluster deployment gate)
CLUSTER_INFRA_PROVISIONED=$(echo "$CLUSTER_JSON" | jq -r '.status.initialization.infrastructureProvisioned // null')

OUTPUT=$(echo "$OUTPUT" | jq --argjson conditions "$CLUSTER_CONDITIONS" \
    --arg phase "$CLUSTER_PHASE" \
    --argjson infraReady "$CLUSTER_INFRA_READY" \
    --argjson cpReady "$CLUSTER_CONTROL_PLANE_READY" \
    --argjson infraProvisioned "$CLUSTER_INFRA_PROVISIONED" \
    '.cluster = {
        name: .metadata.clusterName,
        namespace: .metadata.namespace,
        phase: $phase,
        infrastructureReady: $infraReady,
        controlPlaneReady: $cpReady,
        infrastructureProvisioned: $infraProvisioned,
        conditions: $conditions
    }')

# Process Infrastructure Cluster
INFRA_REF=$(echo "$CLUSTER_JSON" | jq -r '.spec.infrastructureRef')
INFRA_KIND=$(echo "$INFRA_REF" | jq -r '.kind // ""')
INFRA_NAME=$(echo "$INFRA_REF" | jq -r '.name // ""')

if [[ -n "$INFRA_KIND" ]] && [[ -n "$INFRA_NAME" ]]; then
    INFRA_RESOURCE=$(echo "$INFRA_KIND" | tr '[:upper:]' '[:lower:]')s
    INFRA_JSON=$("${KUBECTL_CMD[@]}" get "$INFRA_RESOURCE" "$INFRA_NAME" -n "$NAMESPACE" -o json 2>/dev/null || echo '{}')

    if [[ "$INFRA_JSON" != "{}" ]]; then
        INFRA_READY=$(echo "$INFRA_JSON" | jq -r '.status.ready // null')
        INFRA_CONDITIONS=$(echo "$INFRA_JSON" | jq '.status.conditions // []')
        INFRA_RESOURCES=$(echo "$INFRA_JSON" | jq '.status.resources // []')

        OUTPUT=$(echo "$OUTPUT" | jq --arg kind "$INFRA_KIND" \
            --arg name "$INFRA_NAME" \
            --argjson ready "$INFRA_READY" \
            --argjson conditions "$INFRA_CONDITIONS" \
            --argjson resources "$INFRA_RESOURCES" \
            '.infrastructure = {
                kind: $kind,
                name: $name,
                ready: $ready,
                conditions: $conditions,
                resources: $resources
            }')
    fi
fi

# Process Control Plane
CP_REF=$(echo "$CLUSTER_JSON" | jq -r '.spec.controlPlaneRef')
CP_KIND=$(echo "$CP_REF" | jq -r '.kind // ""')
CP_NAME=$(echo "$CP_REF" | jq -r '.name // ""')

if [[ -n "$CP_KIND" ]] && [[ -n "$CP_NAME" ]]; then
    CP_RESOURCE=$(echo "$CP_KIND" | tr '[:upper:]' '[:lower:]')s
    CP_JSON=$("${KUBECTL_CMD[@]}" get "$CP_RESOURCE" "$CP_NAME" -n "$NAMESPACE" -o json 2>/dev/null || echo '{}')

    if [[ "$CP_JSON" != "{}" ]]; then
        CP_READY=$(echo "$CP_JSON" | jq '.status.ready')
        CP_REPLICAS=$(echo "$CP_JSON" | jq -r '.status.replicas // 0')
        CP_READY_REPLICAS=$(echo "$CP_JSON" | jq -r '.status.readyReplicas // 0')
        CP_CONDITIONS=$(echo "$CP_JSON" | jq '.status.conditions // []')
        CP_RESOURCES=$(echo "$CP_JSON" | jq '.status.resources // []')

        # Extract control plane state from *ControlPlaneReady condition's reason
        # Captures states like: validating, installing, uninstalling, deleting, upgraded, etc.
        CP_STATE=$(echo "$CP_CONDITIONS" | jq -r '.[] | select(.type | endswith("ControlPlaneReady")) | .reason // ""' | head -1)

        OUTPUT=$(echo "$OUTPUT" | jq --arg kind "$CP_KIND" \
            --arg name "$CP_NAME" \
            --argjson ready "$CP_READY" \
            --argjson replicas "$CP_REPLICAS" \
            --argjson readyReplicas "$CP_READY_REPLICAS" \
            --argjson conditions "$CP_CONDITIONS" \
            --argjson resources "$CP_RESOURCES" \
            --arg state "$CP_STATE" \
            '.controlPlane = {
                kind: $kind,
                name: $name,
                ready: $ready,
                replicas: $replicas,
                readyReplicas: $readyReplicas,
                state: (if $state == "" then null else $state end),
                conditions: $conditions,
                resources: $resources
            }')
    fi
fi

# Process Machine Pools
MP_JSON=$("${KUBECTL_CMD[@]}" get machinepool -n "$NAMESPACE" -l cluster.x-k8s.io/cluster-name="$CLUSTER_NAME" -o json 2>/dev/null || echo '{"items":[]}')
MP_COUNT=$(echo "$MP_JSON" | jq '.items | length')

if [[ "$MP_COUNT" -gt 0 ]]; then
    MACHINE_POOLS='[]'

    while IFS= read -r mp; do
        MP_NAME=$(echo "$mp" | jq -r '.metadata.name')
        MP_REPLICAS=$(echo "$mp" | jq -r '.spec.replicas // 0')
        MP_READY_REPLICAS=$(echo "$mp" | jq -r '.status.readyReplicas // 0')
        MP_AVAILABLE_REPLICAS=$(echo "$mp" | jq -r '.status.availableReplicas // 0')
        MP_CONDITIONS=$(echo "$mp" | jq '.status.conditions // []')

        # Get infrastructure MachinePool
        INFRA_MP_REF=$(echo "$mp" | jq -r '.spec.template.spec.infrastructureRef')
        INFRA_MP_KIND=$(echo "$INFRA_MP_REF" | jq -r '.kind // ""')
        INFRA_MP_NAME=$(echo "$INFRA_MP_REF" | jq -r '.name // ""')

        INFRA_MP_DATA='null'
        if [[ -n "$INFRA_MP_KIND" ]] && [[ -n "$INFRA_MP_NAME" ]]; then
            INFRA_MP_RESOURCE=$(echo "$INFRA_MP_KIND" | tr '[:upper:]' '[:lower:]')s
            INFRA_MP_JSON=$("${KUBECTL_CMD[@]}" get "$INFRA_MP_RESOURCE" "$INFRA_MP_NAME" -n "$NAMESPACE" -o json 2>/dev/null || echo '{}')

            if [[ "$INFRA_MP_JSON" != "{}" ]]; then
                INFRA_MP_READY=$(echo "$INFRA_MP_JSON" | jq '.status.ready')
                INFRA_MP_REPLICAS=$(echo "$INFRA_MP_JSON" | jq -r '.status.replicas // 0')
                INFRA_MP_PROVISIONING=$(echo "$INFRA_MP_JSON" | jq -r '.status.provisioningState // ""')
                INFRA_MP_PROVIDER_IDS=$(echo "$INFRA_MP_JSON" | jq '.spec.providerIDList // []')
                INFRA_MP_CONDITIONS=$(echo "$INFRA_MP_JSON" | jq '.status.conditions // []')
                INFRA_MP_RESOURCES=$(echo "$INFRA_MP_JSON" | jq '.status.resources // []')

                INFRA_MP_DATA=$(jq -n \
                    --arg kind "$INFRA_MP_KIND" \
                    --arg name "$INFRA_MP_NAME" \
                    --argjson ready "$INFRA_MP_READY" \
                    --argjson replicas "$INFRA_MP_REPLICAS" \
                    --arg provisioningState "$INFRA_MP_PROVISIONING" \
                    --argjson providerIDList "$INFRA_MP_PROVIDER_IDS" \
                    --argjson conditions "$INFRA_MP_CONDITIONS" \
                    --argjson resources "$INFRA_MP_RESOURCES" \
                    '{
                        kind: $kind,
                        name: $name,
                        ready: $ready,
                        replicas: $replicas,
                        provisioningState: $provisioningState,
                        providerIDList: $providerIDList,
                        providerIDCount: ($providerIDList | length),
                        conditions: $conditions,
                        resources: $resources
                    }')
            fi
        fi

        MP_DATA=$(jq -n \
            --arg name "$MP_NAME" \
            --argjson replicas "$MP_REPLICAS" \
            --argjson readyReplicas "$MP_READY_REPLICAS" \
            --argjson availableReplicas "$MP_AVAILABLE_REPLICAS" \
            --argjson conditions "$MP_CONDITIONS" \
            --argjson infrastructure "$INFRA_MP_DATA" \
            '{
                name: $name,
                replicas: $replicas,
                readyReplicas: $readyReplicas,
                availableReplicas: $availableReplicas,
                conditions: $conditions,
                infrastructure: $infrastructure
            }')

        MACHINE_POOLS=$(echo "$MACHINE_POOLS" | jq --argjson mp "$MP_DATA" '. += [$mp]')
    done < <(echo "$MP_JSON" | jq -c '.items[]')

    OUTPUT=$(echo "$OUTPUT" | jq --argjson mps "$MACHINE_POOLS" '.machinePools = $mps')
else
    # No real MachinePools found - check if we should create a synthetic one
    # ROSA uses defaultMachinePoolSpec in ROSAControlPlane instead of separate MachinePool resources
    if [[ "$CP_KIND" == "ROSAControlPlane" ]] && [[ "$CP_JSON" != "{}" ]]; then
        HAS_DEFAULT_MP_SPEC=$(echo "$CP_JSON" | jq 'has("spec") and (.spec | has("defaultMachinePoolSpec"))')

        if [[ "$HAS_DEFAULT_MP_SPEC" == "true" ]]; then
            # Extract defaultMachinePoolSpec details
            DEFAULT_MP_SPEC=$(echo "$CP_JSON" | jq '.spec.defaultMachinePoolSpec')

            # Get autoscaling config (min/max replicas)
            MIN_REPLICAS=$(echo "$DEFAULT_MP_SPEC" | jq -r '.autoscaling.minReplicas // 3')
            MAX_REPLICAS=$(echo "$DEFAULT_MP_SPEC" | jq -r '.autoscaling.maxReplicas // 6')

            # Derive readiness from control plane status
            # If control plane is ready, assume default machine pool is also ready with min replicas
            SYNTHETIC_READY_REPLICAS=0
            SYNTHETIC_AVAILABLE_REPLICAS=0
            if [[ "$CP_READY" == "true" ]]; then
                SYNTHETIC_READY_REPLICAS=$MIN_REPLICAS
                SYNTHETIC_AVAILABLE_REPLICAS=$MIN_REPLICAS
            fi

            # Create synthetic infrastructure MachinePool
            SYNTHETIC_INFRA_MP=$(jq -n \
                --arg kind "ROSAMachinePool" \
                --arg name "default" \
                --argjson ready "$CP_READY" \
                --argjson replicas "$MIN_REPLICAS" \
                --arg provisioningState "Succeeded" \
                '{
                    kind: $kind,
                    name: $name,
                    ready: $ready,
                    replicas: $replicas,
                    provisioningState: ($ready | if . == true then "Succeeded" else "Provisioning" end),
                    providerIDList: [],
                    providerIDCount: 0,
                    conditions: [],
                    resources: []
                }')

            # Create synthetic MachinePool entry
            SYNTHETIC_MP=$(jq -n \
                --arg name "default" \
                --argjson replicas "$MIN_REPLICAS" \
                --argjson readyReplicas "$SYNTHETIC_READY_REPLICAS" \
                --argjson availableReplicas "$SYNTHETIC_AVAILABLE_REPLICAS" \
                --argjson infrastructure "$SYNTHETIC_INFRA_MP" \
                '{
                    name: $name,
                    replicas: $replicas,
                    readyReplicas: $readyReplicas,
                    availableReplicas: $availableReplicas,
                    conditions: [],
                    infrastructure: $infrastructure
                }')

            # Add synthetic MachinePool to output
            MACHINE_POOLS=$(jq -n --argjson mp "$SYNTHETIC_MP" '[$mp]')
            OUTPUT=$(echo "$OUTPUT" | jq --argjson mps "$MACHINE_POOLS" '.machinePools = $mps')

            # Update MP_COUNT to reflect the synthetic pool
            MP_COUNT=1
        fi
    fi
fi

# Process Nodes
KUBECONFIG_SECRET="${CLUSTER_NAME}-kubeconfig"
NODES_ERROR=""
if "${KUBECTL_CMD[@]}" get secret "$KUBECONFIG_SECRET" -n "$NAMESPACE" &>/dev/null; then
    # Try to get nodes, capturing both stdout and stderr
    # Note: Use plain 'kubectl' without context since we're using the workload cluster's kubeconfig via stdin
    NODES_RESULT=$("${KUBECTL_CMD[@]}" get secret "$KUBECONFIG_SECRET" -n "$NAMESPACE" -o jsonpath='{.data.value}' 2>/dev/null | base64 -d | \
        KUBECONFIG=/dev/stdin kubectl get nodes -o json 2>&1)
    NODES_EXIT_CODE=$?

    if [[ $NODES_EXIT_CODE -eq 0 ]]; then
        NODES_JSON="$NODES_RESULT"
    else
        # Command failed - capture the error
        NODES_JSON='{"items":[]}'
        NODES_ERROR="$NODES_RESULT"
    fi

    if [[ "$NODES_JSON" != '{"items":[]}' ]] && [[ -z "$NODES_ERROR" ]]; then
        NODES='[]'
        while IFS= read -r node; do
            NODE_NAME=$(echo "$node" | jq -r '.metadata.name')
            NODE_PROVIDER_ID=$(echo "$node" | jq -r '.spec.providerID // ""')
            NODE_VERSION=$(echo "$node" | jq -r '.status.nodeInfo.kubeletVersion // ""')
            NODE_READY=$(echo "$node" | jq -r '.status.conditions[] | select(.type=="Ready") | .status')
            NODE_ROLES=$(echo "$node" | jq -r '.metadata.labels | to_entries | map(select(.key | startswith("node-role.kubernetes.io/"))) | map(.key | sub("node-role.kubernetes.io/"; "")) | join(",")')

            NODE_DATA=$(jq -n \
                --arg name "$NODE_NAME" \
                --arg providerID "$NODE_PROVIDER_ID" \
                --arg version "$NODE_VERSION" \
                --arg ready "$NODE_READY" \
                --arg roles "$NODE_ROLES" \
                '{
                    name: $name,
                    providerID: $providerID,
                    version: $version,
                    ready: $ready,
                    roles: $roles
                }')

            NODES=$(echo "$NODES" | jq --argjson node "$NODE_DATA" '. += [$node]')
        done < <(echo "$NODES_JSON" | jq -c '.items[]')

        OUTPUT=$(echo "$OUTPUT" | jq --argjson nodes "$NODES" \
            '.nodes = $nodes | .nodesError = null')
    else
        # No nodes yet or error occurred
        OUTPUT=$(echo "$OUTPUT" | jq --arg error "$NODES_ERROR" \
            '.nodes = [] | .nodesError = (if $error == "" then null else $error end)')
    fi
else
    OUTPUT=$(echo "$OUTPUT" | jq '.nodes = null | .nodesError = "Kubeconfig secret not found"')
fi

# Build Summary
NODE_COUNT=$(echo "$OUTPUT" | jq '.nodes | if . == null then 0 else length end')
READY_CONDITIONS=$(echo "$CLUSTER_CONDITIONS" | jq '[.[] | select(.status == "True")] | length')
TOTAL_CONDITIONS=$(echo "$CLUSTER_CONDITIONS" | jq 'length')

OUTPUT=$(echo "$OUTPUT" | jq \
    --arg phase "$CLUSTER_PHASE" \
    --argjson infraReady "$CLUSTER_INFRA_READY" \
    --argjson cpReady "$CLUSTER_CONTROL_PLANE_READY" \
    --argjson mpCount "$MP_COUNT" \
    --argjson nodeCount "$NODE_COUNT" \
    --argjson readyConditions "$READY_CONDITIONS" \
    --argjson totalConditions "$TOTAL_CONDITIONS" \
    '.summary = {
        clusterName: .metadata.clusterName,
        namespace: .metadata.namespace,
        phase: $phase,
        infrastructureReady: $infraReady,
        controlPlaneReady: $cpReady,
        machinePoolCount: $mpCount,
        nodeCount: $nodeCount,
        conditions: {
            ready: $readyConditions,
            total: $totalConditions
        }
    }')

# Output final JSON
echo "$OUTPUT" | jq .
