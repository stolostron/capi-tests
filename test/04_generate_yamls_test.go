package test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// expectedYAMLFiles returns the list of YAML files expected to be generated
func expectedYAMLFiles() []string {
	return []string{
		"credentials.yaml",
		"is.yaml",
		"aro.yaml",
	}
}

// verifyYAMLFileExists checks if a YAML file exists in the output directory
// Returns the file path and true if exists, empty string and false otherwise
func verifyYAMLFileExists(t *testing.T, filename string) (string, bool) {
	t.Helper()
	config := NewTestConfig()
	outputDir := filepath.Join(config.RepoDir, config.GetOutputDirName())

	if !DirExists(outputDir) {
		t.Skipf("Output directory does not exist: %s", outputDir)
		return "", false
	}

	filePath := filepath.Join(outputDir, filename)
	if !FileExists(filePath) {
		t.Errorf("%s not found", filename)
		return "", false
	}

	return filePath, true
}

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
}

// TestInfrastructure_GenerateCredentialsYAML tests generation of credentials.yaml
func TestInfrastructure_GenerateCredentialsYAML(t *testing.T) {
	t.Log("Verifying generation of credentials.yaml")

	filePath, ok := verifyYAMLFileExists(t, "credentials.yaml")
	if !ok {
		return
	}

	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Failed to stat credentials.yaml: %v", err)
	}

	if info.Size() == 0 {
		t.Errorf("Generated credentials.yaml is empty")
	} else {
		t.Logf("Successfully generated credentials.yaml (size: %d bytes)", info.Size())
	}
}

// TestInfrastructure_GenerateInfrastructureSecretsYAML tests generation of is.yaml (infrastructure secrets)
func TestInfrastructure_GenerateInfrastructureSecretsYAML(t *testing.T) {
	t.Log("Verifying generation of is.yaml (infrastructure secrets)")

	filePath, ok := verifyYAMLFileExists(t, "is.yaml")
	if !ok {
		return
	}

	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Failed to stat is.yaml: %v", err)
	}

	if info.Size() == 0 {
		t.Errorf("Generated is.yaml is empty")
	} else {
		t.Logf("Successfully generated is.yaml (size: %d bytes)", info.Size())
	}
}

// TestInfrastructure_GenerateAROClusterYAML tests generation of aro.yaml (ARO cluster configuration)
func TestInfrastructure_GenerateAROClusterYAML(t *testing.T) {
	t.Log("Verifying generation of aro.yaml (ARO cluster configuration)")

	filePath, ok := verifyYAMLFileExists(t, "aro.yaml")
	if !ok {
		return
	}

	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Failed to stat aro.yaml: %v", err)
	}

	if info.Size() == 0 {
		t.Errorf("Generated aro.yaml is empty")
	} else {
		t.Logf("Successfully generated aro.yaml (size: %d bytes)", info.Size())
	}
}

// TestInfrastructure_VerifyGeneratedFiles verifies all generated resource files exist
func TestInfrastructure_VerifyGeneratedFiles(t *testing.T) {

	config := NewTestConfig()
	outputDir := filepath.Join(config.RepoDir, config.GetOutputDirName())

	if !DirExists(outputDir) {
		t.Skipf("Output directory does not exist: %s", outputDir)
	}

	// Get expected files from centralized list
	expectedFiles := expectedYAMLFiles()

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

// TestInfrastructure_VerifyCredentialsYAML verifies credentials.yaml exists and is valid
func TestInfrastructure_VerifyCredentialsYAML(t *testing.T) {
	t.Log("Verifying credentials.yaml")

	filePath, ok := verifyYAMLFileExists(t, "credentials.yaml")
	if !ok {
		return
	}

	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Failed to stat credentials.yaml: %v", err)
	}

	if info.Size() == 0 {
		t.Errorf("credentials.yaml is empty")
	} else {
		t.Logf("credentials.yaml is valid (size: %d bytes)", info.Size())
	}
}

