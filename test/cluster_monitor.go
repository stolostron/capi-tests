package test

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// ClusterMonitorData represents the full JSON output from monitor-cluster-json.sh
type ClusterMonitorData struct {
	Metadata       ClusterMetadata      `json:"metadata"`
	Cluster        ClusterStatus        `json:"cluster"`
	Infrastructure InfrastructureStatus `json:"infrastructure"`
	ControlPlane   ControlPlaneStatus   `json:"controlPlane"`
	MachinePools   []MachinePoolStatus  `json:"machinePools"`
	Nodes          []NodeStatus         `json:"nodes"`
	NodesError     *string              `json:"nodesError"` // Error message when failing to connect to workload cluster
	Summary        ClusterSummary       `json:"summary"`
}

// ClusterMetadata contains metadata about the monitoring snapshot
type ClusterMetadata struct {
	Timestamp   string `json:"timestamp"`
	Namespace   string `json:"namespace"`
	ClusterName string `json:"clusterName"`
}

// ClusterStatus represents the CAPI Cluster resource status
type ClusterStatus struct {
	Name                      string         `json:"name"`
	Namespace                 string         `json:"namespace"`
	Phase                     string         `json:"phase"`
	InfrastructureReady       bool           `json:"infrastructureReady"`
	ControlPlaneReady         bool           `json:"controlPlaneReady"`
	InfrastructureProvisioned bool           `json:"infrastructureProvisioned"` // cluster.status.initialization.infrastructureProvisioned
	Conditions                []K8sCondition `json:"conditions"`
}

// InfrastructureStatus represents the infrastructure cluster (AROCluster, ROSACluster, etc.)
type InfrastructureStatus struct {
	Kind       string         `json:"kind"`
	Name       string         `json:"name"`
	Ready      bool           `json:"ready"`
	Conditions []K8sCondition `json:"conditions"`
	Resources  []interface{}  `json:"resources"` // ASO/ACK resources array
}

// ControlPlaneStatus represents the control plane resource (AROControlPlane, ROSAControlPlane, etc.)
type ControlPlaneStatus struct {
	Kind          string         `json:"kind"`
	Name          string         `json:"name"`
	Ready         bool           `json:"ready"`
	Replicas      int            `json:"replicas"`
	ReadyReplicas int            `json:"readyReplicas"`
	State         *string        `json:"state"` // Control plane state from *ControlPlaneReady condition (e.g., validating, installing, uninstalling)
	Conditions    []K8sCondition `json:"conditions"`
	Resources     []interface{}  `json:"resources"` // ASO resources array
}

// MachinePoolStatus represents a MachinePool and its infrastructure counterpart
type MachinePoolStatus struct {
	Name              string                     `json:"name"`
	Replicas          int                        `json:"replicas"`
	ReadyReplicas     int                        `json:"readyReplicas"`
	AvailableReplicas int                        `json:"availableReplicas"`
	Conditions        []K8sCondition             `json:"conditions"`
	Infrastructure    *MachinePoolInfrastructure `json:"infrastructure"`
}

// MachinePoolInfrastructure represents the infrastructure-specific MachinePool (AROMachinePool, ROSAMachinePool)
type MachinePoolInfrastructure struct {
	Kind              string         `json:"kind"`
	Name              string         `json:"name"`
	Ready             bool           `json:"ready"`
	Replicas          int            `json:"replicas"`
	ProvisioningState string         `json:"provisioningState"`
	ProviderIDList    []string       `json:"providerIDList"`
	ProviderIDCount   int            `json:"providerIDCount"`
	Conditions        []K8sCondition `json:"conditions"`
	Resources         []interface{}  `json:"resources"` // ASO resources array
}

// NodeStatus represents a workload cluster node
type NodeStatus struct {
	Name       string `json:"name"`
	ProviderID string `json:"providerID"`
	Version    string `json:"version"`
	Ready      string `json:"ready"` // "True" or "False"
	Roles      string `json:"roles"` // comma-separated roles
}

// K8sCondition represents a Kubernetes condition
type K8sCondition struct {
	Type               string `json:"type"`
	Status             string `json:"status"`
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	Reason             string `json:"reason,omitempty"`
	Message            string `json:"message,omitempty"`
}

