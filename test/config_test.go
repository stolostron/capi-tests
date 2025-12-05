package test

import (
	"os"
	"strings"
	"testing"
)

func TestGetDefaultRepoDir_EnvVariable(t *testing.T) {
	// This test must check current behavior, not set environment
	// because sync.Once means the first call wins for the entire test process

	config := NewTestConfig()

	// Check if ARO_REPO_DIR is currently set
	if envDir := os.Getenv("ARO_REPO_DIR"); envDir != "" {
		// If env var is set, config should use it
		if config.RepoDir != envDir {
			t.Errorf("When ARO_REPO_DIR is set, RepoDir should be %s, got: %s", envDir, config.RepoDir)
		}
		t.Logf("ARO_REPO_DIR is set to: %s", envDir)
	} else {
		// If env var is not set, should generate stable path
		if !strings.Contains(config.RepoDir, "cluster-api-installer-aro") {
			t.Errorf("When ARO_REPO_DIR not set, should generate stable path, got: %s", config.RepoDir)
		}
		if !strings.HasPrefix(config.RepoDir, os.TempDir()) {
			t.Errorf("Generated path should be in temp directory (%s), got: %s", os.TempDir(), config.RepoDir)
		}
		t.Logf("Generated stable path: %s", config.RepoDir)
	}
}

func TestGetDefaultRepoDir_Consistency(t *testing.T) {
	// Create multiple configs
	config1 := NewTestConfig()
	config2 := NewTestConfig()
	config3 := NewTestConfig()

	// All should return the same path due to sync.Once
	if config1.RepoDir != config2.RepoDir {
		t.Errorf("getDefaultRepoDir() not consistent across calls: %s != %s", config1.RepoDir, config2.RepoDir)
	}

	if config1.RepoDir != config3.RepoDir {
		t.Errorf("getDefaultRepoDir() not consistent across calls: %s != %s", config1.RepoDir, config3.RepoDir)
	}

	t.Logf("All configs consistently use: %s", config1.RepoDir)
}

func TestGetDefaultRepoDir_PathFormat(t *testing.T) {
	config := NewTestConfig()

	// If ARO_REPO_DIR env var is set, skip format validation
	if os.Getenv("ARO_REPO_DIR") != "" {
		t.Skip("ARO_REPO_DIR is set, skipping format validation")
	}

	// Verify the path contains expected prefix
	if !strings.Contains(config.RepoDir, "cluster-api-installer-aro") {
		t.Errorf("Generated path should contain 'cluster-api-installer-aro' prefix, got: %s", config.RepoDir)
	}

	// Verify it's in the temp directory
	if !strings.HasPrefix(config.RepoDir, os.TempDir()) {
		t.Errorf("Generated path should be in temp directory (%s), got: %s", os.TempDir(), config.RepoDir)
	}

	// Verify stable path format (no PID or timestamp)
	// Path format: <os.TempDir()>/cluster-api-installer-aro (e.g., /tmp/cluster-api-installer-aro on Linux, /var/folders/.../cluster-api-installer-aro on macOS)
	expectedPath := os.TempDir() + "/cluster-api-installer-aro"
	if config.RepoDir != expectedPath {
		t.Errorf("Generated path should be %s, got: %s", expectedPath, config.RepoDir)
	}

	t.Logf("Path format validated: %s", config.RepoDir)
}
