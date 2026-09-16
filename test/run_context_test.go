package test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestInitializeRunContext is the command-safe entry point used by Make before
// any phase process starts. It exercises the same shared logic as normal
// TestConfig construction.
func TestInitializeRunContext(t *testing.T) {
	context, err := EnsureRunContext()
	if err != nil {
		t.Fatalf("failed to initialize run context: %v", err)
	}
	t.Logf("run context initialized at %s for %s", RunContextFilePath(), context.ClusterNamePrefix)
}

func TestEnsureRunContextInitializesAndPersistsExplicitIdentity(t *testing.T) {
	contextPath := filepath.Join(t.TempDir(), "run-context.json")
	t.Setenv("CAPI_TEST_CONTEXT_FILE", contextPath)
	t.Setenv("CS_CLUSTER_NAME", "cate-fixed")
	t.Setenv("RESOURCEGROUPNAME", "cate-fixed-resgroup")
	t.Setenv("WORKLOAD_CLUSTER_NAMESPACE", "capz-test-fixed")
	t.Setenv("INFRA_PROVIDER", "aro")
	t.Setenv("DEPLOYMENT_ENV", "stage")

	context, err := EnsureRunContext()
	if err != nil {
		t.Fatalf("EnsureRunContext() unexpected error: %v", err)
	}

	if context.ClusterNamePrefix != "cate-fixed" {
		t.Errorf("ClusterNamePrefix = %q, want %q", context.ClusterNamePrefix, "cate-fixed")
	}
	if context.ResourceGroupName != "cate-fixed-resgroup" {
		t.Errorf("ResourceGroupName = %q, want %q", context.ResourceGroupName, "cate-fixed-resgroup")
	}
	if context.WorkloadClusterNamespace != "capz-test-fixed" {
		t.Errorf("WorkloadClusterNamespace = %q, want %q", context.WorkloadClusterNamespace, "capz-test-fixed")
	}
	if context.TestRunID == "" {
		t.Fatal("TestRunID is empty; initialization must persist a run identity")
	}
	if context.InfraProvider != "aro" || context.DeploymentEnvironment != "stage" {
		t.Errorf("provider/environment = %q/%q, want %q/%q", context.InfraProvider, context.DeploymentEnvironment, "aro", "stage")
	}

	data, err := os.ReadFile(contextPath)
	if err != nil {
		t.Fatalf("failed to read persisted context: %v", err)
	}
	var persisted RunContext
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatalf("failed to parse persisted context: %v", err)
	}
	if persisted != *context {
		t.Errorf("persisted context = %+v, want %+v", persisted, *context)
	}
}

func TestEnsureRunContextReusesExistingIdentityByteForByte(t *testing.T) {
	contextPath := filepath.Join(t.TempDir(), "run-context.json")
	original := `{"cluster_name_prefix":"cate-a1b2c","resource_group_name":"capz-tests-a1b2c-resgroup","workload_cluster_namespace":"capz-test-20260916-120000","test_run_id":"a1b2c"}`
	if err := os.WriteFile(contextPath, []byte(original), 0600); err != nil {
		t.Fatalf("failed to write context fixture: %v", err)
	}
	t.Setenv("CAPI_TEST_CONTEXT_FILE", contextPath)

	first, err := EnsureRunContext()
	if err != nil {
		t.Fatalf("first EnsureRunContext() unexpected error: %v", err)
	}
	firstBytes, err := os.ReadFile(contextPath)
	if err != nil {
		t.Fatalf("failed to read context after first load: %v", err)
	}

	second, err := EnsureRunContext()
	if err != nil {
		t.Fatalf("second EnsureRunContext() unexpected error: %v", err)
	}
	secondBytes, err := os.ReadFile(contextPath)
	if err != nil {
		t.Fatalf("failed to read context after second load: %v", err)
	}

	if string(firstBytes) != original || string(secondBytes) != original {
		t.Fatalf("EnsureRunContext() rewrote an existing context: first=%q second=%q want=%q", firstBytes, secondBytes, original)
	}
	if *first != *second {
		t.Errorf("repeated loads differ: first=%+v second=%+v", *first, *second)
	}
}

