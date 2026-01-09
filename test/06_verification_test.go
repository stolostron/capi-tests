package test

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// getKubeconfigPath returns the path where the workload cluster kubeconfig is stored.
// This is calculated deterministically from the config, allowing tests to find the
// kubeconfig without relying on environment variables that may be cleaned up.
func getKubeconfigPath(config *TestConfig) string {
	provisionedClusterName := config.GetProvisionedClusterName()
	return filepath.Join(os.TempDir(), fmt.Sprintf("%s-kubeconfig.yaml", provisionedClusterName))
}

// TestVerification_RetrieveKubeconfig tests retrieving the cluster kubeconfig
func TestVerification_RetrieveKubeconfig(t *testing.T) {

	config := NewTestConfig()
	context := fmt.Sprintf("kind-%s", config.ManagementClusterName)

	// Use the provisioned cluster name from aro.yaml
	provisionedClusterName := config.GetProvisionedClusterName()

	// Check cluster phase before attempting kubeconfig retrieval (fixes #275)
	// When a cluster is still provisioning, ASO creates the kubeconfig secret with an empty
	// value, which causes confusing "Secret value is empty" errors.
	clusterPhase, err := GetClusterPhase(t, context, config.TestNamespace, provisionedClusterName)
	if err != nil {
		t.Skipf("Cannot determine cluster phase: %v (cluster resource may not exist yet)", err)
	}

	if clusterPhase != ClusterPhaseProvisioned {
		t.Skipf("Cluster is not ready (current phase: %s), skipping kubeconfig retrieval. "+
			"Wait for cluster provisioning to complete or run TestDeployment_WaitForControlPlane first.", clusterPhase)
	}

	// Kubeconfig output path - use helper for consistency
	kubeconfigPath := getKubeconfigPath(config)

	t.Logf("Retrieving kubeconfig for cluster '%s' (namespace: %s)", provisionedClusterName, config.TestNamespace)

	// Method 1: Using kubectl to get secret
	secretName := fmt.Sprintf("%s-kubeconfig", provisionedClusterName)

	t.Logf("Attempting Method 1: kubectl --context %s -n %s get secret %s -o jsonpath={.data.value}", context, config.TestNamespace, secretName)
	output, err := RunCommand(t, "kubectl", "--context", context, "-n", config.TestNamespace, "get", "secret",
		secretName, "-o", "jsonpath={.data.value}")

	if err != nil {
		t.Logf("Method 1 (kubectl get secret) failed: %v", err)

		// Method 2: Try using clusterctl
		clusterctlPath := filepath.Join(config.RepoDir, config.ClusterctlBinPath)
		if !FileExists(clusterctlPath) && CommandExists("clusterctl") {
			clusterctlPath = "clusterctl"
		}

		if FileExists(clusterctlPath) || CommandExists("clusterctl") {
			t.Logf("Attempting Method 2: %s get kubeconfig %s -n %s", clusterctlPath, provisionedClusterName, config.TestNamespace)

			output, err = RunCommand(t, clusterctlPath, "get", "kubeconfig", provisionedClusterName, "-n", config.TestNamespace)
			if err != nil {
				t.Errorf("Both kubeconfig retrieval methods failed: %v", err)
				return
			}

			// Write kubeconfig to file
			if err := os.WriteFile(kubeconfigPath, []byte(output), 0600); err != nil {
				t.Errorf("Failed to write kubeconfig to file: %v", err)
				return
			}

			t.Logf("Kubeconfig retrieved using clusterctl and saved to %s", kubeconfigPath)
		} else {
			t.Skipf("No method available to retrieve kubeconfig")
		}
	} else {
		// Validate secret output is not empty
		if strings.TrimSpace(output) == "" {
			t.Errorf("Secret value is empty, cannot decode kubeconfig")
			return
		}

		// Decode base64 using Go's encoding/base64 package (safe from command injection)
		decoded, err := base64.StdEncoding.DecodeString(output)
		if err != nil {
			t.Errorf("Failed to decode kubeconfig (invalid base64): %v", err)
			return
		}

		// Validate decoded content is not empty
		if len(decoded) == 0 {
			t.Errorf("Decoded kubeconfig is empty")
			return
		}

		if err := os.WriteFile(kubeconfigPath, decoded, 0600); err != nil {
			t.Errorf("Failed to write kubeconfig to file: %v", err)
			return
		}

		t.Logf("Kubeconfig retrieved using kubectl and saved to %s", kubeconfigPath)
	}

	// Store kubeconfig path for other tests
	SetEnvVar(t, "ARO_CLUSTER_KUBECONFIG", kubeconfigPath)
}

