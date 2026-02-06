package test

import (
	"os"
	"path/filepath"
	"testing"
)

// infrastructureGenerationSucceeded tracks whether TestInfrastructure_GenerateResources completed successfully
// within the current test process. This is set to true when generation completes or when existing files are
// detected (idempotency). Note: Verification tests now use file-based detection instead of this flag,
// so they work correctly when run in separate test invocations.
var infrastructureGenerationSucceeded bool

// TestInfrastructure_GenerateResources tests generating ARO infrastructure resources
func TestInfrastructure_GenerateResources(t *testing.T) {

	config := NewTestConfig()

	if !DirExists(config.RepoDir) {
		t.Skipf("Repository not cloned yet at %s", config.RepoDir)
	}

	// Validate domain prefix length before attempting YAML generation
	// The domain prefix is derived from USER and DEPLOYMENT_ENV and must not exceed 15 characters
	if err := ValidateDomainPrefix(config.CAPZUser, config.Environment); err != nil {
		t.Fatalf("Domain prefix validation failed: %v", err)
	}
	t.Logf("Domain prefix validation passed: '%s' (%d chars)",
		GetDomainPrefix(config.CAPZUser, config.Environment),
		len(GetDomainPrefix(config.CAPZUser, config.Environment)))

	// Output directory for generated resources
	outputDir := filepath.Join(config.RepoDir, config.GetOutputDirName())

	// Check if all expected files already exist (idempotency)
	// This allows safe re-runs without regenerating existing infrastructure
	expectedFiles := []string{
		"credentials.yaml",
		"is.yaml",
		"aro.yaml",
	}

	if DirExists(outputDir) {
		allFilesExist := true
		var missingFiles []string
		for _, file := range expectedFiles {
			if !FileExists(filepath.Join(outputDir, file)) {
				allFilesExist = false
				missingFiles = append(missingFiles, file)
			}
		}
		if allFilesExist {
			// Check if existing YAMLs match current config before skipping
			aroYAMLPath := filepath.Join(outputDir, "aro.yaml")
			isYAMLPath := filepath.Join(outputDir, "is.yaml")

			// Check 1: Prefix mismatch (e.g., CAPZ_USER changed)
			prefixMatches, existingPrefix := CheckYAMLConfigMatch(t, aroYAMLPath, config.ClusterNamePrefix)
			if !prefixMatches {
				PrintToTTY("\n⚠️  Configuration prefix mismatch detected!\n")
				PrintToTTY("Existing YAMLs use prefix: %s\n", existingPrefix)
				PrintToTTY("Current config expects: %s\n", config.ClusterNamePrefix)
				PrintToTTY("Will regenerate infrastructure...\n\n")
				t.Logf("Prefix mismatch: existing=%s, expected=%s - will regenerate",
					existingPrefix, config.ClusterNamePrefix)
				// Fall through to regeneration (don't return)
			} else {
				// Check 2: Namespace mismatch (new test run with unique namespace)
				existingNamespace, err := ExtractNamespaceFromYAML(isYAMLPath)
				if err != nil {
					PrintToTTY("\n⚠️  Cannot read namespace from existing YAML: %v\n", err)
					PrintToTTY("Will regenerate infrastructure...\n\n")
					t.Logf("Cannot read namespace from existing YAML: %v - will regenerate", err)
					// Fall through to regeneration
				} else if existingNamespace != config.WorkloadClusterNamespace {
					PrintToTTY("\n⚠️  Namespace mismatch detected!\n")
					PrintToTTY("Existing YAMLs use namespace: %s\n", existingNamespace)
					PrintToTTY("Current config expects: %s\n", config.WorkloadClusterNamespace)
					PrintToTTY("Will regenerate infrastructure...\n\n")
					t.Logf("Namespace mismatch: existing=%s, expected=%s - will regenerate",
						existingNamespace, config.WorkloadClusterNamespace)
					// Fall through to regeneration
				} else {
					// Both prefix and namespace match - safe to skip generation
					PrintToTTY("\n=== Infrastructure YAML files already exist ===\n")
					PrintToTTY("✅ All expected files found in: %s\n", outputDir)
					PrintToTTY("✅ Prefix matches: %s\n", existingPrefix)
					PrintToTTY("✅ Namespace matches: %s\n", existingNamespace)
					for _, file := range expectedFiles {
						PrintToTTY("  ✅ %s\n", file)
					}
					PrintToTTY("\nSkipping generation (idempotent - already complete)\n")
					PrintToTTY("To force regeneration, delete the output directory:\n")
					PrintToTTY("  rm -rf %s\n\n", outputDir)
					t.Logf("Infrastructure already generated at %s, skipping", outputDir)
					infrastructureGenerationSucceeded = true
					return
				}
			}
		} else {
			// Partial state detected - log warning and regenerate
			PrintToTTY("\n⚠️  Output directory exists but missing files: %v\n", missingFiles)
			PrintToTTY("Will regenerate infrastructure...\n\n")
			t.Logf("Partial state detected, missing files: %v - will regenerate", missingFiles)
		}
	}

	genScriptPath := filepath.Join(config.RepoDir, config.GenScriptPath)
	if !FileExists(genScriptPath) {
		t.Errorf("Generation script not found: %s", genScriptPath)
		return
	}

	t.Logf("Generating infrastructure resources for cluster '%s' (env: %s)", config.WorkloadClusterName, config.Environment)

	// Set environment variables for the generation script
	SetEnvVar(t, "DEPLOYMENT_ENV", config.Environment)
	SetEnvVar(t, "USER", config.CAPZUser)
	SetEnvVar(t, "WORKLOAD_CLUSTER_NAME", config.WorkloadClusterName)
	SetEnvVar(t, "REGION", config.Region)
	SetEnvVar(t, "CS_CLUSTER_NAME", config.ClusterNamePrefix)
	SetEnvVar(t, "OCP_VERSION", config.OCPVersion)
	// Pass namespace as NAMESPACE env var for YAML generation script
	// This namespace will be embedded in generated YAMLs for Azure resources
	SetEnvVar(t, "NAMESPACE", config.WorkloadClusterNamespace)

	if config.AzureSubscriptionName != "" {
		SetEnvVar(t, "AZURE_SUBSCRIPTION_NAME", config.AzureSubscriptionName)
	}

	PrintToTTY("Workload cluster namespace: %s\n", config.WorkloadClusterNamespace)

	// Change to repository directory for script execution
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Logf("Warning: failed to change back to original directory: %v", err)
		}
	}()

	if err := os.Chdir(config.RepoDir); err != nil {
		t.Fatalf("Failed to change to repository directory: %v", err)
	}

	// Run the generation script
	PrintToTTY("\n=== Generating infrastructure resources ===\n")
	PrintToTTY("Running infrastructure generation script: %s %s\n", genScriptPath, config.GetOutputDirName())
	t.Log("Running infrastructure generation script...")
	output, err := RunCommand(t, "bash", genScriptPath, config.GetOutputDirName())
	if err != nil {
		// On error, show output for debugging (may contain sensitive info, but needed for troubleshooting)
		t.Errorf("Failed to generate infrastructure resources: %v\nOutput: %s", err, output)
		return
	}

	// Don't log full output as it may contain Azure resource IDs and other sensitive information
	PrintToTTY("✅ Infrastructure generation completed successfully\n")
	t.Log("Infrastructure generation completed successfully")

	// Verify generated files exist
	if !DirExists(outputDir) {
		t.Errorf("Output directory not created: %s", outputDir)
		return
	}

	PrintToTTY("Output directory created: %s\n", outputDir)
	t.Logf("Output directory created: %s", outputDir)

	// Log paths of all generated files (expectedFiles defined earlier for idempotency check)
	for _, file := range expectedFiles {
		filePath := filepath.Join(outputDir, file)
		if FileExists(filePath) {
			info, err := os.Stat(filePath)
			if err != nil {
				PrintToTTY("  ⚠️  Generated file: %s (unable to stat: %v)\n", filePath, err)
				t.Logf("Generated file: %s (unable to stat: %v)", filePath, err)
			} else {
				PrintToTTY("  ✅ Generated file: %s (%d bytes)\n", filePath, info.Size())
				t.Logf("Generated file: %s (size: %d bytes)", filePath, info.Size())
			}
		} else {
			PrintToTTY("  ❌ Expected generated file not found: %s\n", filePath)
			t.Errorf("Expected generated file not found: %s", filePath)
		}
	}
	PrintToTTY("\n")

	// Mark generation as successful only if no errors occurred
	if !t.Failed() {
		infrastructureGenerationSucceeded = true

		// Save deployment state to track namespace and other config for cleanup
		if err := WriteDeploymentState(config); err != nil {
			t.Logf("Warning: failed to write deployment state: %v", err)
		} else {
			PrintToTTY("📝 Deployment state saved to %s\n", DeploymentStateFile)
			t.Logf("Deployment state saved (namespace: %s)", config.WorkloadClusterNamespace)
		}
	}
}

