package test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestDeployment_00_CreateNamespace creates the workload cluster namespace before deploying resources.
// The namespace is unique per test run (prefix + timestamp) to allow parallel test runs
// and easy cleanup. This namespace is where CAPI CRs (Cluster, AROControlPlane, MachinePool)
// are deployed, which then create Azure resources.
func TestDeployment_00_CreateNamespace(t *testing.T) {
	config := NewTestConfig()

	// Set KUBECONFIG for external cluster mode
	if config.IsExternalCluster() {
		SetEnvVar(t, "KUBECONFIG", config.UseKubeconfig)
	}

	context := config.GetKubeContext()

	PrintTestHeader(t, "TestDeployment_00_CreateNamespace",
		fmt.Sprintf("Create test namespace: %s", config.WorkloadClusterNamespace))

	PrintToTTY("\n=== Creating test namespace ===\n")
	PrintToTTY("Namespace: %s\n", config.WorkloadClusterNamespace)
	PrintToTTY("Context: %s\n\n", context)

	// Check if namespace already exists
	_, err := RunCommandQuiet(t, "kubectl", "--context", context, "get", "namespace", config.WorkloadClusterNamespace)
	if err == nil {
		PrintToTTY("✅ Namespace '%s' already exists\n\n", config.WorkloadClusterNamespace)
		t.Logf("Namespace '%s' already exists", config.WorkloadClusterNamespace)
		return
	}

	// Create the namespace
	PrintToTTY("Creating namespace '%s'...\n", config.WorkloadClusterNamespace)
	output, err := RunCommand(t, "kubectl", "--context", context, "create", "namespace", config.WorkloadClusterNamespace)
	if err != nil {
		PrintToTTY("❌ Failed to create namespace: %v\n", err)
		t.Fatalf("Failed to create namespace '%s': %v\nOutput: %s", config.WorkloadClusterNamespace, err, output)
		return
	}

	PrintToTTY("✅ Namespace '%s' created successfully\n\n", config.WorkloadClusterNamespace)
	t.Logf("Created namespace: %s", config.WorkloadClusterNamespace)

	// Add labels for easy identification and cleanup
	PrintToTTY("Adding labels to namespace...\n")
	_, err = RunCommand(t, "kubectl", "--context", context, "label", "namespace", config.WorkloadClusterNamespace,
		fmt.Sprintf("%s=true", config.TestLabelPrefix),
		fmt.Sprintf("%s-prefix=%s", config.TestLabelPrefix, GetEnvOrDefault("WORKLOAD_CLUSTER_NAMESPACE_PREFIX", config.TestLabelPrefix)),
		"--overwrite")
	if err != nil {
		PrintToTTY("⚠️  Failed to add labels (non-fatal): %v\n", err)
		t.Logf("Warning: failed to add labels to namespace: %v", err)
	} else {
		PrintToTTY("✅ Labels added to namespace\n\n")
	}
}

