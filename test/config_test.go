package test

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestGetDefaultRepoDir_EnvVariable(t *testing.T) {
	// This test must check current behavior, not set environment
	// because sync.Once means the first call wins for the entire test process

	config := NewTestConfig()

	// Check if ARO_REPO_DIR is currently set
	if envDir := os.Getenv("ARO_REPO_DIR"); envDir != "" {
		// If env var is set, config should use it
		if config.RepoDir != envDir {
			t.Errorf("When ARO_REPO_DIR is set, RepoDir should be %s, got: %s", envDir, config.RepoDir)
		}
		t.Logf("ARO_REPO_DIR is set to: %s", envDir)
	} else {
		// If env var is not set, should generate stable path
		if !strings.Contains(config.RepoDir, "cluster-api-installer-aro") {
			t.Errorf("When ARO_REPO_DIR not set, should generate stable path, got: %s", config.RepoDir)
		}
		if !strings.HasPrefix(config.RepoDir, os.TempDir()) {
			t.Errorf("Generated path should be in temp directory (%s), got: %s", os.TempDir(), config.RepoDir)
		}
		t.Logf("Generated stable path: %s", config.RepoDir)
	}
}

func TestGetDefaultRepoDir_Consistency(t *testing.T) {
	// Create multiple configs
	config1 := NewTestConfig()
	config2 := NewTestConfig()
	config3 := NewTestConfig()

	// All should return the same path due to sync.Once
	if config1.RepoDir != config2.RepoDir {
		t.Errorf("getDefaultRepoDir() not consistent across calls: %s != %s", config1.RepoDir, config2.RepoDir)
	}

	if config1.RepoDir != config3.RepoDir {
		t.Errorf("getDefaultRepoDir() not consistent across calls: %s != %s", config1.RepoDir, config3.RepoDir)
	}

	t.Logf("All configs consistently use: %s", config1.RepoDir)
}

func TestGetDefaultRepoDir_PathFormat(t *testing.T) {
	config := NewTestConfig()

	// If ARO_REPO_DIR env var is set, skip format validation
	if os.Getenv("ARO_REPO_DIR") != "" {
		t.Skip("ARO_REPO_DIR is set, skipping format validation")
	}

	// Verify the path contains expected prefix
	if !strings.Contains(config.RepoDir, "cluster-api-installer-aro") {
		t.Errorf("Generated path should contain 'cluster-api-installer-aro' prefix, got: %s", config.RepoDir)
	}

	// Verify it's in the temp directory
	if !strings.HasPrefix(config.RepoDir, os.TempDir()) {
		t.Errorf("Generated path should be in temp directory (%s), got: %s", os.TempDir(), config.RepoDir)
	}

	// Verify stable path format (no PID or timestamp)
	// Path format: <os.TempDir()>/cluster-api-installer-aro (e.g., /tmp/cluster-api-installer-aro on Linux, /var/folders/.../cluster-api-installer-aro on macOS)
	expectedPath := os.TempDir() + "/cluster-api-installer-aro"
	if config.RepoDir != expectedPath {
		t.Errorf("Generated path should be %s, got: %s", expectedPath, config.RepoDir)
	}

	t.Logf("Path format validated: %s", config.RepoDir)
}

func TestParseDeploymentTimeout_Default(t *testing.T) {
	// Ensure DEPLOYMENT_TIMEOUT is not set
	originalValue := os.Getenv("DEPLOYMENT_TIMEOUT")
	_ = os.Unsetenv("DEPLOYMENT_TIMEOUT")
	defer func() {
		if originalValue != "" {
			_ = os.Setenv("DEPLOYMENT_TIMEOUT", originalValue)
		}
	}()

	timeout := parseDeploymentTimeout()
	if timeout != DefaultDeploymentTimeout {
		t.Errorf("Expected default timeout %v, got %v", DefaultDeploymentTimeout, timeout)
	}
	t.Logf("Default timeout: %v", timeout)
}

func TestParseDeploymentTimeout_ValidDuration(t *testing.T) {
	testCases := []struct {
		input    string
		expected time.Duration
	}{
		{"30m", 30 * time.Minute},
		{"1h", 1 * time.Hour},
		{"90m", 90 * time.Minute},
		{"2h30m", 2*time.Hour + 30*time.Minute},
	}

	originalValue := os.Getenv("DEPLOYMENT_TIMEOUT")
	defer func() {
		if originalValue != "" {
			_ = os.Setenv("DEPLOYMENT_TIMEOUT", originalValue)
		} else {
			_ = os.Unsetenv("DEPLOYMENT_TIMEOUT")
		}
	}()

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			_ = os.Setenv("DEPLOYMENT_TIMEOUT", tc.input)
			timeout := parseDeploymentTimeout()
			if timeout != tc.expected {
				t.Errorf("For input '%s', expected %v, got %v", tc.input, tc.expected, timeout)
			}
		})
	}
}