// ClusterSummary provides a high-level summary of cluster status
type ClusterSummary struct {
	ClusterName         string            `json:"clusterName"`
	Namespace           string            `json:"namespace"`
	Phase               string            `json:"phase"`
	InfrastructureReady bool              `json:"infrastructureReady"`
	ControlPlaneReady   bool              `json:"controlPlaneReady"`
	MachinePoolCount    int               `json:"machinePoolCount"`
	NodeCount           int               `json:"nodeCount"`
	Conditions          ConditionsSummary `json:"conditions"`
}

// ConditionsSummary summarizes condition status
type ConditionsSummary struct {
	Ready int `json:"ready"`
	Total int `json:"total"`
}

// MonitorCluster runs the monitor-cluster-json.sh script and parses its JSON output.
// This provides a provider-agnostic way to monitor any CAPI cluster (ARO, ROSA, etc.)
//
// The script is located in the repository's scripts/ directory and runs locally.
//
// Parameters:
//   - kubeContext: Kubernetes context to use for kubectl commands (e.g., "kind-capz-tests-stage")
//   - namespace: Kubernetes namespace containing the cluster
//   - clusterName: Name of the CAPI Cluster resource
//
// Returns:
//   - ClusterMonitorData: Parsed cluster status
//   - error: Any errors during execution or parsing
func MonitorCluster(t *testing.T, kubeContext, namespace, clusterName string) (*ClusterMonitorData, error) {
	t.Helper()

	// Validate inputs don't contain shell metacharacters
	if err := ValidateRFC1123Name(namespace, "namespace"); err != nil {
		return nil, fmt.Errorf("invalid namespace: %w", err)
	}
	if err := ValidateRFC1123Name(clusterName, "cluster name"); err != nil {
		return nil, fmt.Errorf("invalid cluster name: %w", err)
	}

	// When running tests with 'go test ./test', the working directory is the test/ package directory
	// So we need to go up one level to find scripts/
	scriptPath := "../scripts/monitor-cluster-json.sh"

	// Run the monitoring script with --context parameter
	// #nosec G204 -- scriptPath is hardcoded, and kubeContext/namespace/clusterName are validated
	// as RFC 1123 compliant (alphanumeric + hyphens only), making shell injection impossible
	cmd := exec.Command("bash", scriptPath, "--context", kubeContext, namespace, clusterName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to run monitor script: %w\nOutput: %s", err, string(output))
	}

	// Parse JSON output
	var data ClusterMonitorData
	if err := json.Unmarshal(output, &data); err != nil {
		return nil, fmt.Errorf("failed to parse monitor output: %w\nOutput: %s", err, string(output))
	}

	return &data, nil
}

// GetProviderType detects the provider type based on infrastructure and control plane kinds.
// Returns "aro", "rosa", or "unknown"
func (d *ClusterMonitorData) GetProviderType() string {
	infraKind := d.Infrastructure.Kind
	cpKind := d.ControlPlane.Kind

	// Check for ARO
	if infraKind == "AROCluster" || cpKind == "AROControlPlane" {
		return "aro"
	}

	// Check for ROSA
	if infraKind == "ROSACluster" || cpKind == "ROSAControlPlane" {
		return "rosa"
	}

	return "unknown"
}

// IsReady returns true if the cluster is fully ready (all conditions met)
func (d *ClusterMonitorData) IsReady() bool {
	return d.Summary.InfrastructureReady &&
		d.Summary.ControlPlaneReady &&
		d.Summary.Phase == "Provisioned"
}

// IsControlPlaneReady returns true if the control plane is ready
func (d *ClusterMonitorData) IsControlPlaneReady() bool {
	return d.Summary.ControlPlaneReady && d.ControlPlane.Ready
}

// HasNodes returns true if the cluster has at least one node
func (d *ClusterMonitorData) HasNodes() bool {
	return d.Summary.NodeCount > 0
}