// TestDeployment_01_CheckExistingClusters checks for existing Cluster CRs that don't match current config.
// This fail-fast check prevents deploying new clusters alongside stale resources from previous
// configurations (e.g., when CAPZ_USER was changed without cleanup).
func TestDeployment_01_CheckExistingClusters(t *testing.T) {

	config := NewTestConfig()

	// Set KUBECONFIG for external cluster mode
	if config.IsExternalCluster() {
		SetEnvVar(t, "KUBECONFIG", config.UseKubeconfig)
	}

	context := config.GetKubeContext()

	PrintToTTY("\n=== Checking for existing Cluster resources ===\n")
	PrintToTTY("Namespace: %s\n", config.WorkloadClusterNamespace)
	PrintToTTY("Expected prefix: %s\n\n", config.ClusterNamePrefix)

	// Check for existing clusters that don't match current config
	mismatched, err := CheckForMismatchedClusters(t, context, config.WorkloadClusterNamespace, config.ClusterNamePrefix)
	if err != nil {
		// Non-fatal: log warning and continue if check fails
		// This allows tests to proceed on clusters without CAPI installed
		PrintToTTY("⚠️  Could not check existing clusters: %v\n", err)
		t.Logf("Warning: Could not check existing clusters: %v", err)
		PrintToTTY("Continuing with deployment...\n\n")
		return
	}

	// Also get all existing clusters for informational purposes
	existing, _ := GetExistingClusterNames(t, context, config.WorkloadClusterNamespace)
	if len(existing) > 0 {
		PrintToTTY("Found %d existing Cluster resource(s):\n", len(existing))
		for _, name := range existing {
			if strings.HasPrefix(name, config.ClusterNamePrefix) {
				PrintToTTY("  ✅ %s (matches current config)\n", name)
			} else {
				PrintToTTY("  ❌ %s (does NOT match current config)\n", name)
			}
		}
		PrintToTTY("\n")
	} else {
		PrintToTTY("✅ No existing Cluster resources found\n\n")
	}

	// Fail if there are mismatched clusters
	if len(mismatched) > 0 {
		errorMsg := FormatMismatchedClustersError(mismatched, config.ClusterNamePrefix, config.WorkloadClusterNamespace)
		PrintToTTY("%s", errorMsg)

		t.Fatalf("Mismatched Cluster CRs found. Clean up existing clusters before deploying with new CAPZ_USER.\n"+
			"Found %d cluster(s) not matching prefix '%s': %v",
			len(mismatched), config.ClusterNamePrefix, mismatched)
	}

	PrintToTTY("✅ All existing clusters match current configuration\n\n")
}

// TestDeployment_ApplyResources tests applying generated resources to the cluster
func TestDeployment_ApplyResources(t *testing.T) {

	config := NewTestConfig()

	// Set KUBECONFIG for external cluster mode
	if config.IsExternalCluster() {
		SetEnvVar(t, "KUBECONFIG", config.UseKubeconfig)
	}

	outputDir := filepath.Join(config.RepoDir, config.GetOutputDirName())

	if !DirExists(outputDir) {
		PrintToTTY("⚠️  Output directory does not exist: %s\n", outputDir)
		t.Skipf("Output directory does not exist: %s", outputDir)
	}

	PrintToTTY("\n=== Applying Kubernetes resources ===\n")

	// Get files to apply (credentials.yaml and aro.yaml)
	expectedFiles := config.GetExpectedFiles()

	// Set kubectl context
	context := config.GetKubeContext()

	// Verify cluster is healthy before applying resources
	// This addresses connection issues after long controller startup periods (issue #265)
	if err := WaitForClusterHealthy(t, context, DefaultHealthCheckTimeout); err != nil {
		t.Fatalf("Cluster health check failed: %v", err)
	}

	for _, file := range expectedFiles {
		filePath := filepath.Join(outputDir, file)
		if !FileExists(filePath) {
			PrintToTTY("❌ Cannot apply missing file: %s\n", file)
			t.Errorf("Cannot apply missing file: %s", file)
			continue
		}

		PrintToTTY("Applying resource file: %s...\n", file)
		t.Logf("Applying resource file: %s", file)

		// Use ApplyWithRetry to handle transient connection issues
		if err := ApplyWithRetry(t, context, filePath, DefaultApplyMaxRetries); err != nil {
			PrintToTTY("❌ Failed to apply %s: %v\n", file, err)
			t.Errorf("Failed to apply %s: %v", file, err)
			continue
		}
	}

	PrintToTTY("\n=== Resource application complete ===\n\n")
}

