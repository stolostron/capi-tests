package test

import (
	"os"
	"testing"
)

// TestRecovery_Preflight validates the persisted deployment identity before
// Make starts any recovery phase. It intentionally performs no cloud or
// Kubernetes operations, so an interrupted run can fail safely and explain
// what is missing before touching existing resources.
func TestRecovery_Preflight(t *testing.T) {
	if os.Getenv("RECOVERY_PREFLIGHT") != "1" {
		t.Skip("recovery preflight runs only through 'make recover'")
	}

	state, err := ReadDeploymentState()
	if err != nil {
		t.Fatalf("cannot read deployment state for recovery: %v", err)
	}
	if err := ValidateDeploymentStateForRecovery(state); err != nil {
		t.Fatalf("cannot recover interrupted deployment: %v", err)
	}

	t.Logf("recovery state validated for resource group %q, workload cluster %q in namespace %q",
		state.ResourceGroup, state.WorkloadClusterName, state.WorkloadClusterNamespace)
}
