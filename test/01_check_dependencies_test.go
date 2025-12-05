package test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
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

// TestCheckDependencies_AzureCLILogin_IsLoggedIn checks if Azure CLI is logged in
func TestCheckDependencies_AzureCLILogin_IsLoggedIn(t *testing.T) {
	output, err := RunCommand(t, "az", "account", "show")
	if err != nil {
		t.Errorf("Azure CLI not logged in. Please run 'az login': %v", err)
		return
	}

	t.Logf("Azure CLI is logged in\n%s", output)
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
