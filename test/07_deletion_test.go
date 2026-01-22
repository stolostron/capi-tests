package test

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestDeletion_DeleteCluster tests deleting the workload cluster from the management cluster.
// This initiates the deletion by removing the Cluster resource, which triggers
// CAPI/CAPZ to clean up all associated resources including Azure resources.
func TestDeletion_DeleteCluster(t *testing.T) {
	config := NewTestConfig()
	context := fmt.Sprintf("kind-%s", config.ManagementClusterName)

	// Get the provisioned cluster name from aro.yaml
	provisionedClusterName := config.GetProvisionedClusterName()

	PrintTestHeader(t, "TestDeletion_DeleteCluster",
		"Delete the workload cluster from the management cluster")

	// Check if cluster exists before attempting deletion
	_, err := RunCommand(t, "kubectl", "--context", context, "-n", config.TestNamespace,
		"get", "cluster", provisionedClusterName)
	if err != nil {
		PrintToTTY("⚠️  Cluster '%s' not found in namespace '%s'\n", provisionedClusterName, config.TestNamespace)
		t.Skipf("Cluster '%s' not found (may not have been deployed or already deleted)", provisionedClusterName)
	}

	PrintToTTY("📋 Cluster '%s' found in namespace '%s'\n", provisionedClusterName, config.TestNamespace)
	PrintToTTY("🗑️  Initiating cluster deletion...\n\n")
	t.Logf("Deleting cluster '%s' from namespace '%s'", provisionedClusterName, config.TestNamespace)

	// Delete the cluster resource - this triggers cascading deletion of all related resources
	// Use --wait=false to return immediately so the next test can monitor deletion progress
	output, err := RunCommand(t, "kubectl", "--context", context, "-n", config.TestNamespace,
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
// This monitors the cluster resource until it no longer exists, showing detailed
// progress information about all resources being deleted.
func TestDeletion_WaitForClusterDeletion(t *testing.T) {
	config := NewTestConfig()
	context := fmt.Sprintf("kind-%s", config.ManagementClusterName)

	// Get the provisioned cluster name from aro.yaml
	provisionedClusterName := config.GetProvisionedClusterName()

	// Azure resource group name
	resourceGroup := fmt.Sprintf("%s-resgroup", config.ClusterNamePrefix)

	PrintTestHeader(t, "TestDeletion_WaitForClusterDeletion",
		"Wait for cluster resource to be fully deleted")

	// Use the deployment timeout for deletion as well (deletion can take significant time)
	timeout := config.DeploymentTimeout
	pollInterval := 30 * time.Second
	startTime := time.Now()

	PrintToTTY("⏳ Waiting for cluster '%s' to be deleted...\n", provisionedClusterName)
	PrintToTTY("Namespace: %s | Timeout: %v | Poll interval: %v\n", config.TestNamespace, timeout, pollInterval)
	PrintToTTY("Azure Resource Group: %s\n\n", resourceGroup)
	t.Logf("Waiting for cluster '%s' deletion (namespace: %s, timeout: %v)...", provisionedClusterName, config.TestNamespace, timeout)

	iteration := 0
	for {
		elapsed := time.Since(startTime)
		remaining := timeout - elapsed

		if elapsed > timeout {
			PrintToTTY("\n❌ Timeout waiting for cluster deletion after %v\n\n", elapsed.Round(time.Second))
			t.Errorf("Timeout waiting for cluster '%s' to be deleted after %v.\n\n"+
				"Troubleshooting steps:\n"+
				"  1. Check cluster status: kubectl --context %s -n %s get cluster %s -o yaml\n"+
				"  2. Check for stuck finalizers: kubectl --context %s -n %s get cluster %s -o jsonpath='{.metadata.finalizers}'\n"+
				"  3. Check remaining CAPI resources: kubectl --context %s -n %s get arocontrolplane,machinepool\n"+
				"  4. Check Azure resource group: az group show --name %s 2>/dev/null\n\n"+
				"Common causes:\n"+
				"  - Azure resource deletion taking longer than expected\n"+
				"  - Finalizers blocking resource deletion\n"+
				"  - Azure resource stuck in 'Deleting' state\n\n"+
				"To increase timeout: export DEPLOYMENT_TIMEOUT=60m\n"+
				"To manually clean up:\n"+
				"  make clean-azure  # Removes Azure resources",
				provisionedClusterName, elapsed.Round(time.Second),
				context, config.TestNamespace, provisionedClusterName,
				context, config.TestNamespace, provisionedClusterName,
				context, config.TestNamespace,
				resourceGroup)
			return
		}

		iteration++

		// Get comprehensive deletion status
		status := GetDeletionResourceStatus(t, context, config.TestNamespace, provisionedClusterName, resourceGroup)

		// Check if cluster is fully deleted
		if !status.ClusterExists {
			PrintToTTY("\n✅ Cluster '%s' has been deleted (took %v)\n\n", provisionedClusterName, elapsed.Round(time.Second))
			t.Logf("Cluster '%s' deleted successfully (took %v)", provisionedClusterName, elapsed.Round(time.Second))

			// Show final status
			PrintToTTY("%s", FormatDeletionProgress(status))
			return
		}

		// Report detailed deletion progress
		ReportDeletionProgress(t, iteration, elapsed, remaining, status)

		time.Sleep(pollInterval)
	}
}

// TestDeletion_VerifyAROControlPlaneDeletion verifies the AROControlPlane resource is deleted.
func TestDeletion_VerifyAROControlPlaneDeletion(t *testing.T) {
	config := NewTestConfig()
	context := fmt.Sprintf("kind-%s", config.ManagementClusterName)

	PrintTestHeader(t, "TestDeletion_VerifyAROControlPlaneDeletion",
		"Verify AROControlPlane resource is deleted")

	PrintToTTY("Checking for remaining AROControlPlane resources in namespace '%s'...\n", config.TestNamespace)
	t.Logf("Checking for remaining AROControlPlane resources in namespace '%s'", config.TestNamespace)

	output, err := RunCommand(t, "kubectl", "--context", context, "-n", config.TestNamespace,
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
	context := fmt.Sprintf("kind-%s", config.ManagementClusterName)

	PrintTestHeader(t, "TestDeletion_VerifyMachinePoolDeletion",
		"Verify machine pool resources are deleted")

	PrintToTTY("Checking for remaining MachinePool resources in namespace '%s'...\n", config.TestNamespace)
	t.Logf("Checking for remaining MachinePool resources in namespace '%s'", config.TestNamespace)

	output, err := RunCommand(t, "kubectl", "--context", context, "-n", config.TestNamespace,
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
	context := fmt.Sprintf("kind-%s", config.ManagementClusterName)

	PrintTestHeader(t, "TestDeletion_Summary",
		"Summary of cluster deletion status")

	// Check remaining cluster resources
	PrintToTTY("=== Deletion Summary ===\n\n")

	// Check for any remaining clusters
	output, err := RunCommand(t, "kubectl", "--context", context, "-n", config.TestNamespace,
		"get", "clusters", "--ignore-not-found", "-o", "custom-columns=NAME:.metadata.name,PHASE:.status.phase")
	if err != nil {
		PrintToTTY("⚠️  Could not list clusters: %v\n\n", err)
	} else if strings.TrimSpace(output) == "" || !strings.Contains(output, config.GetProvisionedClusterName()) {
		PrintToTTY("✅ Workload cluster deleted successfully\n")
	} else {
		PrintToTTY("⚠️  Cluster resources remaining:\n%s\n", output)
	}

	// Check for remaining CAPI resources
	output, err = RunCommandQuiet(t, "kubectl", "--context", context, "-n", config.TestNamespace,
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
