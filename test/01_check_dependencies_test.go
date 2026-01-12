package test

import (
	"encoding/json"
	"fmt"
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

// TestCheckDependencies_PythonVersion validates Python version compatibility.
// Python 3.14.0 has known incompatibilities with az cli and will fail fast.
// Python 3.14.2 is the tested and recommended version.
// Other versions will show a warning but allow tests to continue.
func TestCheckDependencies_PythonVersion(t *testing.T) {
	// Determine which Python command to use
	var pythonCmd string
	if CommandExists("python3") {
		pythonCmd = "python3"
	} else if CommandExists("python") {
		pythonCmd = "python"
	} else {
		t.Fatalf("Python is not installed or not in PATH.\n\n" +
			"Python is required for the cluster-api-installer scripts.\n" +
			"Tested version: Python 3.14.2\n\n" +
			"Installation options:\n" +
			"  - Using pyenv: pyenv install 3.14.2 && pyenv global 3.14.2\n" +
			"  - Using dnf (Fedora): sudo dnf install python3\n" +
			"  - Using apt (Ubuntu/Debian): sudo apt install python3")
		return
	}

	// Get Python version
	output, err := RunCommand(t, pythonCmd, "--version")
	if err != nil {
		t.Fatalf("Failed to get Python version: %v", err)
		return
	}

	// Parse version string (e.g., "Python 3.12.4" or "Python 3.13.0")
	versionStr := strings.TrimSpace(output)
	t.Logf("Detected: %s", versionStr)

	// Extract version numbers
	// Format: "Python X.Y.Z" or "Python X.Y"
	parts := strings.Fields(versionStr)
	if len(parts) < 2 {
		t.Fatalf("Could not parse Python version from: %s", versionStr)
		return
	}

	versionParts := strings.Split(parts[1], ".")
	if len(versionParts) < 2 {
		t.Fatalf("Could not parse Python version numbers from: %s", parts[1])
		return
	}

	// Parse major, minor, and patch version
	var major, minor, patch int
	_, err = Sscanf(versionParts[0], "%d", &major)
	if err != nil {
		t.Fatalf("Could not parse Python major version from: %s", versionParts[0])
		return
	}
	_, err = Sscanf(versionParts[1], "%d", &minor)
	if err != nil {
		t.Fatalf("Could not parse Python minor version from: %s", versionParts[1])
		return
	}
	// Parse patch version if present (default to 0)
	if len(versionParts) >= 3 {
		_, err = Sscanf(versionParts[2], "%d", &patch)
		if err != nil {
			// Non-fatal: treat as patch 0 if parsing fails
			patch = 0
		}
	}

	// Python 3.14.0 is known to have incompatibilities with az cli
	if major == 3 && minor == 14 && patch == 0 {
		t.Fatalf("Python 3.14.0 has known incompatibilities with az cli.\n\n"+
			"Detected: %s\n"+
			"Recommended: Python 3.14.2\n\n"+
			"To switch to Python 3.14.2:\n"+
			"  - Using pyenv: pyenv install 3.14.2 && pyenv global 3.14.2\n"+
			"  - Update your system Python to a newer patch version",
			versionStr)
		return
	}

	// Python 3.14.2 is the tested version - pass without warning
	if major == 3 && minor == 14 && patch == 2 {
		t.Logf("Python %d.%d.%d is the tested version", major, minor, patch)
		return
	}

	// All other versions - warn but allow to continue
	t.Logf("Warning: Python %d.%d.%d detected.\n"+
		"This version has not been tested. Tested version: Python 3.14.2\n"+
		"If you encounter issues, consider switching to Python 3.14.2.", major, minor, patch)
}

// Sscanf is a simple helper to parse a single integer from a string
func Sscanf(s string, format string, a ...interface{}) (int, error) {
	return fmt.Sscanf(s, format, a...)
}

// TestCheckDependencies_AzureAuthentication validates Azure authentication is available.
// Supports two authentication methods:
// 1. Service principal credentials (AZURE_CLIENT_ID, AZURE_CLIENT_SECRET, AZURE_TENANT_ID) - preferred for CI/automation
// 2. Azure CLI login (az login) - convenient for interactive development
//
// Service principal authentication is checked first. If service principal credentials are set,
// they are validated by performing an actual login. If not set, the test falls back to checking
// Azure CLI login status.
func TestCheckDependencies_AzureAuthentication(t *testing.T) {
	// Skip in CI environments where Azure login may not be available
	if os.Getenv("CI") == "true" || os.Getenv("GITHUB_ACTIONS") == "true" {
		t.Skip("Skipping Azure authentication check in CI environment")
		return
	}

	// Detect authentication mode
	authMode := DetectAzureAuthMode(t)

	switch authMode {
	case AzureAuthModeServicePrincipal:
		// Validate service principal credentials by performing login
		if err := ValidateServicePrincipalCredentials(t); err != nil {
			t.Errorf("Service principal authentication failed: %v", err)
			return
		}
		t.Log("Azure authentication via service principal is valid")

	case AzureAuthModeCLI:
		t.Log("Azure authentication via CLI is valid")

	case AzureAuthModeNone:
		t.Errorf("No Azure authentication available.\n\n" +
			"Please authenticate using one of these methods:\n\n" +
			"Option 1: Service principal (recommended for CI/automation)\n" +
			"  export AZURE_CLIENT_ID=<client-id>\n" +
			"  export AZURE_CLIENT_SECRET=<client-secret>\n" +
			"  export AZURE_TENANT_ID=<tenant-id>\n\n" +
			"Option 2: Azure CLI (convenient for development)\n" +
			"  az login\n\n" +
			"To create a service principal:\n" +
			"  az ad sp create-for-rbac --name <name> --role Contributor --scopes /subscriptions/<subscription-id>")
		return
	}
}

// TestCheckDependencies_AzureEnvironment validates required Azure environment variables.
// When using service principal authentication, AZURE_TENANT_ID is already required and set.
// When using Azure CLI, environment variables are auto-extracted if not set.
// This provides seamless UX for users who are logged in with Azure CLI.
func TestCheckDependencies_AzureEnvironment(t *testing.T) {
	// Skip in CI environments where Azure env vars may not be set
	if os.Getenv("CI") == "true" || os.Getenv("GITHUB_ACTIONS") == "true" {
		t.Skip("Skipping Azure environment validation in CI environment")
		return
	}

	// Track if any required variables are missing after auto-extraction attempt
	var missingVars []string

	// Check if using service principal authentication
	usingServicePrincipal := HasServicePrincipalCredentials()

	// Check AZURE_TENANT_ID - already set if using service principal, otherwise try to auto-extract
	t.Run("AZURE_TENANT_ID", func(t *testing.T) {
		if os.Getenv("AZURE_TENANT_ID") != "" {
			if usingServicePrincipal {
				t.Log("AZURE_TENANT_ID is set via service principal credentials")
			} else {
				t.Log("AZURE_TENANT_ID is set via environment variable")
			}
			return
		}

		// Try to extract from Azure CLI (only possible if not using SP auth)
		output, err := RunCommandQuiet(t, "az", "account", "show", "--query", "tenantId", "-o", "tsv")
		if err != nil {
			missingVars = append(missingVars, "AZURE_TENANT_ID")
			t.Errorf("AZURE_TENANT_ID is not set and could not be extracted from Azure CLI.\n\n"+
				"To fix this, either:\n"+
				"  Option 1 (Service Principal): export AZURE_TENANT_ID=<tenant-id>\n"+
				"  Option 2 (Azure CLI): export AZURE_TENANT_ID=$(az account show --query tenantId -o tsv)\n\n"+
				"Error: %v", err)
			return
		}

		tenantID := strings.TrimSpace(output)
		if tenantID == "" {
			missingVars = append(missingVars, "AZURE_TENANT_ID")
			t.Errorf("AZURE_TENANT_ID is not set and Azure CLI returned empty tenant ID.\n\n" +
				"To fix this, either:\n" +
				"  Option 1 (Service Principal): export AZURE_TENANT_ID=<tenant-id>\n" +
				"  Option 2 (Azure CLI): export AZURE_TENANT_ID=$(az account show --query tenantId -o tsv)")
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
				"To fix this, set one of:\n"+
				"  export AZURE_SUBSCRIPTION_ID=<subscription-id>\n"+
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
				"To fix this, set one of:\n" +
				"  export AZURE_SUBSCRIPTION_ID=<subscription-id>\n" +
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
//
// On macOS, clusterctl MUST be installed separately because the cluster-api-installer
// Makefile only downloads the linux-amd64 binary. This test fails on Mac when clusterctl
// is missing to prevent confusing deployment failures later.
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

	// clusterctl not in PATH - behavior depends on platform
	// On Mac: fail with prominent message (cluster-api-installer Makefile doesn't work on Mac)
	// On Linux: warn only (cluster-api-installer's bin directory may provide it)

	if runtime.GOOS == "darwin" {
		// On macOS, print a prominent warning to TTY and fail the test
		PrintToTTY("\n")
		PrintToTTY("================================================================================\n")
		PrintToTTY("  WARNING: clusterctl not found!\n")
		PrintToTTY("================================================================================\n")
		PrintToTTY("\n")
		PrintToTTY("  On macOS, you MUST install clusterctl manually.\n")
		PrintToTTY("  The cluster-api-installer Makefile only downloads linux-amd64 binaries.\n")
		PrintToTTY("\n")
		PrintToTTY("  Install clusterctl by running:\n")
		PrintToTTY("\n")
		PrintToTTY("    brew install clusterctl\n")
		PrintToTTY("\n")
		PrintToTTY("  Or manually download for your architecture:\n")
		PrintToTTY("\n")
		PrintToTTY("    # For Apple Silicon (M1/M2/M3):\n")
		PrintToTTY("    curl -L https://github.com/kubernetes-sigs/cluster-api/releases/latest/download/clusterctl-darwin-arm64 -o /usr/local/bin/clusterctl\n")
		PrintToTTY("    chmod +x /usr/local/bin/clusterctl\n")
		PrintToTTY("\n")
		PrintToTTY("    # For Intel Mac:\n")
		PrintToTTY("    curl -L https://github.com/kubernetes-sigs/cluster-api/releases/latest/download/clusterctl-darwin-amd64 -o /usr/local/bin/clusterctl\n")
		PrintToTTY("    chmod +x /usr/local/bin/clusterctl\n")
		PrintToTTY("\n")
		PrintToTTY("  clusterctl is required for:\n")
		PrintToTTY("    - Cluster monitoring (TestDeployment_MonitorCluster)\n")
		PrintToTTY("    - Kubeconfig retrieval (TestVerification_GetKubeconfig)\n")
		PrintToTTY("\n")
		PrintToTTY("================================================================================\n")
		PrintToTTY("\n")

		t.Fatalf("clusterctl is required on macOS but was not found.\n\n" +
			"Install with: brew install clusterctl\n\n" +
			"See the warning above for detailed instructions.")
		return
	}

	// On Linux/other platforms, warn but don't fail
	// cluster-api-installer's Makefile will download clusterctl to its bin directory
	var installInstructions string
	switch runtime.GOOS {
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

// TestCheckDependencies_NamingConstraints validates that cluster naming configuration
// is within Azure/ARO limits. This catches configuration errors early (in phase 1)
// rather than waiting for deployment failures during CR reconciliation.
func TestCheckDependencies_NamingConstraints(t *testing.T) {
	// Skip in CI environments where these env vars may not be configured
	if os.Getenv("CI") == "true" || os.Getenv("GITHUB_ACTIONS") == "true" {
		t.Skip("Skipping naming constraints validation in CI environment")
		return
	}

	config := NewTestConfig()

	// Validate domain prefix: ${CAPZ_USER}-${DEPLOYMENT_ENV} ≤ 15 chars
	t.Run("DomainPrefix", func(t *testing.T) {
		if err := ValidateDomainPrefix(config.User, config.Environment); err != nil {
			t.Errorf("Domain prefix validation failed:\n%v", err)
		} else {
			prefix := GetDomainPrefix(config.User, config.Environment)
			t.Logf("Domain prefix '%s' (%d chars) is valid (max: %d)",
				prefix, len(prefix), MaxDomainPrefixLength)
		}
	})

	// Validate ExternalAuth ID: ${CS_CLUSTER_NAME}-ea ≤ 15 chars
	t.Run("ExternalAuthID", func(t *testing.T) {
		if err := ValidateExternalAuthID(config.ClusterNamePrefix); err != nil {
			t.Errorf("ExternalAuth ID validation failed:\n%v", err)
		} else {
			externalAuthID := GetExternalAuthID(config.ClusterNamePrefix)
			t.Logf("ExternalAuth ID '%s' (%d chars) is valid (max: %d)",
				externalAuthID, len(externalAuthID), MaxExternalAuthIDLength)
		}
	})
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

// TestCheckDependencies_NamingCompliance validates that CAPZ_USER, CS_CLUSTER_NAME,
// and DEPLOYMENT_ENV are RFC 1123 compliant. This prevents late deployment failures
// where generated Kubernetes resource names contain invalid characters (e.g., uppercase).
//
// The RFC 1123 subdomain naming rules require:
// - Only lowercase alphanumeric characters and hyphens
// - Must start and end with an alphanumeric character
//
// Failing early in prerequisites saves significant time compared to waiting for
// deployment to fail in phase 5 (CR deployment).
func TestCheckDependencies_NamingCompliance(t *testing.T) {
	config := NewTestConfig()

	// Track validation failures
	var validationErrors []string

	// Validate CAPZ_USER
	t.Run("CAPZ_USER", func(t *testing.T) {
		if err := ValidateRFC1123Name(config.User, "CAPZ_USER"); err != nil {
			validationErrors = append(validationErrors, err.Error())
			t.Error(err)
		} else {
			t.Logf("CAPZ_USER '%s' is RFC 1123 compliant", config.User)
		}
	})

	// Validate DEPLOYMENT_ENV
	t.Run("DEPLOYMENT_ENV", func(t *testing.T) {
		if err := ValidateRFC1123Name(config.Environment, "DEPLOYMENT_ENV"); err != nil {
			validationErrors = append(validationErrors, err.Error())
			t.Error(err)
		} else {
			t.Logf("DEPLOYMENT_ENV '%s' is RFC 1123 compliant", config.Environment)
		}
	})

	// Validate CS_CLUSTER_NAME
	t.Run("CS_CLUSTER_NAME", func(t *testing.T) {
		if err := ValidateRFC1123Name(config.ClusterNamePrefix, "CS_CLUSTER_NAME"); err != nil {
			validationErrors = append(validationErrors, err.Error())
			t.Error(err)
		} else {
			t.Logf("CS_CLUSTER_NAME '%s' is RFC 1123 compliant", config.ClusterNamePrefix)
		}
	})

	// Validate TEST_NAMESPACE
	t.Run("TEST_NAMESPACE", func(t *testing.T) {
		if err := ValidateRFC1123Name(config.TestNamespace, "TEST_NAMESPACE"); err != nil {
			validationErrors = append(validationErrors, err.Error())
			t.Error(err)
		} else {
			t.Logf("TEST_NAMESPACE '%s' is RFC 1123 compliant", config.TestNamespace)
		}
	})

	// Print summary on cleanup
	t.Cleanup(func() {
		if len(validationErrors) > 0 {
			PrintToTTY("\n❌ RFC 1123 naming compliance validation failed!\n")
			PrintToTTY("Deployment will fail with 'Invalid value' errors during CR deployment (phase 5).\n")
			PrintToTTY("Fix the following before continuing:\n\n")
			for _, err := range validationErrors {
				PrintToTTY("%s\n\n", err)
			}
		}
	})
}
