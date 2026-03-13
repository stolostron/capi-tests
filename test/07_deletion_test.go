package test

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestDeletion_DeleteCluster tests deleting the workload cluster from the management cluster.
// This initiates the deletion by removing the Cluster resource, which triggers
// CAPI to clean up all associated resources including cloud provider resources.
func TestDeletion_DeleteCluster(t *testing.T) {
	config := NewTestConfig()

	// Set KUBECONFIG for external cluster mode
	if config.IsExternalCluster() {
		SetEnvVar(t, "KUBECONFIG", config.UseKubeconfig)
	}

	context := config.GetKubeContext()

	// Get the provisioned cluster name from the cluster YAML
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

	// ROSA-specific deletion: Delete ROSAControlPlane first to avoid minimum replica constraint errors
	// ROSA enforces minimum 2 replicas across all machine pools, so deleting machine pools individually fails
	// Deleting the control plane first triggers proper cluster deletion in AWS
	if config.HasProvider("rosa") {
		controlPlaneName := config.GetProvisionedControlPlaneName()

		// Check if ROSAControlPlane exists
		_, cpErr := RunCommand(t, "kubectl", "--context", context, "-n", config.WorkloadClusterNamespace,
			"get", "rosacontrolplane", controlPlaneName)
		if cpErr == nil {
			PrintToTTY("🗑️  Deleting ROSAControlPlane '%s' first...\n", controlPlaneName)
			t.Logf("Deleting ROSAControlPlane '%s' before cluster", controlPlaneName)

			cpOutput, err := RunCommand(t, "kubectl", "--context", context, "-n", config.WorkloadClusterNamespace,
				"delete", "rosacontrolplane", controlPlaneName, "--wait=false")
			if err != nil {
				PrintToTTY("⚠️  Failed to delete ROSAControlPlane: %v\n", err)
				t.Logf("Warning: Failed to delete ROSAControlPlane: %v\nOutput: %s", err, cpOutput)
			} else {
				PrintToTTY("✅ ROSAControlPlane deletion initiated\n")
				t.Logf("ROSAControlPlane deletion initiated: %s", cpOutput)
			}
		}
	}

	// Delete the cluster resource - this triggers cascading deletion of all related resources
	// Use --wait=false to return immediately so the next test can monitor deletion progress
	PrintToTTY("🗑️  Deleting Cluster resource...\n")
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
// This monitors the cluster resource until it no longer exists, showing detailed
// progress information about all resources being deleted.
func TestDeletion_WaitForClusterDeletion(t *testing.T) {
	config := NewTestConfig()

	// Set KUBECONFIG for external cluster mode
	if config.IsExternalCluster() {
		SetEnvVar(t, "KUBECONFIG", config.UseKubeconfig)
	}

	context := config.GetKubeContext()

	// Get the provisioned cluster name from the cluster YAML
	provisionedClusterName := config.GetProvisionedClusterName()

	// Azure resource group name (only for ARO provider)
	resourceGroup := ""
	if config.HasProvider("aro") {
		resourceGroup = fmt.Sprintf("%s-resgroup", config.ClusterNamePrefix)
	}

	PrintTestHeader(t, "TestDeletion_WaitForClusterDeletion",
		"Wait for cluster resource to be fully deleted")

	// Use the deployment timeout for deletion as well (deletion can take significant time)
	timeout := config.DeploymentTimeout
	pollInterval := 30 * time.Second
	startTime := time.Now()

	PrintToTTY("⏳ Waiting for cluster '%s' to be deleted...\n", provisionedClusterName)
	PrintToTTY("Namespace: %s | Timeout: %v | Poll interval: %v\n", config.WorkloadClusterNamespace, timeout, pollInterval)
	if resourceGroup != "" {
		PrintToTTY("Azure Resource Group: %s\n", resourceGroup)
	}
	PrintToTTY("\n")
	t.Logf("Waiting for cluster '%s' deletion (namespace: %s, timeout: %v)...", provisionedClusterName, config.WorkloadClusterNamespace, timeout)

	iteration := 0
	for {
		elapsed := time.Since(startTime)
		remaining := timeout - elapsed

		if elapsed > timeout {
			PrintToTTY("\n❌ Timeout waiting for cluster deletion after %v\n\n", elapsed.Round(time.Second))

			// Build provider-agnostic troubleshooting message
			controlPlaneResource := "controlplane"
			cleanupCommand := "make clean"
			additionalSteps := ""

			if config.HasProvider("aro") {
				controlPlaneResource = "arocontrolplane"
				cleanupCommand = "make clean-azure"
				if resourceGroup != "" {
					additionalSteps = fmt.Sprintf("  4. Check Azure resource group: az group show --name %s 2>/dev/null\n", resourceGroup)
				}
			} else if config.HasProvider("rosa") {
				controlPlaneResource = "rosacontrolplane"
			}

			t.Errorf("Timeout waiting for cluster '%s' to be deleted after %v.\n\n"+
				"Troubleshooting steps:\n"+
				"  1. Check cluster status: kubectl --context %s -n %s get cluster %s -o yaml\n"+
				"  2. Check for stuck finalizers: kubectl --context %s -n %s get cluster %s -o jsonpath='{.metadata.finalizers}'\n"+
				"  3. Check remaining CAPI resources: kubectl --context %s -n %s get %s,machinepool\n"+
				"%s\n"+
				"Common causes:\n"+
				"  - Cloud resource deletion taking longer than expected\n"+
				"  - Finalizers blocking resource deletion\n"+
				"  - Cloud resources stuck in 'Deleting' state\n\n"+
				"To increase timeout: export DEPLOYMENT_TIMEOUT=60m\n"+
				"To manually clean up:\n"+
				"  %s",
				provisionedClusterName, elapsed.Round(time.Second),
				context, config.WorkloadClusterNamespace, provisionedClusterName,
				context, config.WorkloadClusterNamespace, provisionedClusterName,
				context, config.WorkloadClusterNamespace, controlPlaneResource,
				additionalSteps,
				cleanupCommand)
			return
		}

		iteration++

		// Get comprehensive deletion status
		status := GetDeletionResourceStatus(t, context, config.WorkloadClusterNamespace, provisionedClusterName, resourceGroup)

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

// TestDeletion_VerifyControlPlaneDeletion verifies the control plane resource is deleted.
func TestDeletion_VerifyControlPlaneDeletion(t *testing.T) {
	config := NewTestConfig()

	// Set KUBECONFIG for external cluster mode
	if config.IsExternalCluster() {
		SetEnvVar(t, "KUBECONFIG", config.UseKubeconfig)
	}

	context := config.GetKubeContext()
	provisionedClusterName := config.GetProvisionedClusterName()

	PrintTestHeader(t, "TestDeletion_VerifyControlPlaneDeletion",
		"Verify control plane resource is deleted")

	// Use monitor script to get cluster status
	data, err := MonitorCluster(t, context, config.WorkloadClusterNamespace, provisionedClusterName)
	if err != nil {
		// Check if this is "not found" (deletion complete) vs. a real error
		errMsg := err.Error()
		if strings.Contains(strings.ToLower(errMsg), "not found") ||
			strings.Contains(strings.ToLower(errMsg), "notfound") {
			// Cluster not found - deletion complete
			PrintToTTY("✅ Cluster not found - all resources deleted\n\n")
			t.Log("Cluster not found - control plane deletion verified")
			return
		}
		// Real error - not just "not found"
		PrintToTTY("⚠️  Error checking cluster status: %v\n\n", err)
		t.Logf("Warning: Could not verify control plane deletion: %v", err)
		return
	}

	// Check if control plane exists
	if data.ControlPlane.Name == "" {
		PrintToTTY("✅ No control plane resources found (deleted successfully)\n\n")
		t.Log("No control plane resources found - deletion successful")
	} else {
		PrintToTTY("⚠️  %s resource still exists: %s\n\n", data.ControlPlane.Kind, data.ControlPlane.Name)
		t.Logf("Warning: %s resource still exists: %s", data.ControlPlane.Kind, data.ControlPlane.Name)
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
	provisionedClusterName := config.GetProvisionedClusterName()

	PrintTestHeader(t, "TestDeletion_VerifyMachinePoolDeletion",
		"Verify machine pool resources are deleted")

	// Use monitor script to get cluster status
	data, err := MonitorCluster(t, context, config.WorkloadClusterNamespace, provisionedClusterName)
	if err != nil {
		// Check if this is "not found" (deletion complete) vs. a real error
		errMsg := err.Error()
		if strings.Contains(strings.ToLower(errMsg), "not found") ||
			strings.Contains(strings.ToLower(errMsg), "notfound") {
			// Cluster not found - deletion complete
			PrintToTTY("✅ Cluster not found - all resources deleted\n\n")
			t.Log("Cluster not found - machine pool deletion verified")
			return
		}
		// Real error - not just "not found"
		PrintToTTY("⚠️  Error checking cluster status: %v\n\n", err)
		t.Logf("Warning: Could not verify machine pool deletion: %v", err)
		return
	}

	// Check if machine pools exist
	machinePoolCount := len(data.MachinePools)
	if machinePoolCount == 0 {
		PrintToTTY("✅ No MachinePool resources found (deleted successfully)\n\n")
		t.Log("No MachinePool resources found - deletion successful")
	} else {
		PrintToTTY("⚠️  MachinePool resources still exist: %d remaining\n", machinePoolCount)
		for _, mp := range data.MachinePools {
			PrintToTTY("  - %s\n", mp.Name)
		}
		PrintToTTY("\n")
		t.Logf("Warning: %d MachinePool resources still exist", machinePoolCount)
	}
}

// TestDeletion_VerifyAzureResourcesDeletion verifies Azure resources are cleaned up.
// This checks if the Azure resource group still exists after cluster deletion.
// This test is ARO-specific and skipped for other providers.
func TestDeletion_VerifyAzureResourcesDeletion(t *testing.T) {
	config := NewTestConfig()

	// Skip for non-ARO providers
	if !config.HasProvider("aro") {
		t.Skip("Skipping ARO-specific test (Azure resource group verification is ARO-specific)")
	}

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
	provisionedClusterName := config.GetProvisionedClusterName()

	PrintTestHeader(t, "TestDeletion_Summary",
		"Summary of cluster deletion status")

	PrintToTTY("=== Deletion Summary ===\n\n")

	// Use monitor script to get cluster status
	data, err := MonitorCluster(t, context, config.WorkloadClusterNamespace, provisionedClusterName)
	if err != nil {
		// Check if this is "not found" (deletion complete) vs. a real error
		errMsg := err.Error()
		if strings.Contains(strings.ToLower(errMsg), "not found") ||
			strings.Contains(strings.ToLower(errMsg), "notfound") {
			// Cluster not found - deletion complete
			PrintToTTY("✅ Workload cluster deleted successfully\n")
			PrintToTTY("✅ All CAPI resources deleted\n")
		} else {
			// Real error - not just "not found"
			PrintToTTY("⚠️  Could not verify deletion status: %v\n", err)
			t.Logf("Warning: Could not verify deletion status: %v", err)
		}
	} else {
		// Cluster still exists
		PrintToTTY("⚠️  Cluster still exists (Phase: %s)\n", data.Summary.Phase)

		// Check control plane
		if data.ControlPlane.Name != "" {
			PrintToTTY("⚠️  %s resource remains: %s\n", data.ControlPlane.Kind, data.ControlPlane.Name)
		}

		// Check machine pools
		machinePoolCount := len(data.MachinePools)
		if machinePoolCount > 0 {
			PrintToTTY("⚠️  %d MachinePool resource(s) remain\n", machinePoolCount)
		}
	}

	// Summary message
	PrintToTTY("\n=== Deletion Test Complete ===\n\n")
	t.Log("Deletion test phase completed")
}