func TestEnsureRunContextRejectsConflictingExplicitIdentity(t *testing.T) {
	contextPath := filepath.Join(t.TempDir(), "run-context.json")
	original := `{"cluster_name_prefix":"cate-a1b2c","resource_group_name":"capz-tests-a1b2c-resgroup","workload_cluster_namespace":"capz-test-20260916-120000","test_run_id":"a1b2c"}`
	if err := os.WriteFile(contextPath, []byte(original), 0600); err != nil {
		t.Fatalf("failed to write context fixture: %v", err)
	}
	t.Setenv("CAPI_TEST_CONTEXT_FILE", contextPath)
	t.Setenv("CS_CLUSTER_NAME", "cate-other")

	_, err := EnsureRunContext()
	if err == nil {
		t.Fatal("EnsureRunContext() succeeded with a conflicting explicit CS_CLUSTER_NAME")
	}
	for _, expected := range []string{"CS_CLUSTER_NAME", "cate-other", "cate-a1b2c"} {
		if !strings.Contains(err.Error(), expected) {
			t.Errorf("error %q does not identify conflict value %q", err, expected)
		}
	}
}

func TestEnsureRunContextRejectsProviderAndEnvironmentConflicts(t *testing.T) {
	contextPath := filepath.Join(t.TempDir(), "run-context.json")
	original := `{"cluster_name_prefix":"cate-a1b2c","resource_group_name":"capz-tests-a1b2c-resgroup","workload_cluster_namespace":"capz-test-20260916-120000","test_run_id":"a1b2c","infra_provider":"aro","deployment_environment":"stage"}`
	if err := os.WriteFile(contextPath, []byte(original), 0600); err != nil {
		t.Fatalf("failed to write context fixture: %v", err)
	}
	t.Setenv("CAPI_TEST_CONTEXT_FILE", contextPath)
	t.Setenv("INFRA_PROVIDER", "rosa")

	_, err := EnsureRunContext()
	if err == nil || !strings.Contains(err.Error(), "INFRA_PROVIDER") {
		t.Fatalf("EnsureRunContext() error = %v, want INFRA_PROVIDER conflict", err)
	}

	t.Setenv("INFRA_PROVIDER", "")
	t.Setenv("DEPLOYMENT_ENV", "prod")
	_, err = EnsureRunContext()
	if err == nil || !strings.Contains(err.Error(), "DEPLOYMENT_ENV") {
		t.Fatalf("EnsureRunContext() error = %v, want DEPLOYMENT_ENV conflict", err)
	}
}

func TestNewTestConfigReusesPersistedProviderEnvironmentAndUser(t *testing.T) {
	contextPath := filepath.Join(t.TempDir(), "run-context.json")
	contextJSON := `{"cluster_name_prefix":"rosa-fixed","resource_group_name":"rosa-fixed-resgroup","workload_cluster_name":"capa-tests","workload_cluster_namespace":"capa-test-fixed","test_run_id":"fixed","capi_user":"persisted-user","infra_provider":"rosa","deployment_environment":"prod"}`
	if err := os.WriteFile(contextPath, []byte(contextJSON), 0600); err != nil {
		t.Fatalf("failed to write context fixture: %v", err)
	}
	t.Setenv("CAPI_TEST_CONTEXT_FILE", contextPath)
	t.Setenv("INFRA_PROVIDER", "")
	t.Setenv("DEPLOYMENT_ENV", "")
	t.Setenv("CAPI_USER", "")
	t.Setenv("USER", "different-user")

	config := NewTestConfig()
	if config.InfraProviderName != "rosa" {
		t.Errorf("InfraProviderName = %q, want rosa", config.InfraProviderName)
	}
	if config.Environment != "prod" {
		t.Errorf("Environment = %q, want prod", config.Environment)
	}
	if config.CAPIUser != "persisted-user" {
		t.Errorf("CAPIUser = %q, want persisted-user", config.CAPIUser)
	}
	if config.ClusterNamePrefix != "rosa-fixed" ||
		config.ResourceGroupName != "rosa-fixed-resgroup" ||
		config.WorkloadClusterName != "capa-tests" ||
		config.WorkloadClusterNamespace != "capa-test-fixed" ||
		config.TestRunID != "fixed" {
		t.Errorf("config did not reuse persisted identity: %+v", config)
	}
}

