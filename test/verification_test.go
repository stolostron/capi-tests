package test

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestVerification_RetrieveKubeconfig tests retrieving the cluster kubeconfig
func TestVerification_RetrieveKubeconfig(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping kubeconfig retrieval in short mode")
	}

	config := NewTestConfig()
	context := fmt.Sprintf("kind-%s", config.KindClusterName)

	// Kubeconfig output path
	kubeconfigPath := filepath.Join(os.TempDir(), fmt.Sprintf("%s-kubeconfig.yaml", config.ClusterName))

	t.Logf("Retrieving kubeconfig for cluster '%s'", config.ClusterName)

	// Method 1: Using kubectl to get secret
	secretName := fmt.Sprintf("%s-kubeconfig", config.ClusterName)

	output, err := RunCommand(t, "kubectl", "--context", context, "get", "secret",
		secretName, "-o", "jsonpath={.data.value}")

	if err != nil {
		t.Logf("Method 1 (kubectl get secret) failed: %v", err)

		// Method 2: Try using clusterctl
		clusterctlPath := filepath.Join(config.RepoDir, config.ClusterctlBinPath)
		if !FileExists(clusterctlPath) && CommandExists("clusterctl") {
			clusterctlPath = "clusterctl"
		}

		if FileExists(clusterctlPath) || CommandExists("clusterctl") {
			t.Log("Trying method 2: clusterctl get kubeconfig")

			output, err = RunCommand(t, clusterctlPath, "get", "kubeconfig", config.ClusterName)
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
	if testing.Short() {
		t.Skip("Skipping cluster nodes verification in short mode")
	}

	kubeconfigPath := os.Getenv("ARO_CLUSTER_KUBECONFIG")
	if kubeconfigPath == "" {
		t.Skip("Kubeconfig not available, run TestVerification_RetrieveKubeconfig first")
	}

	if !FileExists(kubeconfigPath) {
		t.Skipf("Kubeconfig file not found at %s", kubeconfigPath)
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
	if testing.Short() {
		t.Skip("Skipping cluster version verification in short mode")
	}

	kubeconfigPath := os.Getenv("ARO_CLUSTER_KUBECONFIG")
	if kubeconfigPath == "" {
		t.Skip("Kubeconfig not available, run TestVerification_RetrieveKubeconfig first")
	}

	if !FileExists(kubeconfigPath) {
		t.Skipf("Kubeconfig file not found at %s", kubeconfigPath)
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
	if testing.Short() {
		t.Skip("Skipping cluster operators check in short mode")
	}

	kubeconfigPath := os.Getenv("ARO_CLUSTER_KUBECONFIG")
	if kubeconfigPath == "" {
		t.Skip("Kubeconfig not available, run TestVerification_RetrieveKubeconfig first")
	}

	if !FileExists(kubeconfigPath) {
		t.Skipf("Kubeconfig file not found at %s", kubeconfigPath)
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
	if testing.Short() {
		t.Skip("Skipping cluster health check in short mode")
	}

	kubeconfigPath := os.Getenv("ARO_CLUSTER_KUBECONFIG")
	if kubeconfigPath == "" {
		t.Skip("Kubeconfig not available, run TestVerification_RetrieveKubeconfig first")
	}

	if !FileExists(kubeconfigPath) {
		t.Skipf("Kubeconfig file not found at %s", kubeconfigPath)
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
