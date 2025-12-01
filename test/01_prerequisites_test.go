package test

import (
	"testing"
)

// TestPrerequisites_ToolsAvailable verifies all required tools are installed
func TestPrerequisites_ToolsAvailable(t *testing.T) {
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

// TestPrerequisites_AzureCLILogin checks if Azure CLI is logged in
func TestPrerequisites_AzureCLILogin(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Azure login check in short mode")
	}

	output, err := RunCommand(t, "az", "account", "show")
	if err != nil {
		t.Errorf("Azure CLI not logged in. Please run 'az login': %v", err)
		return
	}

	t.Logf("Azure CLI is logged in\n%s", output)
}

// TestPrerequisites_OpenShiftCLI verifies OpenShift CLI is functional
func TestPrerequisites_OpenShiftCLI(t *testing.T) {
	output, err := RunCommand(t, "oc", "version", "--client")
	if err != nil {
		t.Errorf("OpenShift CLI check failed: %v", err)
		return
	}

	t.Logf("OpenShift CLI version:\n%s", output)
}

// TestPrerequisites_HelmVersion verifies Helm is installed and functional
func TestPrerequisites_HelmVersion(t *testing.T) {
	output, err := RunCommand(t, "helm", "version", "--short")
	if err != nil {
		t.Errorf("Helm version check failed: %v", err)
		return
	}

	t.Logf("Helm version: %s", output)
}

// TestPrerequisites_KindVersion verifies Kind is installed
func TestPrerequisites_KindVersion(t *testing.T) {
	output, err := RunCommand(t, "kind", "version")
	if err != nil {
		t.Errorf("Kind version check failed: %v", err)
		return
	}

	t.Logf("Kind version: %s", output)
}
