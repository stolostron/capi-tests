package test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestDeployment_ApplyResources tests applying generated resources to the cluster
func TestDeployment_ApplyResources(t *testing.T) {

	config := NewTestConfig()
	outputDir := filepath.Join(config.RepoDir, config.GetOutputDirName())

	if !DirExists(outputDir) {
		PrintToTTY("⚠️  Output directory does not exist: %s\n", outputDir)
		t.Skipf("Output directory does not exist: %s", outputDir)
	}

	PrintToTTY("\n=== Applying Kubernetes resources ===\n")

	// Get files to apply from centralized list (from 04_generate_yamls_test.go)
	expectedFiles := []string{
		"credentials.yaml",
		"is.yaml",
		"aro.yaml",
	}

	// Set kubectl context to Kind cluster
	context := fmt.Sprintf("kind-%s", config.ManagementClusterName)

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(outputDir); err != nil {
		t.Fatalf("Failed to change to output directory: %v", err)
	}

	for _, file := range expectedFiles {
		if !FileExists(file) {
			PrintToTTY("❌ Cannot apply missing file: %s\n", file)
			t.Errorf("Cannot apply missing file: %s", file)
			continue
		}

		PrintToTTY("Applying resource file: %s...\n", file)
		t.Logf("Applying resource file: %s", file)

		output, err := RunCommand(t, "kubectl", "--context", context, "apply", "-f", file)
		// kubectl apply may return non-zero exit codes even for successful operations
		// (e.g., when resources are "unchanged"). Check output content for actual errors.
		if err != nil && !IsKubectlApplySuccess(output) {
			// On error, show output for debugging (may contain sensitive info, but needed for troubleshooting)
			PrintToTTY("❌ Failed to apply %s: %v\n", file, err)
			t.Errorf("Failed to apply %s: %v\nOutput: %s", file, err, output)
			continue
		}

		// Don't log full kubectl output as it may contain Azure subscription IDs and resource details
		PrintToTTY("✅ Successfully applied %s\n", file)
		t.Logf("Successfully applied %s", file)
	}

	PrintToTTY("\n=== Resource application complete ===\n\n")
}

// TestDeployment_ApplyCredentialsYAML tests applying credentials.yaml to the cluster
func TestDeployment_ApplyCredentialsYAML(t *testing.T) {
	file := "credentials.yaml"

	PrintToTTY("\n=== Applying %s ===\n", file)
	t.Logf("Applying %s", file)

	config := NewTestConfig()
	outputDir := filepath.Join(config.RepoDir, config.GetOutputDirName())

	if !DirExists(outputDir) {
		PrintToTTY("⚠️  Output directory does not exist: %s\n\n", outputDir)
		t.Skipf("Output directory does not exist: %s", outputDir)
	}

	filePath := filepath.Join(outputDir, file)
	if !FileExists(filePath) {
		PrintToTTY("❌ %s not found at %s\n\n", file, filePath)
		t.Errorf("%s not found", filePath)
		return
	}

	context := fmt.Sprintf("kind-%s", config.ManagementClusterName)
	output, err := RunCommand(t, "kubectl", "--context", context, "apply", "-f", filePath)

	if err != nil && !IsKubectlApplySuccess(output) {
		PrintToTTY("❌ Failed to apply %s: %v\n\n", file, err)
		t.Errorf("Failed to apply %s: %v\nOutput: %s", file, err, output)
		return
	}

	PrintToTTY("✅ Successfully applied %s\n\n", file)
	t.Logf("Successfully applied %s", file)
}

// TestDeployment_ApplyInfrastructureSecretsYAML tests applying is.yaml to the cluster
func TestDeployment_ApplyInfrastructureSecretsYAML(t *testing.T) {
	file := "is.yaml"

	PrintToTTY("\n=== Applying %s (infrastructure secrets) ===\n", file)
	t.Logf("Applying %s (infrastructure secrets)", file)

	config := NewTestConfig()
	outputDir := filepath.Join(config.RepoDir, config.GetOutputDirName())

	if !DirExists(outputDir) {
		PrintToTTY("⚠️  Output directory does not exist: %s\n\n", outputDir)
		t.Skipf("Output directory does not exist: %s", outputDir)
	}

	filePath := filepath.Join(outputDir, file)
	if !FileExists(filePath) {
		PrintToTTY("❌ %s not found at %s\n\n", file, filePath)
		t.Errorf("%s not found", filePath)
		return
	}

	context := fmt.Sprintf("kind-%s", config.ManagementClusterName)
	output, err := RunCommand(t, "kubectl", "--context", context, "apply", "-f", filePath)

	if err != nil && !IsKubectlApplySuccess(output) {
		PrintToTTY("❌ Failed to apply %s: %v\n\n", file, err)
		t.Errorf("Failed to apply %s: %v\nOutput: %s", file, err, output)
		return
	}

	PrintToTTY("✅ Successfully applied %s\n\n", file)
	t.Logf("Successfully applied %s", file)
}

