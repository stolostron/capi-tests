package test

import (
	"testing"
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

	RestoreMCEOriginalStates(t, context)
}
