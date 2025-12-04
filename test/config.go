package test

import (
	"fmt"
	"os"
	"sync"
)

var (
	defaultRepoDir     string
	defaultRepoDirOnce sync.Once
)

// getDefaultRepoDir returns the default repository directory path.
// The path is stable across test runs to allow sequential execution via separate
// make commands (test-prereq, test-setup, test-kind, etc.).
func getDefaultRepoDir() string {
	defaultRepoDirOnce.Do(func() {
		if dir := os.Getenv("ARO_REPO_DIR"); dir != "" {
			defaultRepoDir = dir
			return
		}

		// Use a stable path that persists across test invocations
		// This allows make test-setup and make test-kind to share the same repository
		defaultRepoDir = fmt.Sprintf("%s/cluster-api-installer-aro", os.TempDir())
	})

	return defaultRepoDir
}

// TestConfig holds configuration for ARO-CAPZ tests
type TestConfig struct {
	// Repository configuration
	RepoURL    string
	RepoBranch string
	RepoDir    string

	// Cluster configuration
	KindClusterName   string
	ClusterName       string
	ResourceGroup     string
	OpenShiftVersion  string
	Region            string
	AzureSubscription string
	Environment       string
	User              string

	// Paths
	ClusterctlBinPath string
	ScriptsPath       string
	GenScriptPath     string
}

// NewTestConfig creates a new test configuration with defaults
func NewTestConfig() *TestConfig {
	return &TestConfig{
		// Repository defaults
		RepoURL:    GetEnvOrDefault("ARO_REPO_URL", "https://github.com/RadekCap/cluster-api-installer.git"),
		RepoBranch: GetEnvOrDefault("ARO_REPO_BRANCH", "ARO-ASO"),
		RepoDir:    getDefaultRepoDir(),

		// Cluster defaults
		KindClusterName:   GetEnvOrDefault("KIND_CLUSTER_NAME", "capz-tests-stage"),
		ClusterName:       GetEnvOrDefault("CLUSTER_NAME", "capz-tests-cluster"),
		ResourceGroup:     GetEnvOrDefault("RESOURCE_GROUP", "capz-tests-rg"),
		OpenShiftVersion:  GetEnvOrDefault("OPENSHIFT_VERSION", "4.18"),
		Region:            GetEnvOrDefault("REGION", "uksouth"),
		AzureSubscription: os.Getenv("AZURE_SUBSCRIPTION_NAME"),
		Environment:       GetEnvOrDefault("ENV", "stage"),
		User:              GetEnvOrDefault("USER", os.Getenv("USER")),

		// Paths
		ClusterctlBinPath: GetEnvOrDefault("CLUSTERCTL_BIN", "./bin/clusterctl"),
		ScriptsPath:       GetEnvOrDefault("SCRIPTS_PATH", "./scripts"),
		GenScriptPath:     GetEnvOrDefault("GEN_SCRIPT_PATH", "./doc/aro-hcp-scripts/aro-hcp-gen.sh"),
	}
}

// GetOutputDirName returns the output directory name for generated infrastructure files
func (c *TestConfig) GetOutputDirName() string {
	return fmt.Sprintf("%s-%s", c.ClusterName, c.Environment)
}