func TestParseDeploymentTimeout_InvalidDuration(t *testing.T) {
	originalValue := os.Getenv("DEPLOYMENT_TIMEOUT")
	defer func() {
		if originalValue != "" {
			_ = os.Setenv("DEPLOYMENT_TIMEOUT", originalValue)
		} else {
			_ = os.Unsetenv("DEPLOYMENT_TIMEOUT")
		}
	}()

	// Note: "-1h" is valid Go duration syntax (negative), so not included
	// Empty string is handled separately (returns default without warning)
	invalidValues := []string{"invalid", "abc", "45", "1x"}
	for _, val := range invalidValues {
		t.Run(val, func(t *testing.T) {
			_ = os.Setenv("DEPLOYMENT_TIMEOUT", val)
			timeout := parseDeploymentTimeout()
			if timeout != DefaultDeploymentTimeout {
				t.Errorf("For invalid input '%s', expected default %v, got %v", val, DefaultDeploymentTimeout, timeout)
			}
		})
	}
}

func TestIsKindMode(t *testing.T) {
	testCases := []struct {
		name     string
		envValue string
		expected bool
	}{
		{"not set", "", false},
		{"true", "true", true},
		{"false", "false", false},
		{"invalid", "yes", false},
	}

	originalValue := os.Getenv("USE_KIND")
	defer func() {
		if originalValue != "" {
			_ = os.Setenv("USE_KIND", originalValue)
		} else {
			_ = os.Unsetenv("USE_KIND")
		}
	}()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.envValue != "" {
				_ = os.Setenv("USE_KIND", tc.envValue)
			} else {
				_ = os.Unsetenv("USE_KIND")
			}
			config := NewTestConfig()
			if config.IsKindMode() != tc.expected {
				t.Errorf("IsKindMode() = %v, expected %v (USE_KIND=%q)", config.IsKindMode(), tc.expected, tc.envValue)
			}
		})
	}
}

func TestGetExpectedFiles(t *testing.T) {
	config := NewTestConfig()
	files := config.GetExpectedFiles()

	expected := []string{"credentials.yaml", "aro.yaml"}
	if len(files) != len(expected) {
		t.Fatalf("GetExpectedFiles() returned %d files, expected %d: %v", len(files), len(expected), files)
	}
	for i, file := range files {
		if file != expected[i] {
			t.Errorf("GetExpectedFiles()[%d] = %q, expected %q", i, file, expected[i])
		}
	}
}

func TestNewAzureProvider(t *testing.T) {
	p := NewAzureProvider("capz-system")

	if p.Name != "aro" {
		t.Errorf("Expected provider name 'aro', got %q", p.Name)
	}

	// Verify controllers
	if len(p.Controllers) != 2 {
		t.Fatalf("Expected 2 controllers, got %d", len(p.Controllers))
	}
	if p.Controllers[0].DisplayName != "CAPZ" {
		t.Errorf("Expected first controller 'CAPZ', got %q", p.Controllers[0].DisplayName)
	}
	if p.Controllers[0].DeploymentName != "capz-controller-manager" {
		t.Errorf("Expected CAPZ deployment name, got %q", p.Controllers[0].DeploymentName)
	}
	if p.Controllers[1].DisplayName != "ASO" {
		t.Errorf("Expected second controller 'ASO', got %q", p.Controllers[1].DisplayName)
	}
	if p.Controllers[1].DeploymentName != "azureserviceoperator-controller-manager" {
		t.Errorf("Expected ASO deployment name, got %q", p.Controllers[1].DeploymentName)
	}

	// Verify webhooks
	if len(p.Webhooks) != 2 {
		t.Fatalf("Expected 2 webhooks, got %d", len(p.Webhooks))
	}
	if p.Webhooks[0].ServiceName != "capz-webhook-service" {
		t.Errorf("Expected CAPZ webhook service, got %q", p.Webhooks[0].ServiceName)
	}
	if p.Webhooks[1].ServiceName != "azureserviceoperator-webhook-service" {
		t.Errorf("Expected ASO webhook service, got %q", p.Webhooks[1].ServiceName)
	}

	// Verify credential secret
	if p.CredentialSecret == nil {
		t.Fatal("Expected credential secret to be defined")
	}
	if p.CredentialSecret.Name != "aso-controller-settings" {
		t.Errorf("Expected aso-controller-settings, got %q", p.CredentialSecret.Name)
	}
	if len(p.CredentialSecret.RequiredFields) != 4 {
		t.Errorf("Expected 4 required fields, got %d", len(p.CredentialSecret.RequiredFields))
	}

	// Verify deployment charts
	if len(p.DeploymentCharts) != 1 || p.DeploymentCharts[0] != "cluster-api-provider-azure" {
		t.Errorf("Expected [cluster-api-provider-azure], got %v", p.DeploymentCharts)
	}

	// Verify MCE component
	if p.MCEComponentName != "cluster-api-provider-azure-preview" {
		t.Errorf("Expected MCE component name, got %q", p.MCEComponentName)
	}
}

