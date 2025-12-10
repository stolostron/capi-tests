package test

import (
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

	t.Logf("Generating infrastructure resources for cluster '%s' (env: %s)", config.WorkloadClusterName, config.Environment)

	// Set environment variables for the generation script
	SetEnvVar(t, "DEPLOYMENT_ENV", config.Environment)
	SetEnvVar(t, "USER", config.User)
	SetEnvVar(t, "WORKLOAD_CLUSTER_NAME", config.WorkloadClusterName)
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
		// On error, show output for debugging (may contain sensitive info, but needed for troubleshooting)
		t.Errorf("Failed to generate infrastructure resources: %v\nOutput: %s", err, output)
		return
	}

	// Don't log full output as it may contain Azure resource IDs and other sensitive information
	t.Log("Infrastructure generation completed successfully")

	// Verify generated files exist
	if !DirExists(outputDir) {
		t.Errorf("Output directory not created: %s", outputDir)
		return
	}

	t.Logf("Output directory created: %s", outputDir)

	// Log paths of all generated files
	expectedFiles := []string{
		"credentials.yaml",
		"is.yaml",
		"aro.yaml",
	}
	for _, file := range expectedFiles {
		filePath := filepath.Join(outputDir, file)
		if FileExists(filePath) {
			info, err := os.Stat(filePath)
			if err != nil {
				t.Logf("Generated file: %s (unable to stat: %v)", filePath, err)
			} else {
				t.Logf("Generated file: %s (size: %d bytes)", filePath, info.Size())
			}
		} else {
			t.Errorf("Expected generated file not found: %s", filePath)
		}
	}
}

// TestInfrastructure_VerifyCredentialsYAML verifies credentials.yaml exists and is valid
func TestInfrastructure_VerifyCredentialsYAML(t *testing.T) {
	t.Log("Verifying credentials.yaml")

	config := NewTestConfig()
	outputDir := filepath.Join(config.RepoDir, config.GetOutputDirName())

	if !DirExists(outputDir) {
		t.Skipf("Output directory does not exist: %s", outputDir)
	}

	filePath := filepath.Join(outputDir, "credentials.yaml")
	if !FileExists(filePath) {
		t.Errorf("credentials.yaml not found")
		return
	}

	// Validate YAML syntax and structure
	if err := ValidateYAMLFile(filePath); err != nil {
		t.Errorf("credentials.yaml validation failed: %v", err)
		return
	}

	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Failed to stat credentials.yaml: %v", err)
	}

	t.Logf("credentials.yaml is valid YAML (size: %d bytes)", info.Size())
}

// TestInfrastructure_VerifyInfrastructureSecretsYAML verifies is.yaml exists and is valid
func TestInfrastructure_VerifyInfrastructureSecretsYAML(t *testing.T) {
	t.Log("Verifying is.yaml (infrastructure secrets)")

	config := NewTestConfig()
	outputDir := filepath.Join(config.RepoDir, config.GetOutputDirName())

	if !DirExists(outputDir) {
		t.Skipf("Output directory does not exist: %s", outputDir)
	}

	filePath := filepath.Join(outputDir, "is.yaml")
	if !FileExists(filePath) {
		t.Errorf("is.yaml not found")
		return
	}

	// Validate YAML syntax and structure
	if err := ValidateYAMLFile(filePath); err != nil {
		t.Errorf("is.yaml validation failed: %v", err)
		return
	}

	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Failed to stat is.yaml: %v", err)
	}

	t.Logf("is.yaml is valid YAML (size: %d bytes)", info.Size())
}

// TestInfrastructure_VerifyAROClusterYAML verifies aro.yaml exists and is valid
func TestInfrastructure_VerifyAROClusterYAML(t *testing.T) {
	t.Log("Verifying aro.yaml (ARO cluster configuration)")

	config := NewTestConfig()
	outputDir := filepath.Join(config.RepoDir, config.GetOutputDirName())

	if !DirExists(outputDir) {
		t.Skipf("Output directory does not exist: %s", outputDir)
	}

	filePath := filepath.Join(outputDir, "aro.yaml")
	if !FileExists(filePath) {
		t.Errorf("aro.yaml not found")
		return
	}

	// Validate YAML syntax and structure
	if err := ValidateYAMLFile(filePath); err != nil {
		t.Errorf("aro.yaml validation failed: %v", err)
		return
	}

	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Failed to stat aro.yaml: %v", err)
	}

	t.Logf("aro.yaml is valid YAML (size: %d bytes)", info.Size())
}
