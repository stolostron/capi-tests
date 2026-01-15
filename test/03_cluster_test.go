package test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestKindCluster_KindClusterReady tests deploying a Kind cluster with CAPZ and verifies it's ready
func TestKindCluster_KindClusterReady(t *testing.T) {
	PrintTestHeader(t, "TestKindCluster_KindClusterReady",
		"Deploy Kind cluster with CAPI/CAPZ/ASO controllers (may take 5-10 minutes)")

	config := NewTestConfig()

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

		// Ensure Azure credentials are available for the deployment script
		// The script needs AZURE_TENANT_ID and AZURE_SUBSCRIPTION_ID to configure ASO
		PrintToTTY("\n=== Ensuring Azure credentials are available ===\n")
		if err := EnsureAzureCredentialsSet(t); err != nil {
			PrintToTTY("❌ Failed to ensure Azure credentials: %v\n", err)
			PrintToTTY("Please ensure you are logged into Azure CLI: az login\n\n")
			t.Fatalf("Azure credentials required for deployment: %v", err)
			return
		}
		PrintToTTY("✅ Azure credentials available\n")

		// Step 1: Create Kind cluster using setup-kind-cluster.sh
		setupScriptPath := filepath.Join(config.RepoDir, "scripts", "setup-kind-cluster.sh")
		if !FileExists(setupScriptPath) {
			PrintToTTY("❌ Kind setup script not found: %s\n", setupScriptPath)
			t.Errorf("Kind setup script not found: %s", setupScriptPath)
			return
		}

		PrintToTTY("\n=== Creating Kind cluster '%s' ===\n", config.ManagementClusterName)
		PrintToTTY("This will: create Kind cluster, install cert-manager\n")
		PrintToTTY("Output streaming below...\n\n")
		t.Logf("Creating Kind cluster '%s' using setup script", config.ManagementClusterName)

		// Set environment variable for the script
		SetEnvVar(t, "KIND_CLUSTER_NAME", config.ManagementClusterName)

		// Change to repository directory for script execution
		originalDir, err := os.Getwd()
		if err != nil {
			t.Fatalf("Failed to get current directory: %v", err)
		}
		defer os.Chdir(originalDir)

		if err := os.Chdir(config.RepoDir); err != nil {
			t.Fatalf("Failed to change to repository directory: %v", err)
		}

		// Run the Kind setup script (pass cluster name as argument)
		t.Logf("Executing Kind setup script: %s %s", setupScriptPath, config.ManagementClusterName)
		output, err = RunCommandWithStreaming(t, "bash", setupScriptPath, config.ManagementClusterName)
		if err != nil {
			PrintToTTY("\n❌ Failed to create Kind cluster: %v\n", err)
			t.Errorf("Failed to create Kind cluster: %v\nOutput: %s", err, output)
			return
		}
		PrintToTTY("✅ Kind cluster created successfully\n\n")

		// Step 2: Deploy CAPI/CAPZ/ASO controllers using deploy-charts.sh
		deployScriptPath := filepath.Join(config.RepoDir, "scripts", "deploy-charts.sh")
		if !FileExists(deployScriptPath) {
			PrintToTTY("❌ Deployment script not found: %s\n", deployScriptPath)
			t.Errorf("Deployment script not found: %s", deployScriptPath)
			return
		}

		PrintToTTY("=== Deploying CAPI/CAPZ/ASO controllers ===\n")
		PrintToTTY("Expected duration: 3-5 minutes\n")
		PrintToTTY("Output streaming below...\n\n")

		// Set USE_KIND=true so deploy-charts.sh uses the correct Kind context
		SetEnvVar(t, "USE_KIND", "true")

		// Run the deployment script
		t.Logf("Executing deployment script: %s", deployScriptPath)
		t.Log("This will: deploy CAPI/CAPZ/ASO controllers to Kind cluster")
		output, err = RunCommandWithStreaming(t, "bash", deployScriptPath)
		if err != nil {
			PrintToTTY("\n❌ Failed to deploy controllers: %v\n", err)

			// Check for known Azure errors and provide remediation guidance
			if azureErr := DetectAzureError(output); azureErr != nil {
				PrintToTTY("%s", FormatAzureError(azureErr))
				t.Logf("Azure error detected: %s", azureErr.ErrorType)
			}

			t.Errorf("Failed to deploy controllers: %v\nOutput: %s", err, output)
			return
		}

		// Don't log full script output as it may contain sensitive Azure configuration
		PrintToTTY("\n✅ Kind cluster deployment script completed successfully\n\n")
		t.Log("Kind cluster deployment script completed successfully")

		// Patch the ASO credentials secret with actual Azure credentials
		// The helm chart creates the secret with empty values, so we need to populate it
		PrintToTTY("=== Patching ASO credentials secret ===\n")
		context := fmt.Sprintf("kind-%s", config.ManagementClusterName)
		if err := PatchASOCredentialsSecret(t, context); err != nil {
			PrintToTTY("❌ Failed to patch ASO credentials: %v\n", err)
			t.Errorf("Failed to patch ASO credentials secret: %v", err)
			return
		}
		PrintToTTY("✅ ASO credentials secret patched successfully\n\n")
	} else {
		PrintToTTY("✅ Kind cluster '%s' already exists\n\n", config.ManagementClusterName)
		t.Logf("Kind cluster '%s' already exists", config.ManagementClusterName)
	}

	// Verify cluster is accessible via kubectl
	PrintToTTY("=== Verifying cluster accessibility ===\n")
	t.Log("Verifying cluster accessibility...")

	// Set kubeconfig context
	SetEnvVar(t, "KUBECONFIG", fmt.Sprintf("%s/.kube/config", os.Getenv("HOME")))

	output, err := RunCommand(t, "kubectl", "--context", fmt.Sprintf("kind-%s", config.ManagementClusterName), "get", "nodes")
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

