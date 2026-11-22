package test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Use the persisted run identity for globally unique storage and Event Hubs names.
func diagnosticNameSuffix(config *TestConfig) string {
	identity := strings.Join([]string{
		strings.ToLower(config.AzureSubscriptionID), strings.ToLower(config.ResourceGroupName),
		config.ClusterNamePrefix, config.WorkloadClusterNamespace,
	}, "/")
	digest := sha256.Sum256([]byte(identity))
	return fmt.Sprintf("%x", digest[:8])
}

func diagnosticAccountName(config *TestConfig) string {
	return "capidiag" + diagnosticNameSuffix(config)
}

type diagnosticResourceInfo struct {
	ID                string
	ProvisioningState string
	Tags              map[string]string
}

const diagnosticNamespaceTag = "capi-test-diagnostic-namespace"

// Both destination resources carry the run tags so retries can safely reuse or delete them.
func findDiagnosticResource(t *testing.T, config *TestConfig, service, kind, name string) *diagnosticResourceInfo {
	t.Helper()
	output, err := RunCommand(t, "az", service, kind, "list", "--subscription", config.AzureSubscriptionID,
		"--resource-group", config.ResourceGroupName, "--query", "[?name=='"+name+"'].{id:id,tags:tags,provisioningState:provisioningState}",
		"--only-show-errors", "-o", "json")
	if err != nil {
		t.Fatalf("looking up diagnostic %s %s %s: %v: %s", service, kind, name, err, output)
	}
	var resources []diagnosticResourceInfo
	if err := json.Unmarshal([]byte(output), &resources); err != nil {
		t.Fatalf("decoding %s %s lookup: %v", service, kind, err)
	}
	if len(resources) == 0 {
		return nil
	}
	resource := &resources[0]
	if resource.ID == "" || resource.Tags[diagnosticNamespaceTag] != config.WorkloadClusterNamespace ||
		resource.Tags["capi-test-run-id"] != config.ClusterNamePrefix {
		t.Fatalf("%s %s %s lacks the expected ownership tags for run %s in namespace %s", service, kind, name, config.ClusterNamePrefix, config.WorkloadClusterNamespace)
	}
	return resource
}

func diagnosticTags(config *TestConfig) []string {
	tags := make(map[string]string)
	maps.Copy(tags, config.ResourceTags)
	tags["capi-test-run-id"] = config.ClusterNamePrefix
	tags[diagnosticNamespaceTag] = config.WorkloadClusterNamespace
	return sortedTagPairs(tags, "=")
}

func createDiagnosticStorage(t *testing.T, config *TestConfig, location string) string {
	t.Helper()
	name := diagnosticAccountName(config)
	account := findDiagnosticResource(t, config, "storage", "account", name)
	if account != nil && account.ProvisioningState == "Succeeded" {
		return account.ID
	}
	args := []string{"storage", "account", "create", "--subscription", config.AzureSubscriptionID,
		"--resource-group", config.ResourceGroupName, "--name", name, "--location", location,
		"--kind", "StorageV2", "--sku", "Standard_LRS", "--allow-blob-public-access", "false",
		"--query", "id", "-o", "tsv", "--only-show-errors", "--tags"}
	args = append(args, diagnosticTags(config)...)
	id, err := RunCommand(t, "az", args...)
	if err != nil || id == "" {
		t.Fatalf("creating diagnostic storage account %s: %v: %s", name, err, id)
	}
	return id
}

func diagnosticResourceGroupExists(t *testing.T, config *TestConfig) bool {
	t.Helper()
	if err := resolveHCPSubscriptionID(t, config); err != nil {
		t.Fatal(err)
	}
	// A repeated teardown can run after the run's entire resource group has been removed.
	exists, err := RunCommand(t, "az", "group", "exists", "--subscription", config.AzureSubscriptionID,
		"--name", config.ResourceGroupName, "--only-show-errors", "-o", "json")
	if err != nil {
		t.Fatalf("checking diagnostic resource group: %v: %s", err, exists)
	}
	return exists == "true"
}

func deleteDiagnosticStorage(t *testing.T, config *TestConfig) {
	t.Helper()
	account := findDiagnosticResource(t, config, "storage", "account", diagnosticAccountName(config))
	if account == nil {
		return
	}
	output, err := RunCommand(t, "az", "storage", "account", "delete", "--ids", account.ID, "--yes", "--only-show-errors")
	if err != nil {
		t.Fatalf("deleting diagnostic storage account %s: %v: %s", account.ID, err, output)
	}
}

// Deployment and teardown can run in separate go test processes or be retried independently.
func TestDiagnosticStorageLifecycle(t *testing.T) {
	bin := t.TempDir()
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("ACCOUNT_STATE", filepath.Join(bin, "account.json"))
	const az = `#!/usr/bin/env bash
set -eu
[[ "$1 $2" == "storage account" ]]
case "$3" in
list)
    if [[ -f "$ACCOUNT_STATE" ]]; then cat "$ACCOUNT_STATE"; else echo '[]'; fi ;;
create)
    [[ ! -f "$ACCOUNT_STATE" && " $* " == *" --location westus2 "* &&
        " $* " == *" capi-test-run-id=run1 "* && " $* " == *" capi-test-diagnostic-namespace=ns1 "* ]]
    echo '[{"id":"/account-id","provisioningState":"Succeeded","tags":{"capi-test-run-id":"run1","capi-test-diagnostic-namespace":"ns1"}}]' > "$ACCOUNT_STATE"
    echo /account-id ;;
delete)
    [[ " $* " == *" --ids /account-id "* && -f "$ACCOUNT_STATE" ]]
    rm "$ACCOUNT_STATE" ;;
*) exit 1 ;;
esac
`
	if err := os.WriteFile(filepath.Join(bin, "az"), []byte(az), 0700); err != nil {
		t.Fatal(err)
	}
	config := &TestConfig{AzureSubscriptionID: "sub", ResourceGroupName: "rg", WorkloadClusterNamespace: "ns1", ClusterNamePrefix: "run1"}
	for range 2 {
		if id := createDiagnosticStorage(t, config, "westus2"); id != "/account-id" {
			t.Fatalf("got storage ID %q", id)
		}
	}
	deleteDiagnosticStorage(t, config)
	deleteDiagnosticStorage(t, config)
	if FileExists(os.Getenv("ACCOUNT_STATE")) {
		t.Fatal("diagnostic account remains after teardown")
	}
}
