package test

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSetup_CloneRepository tests cloning the cluster-api-installer repository
func TestSetup_CloneRepository(t *testing.T) {
	config := NewTestConfig()

	// Check if directory already exists
	if DirExists(config.RepoDir) {
		t.Logf("Repository directory already exists at %s", config.RepoDir)

		// Verify it's a git repository
		gitDir := filepath.Join(config.RepoDir, ".git")
		if !DirExists(gitDir) {
			t.Errorf("Directory exists but is not a git repository: %s", config.RepoDir)
			return
		}

		t.Log("Using existing repository")
		return
	}

	// Clone the repository
	t.Logf("Cloning repository from %s (branch: %s)", config.RepoURL, config.RepoBranch)

	output, err := RunCommand(t, "git", "clone", "-b", config.RepoBranch, config.RepoURL, config.RepoDir)
	if err != nil {
		t.Errorf("Failed to clone repository: %v\nOutput: %s", err, output)
		return
	}

	t.Logf("Repository cloned successfully to %s", config.RepoDir)
}

// TestSetup_VerifyRepositoryStructure verifies the cloned repository has required scripts
func TestSetup_VerifyRepositoryStructure(t *testing.T) {
	config := NewTestConfig()

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