func TestNewAzureProvider_Namespace(t *testing.T) {
	p := NewAzureProvider("custom-namespace")

	// Verify namespace propagates to all controllers, webhooks, and credential secret
	for _, ctrl := range p.Controllers {
		if ctrl.Namespace != "custom-namespace" {
			t.Errorf("Controller %s namespace = %q, expected 'custom-namespace'", ctrl.DisplayName, ctrl.Namespace)
		}
	}
	for _, wh := range p.Webhooks {
		if wh.Namespace != "custom-namespace" {
			t.Errorf("Webhook %s namespace = %q, expected 'custom-namespace'", wh.DisplayName, wh.Namespace)
		}
	}
	if p.CredentialSecret.Namespace != "custom-namespace" {
		t.Errorf("Credential secret namespace = %q, expected 'custom-namespace'", p.CredentialSecret.Namespace)
	}
}

func TestNewAWSProvider(t *testing.T) {
	p := NewAWSProvider("capa-system")

	if p.Name != "rosa" {
		t.Errorf("Expected provider name 'rosa', got %q", p.Name)
	}

	// Verify controllers
	if len(p.Controllers) != 1 {
		t.Fatalf("Expected 1 controller, got %d", len(p.Controllers))
	}
	if p.Controllers[0].DisplayName != "CAPA" {
		t.Errorf("Expected controller 'CAPA', got %q", p.Controllers[0].DisplayName)
	}
	if p.Controllers[0].DeploymentName != "capa-controller-manager" {
		t.Errorf("Expected CAPA deployment name, got %q", p.Controllers[0].DeploymentName)
	}
	if p.Controllers[0].PodSelector != "cluster.x-k8s.io/provider=infrastructure-aws" {
		t.Errorf("Expected CAPA pod selector, got %q", p.Controllers[0].PodSelector)
	}

	// Verify webhooks
	if len(p.Webhooks) != 1 {
		t.Fatalf("Expected 1 webhook, got %d", len(p.Webhooks))
	}
	if p.Webhooks[0].ServiceName != "capa-webhook-service" {
		t.Errorf("Expected CAPA webhook service, got %q", p.Webhooks[0].ServiceName)
	}
	if p.Webhooks[0].Port != 443 {
		t.Errorf("Expected webhook port 443, got %d", p.Webhooks[0].Port)
	}

	// Verify credential secret
	if p.CredentialSecret == nil {
		t.Fatal("Expected credential secret to be defined")
	}
	if p.CredentialSecret.Name != "capa-manager-bootstrap-credentials" {
		t.Errorf("Expected capa-manager-bootstrap-credentials, got %q", p.CredentialSecret.Name)
	}
	if len(p.CredentialSecret.RequiredFields) != 1 {
		t.Fatalf("Expected 1 required field, got %d", len(p.CredentialSecret.RequiredFields))
	}
	if p.CredentialSecret.RequiredFields[0] != "credentials" {
		t.Errorf("Expected required field 'credentials', got %q", p.CredentialSecret.RequiredFields[0])
	}

	// Verify YAML generation credentials
	if len(p.YAMLGenCredentials) != 6 {
		t.Fatalf("Expected 6 YAML gen credentials, got %d", len(p.YAMLGenCredentials))
	}
	expectedCreds := []struct {
		name      string
		sensitive bool
	}{
		{"AWS_REGION", false},
		{"OCM_API_URL", false},
		{"OCM_CLIENT_ID", false},
		{"AWS_ACCESS_KEY_ID", false},
		{"AWS_SECRET_ACCESS_KEY", true},
		{"OCM_CLIENT_SECRET", true},
	}
	for i, expected := range expectedCreds {
		if p.YAMLGenCredentials[i].Name != expected.name {
			t.Errorf("Expected YAMLGenCredentials[%d].Name = %q, got %q", i, expected.name, p.YAMLGenCredentials[i].Name)
		}
		if p.YAMLGenCredentials[i].Sensitive != expected.sensitive {
			t.Errorf("Expected YAMLGenCredentials[%d].Sensitive = %v, got %v", i, expected.sensitive, p.YAMLGenCredentials[i].Sensitive)
		}
	}

	// Verify deployment charts
	if len(p.DeploymentCharts) != 1 || p.DeploymentCharts[0] != "cluster-api-provider-aws" {
		t.Errorf("Expected [cluster-api-provider-aws], got %v", p.DeploymentCharts)
	}

	// Verify MCE component
	if p.MCEComponentName != "cluster-api-provider-aws" {
		t.Errorf("Expected MCE component name 'cluster-api-provider-aws', got %q", p.MCEComponentName)
	}
}

