package test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestDeployment_MonitorCluster tests monitoring the ARO cluster deployment
func TestDeployment_MonitorCluster(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping cluster monitoring in short mode")
	}

	config := NewTestConfig()

	if !DirExists(config.RepoDir) {
		t.Skipf("Repository not cloned yet at %s", config.RepoDir)
	}

	clusterctlPath := filepath.Join(config.RepoDir, config.ClusterctlBinPath)

	// If clusterctl binary doesn't exist, try to use system clusterctl
	if !FileExists(clusterctlPath) {
		t.Logf("clusterctl binary not found at %s, checking system PATH", clusterctlPath)
		if CommandExists("clusterctl") {
			clusterctlPath = "clusterctl"
		} else {
			t.Skipf("clusterctl not found")
		}
	}

	// Set kubectl context to Kind cluster
	context := fmt.Sprintf("kind-%s", config.KindClusterName)
	SetEnvVar(t, "KUBECONFIG", fmt.Sprintf("%s/.kube/config", os.Getenv("HOME")))

	// First, check if cluster resource exists
	t.Logf("Checking for cluster resource: %s", config.ClusterName)

	output, err := RunCommand(t, "kubectl", "--context", context, "get", "cluster", config.ClusterName)
	if err != nil {
		t.Skipf("Cluster resource not found (may not be deployed yet): %v", err)
	}

	t.Logf("Cluster resource exists:\n%s", output)

	// Use clusterctl to describe the cluster
	t.Logf("Monitoring cluster deployment status using clusterctl...")

	output, err = RunCommand(t, clusterctlPath, "describe", "cluster", config.ClusterName, "--show-conditions=all")
	if err != nil {
		t.Logf("clusterctl describe failed (cluster may still be initializing): %v\nOutput: %s", err, output)
	} else {
		t.Logf("Cluster status:\n%s", output)
	}
}

// TestDeployment_WaitForControlPlane waits for control plane to be ready
func TestDeployment_WaitForControlPlane(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping control plane wait in short mode")
	}

	config := NewTestConfig()
	context := fmt.Sprintf("kind-%s", config.KindClusterName)

	// Wait for control plane to be ready (with timeout)
	timeout := 30 * time.Minute
	pollInterval := 30 * time.Second
	startTime := time.Now()

	t.Logf("Waiting for control plane to be ready (timeout: %v)...", timeout)

	for {
		if time.Since(startTime) > timeout {
			t.Errorf("Timeout waiting for control plane to be ready")
			return
		}

		output, err := RunCommand(t, "kubectl", "--context", context, "get",
			"kubeadmcontrolplane", "-A", "-o", "jsonpath={.items[0].status.ready}")

		if err == nil && strings.TrimSpace(output) == "true" {
			t.Log("Control plane is ready!")
			return
		}

		t.Logf("Control plane not ready yet, waiting %v... (elapsed: %v)", pollInterval, time.Since(startTime))
		time.Sleep(pollInterval)
	}
}

// TestDeployment_CheckClusterConditions checks various cluster conditions
func TestDeployment_CheckClusterConditions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping cluster conditions check in short mode")
	}

	config := NewTestConfig()
	context := fmt.Sprintf("kind-%s", config.KindClusterName)

	t.Log("Checking cluster conditions...")

	// Check cluster status
	output, err := RunCommand(t, "kubectl", "--context", context, "get", "cluster", config.ClusterName, "-o", "yaml")
	if err != nil {
		t.Errorf("Failed to get cluster status: %v", err)
		return
	}

	// Log the cluster conditions
	if strings.Contains(output, "status:") {
		t.Log("Cluster has status information")
		// Extract conditions section
		if strings.Contains(output, "conditions:") {
			t.Log("Cluster conditions are available in the output")
		}
	}

	// Check for infrastructure ready condition
	output, err = RunCommand(t, "kubectl", "--context", context, "get", "cluster", config.ClusterName,
		"-o", "jsonpath={.status.conditions[?(@.type=='InfrastructureReady')].status}")

	if err == nil && strings.TrimSpace(output) != "" {
		t.Logf("InfrastructureReady status: %s", output)
	}

	// Check for control plane ready condition
	output, err = RunCommand(t, "kubectl", "--context", context, "get", "cluster", config.ClusterName,
		"-o", "jsonpath={.status.conditions[?(@.type=='ControlPlaneReady')].status}")

	if err == nil && strings.TrimSpace(output) != "" {
		t.Logf("ControlPlaneReady status: %s", output)
	}
}