// TestDeployment_ApplyCredentialsYAML tests applying credentials.yaml to the cluster
func TestDeployment_ApplyCredentialsYAML(t *testing.T) {
	file := "credentials.yaml"

	PrintToTTY("\n=== Applying %s ===\n", file)
	t.Logf("Applying %s", file)

	config := NewTestConfig()

	// Set KUBECONFIG for external cluster mode
	if config.IsExternalCluster() {
		SetEnvVar(t, "KUBECONFIG", config.UseKubeconfig)
	}

	outputDir := filepath.Join(config.RepoDir, config.GetOutputDirName())

	if !DirExists(outputDir) {
		PrintToTTY("⚠️  Output directory does not exist: %s\n\n", outputDir)
		t.Skipf("Output directory does not exist: %s", outputDir)
	}

	filePath := filepath.Join(outputDir, file)
	if !FileExists(filePath) {
		PrintToTTY("❌ %s not found at %s\n\n", file, filePath)
		t.Errorf("%s not found at %s.\n\n"+
			"This file should be generated by TestInfrastructure_GenerateResources.\n\n"+
			"To regenerate infrastructure files:\n"+
			"  go test -v ./test -run TestInfrastructure_GenerateResources",
			file, filePath)
		return
	}

	context := config.GetKubeContext()

	// Verify cluster is healthy before applying resources
	// This addresses connection issues after long controller startup periods (issue #265)
	if err := WaitForClusterHealthy(t, context, DefaultHealthCheckTimeout); err != nil {
		t.Fatalf("Cluster health check failed: %v", err)
	}

	// Use ApplyWithRetry to handle transient connection issues
	if err := ApplyWithRetry(t, context, filePath, DefaultApplyMaxRetries); err != nil {
		PrintToTTY("❌ Failed to apply %s: %v\n\n", file, err)
		t.Errorf("Failed to apply %s: %v", file, err)
		return
	}

	PrintToTTY("✅ Successfully applied %s\n\n", file)
}

// TestDeployment_ApplyAROClusterYAML tests applying aro.yaml to the cluster
func TestDeployment_ApplyAROClusterYAML(t *testing.T) {
	file := "aro.yaml"

	PrintToTTY("\n=== Applying %s (ARO cluster configuration) ===\n", file)
	t.Logf("Applying %s (ARO cluster configuration)", file)

	config := NewTestConfig()

	// Set KUBECONFIG for external cluster mode
	if config.IsExternalCluster() {
		SetEnvVar(t, "KUBECONFIG", config.UseKubeconfig)
	}

	outputDir := filepath.Join(config.RepoDir, config.GetOutputDirName())

	if !DirExists(outputDir) {
		PrintToTTY("⚠️  Output directory does not exist: %s\n\n", outputDir)
		t.Skipf("Output directory does not exist: %s", outputDir)
	}

	filePath := filepath.Join(outputDir, file)
	if !FileExists(filePath) {
		PrintToTTY("❌ %s not found at %s\n\n", file, filePath)
		t.Errorf("%s (ARO cluster configuration) not found at %s.\n\n"+
			"This file should be generated by TestInfrastructure_GenerateResources.\n\n"+
			"To regenerate infrastructure files:\n"+
			"  go test -v ./test -run TestInfrastructure_GenerateResources",
			file, filePath)
		return
	}

	context := config.GetKubeContext()

	// Verify cluster is healthy before applying resources
	// This addresses connection issues after long controller startup periods (issue #265)
	if err := WaitForClusterHealthy(t, context, DefaultHealthCheckTimeout); err != nil {
		t.Fatalf("Cluster health check failed: %v", err)
	}

	// Use ApplyWithRetry to handle transient connection issues
	if err := ApplyWithRetry(t, context, filePath, DefaultApplyMaxRetries); err != nil {
		PrintToTTY("❌ Failed to apply %s: %v\n\n", file, err)
		t.Errorf("Failed to apply %s: %v", file, err)
		return
	}

	PrintToTTY("✅ Successfully applied %s\n\n", file)
}