// TestVerification_ClusterNodes verifies cluster nodes are available
func TestVerification_ClusterNodes(t *testing.T) {

	config := NewTestConfig()
	kubeconfigPath := getKubeconfigPath(config)

	if !FileExists(kubeconfigPath) {
		t.Skipf("Kubeconfig not available at %s, run TestVerification_RetrieveKubeconfig first", kubeconfigPath)
	}

	t.Log("Checking cluster nodes...")

	SetEnvVar(t, "KUBECONFIG", kubeconfigPath)

	output, err := RunCommand(t, "kubectl", "get", "nodes")
	if err != nil {
		t.Errorf("Failed to get cluster nodes: %v\nOutput: %s", err, output)
		return
	}

	t.Logf("Cluster nodes:\n%s", output)

	// Verify we have at least one node
	lines := strings.Split(output, "\n")
	if len(lines) < 2 { // Header + at least one node
		t.Errorf("Expected at least one node, got output:\n%s", output)
		return
	}

	t.Logf("Cluster has %d node(s)", len(lines)-1)
}

// TestVerification_ClusterVersion verifies the OpenShift cluster version
func TestVerification_ClusterVersion(t *testing.T) {

	config := NewTestConfig()
	kubeconfigPath := getKubeconfigPath(config)

	if !FileExists(kubeconfigPath) {
		t.Skipf("Kubeconfig not available at %s, run TestVerification_RetrieveKubeconfig first", kubeconfigPath)
	}

	t.Log("Checking OpenShift cluster version...")

	SetEnvVar(t, "KUBECONFIG", kubeconfigPath)

	output, err := RunCommand(t, "oc", "version")
	if err != nil {
		t.Logf("Failed to get cluster version (cluster may still be provisioning): %v\nOutput: %s", err, output)
		return
	}

	t.Logf("OpenShift version:\n%s", output)
}

// TestVerification_ClusterOperators checks cluster operators status
func TestVerification_ClusterOperators(t *testing.T) {

	config := NewTestConfig()
	kubeconfigPath := getKubeconfigPath(config)

	if !FileExists(kubeconfigPath) {
		t.Skipf("Kubeconfig not available at %s, run TestVerification_RetrieveKubeconfig first", kubeconfigPath)
	}

	t.Log("Checking cluster operators...")

	SetEnvVar(t, "KUBECONFIG", kubeconfigPath)

	output, err := RunCommand(t, "oc", "get", "clusteroperators")
	if err != nil {
		t.Logf("Failed to get cluster operators (cluster may still be provisioning): %v\nOutput: %s", err, output)
		return
	}

	t.Logf("Cluster operators:\n%s", output)
}

// TestVerification_ClusterHealth performs basic health checks
func TestVerification_ClusterHealth(t *testing.T) {

	config := NewTestConfig()
	kubeconfigPath := getKubeconfigPath(config)

	if !FileExists(kubeconfigPath) {
		t.Skipf("Kubeconfig not available at %s, run TestVerification_RetrieveKubeconfig first", kubeconfigPath)
	}

	SetEnvVar(t, "KUBECONFIG", kubeconfigPath)

	// Check pods in kube-system namespace
	t.Log("Checking system pods...")

	output, err := RunCommand(t, "kubectl", "get", "pods", "-n", "kube-system")
	if err != nil {
		t.Logf("Failed to get system pods: %v\nOutput: %s", err, output)
	} else {
		t.Logf("System pods:\n%s", output)
	}

	// Check for any failing pods
	output, err = RunCommand(t, "kubectl", "get", "pods", "-A", "--field-selector=status.phase!=Running,status.phase!=Succeeded")
	if err == nil && strings.TrimSpace(output) != "" {
		lines := strings.Split(output, "\n")
		if len(lines) > 1 { // More than just header
			t.Logf("Warning: Found non-running pods:\n%s", output)
		} else {
			t.Log("All pods are in Running or Succeeded state")
		}
	}
}

