package test

import (
	"fmt"
	"os"
	"sync"
	"time"
)

const (
	// DefaultDeploymentTimeout is the default timeout for control plane deployment
	DefaultDeploymentTimeout = 45 * time.Minute
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
	ManagementClusterName string
	WorkloadClusterName   string
	ResourceGroup         string
	OpenShiftVersion      string
	Region                string
	AzureSubscription     string
	Environment           string
	User                  string

	// Paths
	ClusterctlBinPath string
	ScriptsPath       string
	GenScriptPath     string

	// Timeouts
	DeploymentTimeout time.Duration
}

// NewTestConfig creates a new test configuration with defaults
func NewTestConfig() *TestConfig {
	return &TestConfig{
		// Repository defaults
		RepoURL:    GetEnvOrDefault("ARO_REPO_URL", "https://github.com/RadekCap/cluster-api-installer.git"),
		RepoBranch: GetEnvOrDefault("ARO_REPO_BRANCH", "ARO-ASO"),
		RepoDir:    getDefaultRepoDir(),

		// Cluster defaults
		ManagementClusterName: GetEnvOrDefault("MANAGEMENT_CLUSTER_NAME", "capz-tests-stage"),
		WorkloadClusterName:   GetEnvOrDefault("WORKLOAD_CLUSTER_NAME", "capz-tests-cluster"),
		ResourceGroup:         GetEnvOrDefault("RESOURCE_GROUP", "capz-tests-rg"),
		OpenShiftVersion:      GetEnvOrDefault("OPENSHIFT_VERSION", "4.18"),
		Region:                GetEnvOrDefault("REGION", "uksouth"),
		AzureSubscription:     os.Getenv("AZURE_SUBSCRIPTION_NAME"),
		Environment:           GetEnvOrDefault("DEPLOYMENT_ENV", "stage"),
		User:                  GetEnvOrDefault("USER", os.Getenv("USER")),

		// Paths
		ClusterctlBinPath: GetEnvOrDefault("CLUSTERCTL_BIN", "./bin/clusterctl"),
		ScriptsPath:       GetEnvOrDefault("SCRIPTS_PATH", "./scripts"),
		GenScriptPath:     GetEnvOrDefault("GEN_SCRIPT_PATH", "./doc/aro-hcp-scripts/aro-hcp-gen.sh"),

		// Timeouts
		DeploymentTimeout: parseDeploymentTimeout(),
	}
}

// parseDeploymentTimeout parses the DEPLOYMENT_TIMEOUT environment variable.
// Returns the parsed duration or defaults to DefaultDeploymentTimeout.
// Logs a warning if the provided value is invalid.
func parseDeploymentTimeout() time.Duration {
	timeoutStr := os.Getenv("DEPLOYMENT_TIMEOUT")
	if timeoutStr == "" {
		return DefaultDeploymentTimeout
	}

	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: invalid DEPLOYMENT_TIMEOUT '%s', using default %v\n", timeoutStr, DefaultDeploymentTimeout)
		return DefaultDeploymentTimeout
	}
	return timeout
}

// GetOutputDirName returns the output directory name for generated infrastructure files
func (c *TestConfig) GetOutputDirName() string {
	return fmt.Sprintf("%s-%s", c.WorkloadClusterName, c.Environment)
}

// GetProvisionedClusterName returns the actual cluster name from the generated aro.yaml file.
// This is the name defined in the Cluster resource's metadata.name field, which may differ
// from WorkloadClusterName (the local configuration). Use this when interacting with
// the provisioned cluster via kubectl commands.
//
// Returns the extracted cluster name or WorkloadClusterName as fallback if aro.yaml
// doesn't exist yet (e.g., before YAML generation phase).
func (c *TestConfig) GetProvisionedClusterName() string {
	aroYAMLPath := fmt.Sprintf("%s/%s/aro.yaml", c.RepoDir, c.GetOutputDirName())

	name, err := ExtractClusterNameFromYAML(aroYAMLPath)
	if err != nil {
		// Fall back to WorkloadClusterName if aro.yaml doesn't exist or can't be parsed
		// This allows earlier phases (before YAML generation) to still work
		return c.WorkloadClusterName
	}

	return name
}

// GetAROYAMLPath returns the path to the generated aro.yaml file
func (c *TestConfig) GetAROYAMLPath() string {
	return fmt.Sprintf("%s/%s/aro.yaml", c.RepoDir, c.GetOutputDirName())
}