// TestInfrastructure_VerifyInfrastructureSecretsYAML verifies is.yaml exists and is valid
func TestInfrastructure_VerifyInfrastructureSecretsYAML(t *testing.T) {
	t.Log("Verifying is.yaml (infrastructure secrets)")

	filePath, ok := verifyYAMLFileExists(t, "is.yaml")
	if !ok {
		return
	}

	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Failed to stat is.yaml: %v", err)
	}

	if info.Size() == 0 {
		t.Errorf("is.yaml is empty")
	} else {
		t.Logf("is.yaml is valid (size: %d bytes)", info.Size())
	}
}

// TestInfrastructure_VerifyAROClusterYAML verifies aro.yaml exists and is valid
func TestInfrastructure_VerifyAROClusterYAML(t *testing.T) {
	t.Log("Verifying aro.yaml (ARO cluster configuration)")

	filePath, ok := verifyYAMLFileExists(t, "aro.yaml")
	if !ok {
		return
	}

	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Failed to stat aro.yaml: %v", err)
	}

	if info.Size() == 0 {
		t.Errorf("aro.yaml is empty")
	} else {
		t.Logf("aro.yaml is valid (size: %d bytes)", info.Size())
	}
}

// TestInfrastructure_ApplyResources tests applying generated resources to the cluster
func TestInfrastructure_ApplyResources(t *testing.T) {

	config := NewTestConfig()
	outputDir := filepath.Join(config.RepoDir, config.GetOutputDirName())

	if !DirExists(outputDir) {
		t.Skipf("Output directory does not exist: %s", outputDir)
	}

	// Get files to apply from centralized list
	filesToApply := expectedYAMLFiles()

	// Set kubectl context to Kind cluster
	context := fmt.Sprintf("kind-%s", config.ManagementClusterName)

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
			// On error, show output for debugging (may contain sensitive info, but needed for troubleshooting)
			t.Errorf("Failed to apply %s: %v\nOutput: %s", file, err, output)
			continue
		}

		// Don't log full kubectl output as it may contain Azure subscription IDs and resource details
		t.Logf("Successfully applied %s", file)
	}
}

// TestInfrastructure_ApplyCredentialsYAML tests applying credentials.yaml to the cluster
func TestInfrastructure_ApplyCredentialsYAML(t *testing.T) {
	file := "credentials.yaml"
	t.Logf("Applying %s", file)

	filePath, ok := verifyYAMLFileExists(t, file)
	if !ok {
		return
	}

	config := NewTestConfig()
	context := fmt.Sprintf("kind-%s", config.ManagementClusterName)
	output, err := RunCommand(t, "kubectl", "--context", context, "apply", "-f", filePath)

	if err != nil && !IsKubectlApplySuccess(output) {
		t.Errorf("Failed to apply %s: %v\nOutput: %s", file, err, output)
		return
	}

	t.Logf("Successfully applied %s", file)
}

// TestInfrastructure_ApplyInfrastructureSecretsYAML tests applying is.yaml to the cluster
func TestInfrastructure_ApplyInfrastructureSecretsYAML(t *testing.T) {
	file := "is.yaml"
	t.Logf("Applying %s (infrastructure secrets)", file)

	filePath, ok := verifyYAMLFileExists(t, file)
	if !ok {
		return
	}

	config := NewTestConfig()
	context := fmt.Sprintf("kind-%s", config.ManagementClusterName)
	output, err := RunCommand(t, "kubectl", "--context", context, "apply", "-f", filePath)

	if err != nil && !IsKubectlApplySuccess(output) {
		t.Errorf("Failed to apply %s: %v\nOutput: %s", file, err, output)
		return
	}

	t.Logf("Successfully applied %s", file)
}

// TestInfrastructure_ApplyAROClusterYAML tests applying aro.yaml to the cluster
func TestInfrastructure_ApplyAROClusterYAML(t *testing.T) {
	file := "aro.yaml"
	t.Logf("Applying %s (ARO cluster configuration)", file)

	filePath, ok := verifyYAMLFileExists(t, file)
	if !ok {
		return
	}

	config := NewTestConfig()
	context := fmt.Sprintf("kind-%s", config.ManagementClusterName)
	output, err := RunCommand(t, "kubectl", "--context", context, "apply", "-f", filePath)

	if err != nil && !IsKubectlApplySuccess(output) {
		t.Errorf("Failed to apply %s: %v\nOutput: %s", file, err, output)
		return
	}

	t.Logf("Successfully applied %s", file)
}
