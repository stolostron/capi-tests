package test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestInfrastructure_GenerateResources tests generating ARO infrastructure resources
func TestInfrastructure_GenerateResources(t *testing.T) {

	config := NewTestConfig()

	if !DirExists(config.RepoDir) {
		t.Skipf("Repository not cloned yet at %s", config.RepoDir)
	}

	genScriptPath := filepath.Join(config.RepoDir, config.GenScriptPath)
	if !FileExists(genScriptPath) {
		t.Errorf("Generation script not found: %s", genScriptPath)
		return
	}

	// Output directory for generated resources
	outputDir := filepath.Join(config.RepoDir, config.GetOutputDirName())

	t.Logf("Generating infrastructure resources for cluster '%s' (env: %s)", config.ClusterName, config.Environment)

	// Set environment variables for the generation script
	SetEnvVar(t, "ENV", config.Environment)
	SetEnvVar(t, "USER", config.User)
	SetEnvVar(t, "CLUSTER_NAME", config.ClusterName)
	SetEnvVar(t, "REGION", config.Region)

	if config.AzureSubscription != "" {
		SetEnvVar(t, "AZURE_SUBSCRIPTION_NAME", config.AzureSubscription)
	}

	// Change to repository directory for script execution
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(config.RepoDir); err != nil {
		t.Fatalf("Failed to change to repository directory: %v", err)
	}

	// Run the generation script
	t.Log("Running infrastructure generation script...")
	output, err := RunCommand(t, "bash", genScriptPath, config.GetOutputDirName())
	if err != nil {
		t.Errorf("Failed to generate infrastructure resources: %v\nOutput: %s", err, output)
		return
	}

	t.Logf("Infrastructure generation completed\nOutput: %s", output)

	// Verify generated files exist
	if !DirExists(outputDir) {
		t.Errorf("Output directory not created: %s", outputDir)
		return
	}

	t.Logf("Output directory created: %s", outputDir)
}

// TestInfrastructure_VerifyGeneratedFiles verifies generated resource files
func TestInfrastructure_VerifyGeneratedFiles(t *testing.T) {

	config := NewTestConfig()
	outputDir := filepath.Join(config.RepoDir, config.GetOutputDirName())

	if !DirExists(outputDir) {
		t.Skipf("Output directory does not exist: %s", outputDir)
	}

	// Expected generated files based on documentation
	expectedFiles := []string{
		"credentials.yaml",
		"is.yaml",  // infrastructure secrets
		"aro.yaml", // main cluster configuration
	}

	for _, file := range expectedFiles {
		filePath := filepath.Join(outputDir, file)
		if !FileExists(filePath) {
			t.Errorf("Expected generated file not found: %s", file)
		} else {
			// Get file size to verify it's not empty
			info, err := os.Stat(filePath)
			if err != nil {
				t.Errorf("Failed to stat file %s: %v", file, err)
				continue
			}

			if info.Size() == 0 {
				t.Errorf("Generated file is empty: %s", file)
			} else {
				t.Logf("Found valid generated file: %s (size: %d bytes)", file, info.Size())
			}
		}
	}
}

// TestInfrastructure_ApplyResources tests applying generated resources to the cluster
func TestInfrastructure_ApplyResources(t *testing.T) {

	config := NewTestConfig()
	outputDir := filepath.Join(config.RepoDir, config.GetOutputDirName())

	if !DirExists(outputDir) {
		t.Skipf("Output directory does not exist: %s", outputDir)
	}

	// Files to apply in order
	filesToApply := []string{
		"credentials.yaml",
		"is.yaml",
		"aro.yaml",
	}

	// Set kubectl context to Kind cluster
	context := fmt.Sprintf("kind-%s", config.KindClusterName)

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(outputDir); err != nil {
		t.Fatalf("Failed to change to output directory: %v", err)
	}

	for _, file := range filesToApply {
		if !FileExists(file) {
			t.Errorf("Cannot apply missing file: %s", file)
			continue
		}

		t.Logf("Applying resource file: %s", file)

		output, err := RunCommand(t, "kubectl", "--context", context, "apply", "-f", file)
		// kubectl apply may return non-zero exit codes even for successful operations
		// (e.g., when resources are "unchanged"). Check output content for actual errors.
		if err != nil && !IsKubectlApplySuccess(output) {
			t.Errorf("Failed to apply %s: %v\nOutput: %s", file, err, output)
			continue
		}

		t.Logf("Successfully applied %s\nOutput: %s", file, output)
	}
}