// TestDeployment_MonitorCluster tests monitoring the ARO cluster deployment
func TestDeployment_MonitorCluster(t *testing.T) {
	config := NewTestConfig()

	// Set KUBECONFIG for external cluster mode
	if config.IsExternalCluster() {
		SetEnvVar(t, "KUBECONFIG", config.UseKubeconfig)
	}

	provisionedClusterName := config.GetProvisionedClusterName()

	PrintTestHeader(t, "TestDeployment_MonitorCluster",
		"Get cluster status snapshot using monitor-cluster-json.sh")

	context := config.GetKubeContext()

	PrintToTTY("\n=== Monitoring cluster deployment ===\n")
	PrintToTTY("Cluster: %s\n", provisionedClusterName)
	PrintToTTY("Namespace: %s\n", config.WorkloadClusterNamespace)
	PrintToTTY("Context: %s\n", context)
	PrintToTTY("\n")

	// Get cluster status using the monitoring script
	data, err := MonitorCluster(t, context, config.WorkloadClusterNamespace, provisionedClusterName)
	if err != nil {
		PrintToTTY("⚠️  Could not get cluster status (may not be deployed yet): %v\n\n", err)
		t.Skipf("Could not get cluster status: %v", err)
	}

	// Display comprehensive status
	PrintToTTY("✅ Successfully retrieved cluster status\n\n")
	PrintToTTY("%s\n", data.FormatSummary())
	PrintToTTY("\n")

	// Show detailed status
	PrintToTTY("=== Cluster Details ===\n")
	PrintToTTY("Provider: %s\n", data.GetProviderType())
	PrintToTTY("Phase: %s\n", data.Cluster.Phase)
	PrintToTTY("Infrastructure Ready: %v\n", data.Infrastructure.Ready)
	PrintToTTY("Control Plane Ready: %v\n", data.ControlPlane.Ready)

	if len(data.MachinePools) > 0 {
		PrintToTTY("\n=== Machine Pools ===\n")
		for _, mp := range data.MachinePools {
			PrintToTTY("- %s: %d/%d replicas ready\n", mp.Name, mp.ReadyReplicas, mp.Replicas)
			if mp.Infrastructure != nil {
				PrintToTTY("  Infrastructure: %s (ready: %v, provisioning: %s)\n",
					mp.Infrastructure.Kind,
					mp.Infrastructure.Ready,
					mp.Infrastructure.ProvisioningState)
			}
		}
	}

	if data.Nodes != nil && len(data.Nodes) > 0 {
		PrintToTTY("\n=== Nodes (%d total, %d ready) ===\n", data.Summary.NodeCount, data.GetReadyNodeCount())
		for _, node := range data.Nodes {
			PrintToTTY("- %s: ready=%s, roles=%s, version=%s\n",
				node.Name, node.Ready, node.Roles, node.Version)
		}
	} else {
		PrintToTTY("\n=== Nodes ===\n")
		PrintToTTY("No nodes available yet (kubeconfig may not be ready)\n")
	}

	PrintToTTY("\n=== Cluster Monitoring Test Complete ===\n\n")
	t.Logf("Cluster status: %s", data.FormatSummary())
}

// TestDeployment_WaitForControlPlane waits for control plane and machine pools to be ready.
// Uses the generic cluster monitoring function which works for ARO, ROSA, and any CAPI cluster.
func TestDeployment_WaitForControlPlane(t *testing.T) {
	config := NewTestConfig()

	// Set KUBECONFIG for external cluster mode
	if config.IsExternalCluster() {
		SetEnvVar(t, "KUBECONFIG", config.UseKubeconfig)
	}

	provisionedClusterName := config.GetProvisionedClusterName()
	context := config.GetKubeContext()

	PrintTestHeader(t, "TestDeployment_WaitForControlPlane",
		"Wait for control plane and machine pools to become ready")

	PrintToTTY("\n=== Waiting for control plane to be ready ===\n")
	PrintToTTY("Cluster: %s\n", provisionedClusterName)
	PrintToTTY("Namespace: %s\n", config.WorkloadClusterNamespace)
	PrintToTTY("Context: %s\n", context)
	PrintToTTY("Timeout: %v\n\n", config.DeploymentTimeout)

	// Wait for control plane using generic monitoring
	data, err := MonitorControlPlaneUntilReady(t, context, config.WorkloadClusterNamespace, provisionedClusterName, config.DeploymentTimeout)
	if err != nil {
		PrintToTTY("\n❌ %v\n\n", err)
		PrintToTTY("Troubleshooting steps:\n")
		PrintToTTY("  1. Check control plane: kubectl -n %s get %s %s -o yaml\n",
			config.WorkloadClusterNamespace, data.ControlPlane.Kind, data.ControlPlane.Name)
		PrintToTTY("  2. Check cluster: kubectl -n %s get cluster %s -o yaml\n",
			config.WorkloadClusterNamespace, provisionedClusterName)
		PrintToTTY("  3. Check controller logs: kubectl -n %s logs -l control-plane=controller-manager --tail=100\n",
			config.CAPZNamespace)
		PrintToTTY("\nTo increase timeout: export DEPLOYMENT_TIMEOUT=60m\n\n")
		t.Fatalf("Control plane failed to become ready: %v", err)
	}

	// Display final status
	PrintToTTY("\n✅ Control plane is ready!\n\n")
	PrintToTTY("%s\n\n", data.FormatSummary())

	// Show detailed component status
	PrintToTTY("=== Component Details ===\n")
	PrintToTTY("Control Plane: %s (ready: %v, replicas: %d/%d)\n",
		data.ControlPlane.Kind,
		data.ControlPlane.Ready,
		data.ControlPlane.ReadyReplicas,
		data.ControlPlane.Replicas)

	if len(data.ControlPlane.Conditions) > 0 {
		PrintToTTY("\nControl Plane Conditions:\n")
		for _, cond := range data.ControlPlane.Conditions {
			status := "✅"
			if cond.Status != "True" {
				status = "⏳"
			}
			PrintToTTY("  %s %s: %s\n", status, cond.Type, cond.Status)
			if cond.Reason != "" {
				PrintToTTY("     Reason: %s\n", cond.Reason)
			}
		}
	}

	if len(data.MachinePools) > 0 {
		PrintToTTY("\nMachine Pools:\n")
		for _, mp := range data.MachinePools {
			status := "✅"
			if mp.ReadyReplicas < mp.Replicas {
				status = "⏳"
			}
			PrintToTTY("  %s %s: %d/%d replicas ready\n", status, mp.Name, mp.ReadyReplicas, mp.Replicas)
			if mp.Infrastructure != nil {
				PrintToTTY("     Infrastructure: %s (ready: %v, state: %s)\n",
					mp.Infrastructure.Kind,
					mp.Infrastructure.Ready,
					mp.Infrastructure.ProvisioningState)
			}
		}
	}

	PrintToTTY("\n")
	t.Logf("Control plane ready: %s", data.FormatSummary())
}

