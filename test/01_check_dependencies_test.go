package test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestCheckDependencies_ToolAvailable verifies all required tools are installed
func TestCheckDependencies_ToolAvailable(t *testing.T) {
	requiredTools := []string{
		"docker",
		"kind",
		"az",
		"oc",
		"helm",
		"git",
		"kubectl",
		"go",
	}

	for _, tool := range requiredTools {
		t.Run(tool, func(t *testing.T) {
			if !CommandExists(tool) {
				// Check alternative for docker (podman)
				if tool == "docker" && CommandExists("podman") {
					t.Logf("%s not found, but podman is available", tool)
					return
				}
				t.Errorf("Required tool '%s' is not installed or not in PATH", tool)
			} else {
				t.Logf("Tool '%s' is available", tool)
			}
		})
	}
}

// TestCheckDependencies_DockerDaemonRunning verifies the Docker daemon is running and accessible.
// This catches issues early before Kind Cluster tests fail with confusing errors.
// On macOS, provides instructions for starting Docker Desktop or Rancher Desktop.
func TestCheckDependencies_DockerDaemonRunning(t *testing.T) {
	// Skip if using podman instead of docker
	if !CommandExists("docker") {
		if CommandExists("podman") {
			t.Skip("Using podman instead of docker, skipping Docker daemon check")
			return
		}
		t.Skip("Docker not installed, skipping daemon check")
		return
	}

	// Skip in CI environments where Docker may not be available
	if os.Getenv("CI") == "true" || os.Getenv("GITHUB_ACTIONS") == "true" {
		t.Skip("Skipping Docker daemon check in CI environment")
		return
	}

	// Check if docker daemon is responding
	output, err := RunCommandQuiet(t, "docker", "info", "--format", "{{.ServerVersion}}")
	if err != nil {
		// Build platform-specific error message
		var helpMessage string
		switch runtime.GOOS {
		case "darwin":
			helpMessage = "\nTo start Docker on macOS, run one of:\n" +
				"  open -a 'Rancher Desktop'\n" +
				"  open -a 'Docker Desktop'\n" +
				"  open -a Docker\n\n" +
				"Then wait a few seconds for the daemon to start."
		case "linux":
			helpMessage = "\nTo start Docker on Linux, run:\n" +
				"  sudo systemctl start docker\n\n" +
				"Or check if the Docker socket exists:\n" +
				"  ls -la /var/run/docker.sock"
		default:
			helpMessage = "\nPlease start your Docker daemon and try again."
		}

		t.Fatalf("Docker daemon is not running or not accessible.\n%s\n\nError: %v", helpMessage, err)
		return
	}

	serverVersion := strings.TrimSpace(output)
	if serverVersion == "" {
		t.Log("Docker daemon is running (version unknown)")
	} else {
		t.Logf("Docker daemon is running, server version: %s", serverVersion)
	}
}

// TestCheckDependencies_AzureCLILogin_IsLoggedIn checks if Azure CLI is logged in
func TestCheckDependencies_AzureCLILogin_IsLoggedIn(t *testing.T) {
	// Skip in CI environments where Azure login is not available
	if os.Getenv("CI") == "true" || os.Getenv("GITHUB_ACTIONS") == "true" {
		t.Skip("Skipping Azure CLI login check in CI environment")
		return
	}

	_, err := RunCommand(t, "az", "account", "show")
	if err != nil {
		t.Errorf("Azure CLI not logged in. Please run 'az login': %v", err)
		return
	}

	// Successfully logged in - Don't log output as it contains sensitive information (tenant ID, subscription ID)
	t.Log("Azure CLI is logged in")
}

