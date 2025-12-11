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
		t.Skipf("Output directory does not exist: %s", outputDir)
	}

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
			t.Errorf("Cannot apply missing file: %s", file)
			continue
		}

		t.Logf("Applying resource file: %s", file)

		output, err := RunCommand(t, "kubectl", "--context", context, "apply", "-f", file)
		// kubectl apply may return non-zero exit codes even for successful operations
		// (e.g., when resources are "unchanged"). Check output content for actual errors.
		if err != nil && !IsKubectlApplySuccess(output) {
			// On error, show output for debugging (may contain sensitive info, but needed for troubleshooting)
			t.Errorf("Failed to apply %s: %v\nOutput: %s", file, err, output)
			continue
		}

		// Don't log full kubectl output as it may contain Azure subscription IDs and resource details
		t.Logf("Successfully applied %s", file)
	}
}

// TestDeployment_ApplyCredentialsYAML tests applying credentials.yaml to the cluster
func TestDeployment_ApplyCredentialsYAML(t *testing.T) {
	file := "credentials.yaml"
	t.Logf("Applying %s", file)

	config := NewTestConfig()
	outputDir := filepath.Join(config.RepoDir, config.GetOutputDirName())

	if !DirExists(outputDir) {
		t.Skipf("Output directory does not exist: %s", outputDir)
	}

	filePath := filepath.Join(outputDir, file)
	if !FileExists(filePath) {
		t.Errorf("%s not found", filePath)
		return
	}

	context := fmt.Sprintf("kind-%s", config.ManagementClusterName)
	output, err := RunCommand(t, "kubectl", "--context", context, "apply", "-f", filePath)

	if err != nil && !IsKubectlApplySuccess(output) {
		t.Errorf("Failed to apply %s: %v\nOutput: %s", file, err, output)
		return
	}

	t.Logf("Successfully applied %s", file)
}

// TestDeployment_ApplyInfrastructureSecretsYAML tests applying is.yaml to the cluster
func TestDeployment_ApplyInfrastructureSecretsYAML(t *testing.T) {
	file := "is.yaml"
	t.Logf("Applying %s (infrastructure secrets)", file)

	config := NewTestConfig()
	outputDir := filepath.Join(config.RepoDir, config.GetOutputDirName())

	if !DirExists(outputDir) {
		t.Skipf("Output directory does not exist: %s", outputDir)
	}

	filePath := filepath.Join(outputDir, file)
	if !FileExists(filePath) {
		t.Errorf("%s not found", filePath)
		return
	}

	context := fmt.Sprintf("kind-%s", config.ManagementClusterName)
	output, err := RunCommand(t, "kubectl", "--context", context, "apply", "-f", filePath)

	if err != nil && !IsKubectlApplySuccess(output) {
		t.Errorf("Failed to apply %s: %v\nOutput: %s", file, err, output)
		return
	}

	t.Logf("Successfully applied %s", file)
}

// TestDeployment_ApplyAROClusterYAML tests applying aro.yaml to the cluster
func TestDeployment_ApplyAROClusterYAML(t *testing.T) {
	file := "aro.yaml"
	t.Logf("Applying %s (ARO cluster configuration)", file)

	config := NewTestConfig()
	outputDir := filepath.Join(config.RepoDir, config.GetOutputDirName())

	if !DirExists(outputDir) {
		t.Skipf("Output directory does not exist: %s", outputDir)
	}

	filePath := filepath.Join(outputDir, file)
	if !FileExists(filePath) {
		t.Errorf("%s not found", filePath)
		return
	}

	context := fmt.Sprintf("kind-%s", config.ManagementClusterName)
	output, err := RunCommand(t, "kubectl", "--context", context, "apply", "-f", filePath)

	if err != nil && !IsKubectlApplySuccess(output) {
		t.Errorf("Failed to apply %s: %v\nOutput: %s", file, err, output)
		return
	}

	t.Logf("Successfully applied %s", file)
}