// TestDeployment_ApplyAROClusterYAML tests applying aro.yaml to the cluster
func TestDeployment_ApplyAROClusterYAML(t *testing.T) {
	file := "aro.yaml"

	PrintToTTY("\n=== Applying %s (ARO cluster configuration) ===\n", file)
	t.Logf("Applying %s (ARO cluster configuration)", file)

	config := NewTestConfig()
	outputDir := filepath.Join(config.RepoDir, config.GetOutputDirName())

	if !DirExists(outputDir) {
		PrintToTTY("⚠️  Output directory does not exist: %s\n\n", outputDir)
		t.Skipf("Output directory does not exist: %s", outputDir)
	}

	filePath := filepath.Join(outputDir, file)
	if !FileExists(filePath) {
		PrintToTTY("❌ %s not found at %s\n\n", file, filePath)
		t.Errorf("%s not found", filePath)
		return
	}

	context := fmt.Sprintf("kind-%s", config.ManagementClusterName)
	output, err := RunCommand(t, "kubectl", "--context", context, "apply", "-f", filePath)

	if err != nil && !IsKubectlApplySuccess(output) {
		PrintToTTY("❌ Failed to apply %s: %v\n\n", file, err)
		t.Errorf("Failed to apply %s: %v\nOutput: %s", file, err, output)
		return
	}

	PrintToTTY("✅ Successfully applied %s\n\n", file)
	t.Logf("Successfully applied %s", file)
}

// TestDeployment_MonitorCluster tests monitoring the ARO cluster deployment
func TestDeployment_MonitorCluster(t *testing.T) {

	PrintToTTY("\n=== Starting Cluster Monitoring Test ===\n")

	config := NewTestConfig()

	PrintToTTY("Checking prerequisites...\n")
	if !DirExists(config.RepoDir) {
		PrintToTTY("⚠️  Repository not cloned yet at %s\n", config.RepoDir)
		t.Skipf("Repository not cloned yet at %s", config.RepoDir)
	}
	PrintToTTY("✅ Repository directory exists: %s\n", config.RepoDir)

	clusterctlPath := filepath.Join(config.RepoDir, config.ClusterctlBinPath)

	// If clusterctl binary doesn't exist, try to use system clusterctl
	PrintToTTY("Looking for clusterctl binary...\n")
	if !FileExists(clusterctlPath) {
		t.Logf("clusterctl binary not found at %s, checking system PATH", clusterctlPath)
		PrintToTTY("clusterctl binary not found at %s, checking system PATH...\n", clusterctlPath)
		if CommandExists("clusterctl") {
			clusterctlPath = "clusterctl"
			PrintToTTY("✅ Using clusterctl from system PATH\n")
		} else {
			PrintToTTY("❌ clusterctl not found in system PATH\n")
			t.Skipf("clusterctl not found")
		}
	} else {
		PrintToTTY("✅ Found clusterctl at: %s\n", clusterctlPath)
	}

	// Set kubectl context to Kind cluster
	context := fmt.Sprintf("kind-%s", config.ManagementClusterName)
	SetEnvVar(t, "KUBECONFIG", fmt.Sprintf("%s/.kube/config", os.Getenv("HOME")))

	// First, check if cluster resource exists
	// Use the provisioned cluster name from aro.yaml, not WORKLOAD_CLUSTER_NAME
	provisionedClusterName := config.GetProvisionedClusterName()
	PrintToTTY("\n=== Monitoring cluster deployment ===\n")
	PrintToTTY("Cluster: %s\n", provisionedClusterName)
	PrintToTTY("Context: %s\n", context)
	PrintToTTY("\nChecking if cluster resource exists...\n")
	t.Logf("Checking for cluster resource: %s", provisionedClusterName)

	output, err := RunCommand(t, "kubectl", "--context", context, "get", "cluster", provisionedClusterName)
	if err != nil {
		PrintToTTY("⚠️  Cluster resource not found (may not be deployed yet)\n\n")
		t.Skipf("Cluster resource not found (may not be deployed yet): %v", err)
	}

	PrintToTTY("✅ Cluster resource exists\n")
	t.Logf("Cluster resource exists:\n%s", output)

	// Use clusterctl to describe the cluster
	PrintToTTY("\n📊 Fetching cluster status with clusterctl...\n")
	PrintToTTY("Running: %s describe cluster %s --show-conditions=all\n", clusterctlPath, provisionedClusterName)
	PrintToTTY("This may take a few moments...\n")
	t.Logf("Monitoring cluster deployment status using clusterctl...")

	output, err = RunCommand(t, clusterctlPath, "describe", "cluster", provisionedClusterName, "--show-conditions=all")
	if err != nil {
		PrintToTTY("\n⚠️  clusterctl describe failed (cluster may still be initializing)\n")
		PrintToTTY("Error: %v\n\n", err)
		t.Logf("clusterctl describe failed (cluster may still be initializing): %v\nOutput: %s", err, output)
	} else {
		PrintToTTY("\n✅ Successfully retrieved cluster status\n")
		PrintToTTY("\nCluster Status:\n%s\n\n", output)
		t.Logf("Cluster status:\n%s", output)
	}

	PrintToTTY("=== Cluster Monitoring Test Complete ===\n\n")
}