// GetReadyNodeCount returns the number of ready nodes
func (d *ClusterMonitorData) GetReadyNodeCount() int {
	count := 0
	for _, node := range d.Nodes {
		if node.Ready == "True" {
			count++
		}
	}
	return count
}

// FormatSummary returns a human-readable summary of cluster status
func (d *ClusterMonitorData) FormatSummary() string {
	provider := d.GetProviderType()
	return fmt.Sprintf(
		"Cluster: %s/%s | Provider: %s | Phase: %s | InfraReady: %v | CPReady: %v | Nodes: %d/%d ready | Conditions: %d/%d",
		d.Metadata.Namespace,
		d.Metadata.ClusterName,
		provider,
		d.Summary.Phase,
		d.Summary.InfrastructureReady,
		d.Summary.ControlPlaneReady,
		d.GetReadyNodeCount(),
		d.Summary.NodeCount,
		d.Summary.Conditions.Ready,
		d.Summary.Conditions.Total,
	)
}

// MonitorClusterUntilReady polls the cluster status until it's ready or timeout is reached.
// This is a generic monitoring function that works for any CAPI cluster.
// Returns the final cluster data when ready.
func MonitorClusterUntilReady(t *testing.T, kubeContext, namespace, clusterName string, timeout time.Duration) (*ClusterMonitorData, error) {
	t.Helper()

	pollInterval := 30 * time.Second
	startTime := time.Now()
	iteration := 0

	for {
		elapsed := time.Since(startTime)
		if elapsed > timeout {
			return nil, fmt.Errorf("timeout waiting for cluster to be ready after %v", elapsed.Round(time.Second))
		}

		iteration++
		t.Logf("[%d] Checking cluster status (elapsed: %v)...", iteration, elapsed.Round(time.Second))

		data, err := MonitorCluster(t, kubeContext, namespace, clusterName)
		if err != nil {
			t.Logf("[%d] Warning: failed to get cluster status: %v", iteration, err)
			time.Sleep(pollInterval)
			continue
		}

		t.Logf("[%d] %s", iteration, data.FormatSummary())

		// Fail-fast: check for terminal cluster phase
		if data.Cluster.Phase == ClusterPhaseFailed {
			return data, fmt.Errorf("cluster phase is 'Failed' — deployment cannot recover")
		}

		// Fail-fast: check conditions for permanent failures
		if err := CheckK8sConditionsForPermanentFailure(data.ControlPlane.Conditions); err != nil {
			return data, fmt.Errorf("permanent failure in %s: %w", data.ControlPlane.Kind, err)
		}
		if err := CheckK8sConditionsForPermanentFailure(data.Infrastructure.Conditions); err != nil {
			return data, fmt.Errorf("permanent failure in %s: %w", data.Infrastructure.Kind, err)
		}

		if data.IsReady() {
			t.Logf("Cluster is ready after %v", elapsed.Round(time.Second))
			return data, nil
		}

		time.Sleep(pollInterval)
	}
}

// MonitorControlPlaneUntilReady waits for the control plane to become ready.
// This is useful when you want to proceed before worker nodes are available.
// Returns the cluster data when control plane is ready.
func MonitorControlPlaneUntilReady(t *testing.T, kubeContext, namespace, clusterName string, timeout time.Duration) (*ClusterMonitorData, error) {
	t.Helper()

	pollInterval := 30 * time.Second
	startTime := time.Now()
	iteration := 0

	for {
		elapsed := time.Since(startTime)
		if elapsed > timeout {
			return nil, fmt.Errorf("timeout waiting for control plane to be ready after %v", elapsed.Round(time.Second))
		}

		iteration++
		t.Logf("[%d] Checking control plane status (elapsed: %v)...", iteration, elapsed.Round(time.Second))

		data, err := MonitorCluster(t, kubeContext, namespace, clusterName)
		if err != nil {
			t.Logf("[%d] Warning: failed to get cluster status: %v", iteration, err)
			time.Sleep(pollInterval)
			continue
		}

		t.Logf("[%d] %s", iteration, data.FormatSummary())

		// Fail-fast: check for terminal cluster phase
		if data.Cluster.Phase == ClusterPhaseFailed {
			return data, fmt.Errorf("cluster phase is 'Failed' — deployment cannot recover")
		}

		// Fail-fast: check control plane conditions for permanent failures
		if err := CheckK8sConditionsForPermanentFailure(data.ControlPlane.Conditions); err != nil {
			return data, fmt.Errorf("permanent failure in %s: %w", data.ControlPlane.Kind, err)
		}

		if data.IsControlPlaneReady() {
			t.Logf("Control plane is ready after %v", elapsed.Round(time.Second))
			return data, nil
		}

		time.Sleep(pollInterval)
	}
}

