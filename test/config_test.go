package test

import (
	"os"
	"strings"
	"testing"
	"time"
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

func TestParseDeploymentTimeout_Default(t *testing.T) {
	// Ensure DEPLOYMENT_TIMEOUT is not set
	originalValue := os.Getenv("DEPLOYMENT_TIMEOUT")
	_ = os.Unsetenv("DEPLOYMENT_TIMEOUT")
	defer func() {
		if originalValue != "" {
			_ = os.Setenv("DEPLOYMENT_TIMEOUT", originalValue)
		}
	}()

	timeout := parseDeploymentTimeout()
	if timeout != DefaultDeploymentTimeout {
		t.Errorf("Expected default timeout %v, got %v", DefaultDeploymentTimeout, timeout)
	}
	t.Logf("Default timeout: %v", timeout)
}

func TestParseDeploymentTimeout_ValidDuration(t *testing.T) {
	testCases := []struct {
		input    string
		expected time.Duration
	}{
		{"30m", 30 * time.Minute},
		{"1h", 1 * time.Hour},
		{"90m", 90 * time.Minute},
		{"2h30m", 2*time.Hour + 30*time.Minute},
	}

	originalValue := os.Getenv("DEPLOYMENT_TIMEOUT")
	defer func() {
		if originalValue != "" {
			_ = os.Setenv("DEPLOYMENT_TIMEOUT", originalValue)
		} else {
			_ = os.Unsetenv("DEPLOYMENT_TIMEOUT")
		}
	}()

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			_ = os.Setenv("DEPLOYMENT_TIMEOUT", tc.input)
			timeout := parseDeploymentTimeout()
			if timeout != tc.expected {
				t.Errorf("For input '%s', expected %v, got %v", tc.input, tc.expected, timeout)
			}
		})
	}
}

func TestParseDeploymentTimeout_InvalidDuration(t *testing.T) {
	originalValue := os.Getenv("DEPLOYMENT_TIMEOUT")
	defer func() {
		if originalValue != "" {
			_ = os.Setenv("DEPLOYMENT_TIMEOUT", originalValue)
		} else {
			_ = os.Unsetenv("DEPLOYMENT_TIMEOUT")
		}
	}()

	// Note: "-1h" is valid Go duration syntax (negative), so not included
	// Empty string is handled separately (returns default without warning)
	invalidValues := []string{"invalid", "abc", "45", "1x"}
	for _, val := range invalidValues {
		t.Run(val, func(t *testing.T) {
			_ = os.Setenv("DEPLOYMENT_TIMEOUT", val)
			timeout := parseDeploymentTimeout()
			if timeout != DefaultDeploymentTimeout {
				t.Errorf("For invalid input '%s', expected default %v, got %v", val, DefaultDeploymentTimeout, timeout)
			}
		})
	}
}

func TestIsKindMode(t *testing.T) {
	testCases := []struct {
		name     string
		envValue string
		expected bool
	}{
		{"not set", "", false},
		{"true", "true", true},
		{"false", "false", false},
		{"invalid", "yes", false},
	}

	originalValue := os.Getenv("USE_KIND")
	defer func() {
		if originalValue != "" {
			_ = os.Setenv("USE_KIND", originalValue)
		} else {
			_ = os.Unsetenv("USE_KIND")
		}
	}()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.envValue != "" {
				_ = os.Setenv("USE_KIND", tc.envValue)
			} else {
				_ = os.Unsetenv("USE_KIND")
			}
			config := NewTestConfig()
			if config.IsKindMode() != tc.expected {
				t.Errorf("IsKindMode() = %v, expected %v (USE_KIND=%q)", config.IsKindMode(), tc.expected, tc.envValue)
			}
		})
	}
}

func TestGetExpectedFiles(t *testing.T) {
	config := NewTestConfig()
	files := config.GetExpectedFiles()

	expected := []string{"credentials.yaml", "aro.yaml"}
	if len(files) != len(expected) {
		t.Fatalf("GetExpectedFiles() returned %d files, expected %d: %v", len(files), len(expected), files)
	}
	for i, file := range files {
		if file != expected[i] {
			t.Errorf("GetExpectedFiles()[%d] = %q, expected %q", i, file, expected[i])
		}
	}
}
