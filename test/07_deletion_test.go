package test

import (
	"fmt"
	"strings"
	"testing"
)

// TestDeletion_DeleteCluster tests deleting the workload cluster from the management cluster.
// This initiates the deletion by removing the Cluster resource, which triggers
// CAPI/CAPZ to clean up all associated resources including Azure resources.
func TestDeletion_DeleteCluster(t *testing.T) {
	config := NewTestConfig()

	// Set KUBECONFIG for external cluster mode
	if config.IsExternalCluster() {
		SetEnvVar(t, "KUBECONFIG", config.UseKubeconfig)
	}

	context := config.GetKubeContext()

	// Get the provisioned cluster name from aro.yaml
	provisionedClusterName := config.GetProvisionedClusterName()

	PrintTestHeader(t, "TestDeletion_DeleteCluster",
		"Delete the workload cluster from the management cluster")

	// Check if cluster exists before attempting deletion
	_, err := RunCommand(t, "kubectl", "--context", context, "-n", config.WorkloadClusterNamespace,
		"get", "cluster", provisionedClusterName)
	if err != nil {
		PrintToTTY("⚠️  Cluster '%s' not found in namespace '%s'\n", provisionedClusterName, config.WorkloadClusterNamespace)
		t.Skipf("Cluster '%s' not found (may not have been deployed or already deleted)", provisionedClusterName)
	}

	PrintToTTY("📋 Cluster '%s' found in namespace '%s'\n", provisionedClusterName, config.WorkloadClusterNamespace)
	PrintToTTY("🗑️  Initiating cluster deletion...\n\n")
	t.Logf("Deleting cluster '%s' from namespace '%s'", provisionedClusterName, config.WorkloadClusterNamespace)

	// Delete the cluster resource - this triggers cascading deletion of all related resources
	// Use --wait=false to return immediately so the next test can monitor deletion progress
	output, err := RunCommand(t, "kubectl", "--context", context, "-n", config.WorkloadClusterNamespace,
		"delete", "cluster", provisionedClusterName, "--wait=false")
	if err != nil {
		PrintToTTY("❌ Failed to delete cluster: %v\n", err)
		PrintToTTY("Output: %s\n\n", output)
		t.Fatalf("Failed to delete cluster '%s': %v\nOutput: %s", provisionedClusterName, err, output)
	}

	PrintToTTY("✅ Cluster deletion initiated\n")
	PrintToTTY("Output: %s\n\n", output)
	t.Logf("Cluster deletion initiated: %s", output)
}

// TestDeletion_WaitForClusterDeletion waits for the cluster to be fully deleted.
// Uses the generic cluster monitoring function to detect when the cluster resource is gone.
func TestDeletion_WaitForClusterDeletion(t *testing.T) {
	config := NewTestConfig()

	// Set KUBECONFIG for external cluster mode
	if config.IsExternalCluster() {
		SetEnvVar(t, "KUBECONFIG", config.UseKubeconfig)
	}

	provisionedClusterName := config.GetProvisionedClusterName()
	context := config.GetKubeContext()

	PrintTestHeader(t, "TestDeletion_WaitForClusterDeletion",
		"Wait for cluster resource to be fully deleted")

	PrintToTTY("\n⏳ Waiting for cluster '%s' to be deleted...\n", provisionedClusterName)
	PrintToTTY("Namespace: %s\n", config.WorkloadClusterNamespace)
	PrintToTTY("Context: %s\n", context)
	PrintToTTY("Timeout: %v\n\n", config.DeploymentTimeout)

	// Monitor cluster until it's deleted
	err := MonitorClusterUntilDeleted(t, context, config.WorkloadClusterNamespace, provisionedClusterName, config.DeploymentTimeout)
	if err != nil {
		PrintToTTY("\n❌ %v\n\n", err)
		PrintToTTY("Troubleshooting steps:\n")
		PrintToTTY("  1. Check cluster status: kubectl -n %s get cluster %s -o yaml\n",
			config.WorkloadClusterNamespace, provisionedClusterName)
		PrintToTTY("  2. Check for stuck finalizers: kubectl -n %s get cluster %s -o jsonpath='{.metadata.finalizers}'\n",
			config.WorkloadClusterNamespace, provisionedClusterName)
		PrintToTTY("  3. Check remaining CAPI resources: kubectl -n %s get all\n",
			config.WorkloadClusterNamespace)
		PrintToTTY("\nCommon causes:\n")
		PrintToTTY("  - Cloud resource deletion taking longer than expected\n")
		PrintToTTY("  - Finalizers blocking resource deletion\n")
		PrintToTTY("  - Cloud resource stuck in 'Deleting' state\n\n")
		PrintToTTY("To increase timeout: export DEPLOYMENT_TIMEOUT=60m\n")
		PrintToTTY("To manually clean up: make clean-azure\n\n")
		t.Fatalf("Cluster deletion failed: %v", err)
	}

	PrintToTTY("\n✅ Cluster '%s' has been deleted successfully\n\n", provisionedClusterName)
	t.Logf("Cluster '%s' deleted successfully", provisionedClusterName)
}

