package test

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ClusterMonitorStatus represents the JSON output from monitor-cluster-json.sh
type ClusterMonitorStatus struct {
	Metadata struct {
		Timestamp   string `json:"timestamp"`
		Namespace   string `json:"namespace"`
		ClusterName string `json:"clusterName"`
	} `json:"metadata"`
	Cluster struct {
		Name                string        `json:"name"`
		Namespace           string        `json:"namespace"`
		Phase               string        `json:"phase"`
		InfrastructureReady interface{}   `json:"infrastructureReady"` // can be bool or null
		ControlPlaneReady   interface{}   `json:"controlPlaneReady"`   // can be bool or null
		Conditions          []interface{} `json:"conditions"`
	} `json:"cluster"`
	Infrastructure struct {
		Kind       string        `json:"kind"`
		Name       string        `json:"name"`
		Ready      interface{}   `json:"ready"` // can be bool or null
		Conditions []interface{} `json:"conditions"`
		Resources  []interface{} `json:"resources"`
	} `json:"infrastructure"`
	ControlPlane struct {
		Kind          string        `json:"kind"`
		Name          string        `json:"name"`
		Ready         interface{}   `json:"ready"` // can be bool or null
		Replicas      int           `json:"replicas"`
		ReadyReplicas int           `json:"readyReplicas"`
		State         *string       `json:"state"` // Control plane state (validating, installing, etc.)
		Conditions    []interface{} `json:"conditions"`
		Resources     []interface{} `json:"resources"`
	} `json:"controlPlane"`
	MachinePools []struct {
		Name              string        `json:"name"`
		Replicas          int           `json:"replicas"`
		ReadyReplicas     int           `json:"readyReplicas"`
		AvailableReplicas int           `json:"availableReplicas"`
		Conditions        []interface{} `json:"conditions"`
		Infrastructure    *struct {
			Kind              string        `json:"kind"`
			Name              string        `json:"name"`
			Ready             interface{}   `json:"ready"` // can be bool or null
			Replicas          int           `json:"replicas"`
			ProvisioningState string        `json:"provisioningState"`
			ProviderIDList    []string      `json:"providerIDList"`
			ProviderIDCount   int           `json:"providerIDCount"`
			Conditions        []interface{} `json:"conditions"`
			Resources         []interface{} `json:"resources"`
		} `json:"infrastructure"`
	} `json:"machinePools"`
	Nodes      interface{} `json:"nodes"`      // can be array or null
	NodesError *string     `json:"nodesError"` // error message when failing to connect to cluster
	Summary    struct {
		ClusterName         string      `json:"clusterName"`
		Namespace           string      `json:"namespace"`
		Phase               string      `json:"phase"`
		InfrastructureReady interface{} `json:"infrastructureReady"` // can be bool or null
		ControlPlaneReady   interface{} `json:"controlPlaneReady"`   // can be bool or null
		MachinePoolCount    int         `json:"machinePoolCount"`
		NodeCount           int         `json:"nodeCount"`
		Conditions          struct {
			Ready int `json:"ready"`
			Total int `json:"total"`
		} `json:"conditions"`
	} `json:"summary"`
}

