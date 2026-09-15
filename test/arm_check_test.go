package test

import "testing"

func TestBuildHCPResourceID(t *testing.T) {
	config := &TestConfig{
		AzureSubscriptionID: "subscription-123",
		ResourceGroupName:   "capz-tests-resgroup",
		HCPResourceName:     "capz-tests-control-plane",
	}

	want := "/subscriptions/subscription-123/resourceGroups/capz-tests-resgroup/providers/Microsoft.RedHatOpenShift/HCPOpenShiftClusters/capz-tests-control-plane"
	if got := config.BuildHCPResourceID(); got != want {
		t.Fatalf("BuildHCPResourceID() = %q, want %q", got, want)
	}
}
