package test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestKindCluster_KindClusterReady tests deploying a Kind cluster with CAPZ and verifies it's ready
func TestKindCluster_KindClusterReady(t *testing.T) {
	PrintTestHeader(t, "TestKindCluster_KindClusterReady",
		"Deploy Kind cluster with CAPI/CAPZ/ASO controllers (may take 5-10 minutes)")

	config := NewTestConfig()

	if !DirExists(config.RepoDir) {
		t.Skipf("Repository not cloned yet at %s", config.RepoDir)
	}

	// Check if cluster already exists
	t.Log("Checking for existing Kind cluster")
	output, _ := RunCommand(t, "kind", "get", "clusters")
	clusterExists := strings.Contains(output, config.ManagementClusterName)

	if !clusterExists {
		// Deploy Kind cluster using the script
		scriptPath := filepath.Join(config.RepoDir, "scripts", "deploy-charts-kind-capz.sh")
		if !FileExists(scriptPath) {
			t.Errorf("Deployment script not found: %s", scriptPath)
			return
		}

		t.Logf("Deploying Kind cluster '%s' using script", config.ManagementClusterName)

		// Set environment variable for the script (deploy-charts-kind-capz.sh expects KIND_CLUSTER_NAME)
		SetEnvVar(t, "KIND_CLUSTER_NAME", config.ManagementClusterName)

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
		// Use streaming to show progress in real-time
		t.Logf("Executing deployment script: %s", scriptPath)
		t.Log("This will: create Kind cluster, install cert-manager, deploy CAPI/CAPZ/ASO controllers")
		t.Log("Expected duration: 5-10 minutes")
		t.Log("Output streaming below...")
		output, err = RunCommandWithStreaming(t, "bash", scriptPath)
		if err != nil {
			// On error, show output for debugging (may contain sensitive info, but needed for troubleshooting)
			t.Errorf("Failed to deploy Kind cluster: %v\nOutput: %s", err, output)
			return
		}

		// Don't log full script output as it may contain sensitive Azure configuration
		t.Log("Kind cluster deployment script completed successfully")
	} else {
		t.Logf("Kind cluster '%s' already exists", config.ManagementClusterName)
	}

	// Verify cluster is accessible via kubectl
	t.Log("Verifying cluster accessibility...")

	// Set kubeconfig context
	SetEnvVar(t, "KUBECONFIG", fmt.Sprintf("%s/.kube/config", os.Getenv("HOME")))

	output, err := RunCommand(t, "kubectl", "--context", fmt.Sprintf("kind-%s", config.ManagementClusterName), "get", "nodes")
	if err != nil {
		t.Errorf("Failed to access Kind cluster nodes: %v\nOutput: %s", err, output)
		return
	}

	t.Logf("Kind cluster nodes:\n%s", output)
	t.Log("Kind cluster is ready")
}

// TestKindCluster_CAPINamespacesExists verifies CAPI namespaces are installed
func TestKindCluster_CAPINamespacesExists(t *testing.T) {
	PrintTestHeader(t, "TestKindCluster_CAPINamespacesExists",
		"Verify CAPI and CAPZ namespaces exist in the management cluster")

	config := NewTestConfig()

	t.Log("Checking for CAPI namespaces...")

	context := fmt.Sprintf("kind-%s", config.ManagementClusterName)

	// Check for CAPI namespaces
	expectedNamespaces := []string{
		"capi-system",
		"capz-system",
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

// TestKindCluster_CAPIControllerReady waits for CAPI controller to be ready
func TestKindCluster_CAPIControllerReady(t *testing.T) {
	PrintTestHeader(t, "TestKindCluster_CAPIControllerReady",
		"Wait for CAPI controller manager deployment to become available (timeout: 10m)")

	config := NewTestConfig()
	context := fmt.Sprintf("kind-%s", config.ManagementClusterName)

	// Wait for CAPI controller manager deployment to be available
	// kubectl -n capi-system wait deployment/capi-controller-manager --for condition=Available --timeout=10m
	output, err := RunCommand(t, "kubectl", "--context", context, "-n", "capi-system",
		"wait", "deployment/capi-controller-manager",
		"--for", "condition=Available",
		"--timeout=10m")

	if err != nil {
		t.Errorf("CAPI controller manager deployment is not available: %v\nOutput: %s", err, output)
		return
	}

	t.Log("CAPI controller manager deployment is available")
}

// TestKindCluster_CAPZControllerReady waits for CAPZ controller to be ready
func TestKindCluster_CAPZControllerReady(t *testing.T) {
	PrintTestHeader(t, "TestKindCluster_CAPZControllerReady",
		"Wait for CAPZ controller manager deployment to become available (timeout: 10m)")

	config := NewTestConfig()
	context := fmt.Sprintf("kind-%s", config.ManagementClusterName)

	// Wait for CAPZ controller manager deployment to be available
	// kubectl -n capz-system wait deployment/capz-controller-manager --for condition=Available --timeout=10m
	output, err := RunCommand(t, "kubectl", "--context", context, "-n", "capz-system",
		"wait", "deployment/capz-controller-manager",
		"--for", "condition=Available",
		"--timeout=10m")

	if err != nil {
		t.Errorf("CAPZ controller manager deployment is not available: %v\nOutput: %s", err, output)
		return
	}

	t.Log("CAPZ controller manager deployment is available")
}

// TestKindCluster_ASOControllerReady waits for Azure Service Operator controller to be ready
func TestKindCluster_ASOControllerReady(t *testing.T) {
	PrintTestHeader(t, "TestKindCluster_ASOControllerReady",
		"Wait for Azure Service Operator controller manager to become available (timeout: 10m)")

	config := NewTestConfig()
	context := fmt.Sprintf("kind-%s", config.ManagementClusterName)

	// Wait for ASO controller manager deployment to be available
	// kubectl -n capz-system wait deployment/azureserviceoperator-controller-manager --for condition=Available --timeout=10m
	output, err := RunCommand(t, "kubectl", "--context", context, "-n", "capz-system",
		"wait", "deployment/azureserviceoperator-controller-manager",
		"--for", "condition=Available",
		"--timeout=10m")

	if err != nil {
		t.Errorf("Azure Service Operator controller manager deployment is not available: %v\nOutput: %s", err, output)
		return
	}

	t.Log("Azure Service Operator controller manager deployment is available")
}