// TestDeployment_VerifyInfrastructureResources waits for AROCluster infrastructure to be fully ready.
// This test polls AROCluster.status.conditions[] for NetworkInfrastructureReady=True,
// which is the controller's authoritative signal that all infrastructure resources are
// properly reconciled and the deployment can proceed to HCP creation.
//
// Checking resource counts alone (46/46) is insufficient: all resources can report ready=true
// while NetworkInfrastructureReady is still False.
func TestDeployment_VerifyInfrastructureResources(t *testing.T) {
	config := NewTestConfig()

	// Set KUBECONFIG for external cluster mode
	if config.IsExternalCluster() {
		SetEnvVar(t, "KUBECONFIG", config.UseKubeconfig)
	}

	context := config.GetKubeContext()
	provisionedClusterName := config.GetProvisionedClusterName()

	timeout := config.DeploymentTimeout
	pollInterval := 30 * time.Second
	startTime := time.Now()

	PrintToTTY("\n=== Waiting for NetworkInfrastructureReady ===\n")
	PrintToTTY("Cluster: %s | Namespace: %s\n", provisionedClusterName, config.WorkloadClusterNamespace)
	PrintToTTY("Timeout: %v | Poll interval: %v\n\n", timeout, pollInterval)
	t.Logf("Waiting for NetworkInfrastructureReady (namespace: %s, timeout: %v)...", config.WorkloadClusterNamespace, timeout)

	iteration := 0
	for {
		elapsed := time.Since(startTime)
		remaining := timeout - elapsed

		if elapsed > timeout {
			PrintToTTY("\n❌ Timeout reached after %v waiting for NetworkInfrastructureReady\n\n", elapsed.Round(time.Second))
			t.Fatalf("Timeout waiting for NetworkInfrastructureReady after %v.\n\n"+
				"Check AROCluster status:\n"+
				"  kubectl --context %s -n %s get arocluster %s -o yaml",
				elapsed.Round(time.Second), context, config.WorkloadClusterNamespace, provisionedClusterName)
			return
		}

		iteration++

		infraStatus := GetInfrastructureResourceStatus(t, context, config.WorkloadClusterNamespace, provisionedClusterName)

		if infraStatus.TotalResources == 0 {
			PrintToTTY("[%d] ⚠️  No infrastructure resources found yet\n", iteration)
			ReportProgress(t, iteration, elapsed, remaining, timeout)
			time.Sleep(pollInterval)
			continue
		}

		// Display infrastructure progress
		ReportInfrastructureProgress(t, iteration, elapsed, remaining, infraStatus)

		// Check NetworkInfrastructureReady condition
		for _, cond := range infraStatus.Conditions {
			if cond.Type == "NetworkInfrastructureReady" {
				if cond.Status == "True" {
					PrintToTTY("\n✅ NetworkInfrastructureReady is True (took %v)\n", elapsed.Round(time.Second))
					PrintToTTY("✅ %d/%d infrastructure resources reconciled\n\n",
						infraStatus.ReadyResources, infraStatus.TotalResources)
					t.Logf("NetworkInfrastructureReady=True, %d resources reconciled (took %v)",
						infraStatus.TotalResources, elapsed.Round(time.Second))
					return
				}
				detail := cond.Status
				if cond.Reason != "" {
					detail = fmt.Sprintf("%s (%s)", cond.Status, cond.Reason)
				}
				PrintToTTY("[%d] ⏳ NetworkInfrastructureReady: %s\n", iteration, detail)
			}
		}

		ReportProgress(t, iteration, elapsed, remaining, timeout)
		time.Sleep(pollInterval)
	}
}

