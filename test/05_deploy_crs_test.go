package test

import (
	"fmt"
	"os"
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
		"capz-test=true",
		fmt.Sprintf("capz-test-prefix=%s", GetEnvOrDefault("WORKLOAD_CLUSTER_NAMESPACE_PREFIX", "capz-test")),
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

	PrintToTTY("\n=== Starting Cluster Monitoring Test ===\n")

	config := NewTestConfig()

	// Set KUBECONFIG for external cluster mode
	if config.IsExternalCluster() {
		SetEnvVar(t, "KUBECONFIG", config.UseKubeconfig)
	}

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
	context := config.GetKubeContext()
	SetEnvVar(t, "KUBECONFIG", fmt.Sprintf("%s/.kube/config", os.Getenv("HOME")))

	// First, check if cluster resource exists
	// Use the provisioned cluster name from aro.yaml, not WORKLOAD_CLUSTER_NAME
	provisionedClusterName := config.GetProvisionedClusterName()
	PrintToTTY("\n=== Monitoring cluster deployment ===\n")
	PrintToTTY("Cluster: %s\n", provisionedClusterName)
	PrintToTTY("Namespace: %s\n", config.WorkloadClusterNamespace)
	PrintToTTY("Context: %s\n", context)
	PrintToTTY("\nChecking if cluster resource exists...\n")
	t.Logf("Checking for cluster resource: %s (namespace: %s)", provisionedClusterName, config.WorkloadClusterNamespace)

	output, err := RunCommand(t, "kubectl", "--context", context, "-n", config.WorkloadClusterNamespace, "get", "cluster", provisionedClusterName)
	if err != nil {
		PrintToTTY("⚠️  Cluster resource not found (may not be deployed yet)\n\n")
		t.Skipf("Cluster resource not found (may not be deployed yet): %v", err)
	}

	PrintToTTY("✅ Cluster resource exists\n")
	t.Logf("Cluster resource exists:\n%s", output)

	// Use clusterctl to describe the cluster
	PrintToTTY("\n📊 Fetching cluster status with clusterctl...\n")
	PrintToTTY("Running: %s describe cluster %s -n %s --show-conditions=all\n", clusterctlPath, provisionedClusterName, config.WorkloadClusterNamespace)
	PrintToTTY("This may take a few moments...\n")
	t.Logf("Monitoring cluster deployment status using clusterctl...")

	output, err = RunCommand(t, clusterctlPath, "describe", "cluster", provisionedClusterName, "-n", config.WorkloadClusterNamespace, "--show-conditions=all")
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

// TestDeployment_WaitForControlPlane waits for both control plane and machine pool to be ready.
// These two components deploy in parallel:
//   - AROControlPlane.Ready: HCP cluster + kubeconfig created
//   - AROMachinePool: worker node pool provisioned
//
// The test waits for BOTH to be ready before proceeding.
func TestDeployment_WaitForControlPlane(t *testing.T) {

	config := NewTestConfig()

	// Set KUBECONFIG for external cluster mode
	if config.IsExternalCluster() {
		SetEnvVar(t, "KUBECONFIG", config.UseKubeconfig)
	}

	context := config.GetKubeContext()

	// Get the specific resource names for the cluster being deployed
	// This prevents checking the wrong resources when multiple clusters exist (issue #355)
	provisionedClusterName := config.GetProvisionedClusterName()
	aroControlPlaneName := config.GetProvisionedAROControlPlaneName()
	machinePoolName := config.GetProvisionedMachinePoolName()

	// Wait for both to be ready (with configurable timeout)
	timeout := config.DeploymentTimeout
	pollInterval := 30 * time.Second
	startTime := time.Now()

	// Print to stderr for immediate visibility (unbuffered)
	PrintToTTY("\n=== Waiting for control plane and machine pool to be ready ===\n")
	PrintToTTY("Cluster: %s\n", provisionedClusterName)
	PrintToTTY("AROControlPlane: %s\n", aroControlPlaneName)
	PrintToTTY("MachinePool: %s\n", machinePoolName)
	PrintToTTY("Namespace: %s\n", config.WorkloadClusterNamespace)
	PrintToTTY("Timeout: %v | Poll interval: %v\n\n", timeout, pollInterval)
	t.Logf("Waiting for control plane and machine pool (namespace: %s, timeout: %v)...", config.WorkloadClusterNamespace, timeout)

	controlPlaneReady := false
	machinePoolReady := false

	iteration := 0
	for {
		elapsed := time.Since(startTime)
		remaining := timeout - elapsed

		if elapsed > timeout {
			PrintToTTY("\n❌ Timeout reached after %v\n\n", elapsed.Round(time.Second))
			t.Errorf("Timeout waiting for deployment after %v.\n"+
				"  AROControlPlane ready: %v\n"+
				"  MachinePool ready: %v\n\n"+
				"Troubleshooting steps:\n"+
				"  1. Check AROControlPlane status: kubectl --context %s -n %s get arocontrolplane %s -o yaml\n"+
				"  2. Check MachinePool status: kubectl --context %s -n %s get machinepool %s -o yaml\n"+
				"  3. Check cluster conditions: kubectl --context %s -n %s get cluster %s -o yaml\n"+
				"  4. Check controller logs: kubectl --context %s -n capz-system logs -l control-plane=controller-manager --tail=100\n\n"+
				"To increase timeout: export DEPLOYMENT_TIMEOUT=60m",
				elapsed.Round(time.Second),
				controlPlaneReady, machinePoolReady,
				context, config.WorkloadClusterNamespace, aroControlPlaneName,
				context, config.WorkloadClusterNamespace, machinePoolName,
				context, config.WorkloadClusterNamespace, provisionedClusterName,
				context)
			return
		}

		iteration++

		PrintToTTY("[%d] Checking deployment status...\n", iteration)

		// Check AROControlPlane ready status
		if !controlPlaneReady {
			output, err := RunCommandQuiet(t, "kubectl", "--context", context, "get",
				"arocontrolplane", aroControlPlaneName, "-n", config.WorkloadClusterNamespace, "-o", "jsonpath={.status.ready}")
			if err != nil {
				PrintToTTY("[%d] ⚠️  AROControlPlane status check failed: %v\n", iteration, err)
			} else {
				status := strings.TrimSpace(output)
				if status == "true" {
					controlPlaneReady = true
					PrintToTTY("[%d] ✅ AROControlPlane.Ready: true (took %v)\n", iteration, elapsed.Round(time.Second))
					t.Logf("AROControlPlane.Ready=true (took %v)", elapsed.Round(time.Second))
				} else {
					PrintToTTY("[%d] ⏳ AROControlPlane.Ready: %s\n", iteration, status)
				}
			}
		} else {
			PrintToTTY("[%d] ✅ AROControlPlane.Ready: true\n", iteration)
		}

		// Check MachinePool and AROMachinePool status
		if !machinePoolReady {
			// Query each field separately to avoid jsonpath field-collapse bug
			mpPhase, mpPhaseErr := RunCommandQuiet(t, "kubectl", "--context", context, "get",
				"machinepool", machinePoolName, "-n", config.WorkloadClusterNamespace,
				"-o", "jsonpath={.status.phase}")
			mpReady, _ := RunCommandQuiet(t, "kubectl", "--context", context, "get",
				"machinepool", machinePoolName, "-n", config.WorkloadClusterNamespace,
				"-o", "jsonpath={.status.ready}")
			mpReplicas, _ := RunCommandQuiet(t, "kubectl", "--context", context, "get",
				"machinepool", machinePoolName, "-n", config.WorkloadClusterNamespace,
				"-o", "jsonpath={.status.replicas}")
			mpReadyReplicas, _ := RunCommandQuiet(t, "kubectl", "--context", context, "get",
				"machinepool", machinePoolName, "-n", config.WorkloadClusterNamespace,
				"-o", "jsonpath={.status.readyReplicas}")

			phase := strings.TrimSpace(mpPhase)
			ready := strings.TrimSpace(mpReady)
			replicas := strings.TrimSpace(mpReplicas)
			readyReplicas := strings.TrimSpace(mpReadyReplicas)

			// Check AROMachinePool status
			aroMPReady, _ := RunCommandQuiet(t, "kubectl", "--context", context, "get",
				"aromachinepool", machinePoolName, "-n", config.WorkloadClusterNamespace,
				"-o", "jsonpath={.status.ready}")
			aroMPProvState, _ := RunCommandQuiet(t, "kubectl", "--context", context, "get",
				"aromachinepool", machinePoolName, "-n", config.WorkloadClusterNamespace,
				"-o", "jsonpath={.status.provisioningState}")

			aroReady := strings.TrimSpace(aroMPReady)
			aroProvState := strings.TrimSpace(aroMPProvState)

			if mpPhaseErr != nil {
				// MachinePool resource doesn't exist yet
				PrintToTTY("[%d] ⏳ MachinePool: not found yet\n", iteration)
			} else if ready == "true" || phase == "Running" || phase == "Provisioned" {
				machinePoolReady = true
				PrintToTTY("[%d] ✅ MachinePool: %s (replicas: %s/%s, took %v)\n",
					iteration, phase, readyReplicas, replicas, elapsed.Round(time.Second))
				t.Logf("MachinePool %s replicas=%s/%s (took %v)", phase, readyReplicas, replicas, elapsed.Round(time.Second))
			} else if phase != "" {
				PrintToTTY("[%d] ⏳ MachinePool: %s (replicas: %s/%s)\n",
					iteration, phase, readyReplicas, replicas)
			} else {
				PrintToTTY("[%d] ⏳ MachinePool: waiting for status\n", iteration)
			}

			// Display AROMachinePool status
			if aroReady != "" || aroProvState != "" {
				if aroReady == "true" {
					PrintToTTY("[%d] ✅ AROMachinePool: ready=%s provisioningState=%s\n", iteration, aroReady, aroProvState)
				} else {
					PrintToTTY("[%d] ⏳ AROMachinePool: ready=%s provisioningState=%s\n", iteration, aroReady, aroProvState)
				}
			}
		} else {
			PrintToTTY("[%d] ✅ MachinePool: ready\n", iteration)
		}

		// Both ready — done
		if controlPlaneReady && machinePoolReady {
			PrintToTTY("\n✅ Control plane and machine pool are ready! (took %v)\n\n", elapsed.Round(time.Second))
			t.Logf("Both AROControlPlane and MachinePool ready (took %v)", elapsed.Round(time.Second))

			// Display final AROControlPlane conditions
			finalCond, finalErr := RunCommandQuiet(t, "kubectl", "--context", context, "get",
				"arocontrolplane", aroControlPlaneName, "-n", config.WorkloadClusterNamespace, "-o", "jsonpath={.status.conditions}")
			if finalErr == nil && strings.TrimSpace(finalCond) != "" {
				PrintToTTY("📋 Final AROControlPlane conditions:\n")
				PrintToTTY("%s", FormatAROControlPlaneConditions(finalCond))
			}

			// Display final AROCluster infrastructure status
			finalInfra := GetInfrastructureResourceStatus(t, context, config.WorkloadClusterNamespace, provisionedClusterName)
			if finalInfra.TotalResources > 0 {
				ReportInfrastructureProgress(t, iteration, elapsed, time.Duration(0), finalInfra)
			}

			return
		}

		// Fetch and display AROControlPlane conditions for better visibility
		if !controlPlaneReady {
			conditionsOutput, condErr := RunCommandQuiet(t, "kubectl", "--context", context, "get",
				"arocontrolplane", aroControlPlaneName, "-n", config.WorkloadClusterNamespace, "-o", "jsonpath={.status.conditions}")
			if condErr == nil && strings.TrimSpace(conditionsOutput) != "" {
				PrintToTTY("[%d] 📋 AROControlPlane conditions:\n", iteration)
				PrintToTTY("%s", FormatAROControlPlaneConditions(conditionsOutput))
			}
		}

		// Fetch and display AROCluster infrastructure resource progress
		infraStatus := GetInfrastructureResourceStatus(t, context, config.WorkloadClusterNamespace, provisionedClusterName)
		if infraStatus.TotalResources > 0 {
			ReportInfrastructureProgress(t, iteration, elapsed, remaining, infraStatus)
		}

		// Report progress using helper function
		ReportProgress(t, iteration, elapsed, remaining, timeout)

		time.Sleep(pollInterval)
	}
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
	PrintToTTY("Cluster: %s | Namespace: %s\n\n", provisionedClusterName, config.WorkloadClusterNamespace)

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

		PrintToTTY("⏳ AROCluster.Ready: %s\n", strings.TrimSpace(output))
		time.Sleep(pollInterval)
	}
}

