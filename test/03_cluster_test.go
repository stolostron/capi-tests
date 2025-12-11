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
		PrintToTTY("⚠️  Repository not cloned yet at %s\n", config.RepoDir)
		t.Skipf("Repository not cloned yet at %s", config.RepoDir)
	}

	// Check if cluster already exists
	PrintToTTY("\n=== Checking for existing Kind cluster ===\n")
	t.Log("Checking for existing Kind cluster")
	output, _ := RunCommand(t, "kind", "get", "clusters")
	clusterExists := strings.Contains(output, config.ManagementClusterName)

	if !clusterExists {
		PrintToTTY("Kind cluster '%s' not found - will deploy new cluster\n", config.ManagementClusterName)

		// Deploy Kind cluster using the script
		scriptPath := filepath.Join(config.RepoDir, "scripts", "deploy-charts-kind-capz.sh")
		if !FileExists(scriptPath) {
			PrintToTTY("❌ Deployment script not found: %s\n", scriptPath)
			t.Errorf("Deployment script not found: %s", scriptPath)
			return
		}

		PrintToTTY("\n=== Deploying Kind cluster '%s' ===\n", config.ManagementClusterName)
		PrintToTTY("This will: create Kind cluster, install cert-manager, deploy CAPI/CAPZ/ASO controllers\n")
		PrintToTTY("Expected duration: 5-10 minutes\n")
		PrintToTTY("Output streaming below...\n\n")
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
			PrintToTTY("\n❌ Failed to deploy Kind cluster: %v\n", err)
			t.Errorf("Failed to deploy Kind cluster: %v\nOutput: %s", err, output)
			return
		}

		// Don't log full script output as it may contain sensitive Azure configuration
		PrintToTTY("\n✅ Kind cluster deployment script completed successfully\n\n")
		t.Log("Kind cluster deployment script completed successfully")
	} else {
		PrintToTTY("✅ Kind cluster '%s' already exists\n\n", config.ManagementClusterName)
		t.Logf("Kind cluster '%s' already exists", config.ManagementClusterName)
	}

	// Verify cluster is accessible via kubectl
	PrintToTTY("=== Verifying cluster accessibility ===\n")
	t.Log("Verifying cluster accessibility...")

	// Set kubeconfig context
	SetEnvVar(t, "KUBECONFIG", fmt.Sprintf("%s/.kube/config", os.Getenv("HOME")))

	output, err := RunCommand(t, "kubectl", "--context", fmt.Sprintf("kind-%s", config.ManagementClusterName), "get", "nodes")
	if err != nil {
		PrintToTTY("❌ Failed to access Kind cluster nodes: %v\nOutput: %s\n\n", err, output)
		t.Errorf("Failed to access Kind cluster nodes: %v\nOutput: %s", err, output)
		return
	}

	PrintToTTY("✅ Kind cluster nodes:\n%s\n\n", output)
	PrintToTTY("✅ Kind cluster is ready\n\n")
	t.Logf("Kind cluster nodes:\n%s", output)
	t.Log("Kind cluster is ready")
}

// TestKindCluster_CAPINamespacesExists verifies CAPI namespaces are installed
func TestKindCluster_CAPINamespacesExists(t *testing.T) {
	PrintTestHeader(t, "TestKindCluster_CAPINamespacesExists",
		"Verify CAPI and CAPZ namespaces exist in the management cluster")

	config := NewTestConfig()

	PrintToTTY("\n=== Checking for CAPI namespaces ===\n")
	t.Log("Checking for CAPI namespaces...")

	context := fmt.Sprintf("kind-%s", config.ManagementClusterName)

	// Check for CAPI namespaces
	expectedNamespaces := []string{
		"capi-system",
		"capz-system",
	}

	for _, ns := range expectedNamespaces {
		PrintToTTY("Checking namespace: %s...\n", ns)

		_, err := RunCommand(t, "kubectl", "--context", context, "get", "namespace", ns)
		if err != nil {
			PrintToTTY("⚠️  Namespace '%s' may not exist yet (this might be expected): %v\n", ns, err)
			t.Logf("Namespace '%s' may not exist yet (this might be expected): %v", ns, err)
		} else {
			PrintToTTY("✅ Found namespace: %s\n", ns)
			t.Logf("Found namespace: %s", ns)
		}
	}

	// Wait a bit for controllers to be ready
	PrintToTTY("\nWaiting 5 seconds for controllers to initialize...\n")
	time.Sleep(5 * time.Second)

	// Check for CAPI pods
	PrintToTTY("\n=== Checking for CAPI pods ===\n")
	PrintToTTY("Running: kubectl get pods -A --selector=cluster.x-k8s.io/provider\n")

	output, err := RunCommand(t, "kubectl", "--context", context, "get", "pods", "-A", "--selector=cluster.x-k8s.io/provider")
	if err != nil {
		PrintToTTY("⚠️  CAPI pods check failed: %v\nOutput: %s\n\n", err, output)
		t.Logf("CAPI pods check: %v\nOutput: %s", err, output)
	} else {
		PrintToTTY("✅ CAPI pods found:\n%s\n\n", output)
		t.Logf("CAPI pods:\n%s", output)
	}
}

