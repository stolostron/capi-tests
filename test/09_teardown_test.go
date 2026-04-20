package test

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestTeardown_RevertMCEComponents reverts MCE components to their original states.
// This undoes changes made by TestExternalCluster_01b_MCEBaselineStatus and
// TestExternalCluster_02_EnsureMCEComponents, restoring the cluster to its pre-test state.
//
// Runs only when USE_KUBECONFIG is set and MCE is installed.
// Reads saved original states from the deployment state file.
func TestTeardown_RevertMCEComponents(t *testing.T) {
	if configError != nil {
		t.Fatalf("Configuration initialization failed: %s", *configError)
	}

	config := NewTestConfig()

	if !config.IsExternalCluster() {
		t.Skip("Not using external cluster (USE_KUBECONFIG not set)")
	}

	SetEnvVar(t, "KUBECONFIG", config.UseKubeconfig)
	context := config.GetKubeContext()

	if !IsMCECluster(t, context) {
		t.Skip("Not an MCE cluster, no MCE teardown needed")
	}

	PrintTestHeader(t, "TestTeardown_RevertMCEComponents",
		"Revert MCE components to their original pre-test states")

	state, err := ReadDeploymentState()
	if err != nil {
		t.Fatalf("Failed to read deployment state: %v", err)
	}

	if state == nil || len(state.MCEOriginalStates) == 0 {
		PrintToTTY("No MCE original states saved — nothing to revert\n\n")
		t.Skip("No MCE original states found in deployment state file")
	}

	// Show what will be reverted
	PrintToTTY("\n=== MCE component teardown plan ===\n")
	PrintToTTY("%-40s %-12s %s\n", "COMPONENT", "ORIGINAL", "CURRENT")
	PrintToTTY("%s\n", strings.Repeat("-", 65))

	type revertAction struct {
		name            string
		originalEnabled bool
	}
	var actions []revertAction
	var queryErrors []string

	for component, originalEnabled := range state.MCEOriginalStates {
		current, err := GetMCEComponentStatus(t, context, component)
		if err != nil {
			queryErrors = append(queryErrors, fmt.Sprintf("%s: %v", component, err))
			PrintToTTY("%-40s %-12s ⚠️  error: %v\n", component, fmtEnabled(originalEnabled), err)
			continue
		}

		currentStr := fmtEnabled(current.Enabled)
		originalStr := fmtEnabled(originalEnabled)

		if current.Enabled == originalEnabled {
			PrintToTTY("%-40s %-12s %-12s (no change needed)\n", component, originalStr, currentStr)
		} else {
			PrintToTTY("%-40s %-12s %-12s ← will revert\n", component, originalStr, currentStr)
			actions = append(actions, revertAction{name: component, originalEnabled: originalEnabled})
		}
	}

	PrintToTTY("%s\n", strings.Repeat("-", 65))

	if len(queryErrors) > 0 {
		PrintToTTY("\n⚠️  Failed to query %d component(s):\n", len(queryErrors))
		for _, e := range queryErrors {
			PrintToTTY("   - %s\n", e)
		}
	}

	if len(actions) == 0 {
		PrintToTTY("\n✅ All MCE components are already in their original state — no revert needed\n\n")
		t.Log("All MCE components already in original state")
		return
	}

	// Revert components
	PrintToTTY("\n=== Reverting %d MCE component(s) ===\n\n", len(actions))

	var revertErrors []string
	var reverted []string

	for _, action := range actions {
		if err := SetMCEComponentState(t, context, action.name, action.originalEnabled); err != nil {
			revertErrors = append(revertErrors, fmt.Sprintf("%s: %v", action.name, err))
			PrintToTTY("❌ Failed to revert %s: %v\n", action.name, err)
		} else {
			reverted = append(reverted, fmt.Sprintf("%s → %s", action.name, fmtEnabled(action.originalEnabled)))
		}
	}

	if len(revertErrors) > 0 {
		PrintToTTY("\n❌ Failed to revert %d component(s):\n", len(revertErrors))
		for _, e := range revertErrors {
			PrintToTTY("   - %s\n", e)
		}
		t.Errorf("Failed to revert MCE components: %v", revertErrors)
	}

	if len(reverted) > 0 {
		PrintToTTY("\n✅ Successfully reverted %d component(s):\n", len(reverted))
		for _, r := range reverted {
			PrintToTTY("   - %s\n", r)
		}
		t.Logf("Reverted MCE components: %v", reverted)

		// Wait for MCE to reconcile
		PrintToTTY("\n=== Waiting for MCE to reconcile ===\n")
		PrintToTTY("Waiting 30 seconds for MCE to start reconciling...\n")
		time.Sleep(30 * time.Second)
		PrintToTTY("✅ MCE reconciliation wait complete\n\n")
	}

	PrintToTTY("✅ MCE teardown complete\n\n")
	t.Log("MCE component teardown completed")
}

func fmtEnabled(enabled bool) string {
	if enabled {
		return "enabled"
	}
	return "disabled"
}