// TestDeployment_VerifyClusterProvisioned verifies cluster.status.initialization.provisioned becomes True.
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

	PrintToTTY("\n=== Waiting for Cluster.Initialization.Provisioned ===\n")
	PrintToTTY("Cluster: %s | Namespace: %s\n\n", provisionedClusterName, config.WorkloadClusterNamespace)

	for {
		elapsed := time.Since(startTime)
		if elapsed > timeout {
			t.Fatalf("Timeout after %v waiting for cluster.status.initialization.provisioned=true.\n"+
				"  kubectl --context %s -n %s get cluster %s -o yaml",
				elapsed.Round(time.Second), context, config.WorkloadClusterNamespace, provisionedClusterName)
			return
		}

		output, err := RunCommandQuiet(t, "kubectl", "--context", context, "-n", config.WorkloadClusterNamespace,
			"get", "cluster", provisionedClusterName, "-o", "jsonpath={.status.initialization.provisioned}")
		if err == nil && strings.TrimSpace(output) == "true" {
			PrintToTTY("✅ Cluster.Initialization.Provisioned is True (took %v)\n\n", elapsed.Round(time.Second))
			t.Logf("cluster.status.initialization.provisioned=true (took %v)", elapsed.Round(time.Second))
			return
		}

		PrintToTTY("⏳ Cluster.Initialization.Provisioned: %s\n", strings.TrimSpace(output))
		time.Sleep(pollInterval)
	}
}

// TestDeployment_VerifyClusterInfrastructureReady verifies CAPI Cluster InfrastructureReady condition becomes True.
// This follows Cluster.Initialization.Provisioned (step 10) in the deployment sequence.
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
	PrintToTTY("Cluster: %s | Namespace: %s\n\n", provisionedClusterName, config.WorkloadClusterNamespace)

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

		PrintToTTY("⏳ Cluster.InfrastructureReady: %s\n", strings.TrimSpace(output))
		time.Sleep(pollInterval)
	}
}
