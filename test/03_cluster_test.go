package test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestExternalCluster_01_Connectivity validates the external cluster is reachable.
// This test runs only when USE_KUBECONFIG is set, validating pre-installed controllers.
func TestExternalCluster_01_Connectivity(t *testing.T) {
	config := NewTestConfig()

	if !config.IsExternalCluster() {
		t.Skip("Not using external cluster (USE_KUBECONFIG not set)")
	}

	PrintTestHeader(t, "TestExternalCluster_01_Connectivity",
		"Validate external cluster is reachable via kubeconfig")

	// Set KUBECONFIG for kubectl
	SetEnvVar(t, "KUBECONFIG", config.UseKubeconfig)
	context := config.GetKubeContext()

	PrintToTTY("\n=== Testing external cluster connectivity ===\n")
	PrintToTTY("Kubeconfig: %s\n", config.UseKubeconfig)
	PrintToTTY("Context: %s\n\n", context)

	output, err := RunCommand(t, "kubectl", "--context", context, "get", "nodes")
	if err != nil {
		PrintToTTY("❌ Failed to connect to external cluster: %v\n", err)
		t.Fatalf("Cannot connect to external cluster: %v", err)
	}

	PrintToTTY("✅ External cluster nodes:\n%s\n\n", output)
	t.Logf("External cluster nodes:\n%s", output)
}

// MCEComponentExpectation defines the expected state of an MCE component
type MCEComponentExpectation struct {
	Name            string
	ExpectedEnabled bool
}

// ExpectedMCEComponents lists all MCE components and their expected enabled states.
// These are the baseline components that should be configured before enabling CAPI/CAPZ.
var ExpectedMCEComponents = []MCEComponentExpectation{
	// Components that should be enabled
	{"local-cluster", true},
	{"assisted-service", true},
	{"cluster-lifecycle", true},
	{"cluster-manager", true},
	{"discovery", true},
	{"hive", true},
	{"server-foundation", true},
	{"cluster-proxy-addon", true},
	{"managedserviceaccount", true},
	// Components that should be disabled (HyperShift conflicts with CAPI)
	{"hypershift", false},
	{"hypershift-local-hosting", false},
}

// TestExternalCluster_01b_MCEBaselineStatus validates and configures MCE component baseline before enabling CAPI/CAPZ.
// This test ensures the cluster is in the expected state with HyperShift disabled
// (required for CAPI/CAPZ enablement due to MCE component exclusivity).
// Components not in the expected state are automatically corrected.
func TestExternalCluster_01b_MCEBaselineStatus(t *testing.T) {
	config := NewTestConfig()

	if !config.IsExternalCluster() {
		t.Skip("Not using external cluster (USE_KUBECONFIG not set)")
	}

	SetEnvVar(t, "KUBECONFIG", config.UseKubeconfig)
	context := config.GetKubeContext()

	// Check if MCE is installed
	if !IsMCECluster(t, context) {
		t.Skip("Not an MCE cluster, skipping MCE baseline validation")
	}

	PrintTestHeader(t, "TestExternalCluster_01b_MCEBaselineStatus",
		"Validate and configure MCE components baseline (HyperShift disabled, core components enabled)")

	PrintToTTY("\n=== Checking MCE component baseline status ===\n")
	PrintToTTY("%-35s %s\n", "COMPONENT", "STATUS")
	PrintToTTY("%s\n", strings.Repeat("-", 50))

	// Track components that need to be fixed
	type componentFix struct {
		name    string
		enabled bool // target state
	}
	var componentsToFix []componentFix
	var queryErrors []string

	for _, expected := range ExpectedMCEComponents {
		status, err := GetMCEComponentStatus(t, context, expected.Name)
		if err != nil {
			queryErrors = append(queryErrors, fmt.Sprintf("%s: %v", expected.Name, err))
			PrintToTTY("%-35s ⚠️  error: %v\n", expected.Name, err)
			continue
		}

		// Determine actual status display
		actualStatus := "disabled"
		if status.Enabled {
			actualStatus = "enabled"
		}

		// Check if it matches expected
		if status.Enabled == expected.ExpectedEnabled {
			PrintToTTY("%-35s ✅ %s\n", expected.Name, actualStatus)
		} else {
			expectedStatus := "disabled"
			if expected.ExpectedEnabled {
				expectedStatus = "enabled"
			}
			PrintToTTY("%-35s ⚠️  %s (need: %s)\n", expected.Name, actualStatus, expectedStatus)
			componentsToFix = append(componentsToFix, componentFix{
				name:    expected.Name,
				enabled: expected.ExpectedEnabled,
			})
		}
	}

	PrintToTTY("%s\n", strings.Repeat("-", 50))

	// Report query errors (non-fatal, continue with fixes)
	if len(queryErrors) > 0 {
		PrintToTTY("\n⚠️  Failed to query %d component(s):\n", len(queryErrors))
		for _, e := range queryErrors {
			PrintToTTY("   - %s\n", e)
		}
	}

	// Fix components that are not in the expected state
	if len(componentsToFix) > 0 {
		PrintToTTY("\n=== Configuring %d component(s) to expected state ===\n\n", len(componentsToFix))

		var fixErrors []string
		var fixedComponents []string

		for _, fix := range componentsToFix {
			if err := SetMCEComponentState(t, context, fix.name, fix.enabled); err != nil {
				fixErrors = append(fixErrors, fmt.Sprintf("%s: %v", fix.name, err))
				PrintToTTY("❌ Failed to configure %s: %v\n", fix.name, err)
			} else {
				action := "disabled"
				if fix.enabled {
					action = "enabled"
				}
				fixedComponents = append(fixedComponents, fmt.Sprintf("%s → %s", fix.name, action))
			}
		}

		// Report fix errors (fatal if any)
		if len(fixErrors) > 0 {
			PrintToTTY("\n❌ Failed to configure %d component(s):\n", len(fixErrors))
			for _, e := range fixErrors {
				PrintToTTY("   - %s\n", e)
			}
			t.Fatalf("Failed to configure MCE components: %v", fixErrors)
			return
		}

		// Report successful changes
		PrintToTTY("\n✅ Successfully configured %d component(s):\n", len(fixedComponents))
		for _, c := range fixedComponents {
			PrintToTTY("   - %s\n", c)
		}
		t.Logf("Configured MCE components: %v", fixedComponents)
	}

	PrintToTTY("\n✅ All MCE components are in expected baseline state\n\n")
	t.Log("MCE component baseline validation passed")
}

