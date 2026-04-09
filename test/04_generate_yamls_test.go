package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// infrastructureGenerationSucceeded tracks whether TestInfrastructure_GenerateResources completed successfully
// within the current test process. This is set to true when generation completes or when existing files are
// detected (idempotency). Note: Verification tests now use file-based detection instead of this flag,
// so they work correctly when run in separate test invocations.
var infrastructureGenerationSucceeded bool

// TestInfrastructure_01_ValidateCredentials validates that required environment variables
// for YAML generation are set. This runs BEFORE gen.sh to provide clear error messages.
func TestInfrastructure_01_ValidateCredentials(t *testing.T) {
	// Check if config initialization failed
	if configError != nil {
		t.Fatalf("Configuration initialization failed: %s", *configError)
	}

	config := NewTestConfig()

	PrintTestHeader(t, "TestInfrastructure_ValidateCredentials",
		"Validate required environment variables for YAML generation")

	// Check if any provider has credential requirements to validate
	hasCredentials := false
	for _, p := range config.InfraProviders {
		if len(p.YAMLGenCredentials) > 0 {
			hasCredentials = true
			break
		}
	}
	if !hasCredentials {
		t.Skip("No provider credential environment variables to validate")
	}

	var allMissing []EnvVarRequirement
	for _, provider := range config.InfraProviders {
		if len(provider.YAMLGenCredentials) == 0 {
			continue
		}

		PrintToTTY("\n=== Validating %s credentials environment variables ===\n", provider.Name)
		t.Logf("Validating %s environment variables for YAML generation", provider.Name)

		var missing []EnvVarRequirement
		for _, envReq := range provider.YAMLGenCredentials {
			value := os.Getenv(envReq.Name)
			if value == "" {
				missing = append(missing, envReq)
				PrintToTTY("  ❌ %s: NOT SET\n", envReq.Name)
				if envReq.Desc != "" {
					PrintToTTY("     (%s)\n", envReq.Desc)
				}
			} else {
				// Never print credential values to logs - only indicate they're set
				PrintToTTY("  ✅ %s: (set)\n", envReq.Name)
				if envReq.Desc != "" {
					PrintToTTY("     (%s)\n", envReq.Desc)
				}
			}
		}

		if len(missing) > 0 {
			PrintToTTY("\n❌ %s environment validation FAILED\n", provider.Name)
			PrintToTTY("Missing environment variables:\n")
			for _, envReq := range missing {
				PrintToTTY("  - %s: %s\n", envReq.Name, envReq.Desc)
			}
			PrintToTTY("\n")
			allMissing = append(allMissing, missing...)
		} else {
			PrintToTTY("\n✅ %s environment validation PASSED\n\n", provider.Name)
			t.Logf("%s environment variables are properly configured", provider.Name)
		}
	}

	if len(allMissing) > 0 {
		PrintToTTY("❌ YAML generation will fail without these credentials.\n")
		PrintToTTY("Please set the missing environment variables before proceeding.\n\n")
		var missingNames []string
		for _, envReq := range allMissing {
			missingNames = append(missingNames, envReq.Name)
		}
		t.Fatalf("Required environment variables not set: %v", missingNames)
	}
}

// TestInfrastructure_GenerateResources tests generating ARO infrastructure resources
func TestInfrastructure_GenerateResources(t *testing.T) {

	config := NewTestConfig()

	if !DirExists(config.RepoDir) {
		t.Skipf("Repository not cloned yet at %s", config.RepoDir)
	}

	// Validate domain prefix length before attempting YAML generation
	// The domain prefix is derived from USER and DEPLOYMENT_ENV and must not exceed 15 characters
	if err := ValidateDomainPrefix(config.CAPIUser, config.Environment); err != nil {
		t.Fatalf("Domain prefix validation failed: %v", err)
	}
	t.Logf("Domain prefix validation passed: '%s' (%d chars)",
		GetDomainPrefix(config.CAPIUser, config.Environment),
		len(GetDomainPrefix(config.CAPIUser, config.Environment)))

	// Output directory for generated resources
	outputDir := filepath.Join(config.RepoDir, config.GetOutputDirName())

	// Check if all expected files already exist (idempotency)
	// This allows safe re-runs without regenerating existing infrastructure
	expectedFiles := config.GetExpectedFiles()

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
			clusterYAMLPath := filepath.Join(outputDir, config.ClusterYAML)

			// Check 1: Prefix mismatch (e.g., CAPI_USER changed)
			prefixMatches, existingPrefix := CheckYAMLConfigMatch(t, clusterYAMLPath, config.ClusterNamePrefix)
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
				existingNamespace, err := ExtractNamespaceFromYAML(clusterYAMLPath)
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
					if err := WriteDeploymentState(config); err != nil {
						t.Logf("Warning: failed to write deployment state: %v", err)
					} else {
						t.Logf("Deployment state saved (namespace: %s)", config.WorkloadClusterNamespace)
					}
					copyYAMLsToResultsDir(t, outputDir, expectedFiles)
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
	SetEnvVar(t, "USER", config.CAPIUser)
	SetEnvVar(t, "WORKLOAD_CLUSTER_NAME", config.WorkloadClusterName)
	SetEnvVar(t, config.RegionEnvVar, config.Region) // Provider-specific: REGION for ARO, AWS_REGION for ROSA
	SetEnvVar(t, "CS_CLUSTER_NAME", config.ClusterNamePrefix)
	SetEnvVar(t, "OCP_VERSION", config.OCPVersion)
	// ROSA gen.sh reads OPENSHIFT_VERSION (not OCP_VERSION) for the cluster version.
	// Set both so the test's configured version reaches the generation script.
	SetEnvVar(t, "OPENSHIFT_VERSION", config.OCPVersion)
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

		// Tag Azure resource group for parallel run cleanup queries.
		// Resource group may not exist yet (created by CAPI during deployment),
		// so failure here is expected — Phase 05 will retry after deployment.
		if len(config.AzureResourceTags) > 0 && CommandExists("az") {
			PrintToTTY("🏷️  Tagging resource group %s-resgroup...\n", config.ClusterNamePrefix)
			if err := TagAzureResourceGroup(t, config); err != nil {
				t.Logf("Resource group tagging deferred (RG may not exist yet, Phase 05 will retry): %v", err)
			}
		}

		// Copy generated YAMLs to results directory for visibility
		copyYAMLsToResultsDir(t, outputDir, expectedFiles)
	}
}