// TestCheckDependencies_AzureEnvironment validates required Azure environment variables.
// If environment variables are not set, it attempts to auto-extract them from Azure CLI
// (since TestCheckDependencies_AzureCLILogin_IsLoggedIn already verified login).
// This provides seamless UX for users who are logged in with Azure CLI.
func TestCheckDependencies_AzureEnvironment(t *testing.T) {
	// Skip in CI environments where Azure env vars may not be set
	if os.Getenv("CI") == "true" || os.Getenv("GITHUB_ACTIONS") == "true" {
		t.Skip("Skipping Azure environment validation in CI environment")
		return
	}

	// Track if any required variables are missing after auto-extraction attempt
	var missingVars []string

	// Check AZURE_TENANT_ID - try to auto-extract if not set
	t.Run("AZURE_TENANT_ID", func(t *testing.T) {
		if os.Getenv("AZURE_TENANT_ID") != "" {
			t.Log("AZURE_TENANT_ID is set via environment variable")
			return
		}

		// Try to extract from Azure CLI
		output, err := RunCommandQuiet(t, "az", "account", "show", "--query", "tenantId", "-o", "tsv")
		if err != nil {
			missingVars = append(missingVars, "AZURE_TENANT_ID")
			t.Errorf("AZURE_TENANT_ID is not set and could not be extracted from Azure CLI.\n\n"+
				"To fix this, run:\n"+
				"  export AZURE_TENANT_ID=$(az account show --query tenantId -o tsv)\n\n"+
				"Error: %v", err)
			return
		}

		tenantID := strings.TrimSpace(output)
		if tenantID == "" {
			missingVars = append(missingVars, "AZURE_TENANT_ID")
			t.Errorf("AZURE_TENANT_ID is not set and Azure CLI returned empty tenant ID.\n\n" +
				"To fix this, run:\n" +
				"  export AZURE_TENANT_ID=$(az account show --query tenantId -o tsv)")
			return
		}

		// Auto-set the environment variable for subsequent tests
		os.Setenv("AZURE_TENANT_ID", tenantID)
		t.Logf("AZURE_TENANT_ID auto-extracted from Azure CLI: %s...%s", tenantID[:8], tenantID[len(tenantID)-4:])
	})

	// Check AZURE_SUBSCRIPTION_ID or AZURE_SUBSCRIPTION_NAME - try to auto-extract if not set
	t.Run("AZURE_SUBSCRIPTION", func(t *testing.T) {
		subscriptionID := os.Getenv("AZURE_SUBSCRIPTION_ID")
		subscriptionName := os.Getenv("AZURE_SUBSCRIPTION_NAME")

		if subscriptionID != "" {
			t.Log("AZURE_SUBSCRIPTION_ID is set via environment variable")
			return
		}
		if subscriptionName != "" {
			t.Log("AZURE_SUBSCRIPTION_NAME is set via environment variable")
			return
		}

		// Try to extract subscription ID from Azure CLI
		output, err := RunCommandQuiet(t, "az", "account", "show", "--query", "id", "-o", "tsv")
		if err != nil {
			missingVars = append(missingVars, "AZURE_SUBSCRIPTION_ID or AZURE_SUBSCRIPTION_NAME")
			t.Errorf("Neither AZURE_SUBSCRIPTION_ID nor AZURE_SUBSCRIPTION_NAME is set, "+
				"and could not be extracted from Azure CLI.\n\n"+
				"To fix this, run one of:\n"+
				"  export AZURE_SUBSCRIPTION_ID=$(az account show --query id -o tsv)\n"+
				"  export AZURE_SUBSCRIPTION_NAME=$(az account show --query name -o tsv)\n\n"+
				"Error: %v", err)
			return
		}

		subID := strings.TrimSpace(output)
		if subID == "" {
			missingVars = append(missingVars, "AZURE_SUBSCRIPTION_ID or AZURE_SUBSCRIPTION_NAME")
			t.Errorf("Neither AZURE_SUBSCRIPTION_ID nor AZURE_SUBSCRIPTION_NAME is set, " +
				"and Azure CLI returned empty subscription ID.\n\n" +
				"To fix this, run one of:\n" +
				"  export AZURE_SUBSCRIPTION_ID=$(az account show --query id -o tsv)\n" +
				"  export AZURE_SUBSCRIPTION_NAME=$(az account show --query name -o tsv)")
			return
		}

		// Auto-set the environment variable for subsequent tests
		os.Setenv("AZURE_SUBSCRIPTION_ID", subID)
		t.Logf("AZURE_SUBSCRIPTION_ID auto-extracted from Azure CLI: %s...%s", subID[:8], subID[len(subID)-4:])
	})

	// If any required variables are missing, fail the overall test
	t.Cleanup(func() {
		if len(missingVars) > 0 {
			PrintToTTY("\n❌ Azure environment validation failed!\n")
			PrintToTTY("Missing: %v\n", missingVars)
			PrintToTTY("Run 'make test-all' only after setting required environment variables.\n\n")
		}
	})
}

// TestCheckDependencies_OpenShiftCLI_IsAvailable verifies OpenShift CLI is functional
func TestCheckDependencies_OpenShiftCLI_IsAvailable(t *testing.T) {
	output, err := RunCommand(t, "oc", "version", "--client")
	if err != nil {
		t.Errorf("OpenShift CLI check failed: %v", err)
		return
	}

	t.Logf("OpenShift CLI version:\n%s", output)
}

// TestCheckDependencies_Helm_IsAvailable verifies Helm is installed and functional
func TestCheckDependencies_Helm_IsAvailable(t *testing.T) {
	output, err := RunCommand(t, "helm", "version", "--short")
	if err != nil {
		t.Errorf("Helm version check failed: %v", err)
		return
	}

	t.Logf("Helm version: %s", output)
}

// TestCheckDependencies_Kind_IsAvailable verifies Kind is installed
func TestCheckDependencies_Kind_IsAvailable(t *testing.T) {
	output, err := RunCommand(t, "kind", "version")
	if err != nil {
		t.Errorf("Kind version check failed: %v", err)
		return
	}

	t.Logf("Kind version: %s", output)
}

