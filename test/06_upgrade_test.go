package test

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestUpgrade_AROCluster is a standalone, explicitly requested phase.
func TestUpgrade_AROCluster(t *testing.T) {
	config := NewTestConfig()
	if config.AROUpgradeVersion == "" {
		t.Skip("set ARO_UPGRADE_VERSION to explicitly request an upgrade")
	}
	if configError != nil {
		t.Fatalf("configuration initialization failed: %s", *configError)
	}
	if config.InfraProviderName != "aro" {
		t.Fatal("ARO upgrades require INFRA_PROVIDER=aro")
	}
	poolVersion := config.AROUpgradeNodePoolVersion
	if poolVersion == "" {
		poolVersion = config.AROUpgradeVersion
	}
	input, err := os.ReadFile(config.GetClusterYAMLPath())
	if err != nil {
		t.Fatalf("read generated cluster YAML: %v", err)
	}
	updated, resources, err := PrepareAROUpgrade(input, config.GetProvisionedControlPlaneName(), config.WorkloadClusterNamespace, config.AROUpgradeVersion, poolVersion)
	if err != nil {
		t.Fatalf("prepare upgrade: %v", err)
	}
	workloadConfig := getKubeconfigPath(config)
	if !FileExists(workloadConfig) {
		t.Fatalf("workload kubeconfig %s unavailable; run verification first", workloadConfig)
	}
	SetupKubeconfig(t, config)
	ctx, cancel := context.WithTimeout(context.Background(), config.ClusterDeploymentTimeout)
	defer cancel()
	managementArgs := []string{"--context", config.GetKubeContext(), "--namespace", config.WorkloadClusterNamespace, "--request-timeout=30s"}
	err = ReconcileAROUpgrade(ctx, resources, managementArgs, workloadConfig, 15*time.Second,
		func(ctx context.Context, stdin []byte, args ...string) (string, error) {
			return RunUpgradeCommand(t, ctx, stdin, args...)
		},
		func() error { return os.WriteFile(config.GetClusterYAMLPath(), updated, 0600) })
	if err != nil {
		t.Fatalf("upgrade: %v", err)
	}
}