// TestDeployment_MonitorCluster tests monitoring the ARO cluster deployment
func TestDeployment_MonitorCluster(t *testing.T) {

	fmt.Fprintf(os.Stderr, "\n=== Starting Cluster Monitoring Test ===\n")
	os.Stderr.Sync() // Force immediate output

	config := NewTestConfig()

	fmt.Fprintf(os.Stderr, "Checking prerequisites...\n")
	os.Stderr.Sync() // Force immediate output
	if !DirExists(config.RepoDir) {
		fmt.Fprintf(os.Stderr, "⚠️  Repository not cloned yet at %s\n", config.RepoDir)
		os.Stderr.Sync() // Force immediate output
		t.Skipf("Repository not cloned yet at %s", config.RepoDir)
	}
	fmt.Fprintf(os.Stderr, "✅ Repository directory exists: %s\n", config.RepoDir)
	os.Stderr.Sync() // Force immediate output

	clusterctlPath := filepath.Join(config.RepoDir, config.ClusterctlBinPath)

	// If clusterctl binary doesn't exist, try to use system clusterctl
	fmt.Fprintf(os.Stderr, "Looking for clusterctl binary...\n")
	os.Stderr.Sync() // Force immediate output
	if !FileExists(clusterctlPath) {
		t.Logf("clusterctl binary not found at %s, checking system PATH", clusterctlPath)
		fmt.Fprintf(os.Stderr, "clusterctl binary not found at %s, checking system PATH...\n", clusterctlPath)
		os.Stderr.Sync() // Force immediate output
		if CommandExists("clusterctl") {
			clusterctlPath = "clusterctl"
			fmt.Fprintf(os.Stderr, "✅ Using clusterctl from system PATH\n")
			os.Stderr.Sync() // Force immediate output
		} else {
			fmt.Fprintf(os.Stderr, "❌ clusterctl not found in system PATH\n")
			os.Stderr.Sync() // Force immediate output
			t.Skipf("clusterctl not found")
		}
	} else {
		fmt.Fprintf(os.Stderr, "✅ Found clusterctl at: %s\n", clusterctlPath)
		os.Stderr.Sync() // Force immediate output
	}

	// Set kubectl context to Kind cluster
	context := fmt.Sprintf("kind-%s", config.ManagementClusterName)
	SetEnvVar(t, "KUBECONFIG", fmt.Sprintf("%s/.kube/config", os.Getenv("HOME")))

	// First, check if cluster resource exists
	fmt.Fprintf(os.Stderr, "\n=== Monitoring cluster deployment ===\n")
	fmt.Fprintf(os.Stderr, "Cluster: %s\n", config.WorkloadClusterName)
	fmt.Fprintf(os.Stderr, "Context: %s\n", context)
	fmt.Fprintf(os.Stderr, "\nChecking if cluster resource exists...\n")
	os.Stderr.Sync() // Force immediate output
	t.Logf("Checking for cluster resource: %s", config.WorkloadClusterName)

	output, err := RunCommand(t, "kubectl", "--context", context, "get", "cluster", config.WorkloadClusterName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Cluster resource not found (may not be deployed yet)\n\n")
		os.Stderr.Sync() // Force immediate output
		t.Skipf("Cluster resource not found (may not be deployed yet): %v", err)
	}

	fmt.Fprintf(os.Stderr, "✅ Cluster resource exists\n")
	os.Stderr.Sync() // Force immediate output
	t.Logf("Cluster resource exists:\n%s", output)

	// Use clusterctl to describe the cluster
	fmt.Fprintf(os.Stderr, "\n📊 Fetching cluster status with clusterctl...\n")
	fmt.Fprintf(os.Stderr, "Running: %s describe cluster %s --show-conditions=all\n", clusterctlPath, config.WorkloadClusterName)
	fmt.Fprintf(os.Stderr, "This may take a few moments...\n")
	os.Stderr.Sync() // Force immediate output
	t.Logf("Monitoring cluster deployment status using clusterctl...")

	output, err = RunCommand(t, clusterctlPath, "describe", "cluster", config.WorkloadClusterName, "--show-conditions=all")
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n⚠️  clusterctl describe failed (cluster may still be initializing)\n")
		fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
		os.Stderr.Sync() // Force immediate output
		t.Logf("clusterctl describe failed (cluster may still be initializing): %v\nOutput: %s", err, output)
	} else {
		fmt.Fprintf(os.Stderr, "\n✅ Successfully retrieved cluster status\n")
		fmt.Fprintf(os.Stderr, "\nCluster Status:\n%s\n\n", output)
		os.Stderr.Sync() // Force immediate output
		t.Logf("Cluster status:\n%s", output)
	}

	fmt.Fprintf(os.Stderr, "=== Cluster Monitoring Test Complete ===\n\n")
	os.Stderr.Sync() // Force immediate output
}

