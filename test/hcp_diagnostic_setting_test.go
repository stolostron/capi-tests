package test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

const (
	diagnosticVersion    = "v1api20210501preview"
	diagnosticAPIVersion = "insights.azure.com/" + diagnosticVersion
	diagnosticResource   = "diagnosticsettings.insights.azure.com"
	diagnosticAzureName  = "capi-tests-control-plane-logs"
)

var diagnosticLogCategories = []string{
	"kube-apiserver",
	"kube-audit",
	"kube-audit-admin",
	"kube-controller-manager",
	"kube-scheduler",
	"cloud-controller-manager",
	"csi-azuredisk-controller",
	"csi-azurefile-controller",
	"csi-snapshot-controller",
}

func missingDiagnosticCategories(actual []string) []string {
	var missing []string
	for _, category := range diagnosticLogCategories {
		if !slices.Contains(actual, category) {
			missing = append(missing, category)
		}
	}
	return missing
}

// Use the persisted run prefix so cleanup can find the CR even without the generated cluster YAML.
func diagnosticSettingName(config *TestConfig) string {
	return diagnosticAzureName + "-" + config.ClusterNamePrefix
}

func diagnosticSettingConfig(t *testing.T) *TestConfig {
	t.Helper()
	config := NewTestConfig()
	if config.InfraProviderName != "aro" {
		t.Skip("Control-plane diagnostic settings apply only to ARO HCP")
	}
	if configError != nil {
		t.Fatalf("configuration initialization failed: %s", *configError)
	}
	SetupKubeconfig(t, config)
	return config
}

func diagnosticKubectl(t *testing.T, config *TestConfig, args ...string) string {
	t.Helper()
	scoped := []string{"--context", config.GetKubeContext(), "-n", config.WorkloadClusterNamespace}
	output, err := RunCommandQuiet(t, "kubectl", append(scoped, args...)...)
	if err != nil {
		t.Fatalf("kubectl %s: %v: %s", args[0], err, output)
	}
	return filterKubectlWarnings(output)
}

func diagnosticAzure(t *testing.T, args ...string) string {
	t.Helper()
	command := append([]string{"monitor", "diagnostic-settings"}, args...)
	output, err := RunCommand(t, "az", append(command, "--only-show-errors")...)
	if err != nil {
		t.Fatalf("reading Azure Monitor diagnostic settings: %v: %s", err, output)
	}
	return output
}

func diagnosticSettingServed(t *testing.T, config *TestConfig) bool {
	t.Helper()
	return diagnosticKubectl(t, config, "get", "crd", diagnosticResource, "--ignore-not-found", "-o",
		"jsonpath={.spec.versions[?(@.name==\""+diagnosticVersion+"\")].served}") == "true"
}

// Resolve the ASO object's Kubernetes name from the CAPI resource, and its Azure ID from ASO status.
// During cleanup either resource can already be gone.
func diagnosticHCP(t *testing.T, config *TestConfig) (string, string) {
	t.Helper()
	owner := diagnosticKubectl(t, config, "get", "arocontrolplanes.controlplane.cluster.x-k8s.io",
		config.GetProvisionedControlPlaneName(), "--ignore-not-found", "-o",
		`jsonpath={.spec.resources[?(@.kind=="HcpOpenShiftCluster")].metadata.name}`)
	if owner == "" {
		return "", ""
	}
	id := diagnosticKubectl(t, config, "get", "hcpopenshiftclusters.redhatopenshift.azure.com", owner,
		"--ignore-not-found", "-o", "jsonpath={.status.id}")
	return owner, id
}

func applyDiagnosticSetting(t *testing.T, config *TestConfig, owner, storageID, ruleID string) {
	t.Helper()
	var manifest strings.Builder
	fmt.Fprintf(&manifest, `apiVersion: %s
kind: DiagnosticSetting
metadata:
  name: %q
  namespace: %q
spec:
  azureName: %q
  owner:
    group: redhatopenshift.azure.com
    kind: HcpOpenShiftCluster
    name: %q
  storageAccountReference:
    armId: %q
  eventHubName: %q
  eventHubAuthorizationRuleReference:
    armId: %q
  logs:
`, diagnosticAPIVersion, diagnosticSettingName(config), config.WorkloadClusterNamespace,
		diagnosticAzureName, owner, storageID, diagnosticHubName, ruleID)
	for _, category := range diagnosticLogCategories {
		fmt.Fprintf(&manifest, "    - category: %q\n      enabled: true\n", category)
	}
	path := filepath.Join(t.TempDir(), "diagnostic-setting.yaml")
	if err := os.WriteFile(path, []byte(manifest.String()), 0600); err != nil {
		t.Fatal(err)
	}
	if err := ApplyWithRetryInNamespace(t, config.GetKubeContext(), config.WorkloadClusterNamespace, path, DefaultApplyMaxRetries); err != nil {
		t.Fatalf("applying ASO DiagnosticSetting: %v", err)
	}
}

func waitForDiagnosticSetting(t *testing.T, config *TestConfig) {
	t.Helper()
	deadline := time.Now().Add(config.ASOControllerTimeout)
	for {
		output := diagnosticKubectl(t, config, "get", diagnosticResource, diagnosticSettingName(config), "--request-timeout=30s", "-o", "json")
		ready, detail, err := diagnosticReadiness(output)
		if err != nil {
			t.Fatal(err)
		}
		if ready {
			return
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			t.Fatalf("timed out waiting for ASO DiagnosticSetting: %s", detail)
		}
		time.Sleep(min(10*time.Second, remaining))
	}
}