// TestExternalCluster_02_EnableMCE enables CAPI and CAPZ components if not already enabled.
// This test runs only when:
// - USE_KUBECONFIG is set (external cluster mode)
// - MCE is installed on the cluster
// - MCE_AUTO_ENABLE is true (default)
func TestExternalCluster_02_EnableMCE(t *testing.T) {
	config := NewTestConfig()

	if !config.IsExternalCluster() {
		t.Skip("Not using external cluster (USE_KUBECONFIG not set)")
	}

	SetEnvVar(t, "KUBECONFIG", config.UseKubeconfig)
	context := config.GetKubeContext()

	// Check if MCE is installed
	if !IsMCECluster(t, context) {
		t.Skip("Not an MCE cluster, skipping MCE component enablement")
	}

	// Check if auto-enablement is allowed
	if !config.MCEAutoEnable {
		t.Skip("MCE auto-enablement disabled (MCE_AUTO_ENABLE=false)")
	}

	PrintTestHeader(t, "TestExternalCluster_02_EnableMCE",
		"Enable CAPI and CAPZ components in MCE if not already enabled")

	PrintToTTY("\n=== Checking MCE component status ===\n")

	// Build MCE component list from CAPI core + all providers
	components := []string{MCEComponentCAPI}
	for _, p := range config.InfraProviders {
		if p.MCEComponentName != "" {
			components = append(components, p.MCEComponentName)
		}
	}
	enabledCount := 0
	needsEnablement := false

	for _, component := range components {
		status, err := GetMCEComponentStatus(t, context, component)
		if err != nil {
			t.Fatalf("Failed to get status for %s: %v", component, err)
		}

		if status.Enabled {
			PrintToTTY("✅ Component %s: already enabled\n", component)
			t.Logf("Component %s is already enabled", component)
			enabledCount++
			continue
		}

		PrintToTTY("⚠️  Component %s: disabled, will enable...\n", component)
		needsEnablement = true
		if err := EnableMCEComponent(t, context, component); err != nil {
			errStr := err.Error()

			// Check for HyperShift exclusivity error - common MCE constraint
			if strings.Contains(errStr, "component exclusivity violation") ||
				strings.Contains(errStr, "HyperShift") {
				PrintToTTY("\n❌ MCE Component Exclusivity Error\n")
				PrintToTTY("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
				PrintToTTY("HyperShift and Cluster API components cannot be enabled simultaneously.\n\n")
				PrintToTTY("To use CAPZ, you must first disable HyperShift components:\n")
				PrintToTTY("  kubectl patch mce multiclusterengine --type=merge -p '\n")
				PrintToTTY("    {\"spec\":{\"overrides\":{\"components\":[\n")
				PrintToTTY("      {\"name\":\"hypershift\",\"enabled\":false},\n")
				PrintToTTY("      {\"name\":\"hypershift-local-hosting\",\"enabled\":false}\n")
				PrintToTTY("    ]}}}'\n\n")
				PrintToTTY("Or use an MCE cluster without HyperShift enabled.\n")
				PrintToTTY("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")
				t.Fatalf("Cannot enable %s: MCE component exclusivity violation (HyperShift vs Cluster API)", component)
			}

			t.Fatalf("Failed to enable %s: %v\n\n"+
				"Troubleshooting steps:\n"+
				"  1. Verify MCE operator is healthy: kubectl get csv -n multicluster-engine\n"+
				"  2. Check MCE conditions: kubectl get mce multiclusterengine -o yaml\n"+
				"  3. Verify you have cluster-admin permissions\n"+
				"  4. Ensure jq is installed: jq --version\n", component, err)
		}
	}

	if enabledCount == len(components) {
		PrintToTTY("\n✅ All MCE components were already enabled\n\n")
		t.Log("All MCE components were already enabled")
		return
	}

	if needsEnablement {
		PrintToTTY("\n=== Waiting for MCE to reconcile components ===\n")
		PrintToTTY("Initial wait: 30 seconds for MCE to start deploying controllers...\n")
		time.Sleep(30 * time.Second)

		// Wait for controllers to become available (CAPI core + all provider controllers)
		for _, ctrl := range config.AllControllers() {
			if err := WaitForMCEController(t, context, ctrl.Namespace, ctrl.DeploymentName, config.MCEEnablementTimeout); err != nil {
				t.Errorf("Failed waiting for %s controller: %v\n\n"+
					"Troubleshooting steps:\n"+
					"  1. Check component status: kubectl get mce multiclusterengine -o json | jq '.spec.overrides.components'\n"+
					"  2. Check pod status: kubectl get pods -n %s\n"+
					"  3. Check MCE operator logs: kubectl logs -n multicluster-engine -l control-plane=backplane-operator --tail=50\n",
					ctrl.DisplayName, err, ctrl.Namespace)
			}
		}

		PrintToTTY("\n✅ MCE components enabled and controllers ready\n\n")
		t.Log("MCE components enabled and controllers are ready")
	}
}

// TestExternalCluster_03_ControllersReady validates CAPI/CAPZ/ASO controllers are installed.
// This test runs only when USE_KUBECONFIG is set, validating pre-installed controllers.
// If controllers are missing, it provides remediation hints based on whether this is an MCE cluster.
func TestExternalCluster_03_ControllersReady(t *testing.T) {
	config := NewTestConfig()

	if !config.IsExternalCluster() {
		t.Skip("Not using external cluster (USE_KUBECONFIG not set)")
	}

	PrintTestHeader(t, "TestExternalCluster_03_ControllersReady",
		"Validate CAPI/CAPZ/ASO controllers are installed on external cluster")

	// Set KUBECONFIG for kubectl
	SetEnvVar(t, "KUBECONFIG", config.UseKubeconfig)
	context := config.GetKubeContext()

	// Check if this is an MCE cluster for better error messages
	isMCE := IsMCECluster(t, context)

	PrintToTTY("\n=== Checking for pre-installed controllers ===\n")
	for _, ns := range config.AllNamespaces() {
		PrintToTTY("Namespace: %s\n", ns)
	}
	if isMCE {
		PrintToTTY("MCE Cluster: yes\n")
	}
	PrintToTTY("\n")

	allFound := true
	for _, ctrl := range config.AllControllers() {
		PrintToTTY("Checking %s controller manager...\n", ctrl.DisplayName)
		_, err := RunCommand(t, "kubectl", "--context", context, "-n", ctrl.Namespace,
			"get", "deployment", ctrl.DeploymentName)
		if err != nil {
			PrintToTTY("❌ %s controller not found in %s namespace\n", ctrl.DisplayName, ctrl.Namespace)
			allFound = false

			// Provide MCE-specific remediation hints
			if isMCE && !config.MCEAutoEnable {
				t.Errorf("%s controller not found in %s namespace.\n\n"+
					"This is an MCE cluster but MCE_AUTO_ENABLE=false.\n"+
					"To enable auto-enablement: MCE_AUTO_ENABLE=true make test-all\n"+
					"Or manually enable the component:\n"+
					"  kubectl patch mce multiclusterengine --type=merge -p '{\"spec\":{\"overrides\":{\"components\":[{\"name\":\"%s\",\"enabled\":true}]}}}'",
					ctrl.DisplayName, ctrl.Namespace, MCEComponentCAPI)
			} else {
				t.Errorf("%s controller not found in %s namespace: %v", ctrl.DisplayName, ctrl.Namespace, err)
			}
		} else {
			PrintToTTY("✅ %s controller manager found\n", ctrl.DisplayName)
			t.Logf("%s controller manager found in %s", ctrl.DisplayName, ctrl.Namespace)
		}
	}

	if allFound {
		PrintToTTY("\n✅ All required controllers are installed on external cluster\n\n")
	}
}

// TestKindCluster_KindClusterReady tests deploying a Kind cluster with CAPZ and verifies it's ready
func TestKindCluster_KindClusterReady(t *testing.T) {
	config := NewTestConfig()

	// Skip in external cluster mode - cluster is already provisioned
	if config.IsExternalCluster() {
		t.Skip("Using external cluster (USE_KUBECONFIG set), skipping Kind cluster deployment")
	}

	PrintTestHeader(t, "TestKindCluster_KindClusterReady",
		"Deploy Kind cluster with CAPI/CAPZ/ASO controllers (may take 5-10 minutes)")

	if !DirExists(config.RepoDir) {
		PrintToTTY("⚠️  Repository not cloned yet at %s\n", config.RepoDir)
		t.Skipf("Repository not cloned yet at %s", config.RepoDir)
	}

	// Check if cluster already exists
	PrintToTTY("\n=== Checking for existing Kind cluster ===\n")
	t.Log("Checking for existing Kind cluster")
	output, _ := RunCommand(t, "kind", "get", "clusters")
	clusterExists := strings.Contains(output, config.ManagementClusterName)

	if !clusterExists {
		PrintToTTY("Kind cluster '%s' not found - will deploy new cluster\n", config.ManagementClusterName)

		// Deploy Kind cluster and CAPI/CAPZ/ASO controllers using deploy-charts.sh
		// DO_INIT_KIND=true creates the Kind cluster and installs cert-manager
		// DO_DEPLOY=true deploys the specified charts
		deployScriptPath := filepath.Join(config.RepoDir, "scripts", "deploy-charts.sh")
		if !FileExists(deployScriptPath) {
			PrintToTTY("❌ Deployment script not found: %s\n", deployScriptPath)
			t.Errorf("Deployment script not found: %s", deployScriptPath)
			return
		}

		// Generate Kind config file for private registry access
		PrintToTTY("\n=== Generating Kind cluster configuration ===\n")
		kindConfigPath, err := GenerateKindConfig(t, config.RepoDir, config.ManagementClusterName)
		if err != nil {
			PrintToTTY("❌ Failed to generate Kind config: %v\n", err)
			t.Fatalf("Failed to generate Kind config: %v", err)
			return
		}
		if kindConfigPath != "" {
			PrintToTTY("✅ Kind config generated: %s\n", kindConfigPath)
		} else {
			PrintToTTY("⚠️  No Docker config found - Kind nodes will not have registry credentials\n")
			PrintToTTY("   Private image pulls (e.g., quay.io/acm-d/) may fail with ErrImagePull\n")
		}

		PrintToTTY("\n=== Deploying Kind cluster '%s' with CAPI/CAPZ/ASO controllers ===\n", config.ManagementClusterName)
		PrintToTTY("This will: create Kind cluster, install cert-manager, deploy controllers\n")
		PrintToTTY("Expected duration: 5-10 minutes\n")
		PrintToTTY("Output streaming below...\n\n")

		// Set environment variables for deploy-charts.sh
		// USE_KIND or USE_K8S should be set externally by the user
		// DO_INIT_KIND=true: Create Kind cluster (when USE_KIND=true)
		// DO_DEPLOY=true: Deploy the charts
		SetEnvVar(t, "KIND_CLUSTER_NAME", config.ManagementClusterName)
		SetEnvVar(t, "DO_INIT_KIND", "true")
		SetEnvVar(t, "DO_DEPLOY", "true")
		// Disable the script's built-in deployment check — it assumes all providers
		// share a namespace (capi-system), but charts may deploy to provider-specific
		// namespaces (e.g., capz-system). Our own tests validate controller readiness
		// with the correct namespace from InfraProvider config.
		SetEnvVar(t, "DO_CHECK", "false")
		// Format Go duration as a Helm-compatible duration string (e.g., "10m0s")
		SetEnvVar(t, "HELM_INSTALL_TIMEOUT", config.HelmInstallTimeout.String())
		// Pass generated Kind config to setup-kind-cluster.sh so it uses our
		// config with Docker credentials mounted for private registry access
		if kindConfigPath != "" {
			SetEnvVar(t, "KIND_CFG_NAME", kindConfigPath)
		}

		// Change to repository directory for script execution
		originalDir, err := os.Getwd()
		if err != nil {
			t.Fatalf("Failed to get current directory: %v", err)
		}
		defer func() {
			if err := os.Chdir(originalDir); err != nil {
				t.Logf("Warning: failed to change back to original directory: %v", err)
			}
		}()

		if err := os.Chdir(config.RepoDir); err != nil {
			t.Fatalf("Failed to change to repository directory: %v", err)
		}

		// Run the deployment script with chart arguments from provider config
		chartArgs := config.DeploymentChartArgs()
		scriptArgs := append([]string{deployScriptPath}, chartArgs...)
		t.Logf("Executing deployment script: %s %s", deployScriptPath, strings.Join(chartArgs, " "))
		t.Log("This will: deploy CAPI and infrastructure provider controllers to Kind cluster")
		output, err = RunCommandWithStreaming(t, "bash", scriptArgs...)
		if err != nil {
			PrintToTTY("\n❌ Failed to deploy controllers: %v\n", err)

			// Check for known provider errors
			if config.HasProvider("aro") {
				if azureErr := DetectAzureError(output); azureErr != nil {
					PrintToTTY("%s", FormatAzureError(azureErr))
					t.Logf("Azure error detected: %s", azureErr.ErrorType)
				}
			}

			t.Errorf("Failed to deploy controllers: %v\nOutput: %s", err, output)
			return
		}

		// Don't log full script output as it may contain sensitive Azure configuration
		PrintToTTY("\n✅ Kind cluster deployment script completed successfully\n\n")
		t.Log("Kind cluster deployment script completed successfully")

		// Ensure cloud credentials are available before patching secrets
		if config.HasProvider("aro") {
			PrintToTTY("=== Ensuring Azure credentials are available ===\n")
			if err := EnsureAzureCredentialsSet(t); err != nil {
				PrintToTTY("❌ Failed to ensure Azure credentials: %v\n", err)
				PrintToTTY("Please ensure you are logged into Azure CLI: az login\n\n")
				t.Fatalf("Azure credentials required for secret patching: %v", err)
				return
			}
			PrintToTTY("✅ Azure credentials available\n\n")
		}
		if config.HasProvider("rosa") {
			PrintToTTY("=== Ensuring AWS credentials are available ===\n")
			if err := EnsureAWSCredentialsSet(t); err != nil {
				PrintToTTY("❌ Failed to ensure AWS credentials: %v\n", err)
				t.Fatalf("AWS credentials required for secret patching: %v", err)
				return
			}
			PrintToTTY("✅ AWS credentials available\n\n")
		}

		// Patch provider-specific credentials after deployment
		if config.HasProvider("aro") {
			PrintToTTY("=== Patching ASO credentials secret ===\n")
			context := config.GetKubeContext()
			if err := PatchASOCredentialsSecret(t, context); err != nil {
				PrintToTTY("❌ Failed to patch ASO credentials: %v\n", err)
				t.Errorf("Failed to patch ASO credentials secret: %v", err)
				return
			}
			PrintToTTY("✅ ASO credentials secret patched successfully\n\n")
		}
	} else {
		PrintToTTY("✅ Kind cluster '%s' already exists\n\n", config.ManagementClusterName)
		t.Logf("Kind cluster '%s' already exists", config.ManagementClusterName)
	}

	// Verify cluster is accessible via kubectl
	PrintToTTY("=== Verifying cluster accessibility ===\n")
	t.Log("Verifying cluster accessibility...")

	// Set kubeconfig context
	SetEnvVar(t, "KUBECONFIG", fmt.Sprintf("%s/.kube/config", os.Getenv("HOME")))

	output, err := RunCommand(t, "kubectl", "--context", config.GetKubeContext(), "get", "nodes")
	if err != nil {
		PrintToTTY("❌ Failed to access Kind cluster nodes: %v\nOutput: %s\n\n", err, output)
		t.Errorf("Failed to access Kind cluster nodes: %v\nOutput: %s", err, output)
		return
	}

	PrintToTTY("✅ Kind cluster nodes:\n%s\n\n", output)
	PrintToTTY("✅ Kind cluster is ready\n\n")
	t.Logf("Kind cluster nodes:\n%s", output)
	t.Log("Kind cluster is ready")

	// Write deployment state file for cleanup to know what was actually deployed
	if err := WriteDeploymentState(config); err != nil {
		t.Logf("Warning: failed to write deployment state file: %v", err)
	} else {
		PrintToTTY("📝 Deployment state saved to %s\n", DeploymentStateFile)
		t.Logf("Deployment state saved to %s", DeploymentStateFile)
	}
}

// TestKindCluster_CAPINamespacesExists verifies controller namespaces are installed
func TestKindCluster_CAPINamespacesExists(t *testing.T) {
	PrintTestHeader(t, "TestKindCluster_CAPINamespacesExists",
		"Verify CAPI and infrastructure provider namespaces exist in the management cluster")

	config := NewTestConfig()

	// Set KUBECONFIG for external cluster mode
	if config.IsExternalCluster() {
		SetEnvVar(t, "KUBECONFIG", config.UseKubeconfig)
	}

	PrintToTTY("\n=== Checking for controller namespaces ===\n")
	t.Log("Checking for controller namespaces...")

	context := config.GetKubeContext()

	for _, ns := range config.AllNamespaces() {
		PrintToTTY("Checking namespace: %s...\n", ns)

		_, err := RunCommand(t, "kubectl", "--context", context, "get", "namespace", ns)
		if err != nil {
			PrintToTTY("⚠️  Namespace '%s' may not exist yet (this might be expected): %v\n", ns, err)
			t.Logf("Namespace '%s' may not exist yet (this might be expected): %v", ns, err)
		} else {
			PrintToTTY("✅ Found namespace: %s\n", ns)
			t.Logf("Found namespace: %s", ns)
		}
	}

	// Wait a bit for controllers to be ready
	PrintToTTY("\nWaiting 5 seconds for controllers to initialize...\n")
	time.Sleep(5 * time.Second)

	// Check for CAPI pods
	PrintToTTY("\n=== Checking for CAPI pods ===\n")
	PrintToTTY("Running: kubectl get pods -A --selector=cluster.x-k8s.io/provider\n")

	output, err := RunCommand(t, "kubectl", "--context", context, "get", "pods", "-A", "--selector=cluster.x-k8s.io/provider")
	if err != nil {
		PrintToTTY("⚠️  CAPI pods check failed: %v\nOutput: %s\n\n", err, output)
		t.Logf("CAPI pods check: %v\nOutput: %s", err, output)
	} else {
		PrintToTTY("✅ CAPI pods found:\n%s\n\n", output)
		t.Logf("CAPI pods:\n%s", output)
	}
}

// TestKindCluster_CAPIControllerReady waits for CAPI controller to be ready
func TestKindCluster_CAPIControllerReady(t *testing.T) {
	PrintTestHeader(t, "TestKindCluster_CAPIControllerReady",
		"Wait for CAPI controller manager deployment to become available (timeout: 10m)")

	config := NewTestConfig()

	// Set KUBECONFIG for external cluster mode
	if config.IsExternalCluster() {
		SetEnvVar(t, "KUBECONFIG", config.UseKubeconfig)
	}

	context := config.GetKubeContext()

	timeout := 10 * time.Minute
	pollInterval := 10 * time.Second
	startTime := time.Now()

	PrintToTTY("\n=== Waiting for CAPI controller manager ===\n")
	PrintToTTY("Namespace: %s\n", config.CAPINamespace)
	PrintToTTY("Deployment: %s\n", CAPIControllerDeployment)
	PrintToTTY("Timeout: %v | Poll interval: %v\n\n", timeout, pollInterval)

	iteration := 0
	for {
		elapsed := time.Since(startTime)
		remaining := timeout - elapsed

		if elapsed > timeout {
			PrintToTTY("\n❌ Timeout reached after %v\n\n", elapsed.Round(time.Second))
			t.Errorf("Timeout waiting for CAPI controller manager to be available after %v.\n\n"+
				"Troubleshooting steps:\n"+
				"  1. Check pod status: kubectl --context %s -n %s get pods\n"+
				"  2. Check pod logs: kubectl --context %s -n %s logs -l %s --tail=50\n"+
				"  3. Check pod events: kubectl --context %s -n %s describe deployment %s\n"+
				"  4. Verify Kind cluster is healthy: kind get clusters && kubectl --context %s get nodes\n\n"+
				"Common causes:\n"+
				"  - Image pull issues (check network connectivity)\n"+
				"  - Insufficient resources on Kind node\n"+
				"  - cert-manager not ready (controllers depend on it for webhooks)",
				elapsed.Round(time.Second),
				context, config.CAPINamespace,
				context, config.CAPINamespace, CAPIPodSelector,
				context, config.CAPINamespace, CAPIControllerDeployment,
				context)
			return
		}

		iteration++

		PrintToTTY("[%d] Checking deployment status...\n", iteration)

		output, err := RunCommand(t, "kubectl", "--context", context, "-n", config.CAPINamespace,
			"get", "deployment", CAPIControllerDeployment,
			"-o", "jsonpath={.status.conditions[?(@.type=='Available')].status}")

		if err != nil {
			PrintToTTY("[%d] ⚠️  Status check failed: %v\n", iteration, err)
		} else {
			status := strings.TrimSpace(output)
			PrintToTTY("[%d] 📊 Deployment Available status: %s\n", iteration, status)

			if status == "True" {
				PrintToTTY("\n✅ CAPI controller manager is available! (took %v)\n\n", elapsed.Round(time.Second))
				t.Log("CAPI controller manager deployment is available")

				// Also check mce-capi-webhook-config when not in Kind/K8S mode
				if os.Getenv("USE_KIND") != "true" && os.Getenv("USE_K8S") != "true" {
					PrintToTTY("Checking mce-capi-webhook-config deployment...\n")
					mceOutput, mceErr := RunCommand(t, "kubectl", "--context", context, "-n", config.CAPINamespace,
						"get", "deployment", "mce-capi-webhook-config",
						"-o", "jsonpath={.status.conditions[?(@.type=='Available')].status}")
					if mceErr != nil {
						PrintToTTY("⚠️  MCE webhook config check failed: %v\n", mceErr)
					} else if strings.TrimSpace(mceOutput) == "True" {
						PrintToTTY("✅ MCE webhook config is available\n\n")
					} else {
						PrintToTTY("⚠️  MCE webhook config not yet available\n\n")
					}
				}
				return
			}
		}

		ReportProgress(t, iteration, elapsed, remaining, timeout)

		time.Sleep(pollInterval)
	}
}

// TestKindCluster_InfraControllersReady waits for all infrastructure provider controllers to be ready.
// This iterates over all configured providers and validates each controller deployment.
func TestKindCluster_InfraControllersReady(t *testing.T) {
	config := NewTestConfig()

	// Set KUBECONFIG for external cluster mode
	if config.IsExternalCluster() {
		SetEnvVar(t, "KUBECONFIG", config.UseKubeconfig)
	}

	context := config.GetKubeContext()

	for _, provider := range config.InfraProviders {
		for _, ctrl := range provider.Controllers {
			t.Run(ctrl.DisplayName, func(t *testing.T) {
				timeout := ctrl.Timeout
				if timeout == 0 {
					timeout = DefaultControllerTimeout
				}
				pollInterval := 10 * time.Second
				startTime := time.Now()

				PrintToTTY("\n=== Waiting for %s controller manager ===\n", ctrl.DisplayName)
				PrintToTTY("Namespace: %s\n", ctrl.Namespace)
				PrintToTTY("Deployment: %s\n", ctrl.DeploymentName)
				PrintToTTY("Timeout: %v | Poll interval: %v\n\n", timeout, pollInterval)

				iteration := 0
				for {
					elapsed := time.Since(startTime)
					remaining := timeout - elapsed

					if elapsed > timeout {
						PrintToTTY("\n❌ Timeout reached after %v\n\n", elapsed.Round(time.Second))
						t.Errorf("Timeout waiting for %s controller manager to be available after %v.\n\n"+
							"Troubleshooting steps:\n"+
							"  1. Check pod status: kubectl --context %s -n %s get pods\n"+
							"  2. Check pod logs: kubectl --context %s -n %s logs -l %s --tail=50\n"+
							"  3. Check pod events: kubectl --context %s -n %s describe deployment %s\n"+
							"  4. Verify CAPI is ready first: kubectl --context %s -n %s get deployment %s\n\n"+
							"Common causes:\n"+
							"  - CAPI controller not ready yet (infrastructure providers depend on CAPI)\n"+
							"  - Credentials not configured\n"+
							"  - Image pull issues (check network connectivity)",
							ctrl.DisplayName, elapsed.Round(time.Second),
							context, ctrl.Namespace,
							context, ctrl.Namespace, ctrl.PodSelector,
							context, ctrl.Namespace, ctrl.DeploymentName,
							context, config.CAPINamespace, CAPIControllerDeployment)
						return
					}

					iteration++

					PrintToTTY("[%d] Checking deployment status...\n", iteration)

					output, err := RunCommand(t, "kubectl", "--context", context, "-n", ctrl.Namespace,
						"get", "deployment", ctrl.DeploymentName,
						"-o", "jsonpath={.status.conditions[?(@.type=='Available')].status}")

					if err != nil {
						PrintToTTY("[%d] ⚠️  Status check failed: %v\n", iteration, err)
					} else {
						status := strings.TrimSpace(output)
						PrintToTTY("[%d] 📊 Deployment Available status: %s\n", iteration, status)

						if status == "True" {
							PrintToTTY("\n✅ %s controller manager is available! (took %v)\n\n", ctrl.DisplayName, elapsed.Round(time.Second))
							t.Logf("%s controller manager deployment is available", ctrl.DisplayName)
							return
						}
					}

					ReportProgress(t, iteration, elapsed, remaining, timeout)

					time.Sleep(pollInterval)
				}
			})
		}
	}
}

// TestKindCluster_ProviderCredentialsConfigured validates that provider credential secrets
// are properly configured. Iterates over all providers that define a credential secret.
//
// This test runs BEFORE waiting for controllers to become available, providing fast failure
// and clear error messages if credentials are missing.
func TestKindCluster_ProviderCredentialsConfigured(t *testing.T) {
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

	PrintTestHeader(t, "TestKindCluster_ProviderCredentialsConfigured",
		"Validate provider credential secrets are configured")

	for _, provider := range config.InfraProviders {
		if provider.CredentialSecret == nil {
			continue
		}

		cred := provider.CredentialSecret

		t.Run(provider.Name, func(t *testing.T) {
			// Skip if required environment variables are not set
			for _, envVar := range cred.RequiredEnvVars {
				if os.Getenv(envVar) == "" {
					t.Skipf("Skipped: required env var %s not set", envVar)
				}
			}

			PrintToTTY("\n=== Validating %s credentials configuration ===\n", provider.Name)
			PrintToTTY("Namespace: %s\n", cred.Namespace)
			PrintToTTY("Secret: %s\n\n", cred.Name)

			// Check if secret exists
			PrintToTTY("Checking if %s secret exists...\n", cred.Name)
			_, err := RunCommandQuiet(t, "kubectl", "--context", context, "-n", cred.Namespace,
				"get", "secret", cred.Name)
			if err != nil {
				PrintToTTY("❌ Secret '%s' not found in %s namespace\n", cred.Name, cred.Namespace)
				PrintToTTY("\nThe deployment script did not create the credentials secret.\n")
				PrintToTTY("Please check that the deployment completed successfully.\n\n")
				t.Fatalf("%s secret not found: %v", cred.Name, err)
				return
			}
			PrintToTTY("✅ Secret exists\n\n")

			PrintToTTY("Checking credential fields in secret...\n")
			var missingFields []string

			for _, field := range cred.RequiredFields {
				output, err := RunCommandQuiet(t, "kubectl", "--context", context, "-n", cred.Namespace,
					"get", "secret", cred.Name,
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

// TestKindCluster_WebhooksReady waits for all admission webhooks to be responsive
func TestKindCluster_WebhooksReady(t *testing.T) {
	PrintTestHeader(t, "TestKindCluster_WebhooksReady",
		"Wait for CAPI/CAPZ/ASO/MCE webhooks to accept connections (timeout: 5m)")

	config := NewTestConfig()

	// Set KUBECONFIG for external cluster mode
	if config.IsExternalCluster() {
		SetEnvVar(t, "KUBECONFIG", config.UseKubeconfig)
	}

	context := config.GetKubeContext()

	// Build webhook list from CAPI core + all providers
	webhooks := config.AllWebhooks()

	// MCE webhook is only available in full MCE deployment, not in Kind/K8S mode
	if os.Getenv("USE_KIND") != "true" && os.Getenv("USE_K8S") != "true" {
		webhooks = append(webhooks, WebhookDef{
			DisplayName: "MCE",
			Namespace:   config.CAPINamespace,
			ServiceName: "mce-capi-webhook-config-service",
			Port:        9443,
		})
	}

	timeout := 5 * time.Minute
	pollInterval := 5 * time.Second

	PrintToTTY("\n=== Checking webhook readiness ===\n")
	PrintToTTY("Webhooks to verify: %d\n", len(webhooks))
	PrintToTTY("Timeout per webhook: %v | Poll interval: %v\n\n", timeout, pollInterval)

	for _, wh := range webhooks {
		startTime := time.Now()
		iteration := 0

		PrintToTTY("\n--- Checking %s webhook ---\n", wh.DisplayName)
		PrintToTTY("Service: %s.%s.svc:%d\n", wh.ServiceName, wh.Namespace, wh.Port)

		for {
			elapsed := time.Since(startTime)

			if elapsed > timeout {
				PrintToTTY("\n❌ Timeout waiting for %s webhook after %v\n", wh.DisplayName, elapsed.Round(time.Second))
				t.Errorf("Timeout waiting for %s webhook to be responsive after %v.\n\n"+
					"Troubleshooting steps:\n"+
					"  1. Check webhook service exists: kubectl --context %s -n %s get svc %s\n"+
					"  2. Check endpoint has addresses: kubectl --context %s -n %s get endpoints %s\n"+
					"  3. Check controller pod is running: kubectl --context %s -n %s get pods\n"+
					"  4. Check for certificate issues: kubectl --context %s get certificates -A\n\n"+
					"Common causes:\n"+
					"  - Controller manager pod not running or crashing\n"+
					"  - cert-manager hasn't issued webhook certificate yet\n"+
					"  - Service selector doesn't match pod labels",
					wh.DisplayName, elapsed.Round(time.Second),
					context, wh.Namespace, wh.ServiceName,
					context, wh.Namespace, wh.ServiceName,
					context, wh.Namespace,
					context)
				break
			}

			iteration++

			// First check if endpoint exists and has addresses
			endpointOutput, err := RunCommandQuiet(t, "kubectl", "--context", context,
				"get", "endpoints", wh.ServiceName, "-n", wh.Namespace,
				"-o", "jsonpath={.subsets[0].addresses[0].ip}")

			if err != nil || strings.TrimSpace(endpointOutput) == "" {
				PrintToTTY("[%d] ⏳ Waiting for %s endpoint to have addresses...\n", iteration, wh.DisplayName)
				time.Sleep(pollInterval)
				continue
			}

			endpointIP := strings.TrimSpace(endpointOutput)
			PrintToTTY("[%d] 📊 %s endpoint IP: %s\n", iteration, wh.DisplayName, endpointIP)

			// Endpoint addresses only contain pods that pass their readiness probe.
			// If an IP is present, the backing pod is Ready and the webhook is serving.
			PrintToTTY("[%d] ✅ %s webhook is ready (endpoint %s) - took %v\n",
				iteration, wh.DisplayName, endpointIP, elapsed.Round(time.Second))
			t.Logf("%s webhook is ready (endpoint %s)", wh.DisplayName, endpointIP)
			break
		}
	}

	PrintToTTY("\n=== Webhook readiness check complete ===\n\n")
	t.Log("All webhook readiness checks completed")
}