// TestDeployment_00_CreateNamespace creates the workload cluster namespace before deploying resources.
// The namespace is unique per test run (prefix + timestamp) to allow parallel test runs
// and easy cleanup. This namespace is where CAPI CRs (Cluster, AROControlPlane, MachinePool)
// are deployed, which then create Azure resources.
func TestDeployment_00_CreateNamespace(t *testing.T) {
	// Check if config initialization failed
	if configError != nil {
		t.Fatalf("Configuration initialization failed: %s", *configError)
	}

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
// configurations (e.g., when CAPI_USER was changed without cleanup).
func TestDeployment_01_CheckExistingClusters(t *testing.T) {

	config := NewTestConfig()

	// Set KUBECONFIG for external cluster mode
	if config.IsExternalCluster() {
		SetEnvVar(t, "KUBECONFIG", config.UseKubeconfig)
	}

	context := config.GetKubeContext()

	PrintToTTY("\n=== Checking for existing Cluster resources ===\n")
	PrintToTTY("Namespace: %s\n", config.WorkloadClusterNamespace)
	PrintToTTY("Expected cluster name: %s\n\n", config.WorkloadClusterName)

	// Check for existing clusters that don't match current config
	mismatched, err := CheckForMismatchedClusters(t, context, config.WorkloadClusterNamespace, config.WorkloadClusterName)
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
			if name == config.WorkloadClusterName {
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
		errorMsg := FormatMismatchedClustersError(mismatched, config.WorkloadClusterName, config.WorkloadClusterNamespace)
		PrintToTTY("%s", errorMsg)

		t.Fatalf("Mismatched Cluster CRs found. Clean up existing clusters before deploying.\n"+
			"Found %d cluster(s) not matching expected name '%s': %v",
			len(mismatched), config.WorkloadClusterName, mismatched)
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

	// Get files to apply (provider-specific YAML files)
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
// TestDeployment_ApplyClusterYAMLs tests applying all cluster YAML files in order.
// This applies all files returned by GetExpectedFiles() which is provider-aware
// (ARO: credentials.yaml, aro.yaml | ROSA: secrets.yaml, is.yaml, rosa.yaml).
func TestDeployment_ApplyClusterYAMLs(t *testing.T) {
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

	context := config.GetKubeContext()

	// Verify cluster is healthy before applying resources
	// This addresses connection issues after long controller startup periods (issue #265)
	if err := WaitForClusterHealthy(t, context, DefaultHealthCheckTimeout); err != nil {
		t.Fatalf("Cluster health check failed: %v", err)
	}

	// Get all expected files for this provider (order matters!)
	expectedFiles := config.GetExpectedFiles()

	PrintToTTY("\n=== Applying Cluster YAML Files ===\n")
	PrintToTTY("Provider: %s\n", config.InfraProviderName)
	PrintToTTY("Files to apply: %v\n", expectedFiles)
	PrintToTTY("Output directory: %s\n", outputDir)
	PrintToTTY("Context: %s\n", context)
	PrintToTTY("Namespace: %s\n\n", config.WorkloadClusterNamespace)
	t.Logf("Applying %d YAML files for provider %s", len(expectedFiles), config.InfraProviderName)

	// Apply each file in order
	for i, file := range expectedFiles {
		filePath := filepath.Join(outputDir, file)

		if !FileExists(filePath) {
			PrintToTTY("❌ %s not found at %s\n\n", file, filePath)
			t.Fatalf("%s not found at %s.\n\n"+
				"This file should be generated by TestInfrastructure_GenerateResources.\n\n"+
				"To regenerate infrastructure files:\n"+
				"  go test -v ./test -run TestInfrastructure_GenerateResources",
				file, filePath)
		}

		PrintToTTY("[%d/%d] Applying %s...\n", i+1, len(expectedFiles), file)
		t.Logf("Applying %s (%d/%d)", file, i+1, len(expectedFiles))

		// Use ApplyWithRetry to handle transient connection issues
		if err := ApplyWithRetry(t, context, filePath, DefaultApplyMaxRetries); err != nil {
			PrintToTTY("❌ Failed to apply %s: %v\n\n", file, err)
			t.Fatalf("Failed to apply %s: %v", file, err)
		}

		PrintToTTY("✅ Successfully applied %s\n\n", file)
		t.Logf("Successfully applied %s", file)
	}

	PrintToTTY("✅ All %d YAML files applied successfully\n\n", len(expectedFiles))
	t.Logf("All %d YAML files applied successfully", len(expectedFiles))
}

// TestDeployment_ProviderCredentialsConfigured validates that provider credential secrets
// are properly configured after applying YAML files.
// Both ARO and ROSA use namespace-scoped credentials, so no controller restart is needed.
func TestDeployment_ProviderCredentialsConfigured(t *testing.T) {
	config := NewTestConfig()

	// Set KUBECONFIG for external cluster mode
	if config.IsExternalCluster() {
		SetEnvVar(t, "KUBECONFIG", config.UseKubeconfig)
	}

	context := config.GetKubeContext()

	// Check if any provider has credential secrets to validate
	hasCredentials := false
	for _, p := range config.InfraProviders {
		if p.CredentialSecret != nil {
			hasCredentials = true
			break
		}
	}
	if !hasCredentials {
		t.Skip("No provider credential secrets to validate")
	}

	PrintTestHeader(t, "TestDeployment_ProviderCredentialsConfigured",
		"Validate provider credential secrets are configured")

	for _, provider := range config.InfraProviders {
		if provider.CredentialSecret == nil {
			continue
		}

		cred := provider.CredentialSecret

		// Resolve dynamic placeholders in secret name and namespace
		secretName := strings.ReplaceAll(cred.Name, "{WORKLOAD_CLUSTER_NAME}", config.WorkloadClusterName)
		if err := ValidateRFC1123Name(secretName, "credential secret name"); err != nil {
			t.Fatalf("Invalid credential secret name after substitution: %v", err)
		}
		secretNamespace := strings.ReplaceAll(cred.Namespace, "{WORKLOAD_CLUSTER_NAMESPACE}", config.WorkloadClusterNamespace)
		secretNamespace = strings.ReplaceAll(secretNamespace, "{INFRA_PROVIDER_NAMESPACE}", provider.Controllers[0].Namespace)
		if err := ValidateRFC1123Name(secretNamespace, "credential secret namespace"); err != nil {
			t.Fatalf("Invalid credential secret namespace after substitution: %v", err)
		}

		t.Run(provider.Name, func(t *testing.T) {
			PrintToTTY("\n=== Validating %s credentials configuration ===\n", provider.Name)
			PrintToTTY("Namespace: %s\n", secretNamespace)
			PrintToTTY("Secret: %s\n\n", secretName)

			// Check if secret exists
			PrintToTTY("Checking if %s secret exists...\n", secretName)
			_, err := RunCommandQuiet(t, "kubectl", "--context", context, "-n", secretNamespace,
				"get", "secret", secretName)
			if err != nil {
				PrintToTTY("❌ Secret '%s' not found in %s namespace\n", secretName, secretNamespace)
				PrintToTTY("\nThe YAML generation did not create the credentials secret.\n")
				PrintToTTY("Please check that TestDeployment_ApplyClusterYAMLs completed successfully.\n\n")
				t.Fatalf("%s secret not found: %v", secretName, err)
				return
			}
			PrintToTTY("✅ Secret exists\n\n")

			PrintToTTY("Checking credential fields in secret...\n")
			var missingFields []string

			for _, field := range cred.RequiredFields {
				output, err := RunCommandQuiet(t, "kubectl", "--context", context, "-n", secretNamespace,
					"get", "secret", secretName,
					"-o", fmt.Sprintf("jsonpath={.data.%s}", field))

				if err != nil || strings.TrimSpace(output) == "" {
					missingFields = append(missingFields, field)
					PrintToTTY("  ❌ %s: MISSING or EMPTY\n", field)
				} else {
					PrintToTTY("  ✅ %s: configured\n", field)
				}
			}

			if len(missingFields) > 0 {
				PrintToTTY("\n❌ %s credentials validation FAILED\n", provider.Name)
				PrintToTTY("Missing fields: %v\n\n", missingFields)
				t.Fatalf("%s credentials not configured: missing %v", provider.Name, missingFields)
				return
			}

			PrintToTTY("\n✅ %s credentials validation PASSED\n\n", provider.Name)
			t.Logf("%s credentials are properly configured", provider.Name)
		})
	}
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

	context := config.GetKubeContext()

	// First, check if cluster resource exists
	// Use the provisioned cluster name from the cluster YAML, not WORKLOAD_CLUSTER_NAME
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
	controlPlaneName := config.GetProvisionedControlPlaneName()
	machinePoolName := config.GetProvisionedMachinePoolName()

	// Wait for both to be ready (with configurable timeout)
	timeout := config.DeploymentTimeout
	pollInterval := 30 * time.Second
	startTime := time.Now()

	// Get initial status to determine actual control plane kind for display
	monitorScript := "../scripts/monitor-cluster-json.sh"
	initialJSON, _ := RunCommandQuiet(t, monitorScript, "--context", context, config.WorkloadClusterNamespace, provisionedClusterName)
	var initialStatus ClusterMonitorStatus
	controlPlaneKind := "ControlPlane" // fallback if we can't determine
	if err := json.Unmarshal([]byte(initialJSON), &initialStatus); err == nil {
		controlPlaneKind = initialStatus.ControlPlane.Kind
		if initialStatus.ControlPlane.Name != "" {
			controlPlaneName = initialStatus.ControlPlane.Name
		}
	}

	// Print to stderr for immediate visibility (unbuffered)
	PrintToTTY("\n=== Waiting for control plane and machine pool to be ready ===\n")
	PrintToTTY("Cluster: %s\n", provisionedClusterName)
	PrintToTTY("%s: %s\n", controlPlaneKind, controlPlaneName)
	PrintToTTY("MachinePool: %s\n", machinePoolName)
	PrintToTTY("Namespace: %s\n", config.WorkloadClusterNamespace)
	PrintToTTY("Timeout: %v | Poll interval: %v\n\n", timeout, pollInterval)
	t.Logf("Waiting for control plane and machine pool (namespace: %s, timeout: %v)...", config.WorkloadClusterNamespace, timeout)

	controlPlaneReady := false
	machinePoolReady := false

	stallTimeout := config.DeploymentStallTimeout
	stallEnabled := stallTimeout > 0
	lastProgressTime := startTime
	lastProgress := stallProgressState{}
	if stallEnabled {
		if stallTimeout >= timeout {
			PrintToTTY("Stall detection: enabled (timeout: %v) — WARNING: stall timeout >= deployment timeout (%v), stall detection will never trigger\n\n",
				stallTimeout, timeout)
		} else {
			PrintToTTY("Stall detection: enabled (timeout: %v)\n\n", stallTimeout)
		}
	}

	iteration := 0
	for {
		elapsed := time.Since(startTime)
		remaining := timeout - elapsed

		if elapsed > timeout {
			PrintToTTY("\n❌ Timeout reached after %v\n\n", elapsed.Round(time.Second))

			// Dump diagnostics for not-ready infrastructure resources
			CollectAndDumpInfraDiagnostics(t, context, config.WorkloadClusterNamespace, provisionedClusterName)

			t.Errorf("Timeout waiting for deployment after %v.\n"+
				"  ControlPlane ready: %v\n"+
				"  MachinePool ready: %v\n\n"+
				"Troubleshooting steps:\n"+
				"  1. Check ControlPlane status: kubectl --context %s -n %s get %s %s -o yaml\n"+
				"  2. Check MachinePool status: kubectl --context %s -n %s get machinepool %s -o yaml\n"+
				"  3. Check cluster conditions: kubectl --context %s -n %s get cluster %s -o yaml\n"+
				"  4. Check controller logs: kubectl --context %s -n capz-system logs -l control-plane=controller-manager --tail=100\n\n"+
				"To increase timeout: export DEPLOYMENT_TIMEOUT=60m",
				elapsed.Round(time.Second),
				controlPlaneReady, machinePoolReady,
				context, config.WorkloadClusterNamespace, strings.ToLower(controlPlaneKind), controlPlaneName,
				context, config.WorkloadClusterNamespace, machinePoolName,
				context, config.WorkloadClusterNamespace, provisionedClusterName,
				context)
			return
		}

		iteration++

		PrintToTTY("[%d] Checking deployment status...\n", iteration)

		// Use monitor-cluster-json.sh to get status dynamically
		// Note: Script is in the capi-tests repository, not the cloned cluster-api-installer repo
		monitorScript := "../scripts/monitor-cluster-json.sh"
		jsonOutput, err := RunCommandQuiet(t, monitorScript, "--context", context, config.WorkloadClusterNamespace, provisionedClusterName)
		if err != nil {
			PrintToTTY("[%d] ⚠️  monitor-cluster-json.sh failed: %v\n", iteration, err)
			checkStallTimeout(t, stallEnabled, stallTimeout, lastProgressTime, lastProgress, context, config.WorkloadClusterNamespace, provisionedClusterName)
			time.Sleep(pollInterval)
			continue
		}

		// Parse JSON output
		var status ClusterMonitorStatus
		if err := json.Unmarshal([]byte(jsonOutput), &status); err != nil {
			PrintToTTY("[%d] ⚠️  Failed to parse monitor output: %v\n", iteration, err)
			checkStallTimeout(t, stallEnabled, stallTimeout, lastProgressTime, lastProgress, context, config.WorkloadClusterNamespace, provisionedClusterName)
			time.Sleep(pollInterval)
			continue
		}

		// Fail-fast: check Cluster.Phase for terminal failure
		if status.Cluster.Phase == ClusterPhaseFailed {
			PrintToTTY("\n❌ Cluster phase is Failed — aborting early\n\n")
			t.Fatalf("Cluster phase is 'Failed' — deployment cannot recover.\n\n"+
				"Check cluster status:\n"+
				"  kubectl --context %s -n %s get cluster %s -o yaml",
				context, config.WorkloadClusterNamespace, provisionedClusterName)
			return
		}

		// Fail-fast: check ControlPlane conditions for permanent failures
		if err := CheckConditionsForPermanentFailure(status.ControlPlane.Conditions); err != nil {
			PrintToTTY("\n❌ Permanent failure detected in %s conditions — aborting early\n", status.ControlPlane.Kind)
			PrintToTTY("   %v\n\n", err)
			t.Fatalf("Permanent failure in %s conditions — deployment cannot recover.\n%v\n\n"+
				"Check control plane status:\n"+
				"  kubectl --context %s -n %s get %s %s -o yaml",
				status.ControlPlane.Kind, err,
				context, config.WorkloadClusterNamespace, strings.ToLower(status.ControlPlane.Kind), controlPlaneName)
			return
		}

		// Fail-fast: check MachinePool infrastructure conditions for permanent failures
		for _, mp := range status.MachinePools {
			if mp.Infrastructure != nil {
				if err := CheckConditionsForPermanentFailure(mp.Infrastructure.Conditions); err != nil {
					PrintToTTY("\n❌ Permanent failure detected in %s conditions — aborting early\n", mp.Infrastructure.Kind)
					PrintToTTY("   %v\n\n", err)
					infraName := mp.Infrastructure.Name
					if infraName == "" {
						infraName = mp.Name
					}
					t.Fatalf("Permanent failure in %s conditions — deployment cannot recover.\n%v\n\n"+
						"Check machine pool status:\n"+
						"  kubectl --context %s -n %s get %s %s -o yaml",
						mp.Infrastructure.Kind, err,
						context, config.WorkloadClusterNamespace, strings.ToLower(mp.Infrastructure.Kind), infraName)
					return
				}
			}
		}

		// Check ControlPlane ready status (works for ARO/ROSA dynamically)
		if !controlPlaneReady {
			cpKind := status.ControlPlane.Kind
			cpReady := status.ControlPlane.Ready
			cpState := status.ControlPlane.State

			if cpReady == nil {
				if cpState != nil && *cpState != "" {
					PrintToTTY("[%d] ⏳ %s.Ready: null (state: %s)\n", iteration, cpKind, *cpState)
				} else {
					PrintToTTY("[%d] ⏳ %s.Ready: null\n", iteration, cpKind)
				}
			} else if cpReadyBool, ok := cpReady.(bool); ok && cpReadyBool {
				controlPlaneReady = true
				PrintToTTY("[%d] ✅ %s.Ready: true (took %v)\n", iteration, cpKind, elapsed.Round(time.Second))
				t.Logf("%s.Ready=true (took %v)", cpKind, elapsed.Round(time.Second))
			} else {
				if cpState != nil && *cpState != "" {
					PrintToTTY("[%d] ⏳ %s.Ready: %v (state: %s)\n", iteration, cpKind, cpReady, *cpState)
				} else {
					PrintToTTY("[%d] ⏳ %s.Ready: %v\n", iteration, cpKind, cpReady)
				}
			}
		} else {
			PrintToTTY("[%d] ✅ %s.Ready: true\n", iteration, status.ControlPlane.Kind)
		}

		// Check MachinePool status (only for providers that use them, like ARO)
		if !machinePoolReady {
			if len(status.MachinePools) == 0 {
				// No MachinePools (e.g., ROSA with embedded machine pool config)
				machinePoolReady = true
				PrintToTTY("[%d] ✅ MachinePool: not applicable (embedded in control plane)\n", iteration)
			} else {
				// ARO has MachinePools - display first one's status
				mp := status.MachinePools[0]

				// Check if MachinePool phase exists (derive from conditions or replicas)
				var phase string
				ready := false
				if mp.ReadyReplicas > 0 && mp.ReadyReplicas >= mp.Replicas {
					phase = "Running"
					ready = true
				} else if mp.Replicas > 0 {
					phase = "Provisioning"
				}

				if ready {
					machinePoolReady = true
					PrintToTTY("[%d] ✅ MachinePool: %s (replicas: %d/%d, took %v)\n",
						iteration, phase, mp.ReadyReplicas, mp.Replicas, elapsed.Round(time.Second))
					t.Logf("MachinePool %s replicas=%d/%d (took %v)", phase, mp.ReadyReplicas, mp.Replicas, elapsed.Round(time.Second))
				} else if phase != "" {
					PrintToTTY("[%d] ⏳ MachinePool: %s (replicas: %d/%d)\n",
						iteration, phase, mp.ReadyReplicas, mp.Replicas)
				} else {
					PrintToTTY("[%d] ⏳ MachinePool: not found yet\n", iteration)
				}

				// Display provider-specific MachinePool status (e.g., AROMachinePool, ROSAMachinePool)
				if mp.Infrastructure != nil {
					infraKind := mp.Infrastructure.Kind
					infraReady := mp.Infrastructure.Ready
					infraProvState := mp.Infrastructure.ProvisioningState

					// Build status line with only non-empty fields
					statusParts := []string{fmt.Sprintf("ready=%v", infraReady)}
					if infraProvState != "" {
						statusParts = append(statusParts, fmt.Sprintf("provisioningState=%s", infraProvState))
					}
					statusLine := strings.Join(statusParts, " ")

					if infraReady == true {
						PrintToTTY("[%d] ✅ %s: %s\n", iteration, infraKind, statusLine)
					} else {
						PrintToTTY("[%d] ⏳ %s: %s\n", iteration, infraKind, statusLine)
					}

					// Display infrastructure machine pool conditions for better visibility (AROMachinePool, ROSAMachinePool, etc.)
					if len(mp.Infrastructure.Conditions) > 0 {
						// Show all conditions when not ready, only non-True when ready
						if !ready {
							PrintToTTY("[%d] 📋 %s conditions:\n", iteration, infraKind)
							PrintToTTY("%s", FormatControlPlaneConditionsFromParsed(mp.Infrastructure.Conditions))
						} else {
							nonTrueConditions := FormatNonTrueConditionsFromParsed(mp.Infrastructure.Conditions)
							if strings.TrimSpace(nonTrueConditions) != "" {
								PrintToTTY("[%d] ⚠️  %s conditions (not True):\n", iteration, infraKind)
								PrintToTTY("%s", nonTrueConditions)
							}
						}
					}
				}
			}
		} else {
			if len(status.MachinePools) > 0 {
				PrintToTTY("[%d] ✅ MachinePool: ready\n", iteration)
			}
		}

		if stallEnabled {
			currentCPState := ""
			if status.ControlPlane.State != nil {
				currentCPState = *status.ControlPlane.State
			}
			currentMPReplicas := 0
			if len(status.MachinePools) > 0 {
				// Only the first MachinePool is tracked — ARO/ROSA use a single pool
				currentMPReplicas = status.MachinePools[0].ReadyReplicas
			}
			infraReady := 0
			infraStatus := GetInfrastructureResourceStatusFromParsed(status.Infrastructure.Resources, status.Infrastructure.Conditions)
			if infraStatus.TotalResources > 0 {
				infraReady = infraStatus.ReadyResources
			}

			current := stallProgressState{
				cpReady:            controlPlaneReady,
				cpState:            currentCPState,
				mpReadyReplicas:    currentMPReplicas,
				infraResourceReady: infraReady,
			}

			if current != lastProgress {
				lastProgressTime = time.Now()
				lastProgress = current
			}

			checkStallTimeout(t, stallEnabled, stallTimeout, lastProgressTime, lastProgress, context, config.WorkloadClusterNamespace, provisionedClusterName)
		}

		// Both ready — done
		if controlPlaneReady && machinePoolReady {
			cpKind := status.ControlPlane.Kind
			if len(status.MachinePools) > 0 {
				PrintToTTY("\n✅ Control plane and machine pool are ready! (took %v)\n\n", elapsed.Round(time.Second))
				t.Logf("Both %s and MachinePool ready (took %v)", cpKind, elapsed.Round(time.Second))
			} else {
				PrintToTTY("\n✅ Control plane is ready! (took %v)\n\n", elapsed.Round(time.Second))
				t.Logf("%s ready (took %v)", cpKind, elapsed.Round(time.Second))
			}

			// Display final ControlPlane conditions that are not "True" (using already-parsed data from monitor script)
			if len(status.ControlPlane.Conditions) > 0 {
				nonTrueConditions := FormatNonTrueConditionsFromParsed(status.ControlPlane.Conditions)
				if strings.TrimSpace(nonTrueConditions) != "" {
					PrintToTTY("⚠️  Final %s conditions (not True):\n", cpKind)
					PrintToTTY("%s", nonTrueConditions)
				}
			}

			// Display final infrastructure status (using already-parsed data from monitor script)
			finalInfra := GetInfrastructureResourceStatusFromParsed(status.Infrastructure.Resources, status.Infrastructure.Conditions)
			if finalInfra.TotalResources > 0 {
				ReportInfrastructureProgress(t, iteration, elapsed, time.Duration(0), finalInfra)
			}

			return
		}

		// Display control plane conditions (using already-parsed data from monitor script)
		if len(status.ControlPlane.Conditions) > 0 {
			if !controlPlaneReady {
				// Not ready yet: show all conditions
				PrintToTTY("[%d] 📋 %s conditions:\n", iteration, status.ControlPlane.Kind)
				PrintToTTY("%s", FormatControlPlaneConditionsFromParsed(status.ControlPlane.Conditions))
			} else {
				// Ready: show only non-True conditions to highlight any lingering issues
				nonTrueConditions := FormatNonTrueConditionsFromParsed(status.ControlPlane.Conditions)
				if strings.TrimSpace(nonTrueConditions) != "" {
					PrintToTTY("[%d] ⚠️  %s conditions (not True):\n", iteration, status.ControlPlane.Kind)
					PrintToTTY("%s", nonTrueConditions)
				}
			}
		}

		// Display infrastructure resource progress (using already-parsed data from monitor script)
		infraStatus := GetInfrastructureResourceStatusFromParsed(status.Infrastructure.Resources, status.Infrastructure.Conditions)
		if infraStatus.TotalResources > 0 {
			ReportInfrastructureProgress(t, iteration, elapsed, remaining, infraStatus)
		}

		// Report progress using helper function
		ReportProgress(t, iteration, elapsed, remaining, timeout)

		time.Sleep(pollInterval)
	}
}

// TestDeployment_WaitForExternalAuthReady waits for the ExternalAuthReady condition on the control plane
// to become True. ExternalAuth requires at least one ready machine pool, so this test must run after
// TestDeployment_WaitForControlPlane (which waits for both ControlPlane.Ready and MachinePool.Ready).
//
// This is ARO-specific: the AROControlPlane has an ExternalAuthReady condition that tracks whether
// the external authentication configuration (e.g., Azure AD integration) has been reconciled.
// ROSA handles authentication differently and does not use this condition.
//
// The ExternalAuthReady condition often shows as "ReconciliationFailed" with message
// "requires at least one ready machine pool" while machine pools are still provisioning.
// Once machine pools are ready, the controller reconciles ExternalAuth and the condition becomes True.
func TestDeployment_WaitForExternalAuthReady(t *testing.T) {
	config := NewTestConfig()

	// ExternalAuthReady is ARO-specific
	if !config.HasProvider("aro") {
		t.Skip("Skipping ARO-specific test (ExternalAuthReady condition is ARO-specific)")
	}

	if config.IsExternalCluster() {
		SetEnvVar(t, "KUBECONFIG", config.UseKubeconfig)
	}

	context := config.GetKubeContext()
	provisionedClusterName := config.GetProvisionedClusterName()

	timeout := 10 * time.Minute
	pollInterval := 15 * time.Second
	startTime := time.Now()

	PrintToTTY("\n=== Waiting for ExternalAuthReady ===\n")
	PrintToTTY("Cluster: %s | Namespace: %s\n", provisionedClusterName, config.WorkloadClusterNamespace)
	PrintToTTY("Timeout: %v | Poll interval: %v\n\n", timeout, pollInterval)
	t.Logf("Waiting for ExternalAuthReady (namespace: %s, timeout: %v)...", config.WorkloadClusterNamespace, timeout)

	for {
		elapsed := time.Since(startTime)
		if elapsed > timeout {
			PrintToTTY("\n❌ Timeout reached after %v waiting for ExternalAuthReady\n\n", elapsed.Round(time.Second))
			t.Fatalf("Timeout waiting for ExternalAuthReady after %v.\n\n"+
				"Check control plane conditions:\n"+
				"  kubectl --context %s -n %s get arocontrolplane -o yaml",
				elapsed.Round(time.Second), context, config.WorkloadClusterNamespace)
			return
		}

		data, err := MonitorCluster(t, context, config.WorkloadClusterNamespace, provisionedClusterName)
		if err != nil {
			PrintToTTY("⏳ Waiting for cluster data... (%v)\n", elapsed.Round(time.Second))
			time.Sleep(pollInterval)
			continue
		}

		// Fail-fast: check all control plane conditions for permanent failures
		if err := CheckK8sConditionsForPermanentFailure(data.ControlPlane.Conditions); err != nil {
			PrintToTTY("\n❌ Permanent failure detected in %s conditions — aborting early\n", data.ControlPlane.Kind)
			PrintToTTY("   %v\n\n", err)
			t.Fatalf("Permanent failure in %s conditions — deployment cannot recover.\n%v\n\n"+
				"Check control plane status:\n"+
				"  kubectl --context %s -n %s get %s -o yaml",
				data.ControlPlane.Kind, err,
				context, config.WorkloadClusterNamespace, strings.ToLower(data.ControlPlane.Kind))
			return
		}

		// Search for ExternalAuthReady in control plane conditions
		found := false
		for _, cond := range data.ControlPlane.Conditions {
			if cond.Type == "ExternalAuthReady" {
				found = true
				if cond.Status == "True" {
					PrintToTTY("✅ ExternalAuthReady is True (took %v)\n\n", elapsed.Round(time.Second))
					t.Logf("ExternalAuthReady=True (took %v)", elapsed.Round(time.Second))
					return
				}
				// Show status with reason/message for visibility
				detail := cond.Status
				if cond.Reason != "" {
					detail = fmt.Sprintf("%s (%s)", cond.Status, cond.Reason)
				}
				if cond.Message != "" {
					detail = fmt.Sprintf("%s - %s", detail, cond.Message)
				}
				PrintToTTY("⏳ ExternalAuthReady: %s (elapsed %v)\n", detail, elapsed.Round(time.Second))
			}
		}

		if !found {
			PrintToTTY("⏳ ExternalAuthReady condition not found yet (elapsed %v)\n", elapsed.Round(time.Second))
		}

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
//
// NOTE: This is ARO-specific. ROSA uses a managed service model where infrastructure
// is handled automatically - once ROSAControlPlane is ready, deployment can proceed.
func TestDeployment_VerifyInfrastructureResources(t *testing.T) {
	config := NewTestConfig()

	// Skip for non-ARO providers (NetworkInfrastructureReady and .status.resources[] are ARO-specific)
	if !config.HasProvider("aro") {
		t.Skip("Skipping ARO-specific test (NetworkInfrastructureReady condition and infrastructure resource tracking is ARO-specific)")
	}

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

			// Dump diagnostics for not-ready infrastructure resources
			CollectAndDumpInfraDiagnostics(t, context, config.WorkloadClusterNamespace, provisionedClusterName)

			t.Fatalf("Timeout waiting for NetworkInfrastructureReady after %v.\n\n"+
				"Check AROCluster status:\n"+
				"  kubectl --context %s -n %s get arocluster %s -o yaml",
				elapsed.Round(time.Second), context, config.WorkloadClusterNamespace, provisionedClusterName)
			return
		}

		iteration++

		// Use monitor-cluster-json.sh to get status
		jsonOutput, err := RunCommandQuiet(t, "../scripts/monitor-cluster-json.sh", "--context", context, config.WorkloadClusterNamespace, provisionedClusterName)
		if err != nil {
			PrintToTTY("[%d] ⚠️  monitor-cluster-json.sh failed: %v\n", iteration, err)
			time.Sleep(pollInterval)
			continue
		}

		var status ClusterMonitorStatus
		if err := json.Unmarshal([]byte(jsonOutput), &status); err != nil {
			PrintToTTY("[%d] ⚠️  Failed to parse monitor output: %v\n", iteration, err)
			time.Sleep(pollInterval)
			continue
		}

		// Fail-fast: check infrastructure conditions for permanent failures
		if err := CheckConditionsForPermanentFailure(status.Infrastructure.Conditions); err != nil {
			PrintToTTY("\n❌ Permanent failure detected in %s conditions — aborting early\n", status.Infrastructure.Kind)
			PrintToTTY("   %v\n\n", err)
			t.Fatalf("Permanent failure in %s conditions — deployment cannot recover.\n%v\n\n"+
				"Check infrastructure status:\n"+
				"  kubectl --context %s -n %s get %s %s -o yaml",
				status.Infrastructure.Kind, err,
				context, config.WorkloadClusterNamespace, strings.ToLower(status.Infrastructure.Kind), provisionedClusterName)
			return
		}

		// Get infrastructure status from already-parsed data
		infraStatus := GetInfrastructureResourceStatusFromParsed(status.Infrastructure.Resources, status.Infrastructure.Conditions)

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

	// Skip for non-ARO providers (AROCluster.Ready is ARO-specific)
	if !config.HasProvider("aro") {
		t.Skip("Skipping ARO-specific test (AROCluster resource is not used by this provider)")
	}

	if config.IsExternalCluster() {
		SetEnvVar(t, "KUBECONFIG", config.UseKubeconfig)
	}

	context := config.GetKubeContext()
	provisionedClusterName := config.GetProvisionedClusterName()

	timeout := 5 * time.Minute
	pollInterval := 10 * time.Second
	startTime := time.Now()

	// Get initial status to determine infrastructure kind
	initialData, err := MonitorCluster(t, context, config.WorkloadClusterNamespace, provisionedClusterName)
	infraKind := "Infrastructure" // fallback
	if err == nil && initialData.Infrastructure.Kind != "" {
		infraKind = initialData.Infrastructure.Kind
	}
	infraResourceType := strings.ToLower(infraKind) + "s"

	PrintToTTY("\n=== Waiting for %s.Ready ===\n", infraKind)
	PrintToTTY("Cluster: %s | Namespace: %s\n", provisionedClusterName, config.WorkloadClusterNamespace)
	PrintToTTY("Command: kubectl --context %s -n %s get %s %s -o jsonpath={.status.ready}\n\n",
		context, config.WorkloadClusterNamespace, infraResourceType, provisionedClusterName)

	for {
		elapsed := time.Since(startTime)
		if elapsed > timeout {
			// Dump diagnostics for not-ready infrastructure resources
			CollectAndDumpInfraDiagnostics(t, context, config.WorkloadClusterNamespace, provisionedClusterName)

			t.Fatalf("Timeout after %v waiting for %s.Ready=true.\n"+
				"  kubectl --context %s -n %s get %s %s -o yaml",
				elapsed.Round(time.Second), infraKind, context, config.WorkloadClusterNamespace, infraResourceType, provisionedClusterName)
			return
		}

		// Use monitoring script to get infrastructure status
		data, err := MonitorCluster(t, context, config.WorkloadClusterNamespace, provisionedClusterName)
		var ready bool
		var status string
		if err == nil && data.Infrastructure.Ready {
			ready = true
			status = "true"
		} else if err == nil {
			status = "false"
		} else {
			status = "<not set yet>"
		}

		if ready {
			PrintToTTY("✅ %s.Ready is True (took %v)\n\n", data.Infrastructure.Kind, elapsed.Round(time.Second))
			t.Logf("%s.Ready=true (took %v)", data.Infrastructure.Kind, elapsed.Round(time.Second))
			return
		}

		// Fail-fast: check infrastructure conditions for permanent failures
		if err == nil {
			if failErr := CheckK8sConditionsForPermanentFailure(data.Infrastructure.Conditions); failErr != nil {
				PrintToTTY("\n❌ Permanent failure detected in %s conditions — aborting early\n", data.Infrastructure.Kind)
				PrintToTTY("   %v\n\n", failErr)
				t.Fatalf("Permanent failure in %s conditions — deployment cannot recover.\n%v\n\n"+
					"Check infrastructure status:\n"+
					"  kubectl --context %s -n %s get %s %s -o yaml",
					data.Infrastructure.Kind, failErr,
					context, config.WorkloadClusterNamespace, infraResourceType, provisionedClusterName)
				return
			}
		}

		PrintToTTY("⏳ %s.Ready: %s (elapsed %v)\n", data.Infrastructure.Kind, status, elapsed.Round(time.Second))
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
			// Dump diagnostics for not-ready infrastructure resources
			CollectAndDumpInfraDiagnostics(t, context, config.WorkloadClusterNamespace, provisionedClusterName)

			t.Fatalf("Timeout after %v waiting for cluster.status.initialization.infrastructureProvisioned=true.\n"+
				"  kubectl --context %s -n %s get cluster %s -o yaml",
				elapsed.Round(time.Second), context, config.WorkloadClusterNamespace, provisionedClusterName)
			return
		}

		// Use monitoring script to get cluster infrastructure status
		data, err := MonitorCluster(t, context, config.WorkloadClusterNamespace, provisionedClusterName)
		var provisioned bool
		var status string
		if err == nil && data.Cluster.InfrastructureProvisioned {
			provisioned = true
			status = "true"
		} else if err == nil {
			status = "false"
		} else {
			status = "<not set yet>"
		}

		if provisioned {
			PrintToTTY("✅ Cluster.Initialization.InfrastructureProvisioned is True (took %v)\n\n", elapsed.Round(time.Second))
			t.Logf("cluster.status.initialization.infrastructureProvisioned=true (took %v)", elapsed.Round(time.Second))
			return
		}

		// Fail-fast: check cluster phase and conditions for permanent failures
		if err == nil {
			if data.Cluster.Phase == ClusterPhaseFailed {
				PrintToTTY("\n❌ Cluster phase is Failed — aborting early\n\n")
				t.Fatalf("Cluster phase is 'Failed' — deployment cannot recover.\n\n"+
					"Check cluster status:\n"+
					"  kubectl --context %s -n %s get cluster %s -o yaml",
					context, config.WorkloadClusterNamespace, provisionedClusterName)
				return
			}
			if failErr := CheckK8sConditionsForPermanentFailure(data.Cluster.Conditions); failErr != nil {
				PrintToTTY("\n❌ Permanent failure detected in Cluster conditions — aborting early\n")
				PrintToTTY("   %v\n\n", failErr)
				t.Fatalf("Permanent failure in Cluster conditions — deployment cannot recover.\n%v\n\n"+
					"Check cluster status:\n"+
					"  kubectl --context %s -n %s get cluster %s -o yaml",
					failErr, context, config.WorkloadClusterNamespace, provisionedClusterName)
				return
			}
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
			// Dump diagnostics for not-ready infrastructure resources
			CollectAndDumpInfraDiagnostics(t, context, config.WorkloadClusterNamespace, provisionedClusterName)

			t.Fatalf("Timeout after %v waiting for Cluster InfrastructureReady=True.\n"+
				"  kubectl --context %s -n %s get cluster %s -o yaml",
				elapsed.Round(time.Second), context, config.WorkloadClusterNamespace, provisionedClusterName)
			return
		}

		// Use monitoring script to get cluster infrastructure ready condition
		data, err := MonitorCluster(t, context, config.WorkloadClusterNamespace, provisionedClusterName)
		var ready bool
		var status string
		if err == nil && data.Summary.InfrastructureReady {
			ready = true
			status = "True"
		} else if err == nil {
			status = "False"
		} else {
			status = "<not set yet>"
		}

		if ready {
			PrintToTTY("✅ Cluster.InfrastructureReady is True (took %v)\n\n", elapsed.Round(time.Second))
			t.Logf("Cluster InfrastructureReady=True (took %v)", elapsed.Round(time.Second))
			return
		}

		// Fail-fast: check cluster phase and conditions for permanent failures
		if err == nil {
			if data.Cluster.Phase == ClusterPhaseFailed {
				PrintToTTY("\n❌ Cluster phase is Failed — aborting early\n\n")
				t.Fatalf("Cluster phase is 'Failed' — deployment cannot recover.\n\n"+
					"Check cluster status:\n"+
					"  kubectl --context %s -n %s get cluster %s -o yaml",
					context, config.WorkloadClusterNamespace, provisionedClusterName)
				return
			}
			if failErr := CheckK8sConditionsForPermanentFailure(data.Cluster.Conditions); failErr != nil {
				PrintToTTY("\n❌ Permanent failure detected in Cluster conditions — aborting early\n")
				PrintToTTY("   %v\n\n", failErr)
				t.Fatalf("Permanent failure in Cluster conditions — deployment cannot recover.\n%v\n\n"+
					"Check cluster status:\n"+
					"  kubectl --context %s -n %s get cluster %s -o yaml",
					failErr, context, config.WorkloadClusterNamespace, provisionedClusterName)
				return
			}
		}

		PrintToTTY("⏳ Cluster.InfrastructureReady: %s (elapsed %v)\n", status, elapsed.Round(time.Second))
		time.Sleep(pollInterval)
	}
}

// TestDeployment_TagAzureResources tags all Azure resources created by the deployment
// with ownership metadata for parallel run cleanup. Tags the resource group (ARM tags)
// and Azure AD Applications/Service Principals (Microsoft Graph tags).
// Non-fatal: failures are logged as warnings since tagging is for cleanup convenience only.
func TestDeployment_TagAzureResources(t *testing.T) {
	config := NewTestConfig()

	if len(config.AzureResourceTags) == 0 {
		t.Skipf("No Azure resource tags configured")
		return
	}

	if !CommandExists("az") {
		t.Skipf("Azure CLI not available, skipping resource tagging")
		return
	}

	if config.InfraProviderName != "aro" {
		t.Skipf("Azure resource tagging only applies to ARO provider")
		return
	}

	PrintToTTY("\n=== Tagging Azure Resources ===\n")

	PrintToTTY("Tagging resource group %s-resgroup...\n", config.ClusterNamePrefix)
	if err := TagAzureResourceGroup(t, config); err != nil {
		t.Errorf("Failed to tag resource group post-deployment (RG should exist): %v", err)
	}
	tagAzureADApplications(t, config)
	tagAzureServicePrincipals(t, config)

	PrintToTTY("✅ Azure resource tagging completed\n\n")
}

// tagAzureADApplications finds and tags Azure AD Applications matching our prefix.
// AD Application tags are string arrays (not key-value pairs), so we add strings
// in the format "key:value" (e.g., "capi-test-user:cate").
func tagAzureADApplications(t *testing.T, config *TestConfig) {
	t.Helper()

	prefix := config.ClusterNamePrefix

	// Find AD apps matching our prefix
	output, err := RunCommandQuiet(t, "az", "ad", "app", "list",
		"--only-show-errors",
		"--filter", fmt.Sprintf("startswith(displayName, '%s')", prefix),
		"--query", "[].{appId: appId, displayName: displayName}",
		"-o", "json")
	if err != nil {
		t.Logf("Warning: could not list AD Applications: %v", err)
		return
	}

	var apps []struct {
		AppID       string `json:"appId"`
		DisplayName string `json:"displayName"`
	}
	if err := json.Unmarshal([]byte(output), &apps); err != nil {
		t.Logf("Warning: failed to parse AD Applications JSON output: %v", err)
		return
	}
	if len(apps) == 0 {
		t.Log("No Azure AD Applications found to tag")
		return
	}

	// Build tag strings for AD apps (string array format: "key:value")
	tagStrings := sortedTagPairs(config.AzureResourceTags, ":")
	tagsJSON, err := toJSONArray(tagStrings)
	if err != nil {
		t.Logf("Warning: %v — skipping AD Application tagging", err)
		return
	}

	for _, app := range apps {
		args := []string{"ad", "app", "update", "--id", app.AppID, "--set", fmt.Sprintf("tags=%s", tagsJSON)}
		if _, err := RunCommandQuiet(t, "az", args...); err != nil {
			t.Logf("Warning: could not tag AD Application %s (%s): %v", app.DisplayName, app.AppID, err)
		} else {
			PrintToTTY("  Tagged AD Application: %s\n", app.DisplayName)
		}
	}
	t.Logf("Tagged %d Azure AD Application(s)", len(apps))
}

// tagAzureServicePrincipals finds and tags Service Principals matching our prefix.
// SP tags use the same string array format as AD Applications.
func tagAzureServicePrincipals(t *testing.T, config *TestConfig) {
	t.Helper()

	prefix := config.ClusterNamePrefix

	// Find SPs matching our prefix
	output, err := RunCommandQuiet(t, "az", "ad", "sp", "list",
		"--only-show-errors",
		"--filter", fmt.Sprintf("startswith(displayName, '%s')", prefix),
		"--query", "[].{id: id, displayName: displayName}",
		"-o", "json")
	if err != nil {
		t.Logf("Warning: could not list Service Principals: %v", err)
		return
	}

	var sps []struct {
		ID          string `json:"id"`
		DisplayName string `json:"displayName"`
	}
	if err := json.Unmarshal([]byte(output), &sps); err != nil {
		t.Logf("Warning: failed to parse Service Principals JSON output: %v", err)
		return
	}
	if len(sps) == 0 {
		t.Log("No Service Principals found to tag")
		return
	}

	// Build tag strings for SPs (string array format: "key:value")
	tagStrings := sortedTagPairs(config.AzureResourceTags, ":")
	tagsJSON, err := toJSONArray(tagStrings)
	if err != nil {
		t.Logf("Warning: %v — skipping Service Principal tagging", err)
		return
	}

	for _, sp := range sps {
		args := []string{"ad", "sp", "update", "--id", sp.ID, "--set", fmt.Sprintf("tags=%s", tagsJSON)}
		if _, err := RunCommandQuiet(t, "az", args...); err != nil {
			t.Logf("Warning: could not tag Service Principal %s (%s): %v", sp.DisplayName, sp.ID, err)
		} else {
			PrintToTTY("  Tagged Service Principal: %s\n", sp.DisplayName)
		}
	}
	t.Logf("Tagged %d Service Principal(s)", len(sps))
}

// toJSONArray converts a string slice to a JSON array string.
func toJSONArray(items []string) (string, error) {
	data, err := json.Marshal(items)
	if err != nil {
		return "", fmt.Errorf("failed to marshal tag array: %w", err)
	}
	return string(data), nil
}

// stallProgressState tracks deployment progress for stall detection.
// Uses only comparable types so Go's == operator works for change detection.
type stallProgressState struct {
	cpReady            bool
	cpState            string
	mpReadyReplicas    int
	infraResourceReady int
}

// checkStallTimeout fails the test if no deployment progress has been made within the stall timeout.
// Safe to call from error-recovery paths (e.g., monitor script failure) where status data is unavailable.
func checkStallTimeout(t *testing.T, stallEnabled bool, stallTimeout time.Duration, lastProgressTime time.Time, lastProgress stallProgressState, context, namespace, clusterName string) {
	t.Helper()
	if !stallEnabled {
		return
	}
	stallDuration := time.Since(lastProgressTime)
	if stallDuration <= stallTimeout {
		return
	}

	PrintToTTY("\n❌ Deployment stalled: no progress for %v\n", stallDuration.Round(time.Second))
	PrintToTTY("   Last state: ControlPlane.Ready=%v, State=%q, MachinePool.ReadyReplicas=%d, InfraResources=%d\n\n",
		lastProgress.cpReady, lastProgress.cpState, lastProgress.mpReadyReplicas, lastProgress.infraResourceReady)

	CollectAndDumpInfraDiagnostics(t, context, namespace, clusterName)

	t.Fatalf("Deployment stalled: no progress for %v (stall timeout: %v).\n"+
		"  ControlPlane ready: %v\n"+
		"  ControlPlane state: %s\n"+
		"  MachinePool ready replicas: %d\n"+
		"  Infrastructure resources ready: %d\n\n"+
		"This usually indicates an infrastructure-side issue (e.g., ARO HCP stuck in Reconciling).\n"+
		"Check the cloud provider's service health dashboard.\n\n"+
		"To increase stall timeout: export DEPLOYMENT_STALL_TIMEOUT=45m\n"+
		"To disable stall detection: export DEPLOYMENT_STALL_TIMEOUT=0",
		stallDuration.Round(time.Second), stallTimeout,
		lastProgress.cpReady, lastProgress.cpState, lastProgress.mpReadyReplicas,
		lastProgress.infraResourceReady)
}
