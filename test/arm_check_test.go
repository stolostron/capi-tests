package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestCheckHCPFailsForDegradedRequirementsAndMissingIdentity(t *testing.T) {
	binDir := t.TempDir()
	azPath := filepath.Join(binDir, "az")
	azScript := `#!/usr/bin/env bash
set -euo pipefail
case "${1:-} ${2:-}" in
  "resource show")
    cat <<'JSON'
{"properties":{"provisioningState":"Succeeded","platform":{"subnetId":"/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Network/virtualNetworks/vnet/subnets/subnet","vnetIntegrationSubnetId":"/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Network/virtualNetworks/vnet/subnets/integration","networkSecurityGroupId":"/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Network/networkSecurityGroups/nsg"},"status":{"conditions":[{"type":"RequirementsValid","status":"False","reason":"Degraded"}]},"identities":["/subscriptions/sub/resourceGroups/rg/providers/Microsoft.ManagedIdentity/userAssignedIdentities/available","/subscriptions/sub/resourceGroups/rg/providers/Microsoft.ManagedIdentity/userAssignedIdentities/missing"]}}
JSON
    ;;
  "role assignment list") echo '[]' ;;
  "identity show")
    [[ "$*" == *"/available"* ]]
    ;;
  *) exit 0 ;;
esac
`
	if err := os.WriteFile(azPath, []byte(azScript), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	cmd := exec.Command("../scripts/check-hcp", "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.RedHatOpenShift/HCPOpenShiftClusters/hcp")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("check-hcp succeeded; want failure, output:\n%s", output)
	}
	if !strings.Contains(string(output), "RequirementsValid: ❌ False (Degraded)") {
		t.Fatalf("check-hcp output did not contain the degraded requirement:\n%s", output)
	}
}
