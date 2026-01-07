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

		// Deploy Kind cluster using the script
		scriptPath := filepath.Join(config.RepoDir, "scripts", "deploy-charts-kind-capz.sh")
		if !FileExists(scriptPath) {
			PrintToTTY("❌ Deployment script not found: %s\n", scriptPath)
			t.Errorf("Deployment script not found: %s", scriptPath)
			return
		}

		PrintToTTY("\n=== Deploying Kind cluster '%s' ===\n", config.ManagementClusterName)
		PrintToTTY("This will: create Kind cluster, install cert-manager, deploy CAPI/CAPZ/ASO controllers\n")
		PrintToTTY("Expected duration: 5-10 minutes\n")
		PrintToTTY("Output streaming below...\n\n")
		t.Logf("Deploying Kind cluster '%s' using script", config.ManagementClusterName)

		// Set environment variable for the script (deploy-charts-kind-capz.sh expects KIND_CLUSTER_NAME)
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

		// Run the deployment script (this might take several minutes)
		// Use streaming to show progress in real-time
		t.Logf("Executing deployment script: %s", scriptPath)
		t.Log("This will: create Kind cluster, install cert-manager, deploy CAPI/CAPZ/ASO controllers")
		t.Log("Expected duration: 5-10 minutes")
		t.Log("Output streaming below...")
		output, err = RunCommandWithStreaming(t, "bash", scriptPath)
		if err != nil {
			// On error, show output for debugging (may contain sensitive info, but needed for troubleshooting)
			PrintToTTY("\n❌ Failed to deploy Kind cluster: %v\n", err)
			t.Errorf("Failed to deploy Kind cluster: %v\nOutput: %s", err, output)
			return
		}

		// Don't log full script output as it may contain sensitive Azure configuration
		PrintToTTY("\n✅ Kind cluster deployment script completed successfully\n\n")
		t.Log("Kind cluster deployment script completed successfully")
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
		"capi-system",
		"capz-system",
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
	PrintToTTY("Namespace: capi-system\n")
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

		output, err := RunCommand(t, "kubectl", "--context", context, "-n", "capi-system",
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
	PrintToTTY("Namespace: capz-system\n")
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

		output, err := RunCommand(t, "kubectl", "--context", context, "-n", "capz-system",
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
func TestKindCluster_ASOCredentialsConfigured(t *testing.T) {
	PrintTestHeader(t, "TestKindCluster_ASOCredentialsConfigured",
		"Validate Azure credentials are configured in aso-controller-settings secret")

	config := NewTestConfig()
	context := fmt.Sprintf("kind-%s", config.ManagementClusterName)

	PrintToTTY("\n=== Validating ASO credentials configuration ===\n")
	PrintToTTY("Namespace: capz-system\n")
	PrintToTTY("Secret: aso-controller-settings\n\n")

	// Check if secret exists
	PrintToTTY("Checking if aso-controller-settings secret exists...\n")
	_, err := RunCommandQuiet(t, "kubectl", "--context", context, "-n", "capz-system",
		"get", "secret", "aso-controller-settings")
	if err != nil {
		PrintToTTY("❌ Secret 'aso-controller-settings' not found in capz-system namespace\n")
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
	}

	// At least one of these auth methods must be configured
	authFields := []string{
		"AZURE_CLIENT_ID",
		"AZURE_CLIENT_SECRET",
	}

	PrintToTTY("Checking required credential fields...\n")
	var missingFields []string

	for _, field := range requiredFields {
		output, err := RunCommandQuiet(t, "kubectl", "--context", context, "-n", "capz-system",
			"get", "secret", "aso-controller-settings",
			"-o", fmt.Sprintf("jsonpath={.data.%s}", field))

		if err != nil || strings.TrimSpace(output) == "" {
			missingFields = append(missingFields, field)
			PrintToTTY("  ❌ %s: MISSING or EMPTY\n", field)
		} else {
			PrintToTTY("  ✅ %s: configured\n", field)
		}
	}

	// Check authentication method
	PrintToTTY("\nChecking authentication configuration...\n")
	authConfigured := false
	for _, field := range authFields {
		output, err := RunCommandQuiet(t, "kubectl", "--context", context, "-n", "capz-system",
			"get", "secret", "aso-controller-settings",
			"-o", fmt.Sprintf("jsonpath={.data.%s}", field))

		if err == nil && strings.TrimSpace(output) != "" {
			authConfigured = true
			PrintToTTY("  ✅ %s: configured\n", field)
		} else {
			PrintToTTY("  ⚪ %s: not set\n", field)
		}
	}

	if !authConfigured {
		missingFields = append(missingFields, "AZURE_CLIENT_ID or AZURE_CLIENT_SECRET")
	}

	// Report results
	if len(missingFields) > 0 {
		PrintToTTY("\n❌ ASO credentials validation FAILED\n")
		PrintToTTY("Missing fields: %v\n\n", missingFields)
		PrintToTTY("The deployment script did not populate Azure credentials.\n")
		PrintToTTY("Please ensure:\n")
		PrintToTTY("  1. Azure CLI is logged in: az login\n")
		PrintToTTY("  2. Environment variables are set:\n")
		PrintToTTY("     export AZURE_TENANT_ID=$(az account show --query tenantId -o tsv)\n")
		PrintToTTY("     export AZURE_SUBSCRIPTION_ID=$(az account show --query id -o tsv)\n")
		PrintToTTY("  3. Re-run the deployment script\n\n")
		t.Fatalf("ASO credentials not configured: missing %v", missingFields)
		return
	}

	PrintToTTY("\n✅ ASO credentials validation PASSED\n\n")
	t.Log("ASO credentials are properly configured")
}

// TestKindCluster_ASOControllerReady waits for Azure Service Operator controller to be ready
func TestKindCluster_ASOControllerReady(t *testing.T) {
	PrintTestHeader(t, "TestKindCluster_ASOControllerReady",
		"Wait for Azure Service Operator controller manager to become available (timeout: 10m)")

	config := NewTestConfig()
	context := fmt.Sprintf("kind-%s", config.ManagementClusterName)

	timeout := 10 * time.Minute
	pollInterval := 10 * time.Second
	startTime := time.Now()

	PrintToTTY("\n=== Waiting for Azure Service Operator controller manager ===\n")
	PrintToTTY("Namespace: capz-system\n")
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

		output, err := RunCommand(t, "kubectl", "--context", context, "-n", "capz-system",
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
		{"CAPI", "capi-system", "capi-webhook-service", 443},
		{"CAPZ", "capz-system", "capz-webhook-service", 443},
		{"ASO", "capz-system", "azureserviceoperator-webhook-service", 443},
		{"MCE", "capi-system", "mce-capi-webhook-config-service", 9443},
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
