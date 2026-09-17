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

	if config.HCPResourceID == "" && config.AzureSubscriptionID == "" {
		if err := resolveHCPSubscriptionID(t, config); err != nil {
			return fmt.Errorf("cannot resolve Azure subscription ID: %w", err)
		}
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

func resolveHCPSubscriptionID(t *testing.T, config *TestConfig) error {
	t.Helper()

	if config.AzureSubscriptionID != "" {
		return nil
	}

	args := []string{"account", "show"}
	if config.AzureSubscriptionName != "" {
		args = append(args, "--subscription", config.AzureSubscriptionName)
	}
	args = append(args, "--query", "id", "-o", "tsv")
	output, err := RunCommandQuiet(t, "az", args...)
	if err != nil {
		return fmt.Errorf("Azure CLI account lookup failed: %w", err)
	}

	subscriptionID := strings.TrimSpace(output)
	if subscriptionID == "" {
		return fmt.Errorf("Azure CLI returned an empty subscription ID")
	}
	config.AzureSubscriptionID = subscriptionID
	return nil
}

func reportHCPARMCheckFailure(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Logf("HCP ARM state check warning: %v", err)
	}
}