// copyYAMLsToResultsDir copies generated YAML files to the results directory for visibility.
// This ensures generated infrastructure definitions are available alongside other test artifacts
// (controller logs, test summaries) in the results directory.
// Secrets are redacted before writing — any Kubernetes Secret resource has its data/stringData
// values replaced with "***REDACTED***".
func copyYAMLsToResultsDir(t *testing.T, outputDir string, expectedFiles []string) {
	t.Helper()

	resultsDir := GetResultsDir()
	latestDir := "results/latest"
	copyToLatest := resultsDir != latestDir && DirExists(latestDir)

	for _, file := range expectedFiles {
		srcPath := filepath.Join(outputDir, file)
		if !FileExists(srcPath) {
			continue
		}

		// #nosec G304 -- path constructed from trusted outputDir and expected file names
		data, err := os.ReadFile(srcPath)
		if err != nil {
			t.Logf("Warning: failed to read %s for results copy: %v", srcPath, err)
			continue
		}

		redacted, didRedact := redactSecrets(data)
		if didRedact {
			t.Logf("Redacted secrets from %s before copying to results", file)
		}

		destPath := filepath.Join(resultsDir, file)
		if err := os.WriteFile(destPath, redacted, 0600); err != nil {
			t.Logf("Warning: failed to copy %s to results: %v", file, err)
		} else {
			t.Logf("Copied %s to results directory: %s", file, destPath)
		}

		// Also copy to results/latest if it differs from resultsDir
		if copyToLatest {
			latestPath := filepath.Join(latestDir, file)
			if err := os.WriteFile(latestPath, redacted, 0600); err != nil {
				t.Logf("Warning: failed to copy %s to latest: %v", file, err)
			}
		}
	}
}

// redactSecrets processes multi-document YAML content and redacts sensitive values.
// For Kubernetes Secret resources (kind: Secret), all values in data and stringData
// are replaced with "***REDACTED***". Other document types are passed through unchanged.
// Returns the redacted content and whether any redaction was performed.
func redactSecrets(content []byte) ([]byte, bool) {
	docs := strings.Split(string(content), "---")
	redacted := false
	var result []string

	for _, doc := range docs {
		trimmed := strings.TrimSpace(doc)
		if trimmed == "" {
			result = append(result, doc)
			continue
		}

		var parsed map[string]any
		if err := yaml.Unmarshal([]byte(trimmed), &parsed); err != nil {
			result = append(result, doc)
			continue
		}

		kind, _ := parsed["kind"].(string)
		if kind != "Secret" {
			result = append(result, doc)
			continue
		}

		// Redact data values
		if data, ok := parsed["data"].(map[string]any); ok {
			for key := range data {
				data[key] = "***REDACTED***"
			}
			redacted = true
		}

		// Redact stringData values
		if stringData, ok := parsed["stringData"].(map[string]any); ok {
			for key := range stringData {
				stringData[key] = "***REDACTED***"
			}
			redacted = true
		}

		out, err := yaml.Marshal(parsed)
		if err != nil {
			result = append(result, doc)
			continue
		}
		result = append(result, "\n"+string(out))
	}

	return []byte(strings.Join(result, "---")), redacted
}

// TestInfrastructure_VerifyCredentialsYAML verifies credentials.yaml exists and is valid
// This test uses file-based detection for idempotency - it will work correctly
// whether run in the same test invocation as GenerateResources or separately.
func TestInfrastructure_VerifyGeneratedYAMLs(t *testing.T) {
	config := NewTestConfig()
	outputDir := filepath.Join(config.RepoDir, config.GetOutputDirName())

	if !DirExists(outputDir) {
		t.Skipf("Output directory does not exist: %s", outputDir)
	}

	expectedFiles := config.GetExpectedFiles()
	if len(expectedFiles) == 0 {
		t.Skip("No expected files configured for provider")
	}

	t.Logf("Verifying %d YAML files for provider '%s'", len(expectedFiles), config.InfraProviderName)

	for _, filename := range expectedFiles {
		t.Run(filename, func(t *testing.T) {
			filePath := filepath.Join(outputDir, filename)

			if !FileExists(filePath) {
				t.Errorf("%s not found at %s.\n\n"+
					"This file should be generated by TestInfrastructure_GenerateResources.\n\n"+
					"To regenerate:\n"+
					"  go test -v ./test -run TestInfrastructure_GenerateResources\n\n"+
					"Or manually run the generation script:\n"+
					"  cd %s && bash %s %s",
					filename, filePath, config.RepoDir, config.GenScriptPath, config.GetOutputDirName())
				return
			}

			// Validate YAML syntax and structure
			if err := ValidateYAMLFile(filePath); err != nil {
				t.Errorf("%s validation failed: %v", filename, err)
				return
			}

			info, err := os.Stat(filePath)
			if err != nil {
				t.Fatalf("Failed to stat %s: %v", filename, err)
			}

			t.Logf("%s is valid YAML (size: %d bytes)", filename, info.Size())
		})
	}
}