func TestEnsureRunContextRejectsExplicitUserConflict(t *testing.T) {
	contextPath := filepath.Join(t.TempDir(), "run-context.json")
	contextJSON := `{"cluster_name_prefix":"cate-fixed","resource_group_name":"cate-fixed-resgroup","workload_cluster_namespace":"capz-test-fixed","test_run_id":"fixed","capi_user":"persisted-user","infra_provider":"aro","deployment_environment":"stage"}`
	if err := os.WriteFile(contextPath, []byte(contextJSON), 0600); err != nil {
		t.Fatalf("failed to write context fixture: %v", err)
	}
	t.Setenv("CAPI_TEST_CONTEXT_FILE", contextPath)
	t.Setenv("CAPI_USER", "other-user")

	_, err := EnsureRunContext()
	if err == nil || !strings.Contains(err.Error(), "CAPI_USER") {
		t.Fatalf("EnsureRunContext() error = %v, want CAPI_USER conflict", err)
	}
}

func TestEnsureRunContextMigratesLegacyDeploymentState(t *testing.T) {
	workspace := t.TempDir()
	contextPath := filepath.Join(workspace, "run-context.json")
	legacyPath := filepath.Join(workspace, ".deployment-state.json")
	legacy := `{"resource_group":"capz-tests-a1b2c-resgroup","workload_cluster_namespace":"capz-test-20260916-120000","cluster_name_prefix":"cate-a1b2c","test_run_id":"a1b2c"}`
	if err := os.WriteFile(legacyPath, []byte(legacy), 0600); err != nil {
		t.Fatalf("failed to write legacy state fixture: %v", err)
	}
	t.Setenv("CAPI_TEST_CONTEXT_FILE", contextPath)
	t.Setenv("CAPI_TEST_WORKSPACE", workspace)

	context, err := EnsureRunContext()
	if err != nil {
		t.Fatalf("EnsureRunContext() unexpected error: %v", err)
	}
	if context.ClusterNamePrefix != "cate-a1b2c" || context.ResourceGroupName != "capz-tests-a1b2c-resgroup" || context.TestRunID != "a1b2c" {
		t.Errorf("migrated context = %+v, want legacy identity", *context)
	}
	if _, err := os.Stat(contextPath); err != nil {
		t.Fatalf("migrated context was not persisted: %v", err)
	}
	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Fatalf("legacy deployment state still exists after migration: %v", err)
	}

	second, err := EnsureRunContext()
	if err != nil {
		t.Fatalf("second EnsureRunContext() unexpected error: %v", err)
	}
	if *second != *context {
		t.Fatalf("second initialization changed migrated identity: first=%+v second=%+v", *context, *second)
	}
}

func TestDeleteDeploymentStateRemovesCustomContextAndMigratedLegacyState(t *testing.T) {
	workspace := t.TempDir()
	contextPath := filepath.Join(workspace, "custom", "context.json")
	legacyPath := filepath.Join(filepath.Dir(contextPath), ".deployment-state.json")
	if err := os.MkdirAll(filepath.Dir(contextPath), 0700); err != nil {
		t.Fatalf("failed to create custom context directory: %v", err)
	}
	legacy := `{"resource_group":"capz-tests-a1b2c-resgroup","workload_cluster_namespace":"capz-test-20260916-120000","cluster_name_prefix":"cate-a1b2c","test_run_id":"a1b2c"}`
	if err := os.WriteFile(legacyPath, []byte(legacy), 0600); err != nil {
		t.Fatalf("failed to write legacy state fixture: %v", err)
	}
	t.Setenv("CAPI_TEST_CONTEXT_FILE", contextPath)

	if _, err := EnsureRunContext(); err != nil {
		t.Fatalf("EnsureRunContext() unexpected error: %v", err)
	}
	if err := DeleteDeploymentState(); err != nil {
		t.Fatalf("DeleteDeploymentState() unexpected error: %v", err)
	}
	for _, path := range []string{contextPath, deploymentStatePath(), legacyPath} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("cleanup left %s: %v", path, err)
		}
	}
}

