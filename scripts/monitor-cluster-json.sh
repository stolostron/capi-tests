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

# Build kubectl command with optional context
KUBECTL_CMD="kubectl"
if [[ -n "$KUBECTL_CONTEXT" ]]; then
    KUBECTL_CMD="kubectl --context $KUBECTL_CONTEXT"
fi

# Get Cluster
CLUSTER_JSON=$($KUBECTL_CMD get cluster "$CLUSTER_NAME" -n "$NAMESPACE" -o json 2>/dev/null || echo '{}')

if [[ "$CLUSTER_JSON" == "{}" ]]; then
    echo '{"error": "Cluster not found", "namespace": "'"$NAMESPACE"'", "name": "'"$CLUSTER_NAME"'"}' | jq .
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

OUTPUT=$(echo "$OUTPUT" | jq --argjson conditions "$CLUSTER_CONDITIONS" \
    --arg phase "$CLUSTER_PHASE" \
    --argjson infraReady "$CLUSTER_INFRA_READY" \
    --argjson cpReady "$CLUSTER_CONTROL_PLANE_READY" \
    '.cluster = {
        name: .metadata.clusterName,
        namespace: .metadata.namespace,
        phase: $phase,
        infrastructureReady: $infraReady,
        controlPlaneReady: $cpReady,
        conditions: $conditions
    }')

# Process Infrastructure Cluster
INFRA_REF=$(echo "$CLUSTER_JSON" | jq -r '.spec.infrastructureRef')
INFRA_KIND=$(echo "$INFRA_REF" | jq -r '.kind // ""')
INFRA_NAME=$(echo "$INFRA_REF" | jq -r '.name // ""')

if [[ -n "$INFRA_KIND" ]] && [[ -n "$INFRA_NAME" ]]; then
    INFRA_RESOURCE=$(echo "$INFRA_KIND" | tr '[:upper:]' '[:lower:]')s
    INFRA_JSON=$($KUBECTL_CMD get "$INFRA_RESOURCE" "$INFRA_NAME" -n "$NAMESPACE" -o json 2>/dev/null || echo '{}')

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
    CP_JSON=$($KUBECTL_CMD get "$CP_RESOURCE" "$CP_NAME" -n "$NAMESPACE" -o json 2>/dev/null || echo '{}')

    if [[ "$CP_JSON" != "{}" ]]; then
        CP_READY=$(echo "$CP_JSON" | jq -r '.status.ready // null')
        CP_REPLICAS=$(echo "$CP_JSON" | jq -r '.status.replicas // 0')
        CP_READY_REPLICAS=$(echo "$CP_JSON" | jq -r '.status.readyReplicas // 0')
        CP_CONDITIONS=$(echo "$CP_JSON" | jq '.status.conditions // []')
        CP_RESOURCES=$(echo "$CP_JSON" | jq '.status.resources // []')

        OUTPUT=$(echo "$OUTPUT" | jq --arg kind "$CP_KIND" \
            --arg name "$CP_NAME" \
            --argjson ready "$CP_READY" \
            --argjson replicas "$CP_REPLICAS" \
            --argjson readyReplicas "$CP_READY_REPLICAS" \
            --argjson conditions "$CP_CONDITIONS" \
            --argjson resources "$CP_RESOURCES" \
            '.controlPlane = {
                kind: $kind,
                name: $name,
                ready: $ready,
                replicas: $replicas,
                readyReplicas: $readyReplicas,
                conditions: $conditions,
                resources: $resources
            }')
    fi
fi

# Process Machine Pools
MP_JSON=$($KUBECTL_CMD get machinepool -n "$NAMESPACE" -l cluster.x-k8s.io/cluster-name="$CLUSTER_NAME" -o json 2>/dev/null || echo '{"items":[]}')
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
            INFRA_MP_JSON=$($KUBECTL_CMD get "$INFRA_MP_RESOURCE" "$INFRA_MP_NAME" -n "$NAMESPACE" -o json 2>/dev/null || echo '{}')

            if [[ "$INFRA_MP_JSON" != "{}" ]]; then
                INFRA_MP_READY=$(echo "$INFRA_MP_JSON" | jq -r '.status.ready // null')
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
fi

# Process Nodes
KUBECONFIG_SECRET="${CLUSTER_NAME}-kubeconfig"
if $KUBECTL_CMD get secret "$KUBECONFIG_SECRET" -n "$NAMESPACE" &>/dev/null; then
    NODES_JSON=$($KUBECTL_CMD get secret "$KUBECONFIG_SECRET" -n "$NAMESPACE" -o jsonpath='{.data.value}' | base64 -d | \
        KUBECONFIG=/dev/stdin $KUBECTL_CMD get nodes -o json 2>/dev/null || echo '{"items":[]}')

    if [[ "$NODES_JSON" != '{"items":[]}' ]]; then
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

        OUTPUT=$(echo "$OUTPUT" | jq --argjson nodes "$NODES" '.nodes = $nodes')
    else
        OUTPUT=$(echo "$OUTPUT" | jq '.nodes = []')
    fi
else
    OUTPUT=$(echo "$OUTPUT" | jq '.nodes = null')
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
