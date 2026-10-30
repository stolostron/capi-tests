package test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Deliberately outside the TestDeployment prefix: this negative case must be
// selected explicitly and must never accompany the positive deployment phases.
func TestAzurePrerequisites_MissingInfrastructure(t *testing.T) {
	config := NewTestConfig()
	if !config.AROTestMissingInfrastructure {
		t.Skip("Set ARO_TEST_MISSING_INFRA=true and select this test explicitly")
	}
	if !config.HasProvider("aro") {
		t.Fatal("missing-infrastructure scenario requires ARO")
	}
	timeout, err := time.ParseDuration(config.AROMissingInfrastructureTimeout)
	if err != nil || timeout < time.Second || timeout > 10*time.Minute {
		t.Fatal("ARO_MISSING_INFRA_TIMEOUT must be between 1s and 10m")
	}
	outputDir := filepath.Join(config.RepoDir, config.GetOutputDirName())
	data, err := os.ReadFile(filepath.Join(outputDir, config.ClusterYAML))
	if err != nil {
		t.Fatal(err)
	}
	plan, err := splitAROResources(data, config)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateAROPrerequisiteInventory(plan); err != nil {
		t.Fatal(err)
	}
	SetupKubeconfig(t, config)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	kubeContext, err := resolveAROKubeContext(ctx, t, config)
	if err != nil {
		t.Fatal(err)
	}
	commands := aroSplitCommands{run: func(ctx context.Context, args ...string) (string, error) {
		return RunCommandQuietContext(ctx, t, "kubectl", append([]string{"--context", kubeContext, "-n", config.WorkloadClusterNamespace, "--request-timeout=10s"}, args...)...)
	}}
	commands.apply = func(ctx context.Context, path string) error {
		_, err := commands.run(ctx, "apply", "-f", path)
		return err
	}
	diagnostic, err := runAROMissingInfrastructureScenario(ctx, config, plan, commands, outputDir, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("CAPZ rejected missing infrastructure explicitly: %s", diagnostic)
	t.Log("Negative scenario CRs are retained for inspection; remove them before running normal deployment")
}
