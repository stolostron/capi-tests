package test

import (
	"strings"
	"testing"
)

// TestMonitorCluster demonstrates how to use the generic cluster monitoring.
// This test shows how the monitor works with any CAPI cluster (ARO, ROSA, etc.)
func TestMonitorCluster(t *testing.T) {
	config := NewTestConfig()

	// Skip if no repository cloned
	if !DirExists(config.RepoDir) {
		t.Skip("Repository not cloned, skipping monitor test")
	}

	// Skip if no workload cluster namespace configured
	if config.WorkloadClusterNamespace == "" {
		t.Skip("No workload cluster namespace configured, skipping monitor test")
	}

	clusterName := config.GetProvisionedClusterName()

	t.Run("MonitorOnce", func(t *testing.T) {
		context := config.GetKubeContext()

		// Get a single snapshot of cluster status
		data, err := MonitorCluster(t, context, config.WorkloadClusterNamespace, clusterName)
		if err != nil {
			// Only skip if cluster doesn't exist - fail on monitor regressions
			errMsg := err.Error()
			if strings.Contains(strings.ToLower(errMsg), "not found") ||
				strings.Contains(strings.ToLower(errMsg), "notfound") {
				t.Skipf("Cluster not found (may not exist yet): %v", err)
			}
			// Script breakage, auth failure, JSON parsing error - these should fail
			t.Fatalf("Monitor script failed: %v", err)
		}

		// Display the summary
		t.Logf("Cluster status: %s", data.FormatSummary())
		t.Logf("Provider type: %s", data.GetProviderType())
		t.Logf("Infrastructure kind: %s", data.Infrastructure.Kind)
		t.Logf("Control plane kind: %s", data.ControlPlane.Kind)

		// Check various status flags
		t.Logf("Infrastructure ready: %v", data.Infrastructure.Ready)
		t.Logf("Control plane ready: %v (%d/%d replicas)",
			data.ControlPlane.Ready,
			data.ControlPlane.ReadyReplicas,
			data.ControlPlane.Replicas)

		if len(data.MachinePools) > 0 {
			t.Logf("Machine pools: %d", len(data.MachinePools))
			for _, mp := range data.MachinePools {
				infraKind := "unknown"
				if mp.Infrastructure != nil {
					infraKind = mp.Infrastructure.Kind
				}
				t.Logf("  - %s: %d/%d replicas ready (kind: %s)",
					mp.Name,
					mp.ReadyReplicas,
					mp.Replicas,
					infraKind)
			}
		}

		if data.HasNodes() {
			t.Logf("Nodes: %d total, %d ready", data.Summary.NodeCount, data.GetReadyNodeCount())
			for _, node := range data.Nodes {
				t.Logf("  - %s: ready=%s, roles=%s, version=%s",
					node.Name,
					node.Ready,
					node.Roles,
					node.Version)
			}
		} else {
			t.Log("No nodes available yet")
		}
	})
}