func TestNewAWSProvider_Namespace(t *testing.T) {
	p := NewAWSProvider("custom-namespace")

	// Verify namespace propagates to controller, webhook, and credential secret
	if p.Controllers[0].Namespace != "custom-namespace" {
		t.Errorf("Controller namespace = %q, expected 'custom-namespace'", p.Controllers[0].Namespace)
	}
	if p.Webhooks[0].Namespace != "custom-namespace" {
		t.Errorf("Webhook namespace = %q, expected 'custom-namespace'", p.Webhooks[0].Namespace)
	}
	if p.CredentialSecret.Namespace != "custom-namespace" {
		t.Errorf("Credential secret namespace = %q, expected 'custom-namespace'", p.CredentialSecret.Namespace)
	}
}

func TestTestConfig_InfraProviders(t *testing.T) {
	config := NewTestConfig()

	if len(config.InfraProviders) != 1 {
		t.Fatalf("Expected 1 infrastructure provider, got %d", len(config.InfraProviders))
	}
	if config.InfraProviders[0].Name != "aro" {
		t.Errorf("Expected aro provider, got %q", config.InfraProviders[0].Name)
	}
}

func TestTestConfig_AllControllers(t *testing.T) {
	config := NewTestConfig()
	controllers := config.AllControllers()

	// Should have CAPI core + 2 Azure provider controllers = 3 total
	if len(controllers) != 3 {
		t.Fatalf("Expected 3 controllers (CAPI + CAPZ + ASO), got %d", len(controllers))
	}

	// First should be CAPI core
	if controllers[0].DisplayName != "CAPI" {
		t.Errorf("Expected first controller to be CAPI, got %q", controllers[0].DisplayName)
	}
	if controllers[0].DeploymentName != CAPIControllerDeployment {
		t.Errorf("Expected CAPI deployment name %q, got %q", CAPIControllerDeployment, controllers[0].DeploymentName)
	}

	// Second and third should be provider controllers
	if controllers[1].DisplayName != "CAPZ" {
		t.Errorf("Expected second controller to be CAPZ, got %q", controllers[1].DisplayName)
	}
	if controllers[2].DisplayName != "ASO" {
		t.Errorf("Expected third controller to be ASO, got %q", controllers[2].DisplayName)
	}
}

func TestTestConfig_AllWebhooks(t *testing.T) {
	config := NewTestConfig()
	webhooks := config.AllWebhooks()

	// Should have CAPI core + 2 Azure provider webhooks = 3 total
	if len(webhooks) != 3 {
		t.Fatalf("Expected 3 webhooks (CAPI + CAPZ + ASO), got %d", len(webhooks))
	}

	if webhooks[0].DisplayName != "CAPI" {
		t.Errorf("Expected first webhook to be CAPI, got %q", webhooks[0].DisplayName)
	}
	if webhooks[0].ServiceName != CAPIWebhookService {
		t.Errorf("Expected CAPI webhook service %q, got %q", CAPIWebhookService, webhooks[0].ServiceName)
	}
}

func TestTestConfig_AllNamespaces(t *testing.T) {
	config := NewTestConfig()
	namespaces := config.AllNamespaces()

	// Should have at least CAPI namespace
	if len(namespaces) == 0 {
		t.Fatal("Expected at least 1 namespace")
	}
	if namespaces[0] != config.CAPINamespace {
		t.Errorf("Expected first namespace to be CAPI namespace %q, got %q", config.CAPINamespace, namespaces[0])
	}
}