func diagnosticReadiness(output string) (bool, string, error) {
	var setting struct {
		Metadata struct{ Generation int64 }
		Status   struct {
			Conditions []struct {
				K8sCondition
				ObservedGeneration int64
				Severity           string
			}
		}
	}
	if err := json.Unmarshal([]byte(output), &setting); err != nil {
		return false, "", fmt.Errorf("decoding ASO DiagnosticSetting: %w", err)
	}
	for _, c := range setting.Status.Conditions {
		if c.Type != "Ready" {
			continue
		}
		detail := fmt.Sprintf("Ready=%s at generation %d/%d: %s %s", c.Status, c.ObservedGeneration, setting.Metadata.Generation, c.Reason, c.Message)
		current := setting.Metadata.Generation > 0 && c.ObservedGeneration == setting.Metadata.Generation
		if current && c.Status == "False" && c.Severity == "Error" {
			return false, detail, fmt.Errorf("ASO DiagnosticSetting reconciliation failed: %s", detail)
		}
		return current && c.Status == "True", detail, nil
	}
	return false, "ASO has not reported a Ready condition", nil
}

func verifyDiagnosticDestination(output, storageID, ruleID string) error {
	var setting struct {
		EventHubName                string
		EventHubAuthorizationRuleID string
		StorageAccountID            string
		Logs                        []struct {
			Category string
			Enabled  bool
		}
	}
	if err := json.Unmarshal([]byte(output), &setting); err != nil {
		return fmt.Errorf("decoding Azure diagnostic setting: %w", err)
	}
	if !strings.EqualFold(setting.StorageAccountID, storageID) {
		return fmt.Errorf("Azure diagnostic setting storage account %q, expected %q", setting.StorageAccountID, storageID)
	}
	if setting.EventHubName != diagnosticHubName {
		return fmt.Errorf("Azure diagnostic setting event hub %q, expected %q", setting.EventHubName, diagnosticHubName)
	}
	if !strings.EqualFold(setting.EventHubAuthorizationRuleID, ruleID) {
		return fmt.Errorf("Azure diagnostic setting authorization rule %q, expected %q", setting.EventHubAuthorizationRuleID, ruleID)
	}
	var enabled []string
	for _, log := range setting.Logs {
		if log.Enabled {
			enabled = append(enabled, log.Category)
		}
	}
	if missing := missingDiagnosticCategories(enabled); len(missing) > 0 {
		return fmt.Errorf("missing enabled diagnostic log categories: %s", strings.Join(missing, ", "))
	}
	return nil
}

func TestDiagnosticReadiness(t *testing.T) {
	for _, tc := range []struct {
		name, status, severity string
		observed               int
		ready, failed          bool
	}{
		{"current", "True", "", 2, true, false},
		{"stale success", "True", "", 1, false, false},
		{"retrying", "False", "Warning", 2, false, false},
		{"stale failure", "False", "Error", 1, false, false},
		{"terminal failure", "False", "Error", 2, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			output := fmt.Sprintf(`{"metadata":{"generation":2},"status":{"conditions":[{"type":"Ready","status":%q,"severity":%q,"observedGeneration":%d,"reason":"Example","message":"details"}]}}`,
				tc.status, tc.severity, tc.observed)
			ready, detail, err := diagnosticReadiness(output)
			if ready != tc.ready || (err != nil) != tc.failed || !strings.Contains(detail, "Example details") {
				t.Fatalf("ready=%t, detail=%q, err=%v", ready, detail, err)
			}
		})
	}
}

func TestVerifyDiagnosticDestination(t *testing.T) {
	for _, tc := range []struct {
		name, destination, hub, rule, skip, disable, wantError string
	}{
		{name: "all categories enabled", destination: "/A", hub: diagnosticHubName, rule: "/RULE"},
		{name: "wrong hub", destination: "/a", hub: "other", rule: "/rule", wantError: "event hub"},
		{name: "missing rule", destination: "/a", hub: diagnosticHubName, wantError: "authorization rule"},
		{name: "wrong rule", destination: "/a", hub: diagnosticHubName, rule: "/other", wantError: "authorization rule"},
		{name: "wrong destination", destination: "/b", wantError: "storage account"},
		{name: "missing category", destination: "/a", hub: diagnosticHubName, rule: "/rule", skip: "kube-audit", wantError: "kube-audit"},
		{name: "disabled category", destination: "/a", hub: diagnosticHubName, rule: "/rule", disable: "csi-snapshot-controller", wantError: "csi-snapshot-controller"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var logs []string
			for _, category := range diagnosticLogCategories {
				if category != tc.skip {
					logs = append(logs, fmt.Sprintf(`{"category":%q,"enabled":%t}`, category, category != tc.disable))
				}
			}
			output := fmt.Sprintf(`{"storageAccountId":%q,"eventHubName":%q,"eventHubAuthorizationRuleId":%q,"logs":[%s]}`, tc.destination, tc.hub, tc.rule, strings.Join(logs, ","))
			err := verifyDiagnosticDestination(output, "/a", "/rule")
			if tc.wantError == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("expected error containing %q, got %v", tc.wantError, err)
			}
		})
	}
}