// TestDeployment_WaitForControlPlane waits for control plane to be ready
func TestDeployment_WaitForControlPlane(t *testing.T) {

	config := NewTestConfig()
	context := fmt.Sprintf("kind-%s", config.ManagementClusterName)

	// Wait for control plane to be ready (with timeout)
	timeout := 30 * time.Minute
	pollInterval := 30 * time.Second
	startTime := time.Now()

	// Print to stderr for immediate visibility (unbuffered)
	fmt.Fprintf(os.Stderr, "\n=== Waiting for control plane to be ready ===\n")
	fmt.Fprintf(os.Stderr, "Timeout: %v | Poll interval: %v\n\n", timeout, pollInterval)
	os.Stderr.Sync() // Force immediate output
	t.Logf("Waiting for control plane to be ready (timeout: %v)...", timeout)

	iteration := 0
	for {
		elapsed := time.Since(startTime)
		remaining := timeout - elapsed

		if elapsed > timeout {
			fmt.Fprintf(os.Stderr, "\n❌ Timeout reached after %v\n\n", elapsed.Round(time.Second))
			os.Stderr.Sync() // Force immediate output
			t.Errorf("Timeout waiting for control plane to be ready")
			return
		}

		iteration++

		// Print current check status
		fmt.Fprintf(os.Stderr, "[%d] Checking control plane status...\n", iteration)
		os.Stderr.Sync() // Force immediate output

		output, err := RunCommand(t, "kubectl", "--context", context, "get",
			"kubeadmcontrolplane", "-A", "-o", "jsonpath={.items[0].status.ready}")

		// Print the result of the check
		if err != nil {
			fmt.Fprintf(os.Stderr, "[%d] ⚠️  Status check failed: %v (output: %s)\n", iteration, err, output)
			os.Stderr.Sync() // Force immediate output
		} else {
			status := strings.TrimSpace(output)
			fmt.Fprintf(os.Stderr, "[%d] 📊 Control plane ready status: %s\n", iteration, status)
			os.Stderr.Sync() // Force immediate output

			if status == "true" {
				fmt.Fprintf(os.Stderr, "\n✅ Control plane is ready! (took %v)\n\n", elapsed.Round(time.Second))
				os.Stderr.Sync() // Force immediate output
				t.Log("Control plane is ready!")
				return
			}
		}

		// Report progress using helper function
		ReportProgress(t, os.Stderr, iteration, elapsed, remaining, timeout)
		os.Stderr.Sync() // Force immediate output

		time.Sleep(pollInterval)
	}
}

// TestDeployment_CheckClusterConditions checks various cluster conditions
func TestDeployment_CheckClusterConditions(t *testing.T) {

	config := NewTestConfig()
	context := fmt.Sprintf("kind-%s", config.ManagementClusterName)

	t.Log("Checking cluster conditions...")

	// Check cluster status
	output, err := RunCommand(t, "kubectl", "--context", context, "get", "cluster", config.WorkloadClusterName, "-o", "yaml")
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
	output, err = RunCommand(t, "kubectl", "--context", context, "get", "cluster", config.WorkloadClusterName,
		"-o", "jsonpath={.status.conditions[?(@.type=='InfrastructureReady')].status}")

	if err == nil && strings.TrimSpace(output) != "" {
		t.Logf("InfrastructureReady status: %s", output)
	}

	// Check for control plane ready condition
	output, err = RunCommand(t, "kubectl", "--context", context, "get", "cluster", config.WorkloadClusterName,
		"-o", "jsonpath={.status.conditions[?(@.type=='ControlPlaneReady')].status}")

	if err == nil && strings.TrimSpace(output) != "" {
		t.Logf("ControlPlaneReady status: %s", output)
	}
}
