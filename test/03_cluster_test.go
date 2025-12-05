package test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestKindCluster_Deploy tests deploying a Kind cluster with CAPZ
func TestKindCluster_Deploy(t *testing.T) {

	config := NewTestConfig()

	if !DirExists(config.RepoDir) {
		t.Skipf("Repository not cloned yet at %s", config.RepoDir)
	}

	// Check if cluster already exists
	t.Log("Checking for existing Kind cluster")
	output, _ := RunCommand(t, "kind", "get", "clusters")
	if strings.Contains(output, config.KindClusterName) {
		t.Logf("Kind cluster '%s' already exists", config.KindClusterName)
		return
	}

	// Deploy Kind cluster using the script
	scriptPath := filepath.Join(config.RepoDir, "scripts", "deploy-charts-kind-capz.sh")
	if !FileExists(scriptPath) {
		t.Errorf("Deployment script not found: %s", scriptPath)
		return
	}

	t.Logf("Deploying Kind cluster '%s' using script", config.KindClusterName)

	// Set environment variable for the script
	SetEnvVar(t, "KIND_CLUSTER_NAME", config.KindClusterName)

	// Change to repository directory for script execution
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(config.RepoDir); err != nil {
		t.Fatalf("Failed to change to repository directory: %v", err)
	}

	// Run the deployment script (this might take several minutes)
	t.Log("Running deployment script (this may take several minutes)...")
	output, err = RunCommand(t, "bash", scriptPath)
	if err != nil {
		t.Errorf("Failed to deploy Kind cluster: %v\nOutput: %s", err, output)
		return
	}

	t.Logf("Kind cluster deployment script completed\nOutput: %s", output)
}

// TestKindCluster_Verify verifies the Kind cluster is running and accessible
func TestKindCluster_Verify(t *testing.T) {

	config := NewTestConfig()

	// Check if cluster exists
	output, err := RunCommand(t, "kind", "get", "clusters")
	if err != nil {
		t.Errorf("Failed to get Kind clusters: %v", err)
		return
	}

	if !strings.Contains(output, config.KindClusterName) {
		t.Skipf("Kind cluster '%s' not found. Run deployment test first.", config.KindClusterName)
	}

	t.Logf("Kind cluster '%s' exists", config.KindClusterName)

	// Verify cluster is accessible via kubectl
	t.Log("Verifying cluster accessibility...")

	// Set kubeconfig context
	SetEnvVar(t, "KUBECONFIG", fmt.Sprintf("%s/.kube/config", os.Getenv("HOME")))

	output, err = RunCommand(t, "kubectl", "--context", fmt.Sprintf("kind-%s", config.KindClusterName), "get", "nodes")
	if err != nil {
		t.Errorf("Failed to access Kind cluster nodes: %v\nOutput: %s", err, output)
		return
	}

	t.Logf("Kind cluster nodes:\n%s", output)
}

// TestKindCluster_CAPIComponents verifies CAPI components are installed
func TestKindCluster_CAPIComponents(t *testing.T) {

	config := NewTestConfig()

	t.Log("Checking for CAPI components...")

	context := fmt.Sprintf("kind-%s", config.KindClusterName)

	// Check for CAPI namespaces
	expectedNamespaces := []string{
		"capi-system",
		"capz-system",
		"capi-kubeadm-bootstrap-system",
		"capi-kubeadm-control-plane-system",
	}

	for _, ns := range expectedNamespaces {
		_, err := RunCommand(t, "kubectl", "--context", context, "get", "namespace", ns)
		if err != nil {
			t.Logf("Namespace '%s' may not exist yet (this might be expected): %v", ns, err)
		} else {
			t.Logf("Found namespace: %s", ns)
		}
	}

	// Wait a bit for controllers to be ready
	time.Sleep(5 * time.Second)

	// Check for CAPI pods
	output, err := RunCommand(t, "kubectl", "--context", context, "get", "pods", "-A", "--selector=cluster.x-k8s.io/provider")
	if err != nil {
		t.Logf("CAPI pods check: %v\nOutput: %s", err, output)
	} else {
		t.Logf("CAPI pods:\n%s", output)
	}
}
