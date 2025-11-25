package test

import (
	"os"
)

// TestConfig holds configuration for ARO-CAPZ tests
type TestConfig struct {
	// Repository configuration
	RepoURL    string
	RepoBranch string
	RepoDir    string

	// Cluster configuration
	KindClusterName     string
	ClusterName         string
	ResourceGroup       string
	OpenShiftVersion    string
	Region              string
	AzureSubscription   string
	Environment         string
	User                string

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
		RepoDir:    GetEnvOrDefault("ARO_REPO_DIR", "/tmp/cluster-api-installer-aro"),

		// Cluster defaults
		KindClusterName:   GetEnvOrDefault("KIND_CLUSTER_NAME", "capz-stage"),
		ClusterName:       GetEnvOrDefault("CLUSTER_NAME", "test-cluster"),
		ResourceGroup:     GetEnvOrDefault("RESOURCE_GROUP", "test-rg"),
		OpenShiftVersion:  GetEnvOrDefault("OPENSHIFT_VERSION", "4.18"),
		Region:            GetEnvOrDefault("REGION", "eastus"),
		AzureSubscription: os.Getenv("AZURE_SUBSCRIPTION_NAME"),
		Environment:       GetEnvOrDefault("ENV", "stage"),
		User:              GetEnvOrDefault("USER", os.Getenv("USER")),

		// Paths
		ClusterctlBinPath: GetEnvOrDefault("CLUSTERCTL_BIN", "./bin/clusterctl"),
		ScriptsPath:       GetEnvOrDefault("SCRIPTS_PATH", "./scripts"),
		GenScriptPath:     GetEnvOrDefault("GEN_SCRIPT_PATH", "./doc/aro-hcp-scripts/aro-hcp-gen.sh"),
	}
}
