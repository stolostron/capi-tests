package test

import (
	"fmt"
	"strings"
	"testing"
)

// RunHCPARMCheck invokes the ARM-state inspection script and logs its output.
// The error is returned to the caller so deployment failures remain visible.
func RunHCPARMCheck(t *testing.T, config *TestConfig) error {
	t.Helper()

	if config.InfraProviderName != "aro" {
		return nil
	}

	resourceID := config.BuildHCPResourceID()
	if resourceID == "" {
		return fmt.Errorf("cannot build HCP ARM resource ID: AZURE_SUBSCRIPTION_ID, resource group, or HCP name is missing")
	}

	output, err := RunCommand(t, config.CheckHCPScriptPath, resourceID)
	if strings.TrimSpace(output) != "" {
		PrintToTTY("\n=== HCP ARM state check output ===\n%s\n=== End HCP ARM state check ===\n\n", output)
		t.Logf("HCP ARM state check output:\n%s", output)
	}
	if err != nil {
		return fmt.Errorf("HCP ARM state check failed: %w", err)
	}
	return nil
}
