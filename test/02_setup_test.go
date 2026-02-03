package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSetup_CloneRepository tests cloning the cluster-api-installer repository.
// The repository is needed for YAML generation even in external cluster mode.
func TestSetup_CloneRepository(t *testing.T) {
	config := NewTestConfig()

	// Note: We still need the repo in external cluster mode for YAML generation (Phase 04)
	// Only the Kind cluster deployment (Phase 03) is skipped

	// Check if directory already exists (idempotency check)
	if DirExists(config.RepoDir) {
		t.Logf("Repository directory already exists at %s", config.RepoDir)

		// Verify it's a git repository
		gitDir := filepath.Join(config.RepoDir, ".git")
		if !DirExists(gitDir) {
			t.Errorf("Directory exists but is not a git repository: %s", config.RepoDir)
			return
		}

		// Validate repository integrity by checking if HEAD is valid
		// This detects corrupted clones from interrupted operations
		output, err := RunCommandQuiet(t, "git", "-C", config.RepoDir, "rev-parse", "HEAD")
		headSHA := strings.TrimSpace(output)
		if err != nil || headSHA == "" {
			t.Logf("Warning: Repository at %s may be corrupted (git rev-parse HEAD failed)", config.RepoDir)
			t.Logf("Consider deleting and re-cloning: rm -rf %s", config.RepoDir)
			// Don't fail - let subsequent tests determine if repo is usable
		} else {
			t.Logf("Repository HEAD: %s", headSHA[:min(12, len(headSHA))])
		}

		// Register the existing repository for tracking in test output
		RegisterClonedRepository(config.RepoURL, config.RepoBranch, config.RepoDir)

		t.Log("Using existing repository (idempotent - skipping clone)")
		return
	}

	// Clone the repository
	t.Logf("Cloning repository from %s (branch: %s)", config.RepoURL, config.RepoBranch)

	output, err := RunCommand(t, "git", "clone", "-b", config.RepoBranch, config.RepoURL, config.RepoDir)
	if err != nil {
		t.Errorf("Failed to clone repository: %v\nOutput: %s", err, output)
		return
	}

	// Register the cloned repository for tracking in test output
	RegisterClonedRepository(config.RepoURL, config.RepoBranch, config.RepoDir)

	t.Logf("Repository cloned successfully to %s", config.RepoDir)
}

// TestSetup_VerifyRepositoryStructure verifies the cloned repository has required scripts
func TestSetup_VerifyRepositoryStructure(t *testing.T) {
	config := NewTestConfig()

	// Note: Repo is needed in external cluster mode for YAML generation

	if !DirExists(config.RepoDir) {
		t.Skipf("Repository not cloned yet at %s", config.RepoDir)
	}

	// Check for scripts actually used by tests
	requiredScripts := []string{
		"scripts/deploy-charts-kind-capz.sh", // Used by TestKindCluster_Deploy (03_cluster_test.go)
		"doc/aro-hcp-scripts/aro-hcp-gen.sh", // Used by TestInfrastructure_GenerateResources (04_generate_yamls_test.go)
	}

	for _, requiredScript := range requiredScripts {
		fullPath := filepath.Join(config.RepoDir, requiredScript)
		if !FileExists(fullPath) {
			t.Errorf("Required script does not exist: %s", fullPath)
		} else {
			t.Logf("Found required script: %s", requiredScript)
		}
	}
}

// TestSetup_ScriptPermissions verifies scripts have executable permissions
func TestSetup_ScriptPermissions(t *testing.T) {
	config := NewTestConfig()

	// Note: Repo is needed in external cluster mode for YAML generation

	if !DirExists(config.RepoDir) {
		t.Skipf("Repository not cloned yet at %s", config.RepoDir)
	}

	scripts := []string{
		"scripts/deploy-charts-kind-capz.sh",
		"doc/aro-hcp-scripts/aro-hcp-gen.sh",
	}

	for _, script := range scripts {
		scriptPath := filepath.Join(config.RepoDir, script)

		if !FileExists(scriptPath) {
			t.Errorf("Script not found: %s", scriptPath)
			continue
		}

		// Check if file is executable
		info, err := os.Stat(scriptPath)
		if err != nil {
			t.Errorf("Failed to stat script %s: %v", script, err)
			continue
		}

		mode := info.Mode()
		if mode&0111 == 0 {
			t.Logf("Script %s is not executable, making it executable", script)
			if err := os.Chmod(scriptPath, mode|0111); err != nil {
				t.Errorf("Failed to make script executable: %v", err)
			}
		} else {
			t.Logf("Script %s has executable permissions", script)
		}
	}
}