func TestWriteDeploymentStatePreservesImmutableRunIdentity(t *testing.T) {
	workspace := t.TempDir()
	contextPath := filepath.Join(workspace, "run-context.json")
	contextJSON := `{"cluster_name_prefix":"cate-a1b2c","resource_group_name":"capz-tests-a1b2c-resgroup","workload_cluster_name":"capz-tests","workload_cluster_namespace":"capz-test-20260916-120000","test_run_id":"a1b2c","capi_user":"cate"}`
	if err := os.WriteFile(contextPath, []byte(contextJSON), 0600); err != nil {
		t.Fatalf("failed to write context fixture: %v", err)
	}
	t.Setenv("CAPI_TEST_CONTEXT_FILE", contextPath)

	config := &TestConfig{
		ClusterNamePrefix:        "wrong-prefix",
		ResourceGroupName:        "wrong-resgroup",
		WorkloadClusterName:      "wrong-workload",
		WorkloadClusterNamespace: "wrong-namespace",
		TestRunID:                "wrong-id",
		ManagementClusterName:    "test-management",
		Region:                   "uksouth",
		CAPIUser:                 "cate",
		Environment:              "stage",
	}
	if err := WriteDeploymentState(config); err != nil {
		t.Fatalf("WriteDeploymentState() unexpected error: %v", err)
	}

	state, err := ReadDeploymentState()
	if err != nil {
		t.Fatalf("ReadDeploymentState() unexpected error: %v", err)
	}
	if state.ResourceGroup != "capz-tests-a1b2c-resgroup" ||
		state.ClusterNamePrefix != "cate-a1b2c" ||
		state.WorkloadClusterNamespace != "capz-test-20260916-120000" ||
		state.TestRunID != "a1b2c" {
		t.Errorf("deployment state changed immutable identity: %+v", state)
	}
}

func TestSaveMCEOriginalStatesPreservesImmutableRunIdentity(t *testing.T) {
	workspace := t.TempDir()
	contextPath := filepath.Join(workspace, "run-context.json")
	contextJSON := `{"cluster_name_prefix":"cate-a1b2c","resource_group_name":"capz-tests-a1b2c-resgroup","workload_cluster_name":"capz-tests","workload_cluster_namespace":"capz-test-20260916-120000","test_run_id":"a1b2c","capi_user":"cate"}`
	if err := os.WriteFile(contextPath, []byte(contextJSON), 0600); err != nil {
		t.Fatalf("failed to write context fixture: %v", err)
	}
	t.Setenv("CAPI_TEST_CONTEXT_FILE", contextPath)

	if err := SaveMCEOriginalStates(map[string]bool{"cluster-api": true}); err != nil {
		t.Fatalf("SaveMCEOriginalStates() unexpected error: %v", err)
	}
	state, err := ReadDeploymentState()
	if err != nil {
		t.Fatalf("ReadDeploymentState() unexpected error: %v", err)
	}
	if state.ResourceGroup != "capz-tests-a1b2c-resgroup" ||
		state.ClusterNamePrefix != "cate-a1b2c" ||
		state.WorkloadClusterNamespace != "capz-test-20260916-120000" ||
		state.TestRunID != "a1b2c" {
		t.Errorf("MCE state update lost immutable identity: %+v", state)
	}
}

func TestDeleteDeploymentStateRemovesContextAndNextRunGetsNewIdentity(t *testing.T) {
	contextPath := filepath.Join(t.TempDir(), "run-context.json")
	t.Setenv("CAPI_TEST_CONTEXT_FILE", contextPath)

	first, err := EnsureRunContext()
	if err != nil {
		t.Fatalf("first EnsureRunContext() unexpected error: %v", err)
	}
	if err := DeleteDeploymentState(); err != nil {
		t.Fatalf("DeleteDeploymentState() unexpected error: %v", err)
	}
	if _, err := os.Stat(contextPath); !os.IsNotExist(err) {
		t.Fatalf("run context still exists after cleanup: %v", err)
	}

	second, err := EnsureRunContext()
	if err != nil {
		t.Fatalf("second EnsureRunContext() unexpected error: %v", err)
	}
	if first.ClusterNamePrefix == second.ClusterNamePrefix || first.TestRunID == second.TestRunID {
		t.Fatalf("next run reused identity: first=%+v second=%+v", *first, *second)
	}
}
