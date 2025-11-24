package test

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSetup_CloneRepository tests cloning the cluster-api-installer repository
func TestSetup_CloneRepository(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping repository clone in short mode")
	}

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

// TestSetup_VerifyRepositoryStructure verifies the cloned repository has expected structure
func TestSetup_VerifyRepositoryStructure(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping repository structure verification in short mode")
	}

	config := NewTestConfig()

	if !DirExists(config.RepoDir) {
		t.Skipf("Repository not cloned yet at %s", config.RepoDir)
	}

	// Check for expected directories and files
	expectedPaths := []string{
		"scripts",
		"doc/aro-hcp-scripts",
		"doc/ARO-capz.md",
		"doc/aro-hcp-scripts/aro-hcp-gen.sh",
	}

	for _, expectedPath := range expectedPaths {
		fullPath := filepath.Join(config.RepoDir, expectedPath)
		if !FileExists(fullPath) && !DirExists(fullPath) {
			t.Errorf("Expected path does not exist: %s", fullPath)
		} else {
			t.Logf("Found expected path: %s", expectedPath)
		}
	}
}

// TestSetup_ScriptPermissions verifies scripts have executable permissions
func TestSetup_ScriptPermissions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping script permissions check in short mode")
	}

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