// TestDeployment_VerifyAROClusterReady verifies AROCluster.status.ready becomes True.
// This follows AROControlPlane.Ready (step 8) in the deployment sequence.
func TestDeployment_VerifyAROClusterReady(t *testing.T) {
	config := NewTestConfig()

	if config.IsExternalCluster() {
		SetEnvVar(t, "KUBECONFIG", config.UseKubeconfig)
	}

	context := config.GetKubeContext()
	provisionedClusterName := config.GetProvisionedClusterName()

	timeout := 5 * time.Minute
	pollInterval := 10 * time.Second
	startTime := time.Now()

	PrintToTTY("\n=== Waiting for AROCluster.Ready ===\n")
	PrintToTTY("Cluster: %s | Namespace: %s\n", provisionedClusterName, config.WorkloadClusterNamespace)
	PrintToTTY("Command: kubectl --context %s -n %s get arocluster %s -o jsonpath={.status.ready}\n\n",
		context, config.WorkloadClusterNamespace, provisionedClusterName)

	for {
		elapsed := time.Since(startTime)
		if elapsed > timeout {
			t.Fatalf("Timeout after %v waiting for AROCluster.Ready=true.\n"+
				"  kubectl --context %s -n %s get arocluster %s -o yaml",
				elapsed.Round(time.Second), context, config.WorkloadClusterNamespace, provisionedClusterName)
			return
		}

		output, err := RunCommandQuiet(t, "kubectl", "--context", context, "-n", config.WorkloadClusterNamespace,
			"get", "arocluster", provisionedClusterName, "-o", "jsonpath={.status.ready}")
		if err == nil && strings.TrimSpace(output) == "true" {
			PrintToTTY("✅ AROCluster.Ready is True (took %v)\n\n", elapsed.Round(time.Second))
			t.Logf("AROCluster.Ready=true (took %v)", elapsed.Round(time.Second))
			return
		}

		status := strings.TrimSpace(output)
		if status == "" {
			status = "<not set yet>"
		}
		PrintToTTY("⏳ AROCluster.Ready: %s (elapsed %v)\n", status, elapsed.Round(time.Second))
		time.Sleep(pollInterval)
	}
}

