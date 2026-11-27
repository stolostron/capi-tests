package test

import "testing"

const diagnosticHubName = "control-plane-logs"

func diagnosticEventHubNamespace(config *TestConfig) string {
	return "capi-eh-" + diagnosticNameSuffix(config)
}

func createDiagnosticEventHub(t *testing.T, config *TestConfig, location string) string {
	t.Helper()
	name := diagnosticEventHubNamespace(config)
	namespace := findDiagnosticResource(t, config, "eventhubs", "namespace", name)
	if namespace == nil || namespace.ProvisioningState != "Succeeded" {
		args := []string{"eventhubs", "namespace", "create", "--subscription", config.AzureSubscriptionID,
			"--resource-group", config.ResourceGroupName, "--name", name, "--location", location,
			"--sku", "Standard", "--only-show-errors", "-o", "none", "--tags"}
		if output, err := RunCommand(t, "az", append(args, diagnosticTags(config)...)...); err != nil {
			t.Fatalf("creating diagnostic Event Hubs namespace %s: %v: %s", name, err, output)
		}
	}

	// Both child operations are ARM create-or-update requests, including after a partial deployment.
	scope := []string{"--subscription", config.AzureSubscriptionID, "--resource-group", config.ResourceGroupName,
		"--namespace-name", name, "--only-show-errors"}
	hub := []string{"eventhubs", "eventhub", "create", "--name", diagnosticHubName, "-o", "none"}
	if output, err := RunCommand(t, "az", append(hub, scope...)...); err != nil {
		t.Fatalf("creating diagnostic event hub in %s: %v: %s", name, err, output)
	}
	rule := []string{"eventhubs", "namespace", "authorization-rule", "create", "--name", "azure-monitor",
		"--rights", "[Manage,Send,Listen]", "--query", "id", "-o", "tsv"}
	id, err := RunCommand(t, "az", append(rule, scope...)...)
	if err != nil || id == "" {
		t.Fatalf("creating diagnostic namespace authorization rule in %s: %v: %s", name, err, id)
	}
	return id
}

func deleteDiagnosticEventHubNamespace(t *testing.T, config *TestConfig) {
	t.Helper()
	namespace := findDiagnosticResource(t, config, "eventhubs", "namespace", diagnosticEventHubNamespace(config))
	if namespace == nil {
		return
	}
	// Azure removes the event hub and its namespace authorization rule with the namespace.
	output, err := RunCommand(t, "az", "eventhubs", "namespace", "delete", "--ids", namespace.ID, "--only-show-errors")
	if err != nil {
		t.Fatalf("deleting diagnostic Event Hubs namespace %s: %v: %s", namespace.ID, err, output)
	}
}