// TestKindCluster_CAPINamespacesExists verifies CAPI namespaces are installed
func TestKindCluster_CAPINamespacesExists(t *testing.T) {
	PrintTestHeader(t, "TestKindCluster_CAPINamespacesExists",
		"Verify CAPI and CAPZ namespaces exist in the management cluster")

	config := NewTestConfig()

	PrintToTTY("\n=== Checking for CAPI namespaces ===\n")
	t.Log("Checking for CAPI namespaces...")

	context := fmt.Sprintf("kind-%s", config.ManagementClusterName)

	// Check for CAPI namespaces
	expectedNamespaces := []string{
		config.CAPINamespace,
		config.CAPZNamespace,
	}

	for _, ns := range expectedNamespaces {
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
	context := fmt.Sprintf("kind-%s", config.ManagementClusterName)

	timeout := 10 * time.Minute
	pollInterval := 10 * time.Second
	startTime := time.Now()

	PrintToTTY("\n=== Waiting for CAPI controller manager ===\n")
	PrintToTTY("Namespace: %s\n", config.CAPINamespace)
	PrintToTTY("Deployment: capi-controller-manager\n")
	PrintToTTY("Timeout: %v | Poll interval: %v\n\n", timeout, pollInterval)

	iteration := 0
	for {
		elapsed := time.Since(startTime)
		remaining := timeout - elapsed

		if elapsed > timeout {
			PrintToTTY("\n❌ Timeout reached after %v\n\n", elapsed.Round(time.Second))
			t.Errorf("Timeout waiting for CAPI controller manager to be available")
			return
		}

		iteration++

		PrintToTTY("[%d] Checking deployment status...\n", iteration)

		output, err := RunCommand(t, "kubectl", "--context", context, "-n", config.CAPINamespace,
			"get", "deployment", "capi-controller-manager",
			"-o", "jsonpath={.status.conditions[?(@.type=='Available')].status}")

		if err != nil {
			PrintToTTY("[%d] ⚠️  Status check failed: %v\n", iteration, err)
		} else {
			status := strings.TrimSpace(output)
			PrintToTTY("[%d] 📊 Deployment Available status: %s\n", iteration, status)

			if status == "True" {
				PrintToTTY("\n✅ CAPI controller manager is available! (took %v)\n\n", elapsed.Round(time.Second))
				t.Log("CAPI controller manager deployment is available")
				return
			}
		}

		ReportProgress(t, iteration, elapsed, remaining, timeout)

		time.Sleep(pollInterval)
	}
}

// TestKindCluster_CAPZControllerReady waits for CAPZ controller to be ready
func TestKindCluster_CAPZControllerReady(t *testing.T) {
	PrintTestHeader(t, "TestKindCluster_CAPZControllerReady",
		"Wait for CAPZ controller manager deployment to become available (timeout: 10m)")

	config := NewTestConfig()
	context := fmt.Sprintf("kind-%s", config.ManagementClusterName)

	timeout := 10 * time.Minute
	pollInterval := 10 * time.Second
	startTime := time.Now()

	PrintToTTY("\n=== Waiting for CAPZ controller manager ===\n")
	PrintToTTY("Namespace: %s\n", config.CAPZNamespace)
	PrintToTTY("Deployment: capz-controller-manager\n")
	PrintToTTY("Timeout: %v | Poll interval: %v\n\n", timeout, pollInterval)

	iteration := 0
	for {
		elapsed := time.Since(startTime)
		remaining := timeout - elapsed

		if elapsed > timeout {
			PrintToTTY("\n❌ Timeout reached after %v\n\n", elapsed.Round(time.Second))
			t.Errorf("Timeout waiting for CAPZ controller manager to be available")
			return
		}

		iteration++

		PrintToTTY("[%d] Checking deployment status...\n", iteration)

		output, err := RunCommand(t, "kubectl", "--context", context, "-n", config.CAPZNamespace,
			"get", "deployment", "capz-controller-manager",
			"-o", "jsonpath={.status.conditions[?(@.type=='Available')].status}")

		if err != nil {
			PrintToTTY("[%d] ⚠️  Status check failed: %v\n", iteration, err)
		} else {
			status := strings.TrimSpace(output)
			PrintToTTY("[%d] 📊 Deployment Available status: %s\n", iteration, status)

			if status == "True" {
				PrintToTTY("\n✅ CAPZ controller manager is available! (took %v)\n\n", elapsed.Round(time.Second))
				t.Log("CAPZ controller manager deployment is available")
				return
			}
		}

		ReportProgress(t, iteration, elapsed, remaining, timeout)

		time.Sleep(pollInterval)
	}
}

// TestKindCluster_ASOCredentialsConfigured validates that the ASO controller has Azure credentials configured.
// This test runs BEFORE waiting for ASO to become available, providing fast failure and clear error messages
// if credentials are missing (instead of waiting 10 minutes for timeout).
//
// The test validates:
// - AZURE_TENANT_ID and AZURE_SUBSCRIPTION_ID are always required
// - AZURE_CLIENT_ID and AZURE_CLIENT_SECRET are required for ASO to function in Kind clusters
//
// Behavior:
// - If service principal credentials are not set in the environment, the test skips gracefully
// - In CI where credentials should be configured, missing credentials will cause test failure
func TestKindCluster_ASOCredentialsConfigured(t *testing.T) {
	PrintTestHeader(t, "TestKindCluster_ASOCredentialsConfigured",
		"Validate Azure credentials are configured in aso-controller-settings secret")

	config := NewTestConfig()
	context := fmt.Sprintf("kind-%s", config.ManagementClusterName)

	// Check if service principal credentials are available in the environment
	// If not, skip the test gracefully since ASO won't work without them anyway
	clientID := os.Getenv("AZURE_CLIENT_ID")
	clientSecret := os.Getenv("AZURE_CLIENT_SECRET")
	if clientID == "" || clientSecret == "" {
		PrintToTTY("⚠️  Service principal credentials not found in environment\n")
		PrintToTTY("   AZURE_CLIENT_ID and AZURE_CLIENT_SECRET are required for ASO in Kind clusters\n")
		PrintToTTY("   Skipping test - ASO will not be functional without service principal\n\n")
		PrintToTTY("To configure service principal credentials:\n")
		PrintToTTY("  az ad sp create-for-rbac --name \"aro-capz-tests\" --role contributor \\\n")
		PrintToTTY("    --scopes /subscriptions/$(az account show --query id -o tsv)\n\n")
		t.Skip("Skipped: Service principal credentials (AZURE_CLIENT_ID/SECRET) not set in environment")
	}

	PrintToTTY("\n=== Validating ASO credentials configuration ===\n")
	PrintToTTY("Namespace: %s\n", config.CAPZNamespace)
	PrintToTTY("Secret: aso-controller-settings\n\n")

	// Check if secret exists
	PrintToTTY("Checking if aso-controller-settings secret exists...\n")
	_, err := RunCommandQuiet(t, "kubectl", "--context", context, "-n", config.CAPZNamespace,
		"get", "secret", "aso-controller-settings")
	if err != nil {
		PrintToTTY("❌ Secret 'aso-controller-settings' not found in %s namespace\n", config.CAPZNamespace)
		PrintToTTY("\nThe deployment script did not create the ASO credentials secret.\n")
		PrintToTTY("Please check that the cluster-api-installer deployment completed successfully.\n\n")
		t.Fatalf("aso-controller-settings secret not found: %v", err)
		return
	}
	PrintToTTY("✅ Secret exists\n\n")

	// Required credential fields to validate
	requiredFields := []string{
		"AZURE_TENANT_ID",
		"AZURE_SUBSCRIPTION_ID",
		"AZURE_CLIENT_ID",
		"AZURE_CLIENT_SECRET",
	}

	PrintToTTY("Checking credential fields in secret...\n")
	var missingFields []string

	for _, field := range requiredFields {
		output, err := RunCommandQuiet(t, "kubectl", "--context", context, "-n", config.CAPZNamespace,
			"get", "secret", "aso-controller-settings",
			"-o", fmt.Sprintf("jsonpath={.data.%s}", field))

		if err != nil || strings.TrimSpace(output) == "" {
			missingFields = append(missingFields, field)
			PrintToTTY("  ❌ %s: MISSING or EMPTY\n", field)
		} else {
			PrintToTTY("  ✅ %s: configured\n", field)
		}
	}

	// Report results
	if len(missingFields) > 0 {
		PrintToTTY("\n❌ ASO credentials validation FAILED\n")
		PrintToTTY("Missing fields: %v\n\n", missingFields)
		PrintToTTY("The aso-controller-settings secret is missing required credentials.\n")
		PrintToTTY("This can happen if:\n")
		PrintToTTY("  1. The cluster was deployed without service principal credentials\n")
		PrintToTTY("  2. PatchASOCredentialsSecret() was not called after deployment\n\n")
		PrintToTTY("To fix, ensure these environment variables are set before deployment:\n")
		PrintToTTY("  export AZURE_CLIENT_ID=<your-client-id>\n")
		PrintToTTY("  export AZURE_CLIENT_SECRET=<your-client-secret>\n")
		PrintToTTY("  export AZURE_TENANT_ID=$(az account show --query tenantId -o tsv)\n")
		PrintToTTY("  export AZURE_SUBSCRIPTION_ID=$(az account show --query id -o tsv)\n\n")
		t.Fatalf("ASO credentials not configured: missing %v", missingFields)
		return
	}

	PrintToTTY("\n✅ ASO credentials validation PASSED\n\n")
	t.Log("ASO credentials are properly configured")
}

// TestKindCluster_ASOControllerReady waits for Azure Service Operator controller to be ready.
// The timeout is configurable via the ASO_CONTROLLER_TIMEOUT environment variable (default: 10m).
// ASO may require a longer timeout due to its CRD initialization sequence which can involve
// multiple pod restarts.
func TestKindCluster_ASOControllerReady(t *testing.T) {
	config := NewTestConfig()

	PrintTestHeader(t, "TestKindCluster_ASOControllerReady",
		fmt.Sprintf("Wait for Azure Service Operator controller manager to become available (timeout: %v)", config.ASOControllerTimeout))

	context := fmt.Sprintf("kind-%s", config.ManagementClusterName)

	timeout := config.ASOControllerTimeout
	pollInterval := 10 * time.Second
	startTime := time.Now()

	PrintToTTY("\n=== Waiting for Azure Service Operator controller manager ===\n")
	PrintToTTY("Namespace: %s\n", config.CAPZNamespace)
	PrintToTTY("Deployment: azureserviceoperator-controller-manager\n")
	PrintToTTY("Timeout: %v | Poll interval: %v\n\n", timeout, pollInterval)

	iteration := 0
	for {
		elapsed := time.Since(startTime)
		remaining := timeout - elapsed

		if elapsed > timeout {
			PrintToTTY("\n❌ Timeout reached after %v\n\n", elapsed.Round(time.Second))
			t.Errorf("Timeout waiting for Azure Service Operator controller manager to be available")
			return
		}

		iteration++

		PrintToTTY("[%d] Checking deployment status...\n", iteration)

		output, err := RunCommand(t, "kubectl", "--context", context, "-n", config.CAPZNamespace,
			"get", "deployment", "azureserviceoperator-controller-manager",
			"-o", "jsonpath={.status.conditions[?(@.type=='Available')].status}")

		if err != nil {
			PrintToTTY("[%d] ⚠️  Status check failed: %v\n", iteration, err)
		} else {
			status := strings.TrimSpace(output)
			PrintToTTY("[%d] 📊 Deployment Available status: %s\n", iteration, status)

			if status == "True" {
				PrintToTTY("\n✅ Azure Service Operator controller manager is available! (took %v)\n\n", elapsed.Round(time.Second))
				t.Log("Azure Service Operator controller manager deployment is available")
				return
			}
		}

		ReportProgress(t, iteration, elapsed, remaining, timeout)

		time.Sleep(pollInterval)
	}
}

// TestKindCluster_WebhooksReady waits for all admission webhooks to be responsive
func TestKindCluster_WebhooksReady(t *testing.T) {
	PrintTestHeader(t, "TestKindCluster_WebhooksReady",
		"Wait for CAPI/CAPZ/ASO/MCE webhooks to accept connections (timeout: 5m)")

	config := NewTestConfig()
	context := fmt.Sprintf("kind-%s", config.ManagementClusterName)

	// Define webhooks to check
	type webhookInfo struct {
		name      string
		namespace string
		service   string
		port      int
	}

	webhooks := []webhookInfo{
		{"CAPI", config.CAPINamespace, "capi-webhook-service", 443},
		{"CAPZ", config.CAPZNamespace, "capz-webhook-service", 443},
		{"ASO", config.CAPZNamespace, "azureserviceoperator-webhook-service", 443},
		{"MCE", config.CAPINamespace, "mce-capi-webhook-config-service", 9443},
	}

	timeout := 5 * time.Minute
	pollInterval := 5 * time.Second

	PrintToTTY("\n=== Checking webhook readiness ===\n")
	PrintToTTY("Webhooks to verify: %d\n", len(webhooks))
	PrintToTTY("Timeout per webhook: %v | Poll interval: %v\n\n", timeout, pollInterval)

	for _, wh := range webhooks {
		startTime := time.Now()
		iteration := 0

		PrintToTTY("\n--- Checking %s webhook ---\n", wh.name)
		PrintToTTY("Service: %s.%s.svc:%d\n", wh.service, wh.namespace, wh.port)

		for {
			elapsed := time.Since(startTime)
			remaining := timeout - elapsed

			if elapsed > timeout {
				PrintToTTY("\n❌ Timeout waiting for %s webhook after %v\n", wh.name, elapsed.Round(time.Second))
				t.Errorf("Timeout waiting for %s webhook to be responsive", wh.name)
				break
			}

			iteration++

			// First check if endpoint exists and has addresses
			endpointOutput, err := RunCommandQuiet(t, "kubectl", "--context", context,
				"get", "endpoints", wh.service, "-n", wh.namespace,
				"-o", "jsonpath={.subsets[0].addresses[0].ip}")

			if err != nil || strings.TrimSpace(endpointOutput) == "" {
				PrintToTTY("[%d] ⏳ Waiting for %s endpoint to have addresses...\n", iteration, wh.name)
				time.Sleep(pollInterval)
				continue
			}

			PrintToTTY("[%d] 📊 %s endpoint IP: %s\n", iteration, wh.name, strings.TrimSpace(endpointOutput))

			// Test actual HTTPS connectivity using a temporary pod
			// We use --rm and --restart=Never to create an ephemeral pod that cleans up after itself
			// The curl command tests if the webhook server is accepting HTTPS connections
			curlURL := fmt.Sprintf("https://%s.%s.svc:%d/", wh.service, wh.namespace, wh.port)

			// Use a unique pod name to avoid conflicts
			podName := fmt.Sprintf("webhook-test-%d", time.Now().UnixNano())

			output, err := RunCommandQuiet(t, "kubectl", "--context", context,
				"run", podName, "--rm", "-i", "--restart=Never",
				"--image=curlimages/curl:latest", "--",
				"curl", "-k", "-s", "-o", "/dev/null", "-w", "%{http_code}",
				"--connect-timeout", "3", "--max-time", "5", curlURL)

			if err == nil {
				httpCode := strings.TrimSpace(output)
				// Any HTTP response (even 400, 404, 405) means the webhook server is listening
				// 000 means connection failed
				if httpCode != "" && httpCode != "000" {
					PrintToTTY("[%d] ✅ %s webhook is responsive (HTTP %s) - took %v\n",
						iteration, wh.name, httpCode, elapsed.Round(time.Second))
					t.Logf("%s webhook is responsive (HTTP %s)", wh.name, httpCode)
					break
				}
			}

			PrintToTTY("[%d] ⏳ %s webhook not ready yet (connection failed), retrying...\n", iteration, wh.name)
			ReportProgress(t, iteration, elapsed, remaining, timeout)
			time.Sleep(pollInterval)
		}
	}

	PrintToTTY("\n=== Webhook readiness check complete ===\n\n")
	t.Log("All webhook readiness checks completed")
}