// TestCheckDependencies_Clusterctl_IsAvailable checks if clusterctl is available.
// clusterctl is used in later test phases for cluster monitoring and kubeconfig retrieval.
// If not found in system PATH, it will be expected from cluster-api-installer's bin directory.
func TestCheckDependencies_Clusterctl_IsAvailable(t *testing.T) {
	if CommandExists("clusterctl") {
		output, err := RunCommand(t, "clusterctl", "version")
		if err != nil {
			t.Logf("clusterctl found but version check failed: %v", err)
			return
		}
		t.Logf("clusterctl version: %s", strings.TrimSpace(output))
		return
	}

	// clusterctl not in PATH - warn user but don't fail
	// It may be provided by cluster-api-installer's bin directory
	var installInstructions string
	switch runtime.GOOS {
	case "darwin":
		installInstructions = "To install clusterctl on macOS:\n" +
			"  brew install clusterctl\n\n" +
			"Or manually:\n" +
			"  curl -L https://github.com/kubernetes-sigs/cluster-api/releases/latest/download/clusterctl-darwin-arm64 -o /usr/local/bin/clusterctl\n" +
			"  chmod +x /usr/local/bin/clusterctl"
	case "linux":
		installInstructions = "To install clusterctl on Linux:\n" +
			"  curl -L https://github.com/kubernetes-sigs/cluster-api/releases/latest/download/clusterctl-linux-amd64 -o /usr/local/bin/clusterctl\n" +
			"  chmod +x /usr/local/bin/clusterctl"
	default:
		installInstructions = "To install clusterctl:\n" +
			"  See https://cluster-api.sigs.k8s.io/user/quick-start.html#install-clusterctl"
	}

	t.Logf("clusterctl not found in system PATH.\n\n"+
		"clusterctl is required for cluster monitoring (TestDeployment_MonitorCluster) and\n"+
		"kubeconfig retrieval (TestVerification_GetKubeconfig).\n\n"+
		"It will be looked for in cluster-api-installer's bin directory during test execution.\n"+
		"If not available there either, those tests will be skipped.\n\n"+
		"%s", installInstructions)
}

// TestCheckDependencies_DockerCredentialHelper checks that any Docker credential helpers
// configured in the Docker config file (credsStore or credHelpers) are available in PATH.
// Only runs on macOS, where missing credential helpers are a common issue with Docker Desktop alternatives.
func TestCheckDependencies_DockerCredentialHelper(t *testing.T) {
	// Only run on macOS where this is a common issue
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping Docker credential helper check (not macOS)")
		return
	}

	// Check if docker command exists
	if !CommandExists("docker") {
		if CommandExists("podman") {
			t.Skip("Using podman instead of docker")
			return
		}
		t.Skip("Docker not installed")
		return
	}

	// Determine Docker config directory (respect DOCKER_CONFIG env var)
	dockerConfigDir := os.Getenv("DOCKER_CONFIG")
	if dockerConfigDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			t.Logf("Could not determine home directory: %v", err)
			return
		}
		dockerConfigDir = filepath.Join(homeDir, ".docker")
	}

	configPath := filepath.Join(dockerConfigDir, "config.json")
	configData, err := os.ReadFile(configPath)
	if err != nil {
		// No config file is fine
		t.Logf("No Docker config file found at %s (this is OK)", configPath)
		return
	}

	// Parse Docker config
	var config struct {
		CredsStore  string            `json:"credsStore"`
		CredHelpers map[string]string `json:"credHelpers"`
	}

	if err := json.Unmarshal(configData, &config); err != nil {
		t.Logf("Could not parse Docker config: %v", err)
		return
	}

	// Check if credsStore is set
	if config.CredsStore != "" {
		t.Run("credsStore", func(t *testing.T) {
			helper := "docker-credential-" + config.CredsStore
			if !CommandExists(helper) {
				t.Errorf("Docker is configured to use credential helper '%s' but it's not in PATH\n"+
					"This will cause 'docker pull' commands to fail with:\n"+
					"  error getting credentials - err: exec: \"%s\": executable file not found in $PATH\n\n"+
					"To fix this issue, run:\n"+
					"  make fix-docker-config\n\n"+
					"Or manually remove the credsStore from %s",
					config.CredsStore, helper, configPath)
			} else {
				t.Logf("Docker credential helper '%s' is available", helper)
			}
		})
	}

	// Check credHelpers
	for registry, helper := range config.CredHelpers {
		registry := registry // capture range variable
		helper := helper
		t.Run(registry, func(t *testing.T) {
			helperBin := "docker-credential-" + helper
			if !CommandExists(helperBin) {
				t.Errorf("Docker is configured to use credential helper '%s' for registry '%s' but it's not in PATH\n"+
					"To fix this issue, run:\n"+
					"  make fix-docker-config",
					helper, registry)
			} else {
				t.Logf("Docker credential helper '%s' for registry '%s' is available", helper, registry)
			}
		})
	}
}