// TestVerification_TestedVersionsSummary displays a summary of all tested component versions.
// This test collects version information from the management cluster for CAPZ, ASO, CAPI,
// and other infrastructure components, providing a clear summary at the end of testing.
func TestVerification_TestedVersionsSummary(t *testing.T) {

	config := NewTestConfig()
	context := fmt.Sprintf("kind-%s", config.ManagementClusterName)

	PrintTestHeader(t, "TestVerification_TestedVersionsSummary",
		"Display summary of tested infrastructure component versions")

	// Get component versions from the management cluster
	versions := GetComponentVersions(t, context)

	// Format and display the version summary
	summary := FormatComponentVersions(versions, config)
	PrintToTTY("%s", summary)
	t.Log(summary)

	// Log individual component details for test output
	for _, v := range versions {
		if v.Version == "not found" {
			t.Logf("Component %s: not deployed or not accessible", v.Name)
		} else {
			t.Logf("Component %s: version %s (image: %s)", v.Name, v.Version, v.Image)
		}
	}

	// Count successfully retrieved versions
	foundCount := 0
	for _, v := range versions {
		if v.Version != "not found" {
			foundCount++
		}
	}

	if foundCount == 0 {
		t.Log("Warning: No component versions could be retrieved. Management cluster may not be running.")
	} else {
		t.Logf("Successfully retrieved version information for %d/%d components", foundCount, len(versions))
	}
}

// TestVerification_ControllerLogSummary summarizes and saves logs from all controllers.
// This test checks CAPI, CAPZ, and ASO controller logs for errors and warnings,
// provides a summary, and saves the complete logs to the results directory.
func TestVerification_ControllerLogSummary(t *testing.T) {

	config := NewTestConfig()
	context := fmt.Sprintf("kind-%s", config.ManagementClusterName)

	PrintTestHeader(t, "TestVerification_ControllerLogSummary",
		"Summarize and save controller logs (CAPI, CAPZ, ASO)")

	// Get log summaries for all controllers
	summaries := GetAllControllerLogSummaries(t, context)

	// Get the results directory for saving logs
	resultsDir := GetResultsDir()
	t.Logf("Saving controller logs to: %s", resultsDir)

	// Save complete logs and update summaries with file paths
	summaries = SaveAllControllerLogs(t, context, resultsDir, summaries)

	// Format and display the summary
	summaryStr := FormatControllerLogSummaries(summaries)
	PrintToTTY("%s", summaryStr)
	t.Log(summaryStr)

	// Also copy logs to the latest results directory for easy access
	latestDir := "results/latest"
	if resultsDir != latestDir && DirExists(latestDir) {
		for _, s := range summaries {
			if s.LogFile != "" {
				// Extract filename from path
				parts := strings.Split(s.LogFile, "/")
				filename := parts[len(parts)-1]
				destPath := filepath.Join(latestDir, filename)

				// Read source file and write to destination
				if data, err := os.ReadFile(s.LogFile); err == nil {
					if err := os.WriteFile(destPath, data, 0644); err != nil {
						t.Logf("Warning: Failed to copy log file to latest: %v", err)
					}
				}
			}
		}
		t.Logf("Controller logs copied to: %s", latestDir)
	}

	// Count total errors and warnings
	totalErrors := 0
	totalWarnings := 0
	for _, s := range summaries {
		totalErrors += s.ErrorCount
		totalWarnings += s.WarnCount
		t.Logf("Controller %s: %d errors, %d warnings (log: %s)",
			s.Name, s.ErrorCount, s.WarnCount, s.LogFile)
	}

	// Log summary to test output
	if totalErrors > 0 {
		t.Logf("Warning: Found %d errors across all controllers. Review logs for details.", totalErrors)
	} else if totalWarnings > 0 {
		t.Logf("Found %d warnings (no errors) across all controllers.", totalWarnings)
	} else {
		t.Log("All controllers running without errors or warnings.")
	}

	t.Logf("Controller logs saved to: %s", resultsDir)
}