// TestInfrastructure_VerifyCredentialsYAML verifies credentials.yaml exists and is valid
// This test uses file-based detection for idempotency - it will work correctly
// whether run in the same test invocation as GenerateResources or separately.
func TestInfrastructure_VerifyCredentialsYAML(t *testing.T) {
	t.Log("Verifying credentials.yaml")

	config := NewTestConfig()
	outputDir := filepath.Join(config.RepoDir, config.GetOutputDirName())

	if !DirExists(outputDir) {
		t.Skipf("Output directory does not exist: %s", outputDir)
	}

	filePath := filepath.Join(outputDir, "credentials.yaml")
	if !FileExists(filePath) {
		t.Errorf("credentials.yaml not found at %s.\n\n"+
			"This file should be generated by TestInfrastructure_GenerateResources.\n\n"+
			"To regenerate:\n"+
			"  go test -v ./test -run TestInfrastructure_GenerateResources\n\n"+
			"Or manually run the generation script:\n"+
			"  cd %s && bash %s %s",
			filePath, config.RepoDir, config.GenScriptPath, config.GetOutputDirName())
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
// This test uses file-based detection for idempotency - it will work correctly
// whether run in the same test invocation as GenerateResources or separately.
func TestInfrastructure_VerifyInfrastructureSecretsYAML(t *testing.T) {
	t.Log("Verifying is.yaml (infrastructure secrets)")

	config := NewTestConfig()
	outputDir := filepath.Join(config.RepoDir, config.GetOutputDirName())

	if !DirExists(outputDir) {
		t.Skipf("Output directory does not exist: %s", outputDir)
	}

	filePath := filepath.Join(outputDir, "is.yaml")
	if !FileExists(filePath) {
		t.Errorf("is.yaml (infrastructure secrets) not found at %s.\n\n"+
			"This file should be generated by TestInfrastructure_GenerateResources.\n\n"+
			"To regenerate:\n"+
			"  go test -v ./test -run TestInfrastructure_GenerateResources\n\n"+
			"Or manually run the generation script:\n"+
			"  cd %s && bash %s %s",
			filePath, config.RepoDir, config.GenScriptPath, config.GetOutputDirName())
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
// This test uses file-based detection for idempotency - it will work correctly
// whether run in the same test invocation as GenerateResources or separately.
func TestInfrastructure_VerifyAROClusterYAML(t *testing.T) {
	t.Log("Verifying aro.yaml (ARO cluster configuration)")

	config := NewTestConfig()
	outputDir := filepath.Join(config.RepoDir, config.GetOutputDirName())

	if !DirExists(outputDir) {
		t.Skipf("Output directory does not exist: %s", outputDir)
	}

	filePath := filepath.Join(outputDir, "aro.yaml")
	if !FileExists(filePath) {
		t.Errorf("aro.yaml (ARO cluster configuration) not found at %s.\n\n"+
			"This file should be generated by TestInfrastructure_GenerateResources.\n\n"+
			"To regenerate:\n"+
			"  go test -v ./test -run TestInfrastructure_GenerateResources\n\n"+
			"Or manually run the generation script:\n"+
			"  cd %s && bash %s %s",
			filePath, config.RepoDir, config.GenScriptPath, config.GetOutputDirName())
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