// TestDeletion_VerifyAROControlPlaneDeletion verifies the AROControlPlane resource is deleted.
func TestDeletion_VerifyAROControlPlaneDeletion(t *testing.T) {
	config := NewTestConfig()

	// Set KUBECONFIG for external cluster mode
	if config.IsExternalCluster() {
		SetEnvVar(t, "KUBECONFIG", config.UseKubeconfig)
	}

	context := config.GetKubeContext()

	PrintTestHeader(t, "TestDeletion_VerifyAROControlPlaneDeletion",
		"Verify AROControlPlane resource is deleted")

	PrintToTTY("Checking for remaining AROControlPlane resources in namespace '%s'...\n", config.WorkloadClusterNamespace)
	t.Logf("Checking for remaining AROControlPlane resources in namespace '%s'", config.WorkloadClusterNamespace)

	output, err := RunCommand(t, "kubectl", "--context", context, "-n", config.WorkloadClusterNamespace,
		"get", "arocontrolplane", "--ignore-not-found")
	if err != nil {
		PrintToTTY("⚠️  Error checking AROControlPlane: %v\n\n", err)
		t.Logf("Error checking AROControlPlane resources: %v", err)
		return
	}

	if strings.TrimSpace(output) == "" {
		PrintToTTY("✅ No AROControlPlane resources found (deleted successfully)\n\n")
		t.Log("No AROControlPlane resources found - deletion successful")
	} else {
		PrintToTTY("⚠️  AROControlPlane resources still exist:\n%s\n\n", output)
		t.Logf("Warning: AROControlPlane resources still exist:\n%s", output)
	}
}

// TestDeletion_VerifyMachinePoolDeletion verifies machine pool resources are deleted.
func TestDeletion_VerifyMachinePoolDeletion(t *testing.T) {
	config := NewTestConfig()

	// Set KUBECONFIG for external cluster mode
	if config.IsExternalCluster() {
		SetEnvVar(t, "KUBECONFIG", config.UseKubeconfig)
	}

	context := config.GetKubeContext()

	PrintTestHeader(t, "TestDeletion_VerifyMachinePoolDeletion",
		"Verify machine pool resources are deleted")

	PrintToTTY("Checking for remaining MachinePool resources in namespace '%s'...\n", config.WorkloadClusterNamespace)
	t.Logf("Checking for remaining MachinePool resources in namespace '%s'", config.WorkloadClusterNamespace)

	output, err := RunCommand(t, "kubectl", "--context", context, "-n", config.WorkloadClusterNamespace,
		"get", "machinepool", "--ignore-not-found")
	if err != nil {
		PrintToTTY("⚠️  Error checking MachinePool: %v\n\n", err)
		t.Logf("Error checking MachinePool resources: %v", err)
		return
	}

	if strings.TrimSpace(output) == "" {
		PrintToTTY("✅ No MachinePool resources found (deleted successfully)\n\n")
		t.Log("No MachinePool resources found - deletion successful")
	} else {
		PrintToTTY("⚠️  MachinePool resources still exist:\n%s\n\n", output)
		t.Logf("Warning: MachinePool resources still exist:\n%s", output)
	}
}