func TestTestConfig_DeploymentChartArgs(t *testing.T) {
	config := NewTestConfig()
	args := config.DeploymentChartArgs()

	// Should start with CAPI core chart
	if len(args) < 2 {
		t.Fatalf("Expected at least 2 chart args (CAPI + provider), got %d", len(args))
	}
	if args[0] != CAPIDeploymentChartName {
		t.Errorf("Expected first chart to be %q, got %q", CAPIDeploymentChartName, args[0])
	}
	if args[1] != "cluster-api-provider-azure" {
		t.Errorf("Expected second chart to be 'cluster-api-provider-azure', got %q", args[1])
	}
}

func TestNewAzureProvider_RequiredTools(t *testing.T) {
	p := NewAzureProvider("capz-system")

	if len(p.RequiredTools) != 1 || p.RequiredTools[0] != "az" {
		t.Errorf("Expected RequiredTools=[az], got %v", p.RequiredTools)
	}
}

func TestNewAzureProvider_RequiredScripts(t *testing.T) {
	p := NewAzureProvider("capz-system")

	if len(p.RequiredScripts) != 2 {
		t.Fatalf("Expected 2 required scripts, got %d", len(p.RequiredScripts))
	}
	if p.RequiredScripts[0] != "scripts/deploy-charts.sh" {
		t.Errorf("Expected first script 'scripts/deploy-charts.sh', got %q", p.RequiredScripts[0])
	}
	if p.RequiredScripts[1] != "scripts/aro-hcp/gen.sh" {
		t.Errorf("Expected second script 'scripts/aro-hcp/gen.sh', got %q", p.RequiredScripts[1])
	}
}

func TestNewAWSProvider_RequiredTools(t *testing.T) {
	p := NewAWSProvider("capa-system")

	if len(p.RequiredTools) != 1 || p.RequiredTools[0] != "aws" {
		t.Errorf("Expected RequiredTools=[aws], got %v", p.RequiredTools)
	}
}

func TestNewAWSProvider_RequiredScripts(t *testing.T) {
	p := NewAWSProvider("capa-system")

	if len(p.RequiredScripts) != 2 {
		t.Fatalf("Expected 2 required scripts, got %d", len(p.RequiredScripts))
	}
	if p.RequiredScripts[0] != "scripts/deploy-charts.sh" {
		t.Errorf("Expected first script 'scripts/deploy-charts.sh', got %q", p.RequiredScripts[0])
	}
	if p.RequiredScripts[1] != "scripts/rosa-hcp/gen.sh" {
		t.Errorf("Expected second script 'scripts/rosa-hcp/gen.sh', got %q", p.RequiredScripts[1])
	}
}

func TestTestConfig_HasProvider(t *testing.T) {
	config := NewTestConfig()

	// Default provider is ARO
	if !config.HasProvider("aro") {
		t.Error("HasProvider('aro') should return true by default")
	}
	if config.HasProvider("rosa") {
		t.Error("HasProvider('rosa') should return false by default")
	}
	if config.HasProvider("nonexistent") {
		t.Error("HasProvider('nonexistent') should return false")
	}
}

func TestTestConfig_InfraProviderName(t *testing.T) {
	config := NewTestConfig()

	// Default should be "aro"
	if config.InfraProviderName != "aro" {
		t.Errorf("Expected default InfraProviderName 'aro', got %q", config.InfraProviderName)
	}
}

func TestTestConfig_AllRequiredTools(t *testing.T) {
	config := NewTestConfig()
	tools := config.AllRequiredTools()

	// Default (ARO) should include "az"
	if len(tools) != 1 || tools[0] != "az" {
		t.Errorf("Expected AllRequiredTools()=[az] for default provider, got %v", tools)
	}
}

func TestTestConfig_AllRequiredScripts(t *testing.T) {
	config := NewTestConfig()
	scripts := config.AllRequiredScripts()

	// Default (ARO) should include 2 scripts
	if len(scripts) != 2 {
		t.Fatalf("Expected 2 required scripts for default provider, got %d: %v", len(scripts), scripts)
	}
	if scripts[0] != "scripts/deploy-charts.sh" {
		t.Errorf("Expected first script 'scripts/deploy-charts.sh', got %q", scripts[0])
	}
}