// TestDeployment_VerifyClusterProvisioned verifies cluster.status.initialization.infrastructureProvisioned becomes True.
// This follows AROCluster.Ready (step 9) in the deployment sequence.
func TestDeployment_VerifyClusterProvisioned(t *testing.T) {
	config := NewTestConfig()

	if config.IsExternalCluster() {
		SetEnvVar(t, "KUBECONFIG", config.UseKubeconfig)
	}

	context := config.GetKubeContext()
	provisionedClusterName := config.GetProvisionedClusterName()

	timeout := 5 * time.Minute
	pollInterval := 10 * time.Second
	startTime := time.Now()

	PrintToTTY("\n=== Waiting for Cluster.Initialization.InfrastructureProvisioned ===\n")
	PrintToTTY("Cluster: %s | Namespace: %s\n", provisionedClusterName, config.WorkloadClusterNamespace)
	PrintToTTY("Command: kubectl --context %s -n %s get cluster %s -o jsonpath={.status.initialization.infrastructureProvisioned}\n\n",
		context, config.WorkloadClusterNamespace, provisionedClusterName)

	for {
		elapsed := time.Since(startTime)
		if elapsed > timeout {
			t.Fatalf("Timeout after %v waiting for cluster.status.initialization.infrastructureProvisioned=true.\n"+
				"  kubectl --context %s -n %s get cluster %s -o yaml",
				elapsed.Round(time.Second), context, config.WorkloadClusterNamespace, provisionedClusterName)
			return
		}

		output, err := RunCommandQuiet(t, "kubectl", "--context", context, "-n", config.WorkloadClusterNamespace,
			"get", "cluster", provisionedClusterName, "-o", "jsonpath={.status.initialization.infrastructureProvisioned}")
		if err == nil && strings.TrimSpace(output) == "true" {
			PrintToTTY("✅ Cluster.Initialization.InfrastructureProvisioned is True (took %v)\n\n", elapsed.Round(time.Second))
			t.Logf("cluster.status.initialization.infrastructureProvisioned=true (took %v)", elapsed.Round(time.Second))
			return
		}

		status := strings.TrimSpace(output)
		if status == "" {
			status = "<not set yet>"
		}
		PrintToTTY("⏳ Cluster.Initialization.InfrastructureProvisioned: %s (elapsed %v)\n", status, elapsed.Round(time.Second))
		time.Sleep(pollInterval)
	}
}

// TestDeployment_VerifyClusterInfrastructureReady verifies CAPI Cluster InfrastructureReady condition becomes True.
// This follows Cluster.Initialization.InfrastructureProvisioned (step 10) in the deployment sequence.
func TestDeployment_VerifyClusterInfrastructureReady(t *testing.T) {
	config := NewTestConfig()

	if config.IsExternalCluster() {
		SetEnvVar(t, "KUBECONFIG", config.UseKubeconfig)
	}

	context := config.GetKubeContext()
	provisionedClusterName := config.GetProvisionedClusterName()

	timeout := 5 * time.Minute
	pollInterval := 10 * time.Second
	startTime := time.Now()

	PrintToTTY("\n=== Waiting for CAPI Cluster.InfrastructureReady ===\n")
	PrintToTTY("Cluster: %s | Namespace: %s\n", provisionedClusterName, config.WorkloadClusterNamespace)
	PrintToTTY("Command: kubectl --context %s -n %s get cluster %s -o jsonpath={.status.conditions[?(@.type=='InfrastructureReady')].status}\n\n",
		context, config.WorkloadClusterNamespace, provisionedClusterName)

	for {
		elapsed := time.Since(startTime)
		if elapsed > timeout {
			t.Fatalf("Timeout after %v waiting for Cluster InfrastructureReady=True.\n"+
				"  kubectl --context %s -n %s get cluster %s -o yaml",
				elapsed.Round(time.Second), context, config.WorkloadClusterNamespace, provisionedClusterName)
			return
		}

		output, err := RunCommandQuiet(t, "kubectl", "--context", context, "-n", config.WorkloadClusterNamespace,
			"get", "cluster", provisionedClusterName,
			"-o", "jsonpath={.status.conditions[?(@.type=='InfrastructureReady')].status}")
		if err == nil && strings.TrimSpace(output) == "True" {
			PrintToTTY("✅ Cluster.InfrastructureReady is True (took %v)\n\n", elapsed.Round(time.Second))
			t.Logf("Cluster InfrastructureReady=True (took %v)", elapsed.Round(time.Second))
			return
		}

		status := strings.TrimSpace(output)
		if status == "" {
			status = "<not set yet>"
		}
		PrintToTTY("⏳ Cluster.InfrastructureReady: %s (elapsed %v)\n", status, elapsed.Round(time.Second))
		time.Sleep(pollInterval)
	}
}