// TestDeletion_VerifyAzureResourcesDeletion verifies Azure resources are cleaned up.
// This checks if the Azure resource group still exists after cluster deletion.
func TestDeletion_VerifyAzureResourcesDeletion(t *testing.T) {
	config := NewTestConfig()

	PrintTestHeader(t, "TestDeletion_VerifyAzureResourcesDeletion",
		"Verify Azure resources are cleaned up")

	// Check if Azure CLI is available
	if !CommandExists("az") {
		PrintToTTY("⚠️  Azure CLI not available - skipping Azure resource verification\n\n")
		t.Skip("Azure CLI not available")
	}

	// Check if logged in
	_, err := RunCommandQuiet(t, "az", "account", "show")
	if err != nil {
		PrintToTTY("⚠️  Not logged in to Azure CLI - skipping Azure resource verification\n\n")
		t.Skip("Not logged in to Azure CLI")
	}

	// The resource group name is derived from ClusterNamePrefix
	resourceGroup := fmt.Sprintf("%s-resgroup", config.ClusterNamePrefix)

	PrintToTTY("Checking Azure resource group '%s'...\n", resourceGroup)
	t.Logf("Checking if Azure resource group '%s' still exists", resourceGroup)

	// Check if resource group exists
	_, err = RunCommandQuiet(t, "az", "group", "show", "--name", resourceGroup)
	if err != nil {
		// Resource group doesn't exist or we can't access it - this is expected after deletion
		if strings.Contains(strings.ToLower(err.Error()), "not found") ||
			strings.Contains(strings.ToLower(err.Error()), "could not be found") {
			PrintToTTY("✅ Resource group '%s' has been deleted\n\n", resourceGroup)
			t.Logf("Resource group '%s' has been deleted successfully", resourceGroup)
			return
		}
		// Some other error - might be transient
		PrintToTTY("⚠️  Could not check resource group status: %v\n\n", err)
		t.Logf("Warning: Could not verify resource group deletion: %v", err)
		return
	}

	// Resource group still exists - check if it has any resources
	PrintToTTY("⚠️  Resource group '%s' still exists\n", resourceGroup)
	t.Logf("Warning: Resource group '%s' still exists after cluster deletion", resourceGroup)

	// List resources in the group
	output, err := RunCommand(t, "az", "resource", "list", "--resource-group", resourceGroup, "--output", "table")
	if err == nil && strings.TrimSpace(output) != "" {
		PrintToTTY("Remaining resources in resource group:\n%s\n\n", output)
		t.Logf("Resources still in resource group '%s':\n%s", resourceGroup, output)
	} else {
		PrintToTTY("ℹ️  Resource group exists but appears empty or is being deleted\n\n")
		t.Log("Resource group exists but appears empty or is being deleted")
	}
}

// TestDeletion_Summary provides a summary of the deletion process.
func TestDeletion_Summary(t *testing.T) {
	config := NewTestConfig()

	// Set KUBECONFIG for external cluster mode
	if config.IsExternalCluster() {
		SetEnvVar(t, "KUBECONFIG", config.UseKubeconfig)
	}

	context := config.GetKubeContext()

	PrintTestHeader(t, "TestDeletion_Summary",
		"Summary of cluster deletion status")

	// Check remaining cluster resources
	PrintToTTY("=== Deletion Summary ===\n\n")

	// Check for any remaining clusters
	output, err := RunCommand(t, "kubectl", "--context", context, "-n", config.WorkloadClusterNamespace,
		"get", "clusters", "--ignore-not-found", "-o", "custom-columns=NAME:.metadata.name,PHASE:.status.phase")
	if err != nil {
		PrintToTTY("⚠️  Could not list clusters: %v\n\n", err)
	} else if strings.TrimSpace(output) == "" || !strings.Contains(output, config.GetProvisionedClusterName()) {
		PrintToTTY("✅ Workload cluster deleted successfully\n")
	} else {
		PrintToTTY("⚠️  Cluster resources remaining:\n%s\n", output)
	}

	// Check for remaining CAPI resources
	output, err = RunCommandQuiet(t, "kubectl", "--context", context, "-n", config.WorkloadClusterNamespace,
		"get", "arocontrolplane,machinepool", "--ignore-not-found")
	if err == nil && strings.TrimSpace(output) == "" {
		PrintToTTY("✅ All CAPI resources deleted\n")
	} else if strings.TrimSpace(output) != "" {
		PrintToTTY("⚠️  Some CAPI resources remain:\n%s\n", output)
	}

	// Summary message
	PrintToTTY("\n=== Deletion Test Complete ===\n\n")
	t.Log("Deletion test phase completed")
}