// MonitorNodesUntilAvailable waits for at least one node to appear in the cluster.
// Returns the cluster data when nodes are detected.
func MonitorNodesUntilAvailable(t *testing.T, kubeContext, namespace, clusterName string, timeout time.Duration) (*ClusterMonitorData, error) {
	t.Helper()

	pollInterval := 30 * time.Second
	startTime := time.Now()
	iteration := 0

	for {
		elapsed := time.Since(startTime)
		if elapsed > timeout {
			return nil, fmt.Errorf("timeout waiting for nodes after %v", elapsed.Round(time.Second))
		}

		iteration++
		t.Logf("[%d] Checking for nodes (elapsed: %v)...", iteration, elapsed.Round(time.Second))

		data, err := MonitorCluster(t, kubeContext, namespace, clusterName)
		if err != nil {
			t.Logf("[%d] Warning: failed to get cluster status: %v", iteration, err)
			time.Sleep(pollInterval)
			continue
		}

		t.Logf("[%d] %s", iteration, data.FormatSummary())

		if data.HasNodes() {
			t.Logf("Nodes available (%d total, %d ready) after %v",
				data.Summary.NodeCount,
				data.GetReadyNodeCount(),
				elapsed.Round(time.Second))
			return data, nil
		}

		time.Sleep(pollInterval)
	}
}

// MonitorClusterUntilDeleted polls until the cluster resource is deleted.
// This is useful for testing cluster deletion - when MonitorCluster returns an error
// indicating the cluster doesn't exist, deletion is complete.
// Returns nil on successful deletion, error on timeout.
func MonitorClusterUntilDeleted(t *testing.T, kubeContext, namespace, clusterName string, timeout time.Duration) error {
	t.Helper()

	pollInterval := 30 * time.Second
	startTime := time.Now()
	iteration := 0

	for {
		elapsed := time.Since(startTime)
		remaining := timeout - elapsed

		if elapsed > timeout {
			return fmt.Errorf("timeout waiting for cluster deletion after %v", elapsed.Round(time.Second))
		}

		iteration++

		PrintToTTY("[%d] Checking deletion status...\n", iteration)
		t.Logf("[%d] Checking if cluster is deleted (elapsed: %v)...", iteration, elapsed.Round(time.Second))

		_, err := MonitorCluster(t, kubeContext, namespace, clusterName)
		if err != nil {
			// Check if this is "not found" (deletion complete) vs. a real error
			errMsg := err.Error()
			// Check for kubectl-specific "not found" errors
			if strings.Contains(errMsg, "NotFound") ||
				strings.Contains(errMsg, "(NotFound)") ||
				(strings.Contains(errMsg, "not found") && strings.Contains(errMsg, "cluster")) {
				// Cluster not found - deletion complete
				PrintToTTY("[%d] ✅ Cluster resource deleted\n\n", iteration)
				t.Logf("Cluster '%s' has been deleted after %v", clusterName, elapsed.Round(time.Second))
				return nil
			}
			// Real error - not just "not found"
			PrintToTTY("[%d] ⚠️  Error checking cluster status: %v\n", iteration, err)
			t.Logf("[%d] Warning: Error checking cluster status (continuing...): %v", iteration, err)
			// Continue waiting - the error might be transient
		}

		// Cluster still exists
		PrintToTTY("[%d] ⏳ Cluster still exists, waiting for deletion...\n", iteration)
		t.Logf("[%d] Cluster still exists, waiting for deletion...", iteration)

		// Report progress
		ReportProgress(t, iteration, elapsed, remaining, timeout)

		time.Sleep(pollInterval)
	}
}