// TestDeployment_WaitForControlPlane waits for control plane to be ready
func TestDeployment_WaitForControlPlane(t *testing.T) {

	config := NewTestConfig()
	context := fmt.Sprintf("kind-%s", config.ManagementClusterName)

	// Wait for control plane to be ready (with configurable timeout)
	timeout := config.DeploymentTimeout
	pollInterval := 30 * time.Second
	startTime := time.Now()

	// Print to stderr for immediate visibility (unbuffered)
	PrintToTTY("\n=== Waiting for control plane to be ready ===\n")
	PrintToTTY("Timeout: %v | Poll interval: %v\n\n", timeout, pollInterval)
	t.Logf("Waiting for control plane to be ready (timeout: %v)...", timeout)

	iteration := 0
	for {
		elapsed := time.Since(startTime)
		remaining := timeout - elapsed

		if elapsed > timeout {
			PrintToTTY("\n❌ Timeout reached after %v\n\n", elapsed.Round(time.Second))
			t.Errorf("Timeout waiting for control plane to be ready")
			return
		}

		iteration++

		// Print current check status
		PrintToTTY("[%d] Checking control plane status...\n", iteration)

		// ARO uses AROControlPlane, not kubeadmcontrolplane
		output, err := RunCommand(t, "kubectl", "--context", context, "get",
			"arocontrolplane", "-A", "-o", "jsonpath={.items[0].status.ready}")

		// Print the result of the check
		if err != nil {
			PrintToTTY("[%d] ⚠️  Status check failed: %v (output: %s)\n", iteration, err, output)
		} else {
			status := strings.TrimSpace(output)
			PrintToTTY("[%d] 📊 Control plane ready status: %s\n", iteration, status)

			if status == "true" {
				PrintToTTY("\n✅ Control plane is ready! (took %v)\n\n", elapsed.Round(time.Second))
				t.Log("Control plane is ready!")
				return
			}
		}

		// Report progress using helper function
		ReportProgress(t, iteration, elapsed, remaining, timeout)

		time.Sleep(pollInterval)
	}
}

// TestDeployment_CheckClusterConditions checks various cluster conditions
func TestDeployment_CheckClusterConditions(t *testing.T) {

	config := NewTestConfig()
	context := fmt.Sprintf("kind-%s", config.ManagementClusterName)

	// Use the provisioned cluster name from aro.yaml
	provisionedClusterName := config.GetProvisionedClusterName()

	PrintToTTY("\n=== Checking cluster conditions ===\n")
	PrintToTTY("Cluster: %s\n\n", provisionedClusterName)
	t.Log("Checking cluster conditions...")

	// Check cluster status
	PrintToTTY("Fetching cluster status...\n")

	output, err := RunCommand(t, "kubectl", "--context", context, "get", "cluster", provisionedClusterName, "-o", "yaml")
	if err != nil {
		PrintToTTY("❌ Failed to get cluster status: %v\n\n", err)
		t.Errorf("Failed to get cluster status: %v", err)
		return
	}

	// Log the cluster conditions
	if strings.Contains(output, "status:") {
		PrintToTTY("✅ Cluster has status information\n")
		t.Log("Cluster has status information")
		// Extract conditions section
		if strings.Contains(output, "conditions:") {
			PrintToTTY("✅ Cluster conditions are available\n\n")
			t.Log("Cluster conditions are available in the output")
		}
	}

	// Check for infrastructure ready condition
	PrintToTTY("Checking InfrastructureReady condition...\n")

	output, err = RunCommand(t, "kubectl", "--context", context, "get", "cluster", provisionedClusterName,
		"-o", "jsonpath={.status.conditions[?(@.type=='InfrastructureReady')].status}")

	if err == nil && strings.TrimSpace(output) != "" {
		PrintToTTY("📊 InfrastructureReady status: %s\n", output)
		t.Logf("InfrastructureReady status: %s", output)
	}

	// Check for control plane ready condition
	PrintToTTY("Checking ControlPlaneReady condition...\n")

	output, err = RunCommand(t, "kubectl", "--context", context, "get", "cluster", provisionedClusterName,
		"-o", "jsonpath={.status.conditions[?(@.type=='ControlPlaneReady')].status}")

	if err == nil && strings.TrimSpace(output) != "" {
		PrintToTTY("📊 ControlPlaneReady status: %s\n", output)
		t.Logf("ControlPlaneReady status: %s", output)
	}

	PrintToTTY("\n=== Cluster condition check complete ===\n\n")
}