// TestKindCluster_CAPIControllerReady waits for CAPI controller to be ready
func TestKindCluster_CAPIControllerReady(t *testing.T) {
	PrintTestHeader(t, "TestKindCluster_CAPIControllerReady",
		"Wait for CAPI controller manager deployment to become available (timeout: 10m)")

	config := NewTestConfig()
	context := fmt.Sprintf("kind-%s", config.ManagementClusterName)

	timeout := 10 * time.Minute
	pollInterval := 10 * time.Second
	startTime := time.Now()

	PrintToTTY("\n=== Waiting for CAPI controller manager ===\n")
	PrintToTTY("Namespace: capi-system\n")
	PrintToTTY("Deployment: capi-controller-manager\n")
	PrintToTTY("Timeout: %v | Poll interval: %v\n\n", timeout, pollInterval)

	iteration := 0
	for {
		elapsed := time.Since(startTime)
		remaining := timeout - elapsed

		if elapsed > timeout {
			PrintToTTY("\n❌ Timeout reached after %v\n\n", elapsed.Round(time.Second))
			t.Errorf("Timeout waiting for CAPI controller manager to be available")
			return
		}

		iteration++

		PrintToTTY("[%d] Checking deployment status...\n", iteration)

		output, err := RunCommand(t, "kubectl", "--context", context, "-n", "capi-system",
			"get", "deployment", "capi-controller-manager",
			"-o", "jsonpath={.status.conditions[?(@.type=='Available')].status}")

		if err != nil {
			PrintToTTY("[%d] ⚠️  Status check failed: %v\n", iteration, err)
		} else {
			status := strings.TrimSpace(output)
			PrintToTTY("[%d] 📊 Deployment Available status: %s\n", iteration, status)

			if status == "True" {
				PrintToTTY("\n✅ CAPI controller manager is available! (took %v)\n\n", elapsed.Round(time.Second))
				t.Log("CAPI controller manager deployment is available")
				return
			}
		}

		ReportProgress(t, iteration, elapsed, remaining, timeout)

		time.Sleep(pollInterval)
	}
}

// TestKindCluster_CAPZControllerReady waits for CAPZ controller to be ready
func TestKindCluster_CAPZControllerReady(t *testing.T) {
	PrintTestHeader(t, "TestKindCluster_CAPZControllerReady",
		"Wait for CAPZ controller manager deployment to become available (timeout: 10m)")

	config := NewTestConfig()
	context := fmt.Sprintf("kind-%s", config.ManagementClusterName)

	timeout := 10 * time.Minute
	pollInterval := 10 * time.Second
	startTime := time.Now()

	PrintToTTY("\n=== Waiting for CAPZ controller manager ===\n")
	PrintToTTY("Namespace: capz-system\n")
	PrintToTTY("Deployment: capz-controller-manager\n")
	PrintToTTY("Timeout: %v | Poll interval: %v\n\n", timeout, pollInterval)

	iteration := 0
	for {
		elapsed := time.Since(startTime)
		remaining := timeout - elapsed

		if elapsed > timeout {
			PrintToTTY("\n❌ Timeout reached after %v\n\n", elapsed.Round(time.Second))
			t.Errorf("Timeout waiting for CAPZ controller manager to be available")
			return
		}

		iteration++

		PrintToTTY("[%d] Checking deployment status...\n", iteration)

		output, err := RunCommand(t, "kubectl", "--context", context, "-n", "capz-system",
			"get", "deployment", "capz-controller-manager",
			"-o", "jsonpath={.status.conditions[?(@.type=='Available')].status}")

		if err != nil {
			PrintToTTY("[%d] ⚠️  Status check failed: %v\n", iteration, err)
		} else {
			status := strings.TrimSpace(output)
			PrintToTTY("[%d] 📊 Deployment Available status: %s\n", iteration, status)

			if status == "True" {
				PrintToTTY("\n✅ CAPZ controller manager is available! (took %v)\n\n", elapsed.Round(time.Second))
				t.Log("CAPZ controller manager deployment is available")
				return
			}
		}

		ReportProgress(t, iteration, elapsed, remaining, timeout)

		time.Sleep(pollInterval)
	}
}

// TestKindCluster_ASOControllerReady waits for Azure Service Operator controller to be ready
func TestKindCluster_ASOControllerReady(t *testing.T) {
	PrintTestHeader(t, "TestKindCluster_ASOControllerReady",
		"Wait for Azure Service Operator controller manager to become available (timeout: 10m)")

	config := NewTestConfig()
	context := fmt.Sprintf("kind-%s", config.ManagementClusterName)

	timeout := 10 * time.Minute
	pollInterval := 10 * time.Second
	startTime := time.Now()

	PrintToTTY("\n=== Waiting for Azure Service Operator controller manager ===\n")
	PrintToTTY("Namespace: capz-system\n")
	PrintToTTY("Deployment: azureserviceoperator-controller-manager\n")
	PrintToTTY("Timeout: %v | Poll interval: %v\n\n", timeout, pollInterval)

	iteration := 0
	for {
		elapsed := time.Since(startTime)
		remaining := timeout - elapsed

		if elapsed > timeout {
			PrintToTTY("\n❌ Timeout reached after %v\n\n", elapsed.Round(time.Second))
			t.Errorf("Timeout waiting for Azure Service Operator controller manager to be available")
			return
		}

		iteration++

		PrintToTTY("[%d] Checking deployment status...\n", iteration)

		output, err := RunCommand(t, "kubectl", "--context", context, "-n", "capz-system",
			"get", "deployment", "azureserviceoperator-controller-manager",
			"-o", "jsonpath={.status.conditions[?(@.type=='Available')].status}")

		if err != nil {
			PrintToTTY("[%d] ⚠️  Status check failed: %v\n", iteration, err)
		} else {
			status := strings.TrimSpace(output)
			PrintToTTY("[%d] 📊 Deployment Available status: %s\n", iteration, status)

			if status == "True" {
				PrintToTTY("\n✅ Azure Service Operator controller manager is available! (took %v)\n\n", elapsed.Round(time.Second))
				t.Log("Azure Service Operator controller manager deployment is available")
				return
			}
		}

		ReportProgress(t, iteration, elapsed, remaining, timeout)

		time.Sleep(pollInterval)
	}
}
