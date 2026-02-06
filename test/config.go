package test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	// DefaultDeploymentTimeout is the default timeout for control plane deployment
	DefaultDeploymentTimeout = 60 * time.Minute

	// DefaultASOControllerTimeout is the default timeout for ASO controller manager to become ready.
	// ASO may take longer than other controllers due to its CRD initialization sequence:
	// scanning existing CRDs, applying missing ones, and restarting to pick up new CRDs.
	DefaultASOControllerTimeout = 10 * time.Minute

	// DefaultMCEEnablementTimeout is the default timeout for waiting after MCE component enablement.
	// MCE components need time to deploy controllers, pull images, and initialize.
	DefaultMCEEnablementTimeout = 15 * time.Minute

	// DefaultCAPZUser is the default user identifier for CAPZ resources.
	// Used in ClusterNamePrefix (for resource group naming) and User field.
	// Extracted to a constant to ensure consistency across all usages.
	DefaultCAPZUser = "rcapx"

	// DefaultDeploymentEnv is the default deployment environment identifier.
	// Used in ClusterNamePrefix and Environment field.
	DefaultDeploymentEnv = "stage"

	// MCE component names as used in mce.spec.overrides.components
	MCEComponentCAPI = "cluster-api"
	MCEComponentCAPZ = "cluster-api-provider-azure-preview"
)

var (
	defaultRepoDir     string
	defaultRepoDirOnce sync.Once

	workloadClusterNamespace     string
	workloadClusterNamespaceOnce sync.Once
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

// getWorkloadClusterNamespace returns the namespace for workload cluster resources.
// The namespace is unique per test run, combining the configured prefix with a timestamp.
// Format: {prefix}-{YYYYMMDD-HHMMSS} (e.g., "capz-test-20260203-140812")
// This namespace is passed as $NAMESPACE to the YAML generation script and used for
// all Azure resource checks.
//
// Resolution order:
// 1. WORKLOAD_CLUSTER_NAMESPACE env var (explicit override for resume scenarios)
// 2. Existing deployment state file in RepoDir (auto-resume from previous run)
// 3. Generate unique namespace using WORKLOAD_CLUSTER_NAMESPACE_PREFIX (default: "capz-test")
//
// The auto-resume from deployment state ensures that subsequent test phases
// (run as separate go test invocations) use the same namespace as YAML generation.
func getWorkloadClusterNamespace() string {
	workloadClusterNamespaceOnce.Do(func() {
		// Check if a full namespace is explicitly provided (for resume scenarios)
		if ns := os.Getenv("WORKLOAD_CLUSTER_NAMESPACE"); ns != "" {
			workloadClusterNamespace = ns
			return
		}

		// Check for existing deployment state file in RepoDir
		// This handles the case where YAML generation ran in a previous test invocation
		// and we need to use the same namespace for subsequent phases
		repoDir := getDefaultRepoDir()
		stateFilePath := filepath.Join(repoDir, ".deployment-state.json")
		if data, err := os.ReadFile(stateFilePath); err == nil {
			var state struct {
				WorkloadClusterNamespace string `json:"workload_cluster_namespace"`
			}
			if err := json.Unmarshal(data, &state); err == nil && state.WorkloadClusterNamespace != "" {
				workloadClusterNamespace = state.WorkloadClusterNamespace
				return
			}
		}

		// Generate unique namespace with timestamp for fresh runs
		prefix := GetEnvOrDefault("WORKLOAD_CLUSTER_NAMESPACE_PREFIX", "capz-test")
		timestamp := time.Now().Format("20060102-150405")
		workloadClusterNamespace = fmt.Sprintf("%s-%s", prefix, timestamp)
	})

	return workloadClusterNamespace
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
	ClusterNamePrefix     string // Used as CS_CLUSTER_NAME for YAML generation; resource group becomes ${ClusterNamePrefix}-resgroup
	OCPVersion            string
	Region                string
	AzureSubscriptionName string // Azure subscription name (from AZURE_SUBSCRIPTION_NAME env var)
	Environment           string
	CAPZUser                 string // User identifier for CAPZ resources (from CAPZ_USER env var)
	WorkloadClusterNamespace string // Namespace for workload cluster resources on management cluster (unique per test run)
	CAPINamespace            string // Namespace for CAPI controller (default: "capi-system", or "multicluster-engine" when USE_K8S=true)
	CAPZNamespace         string // Namespace for CAPZ/ASO controllers (default: "capz-system", or "multicluster-engine" when USE_K8S=true)

	// External cluster configuration
	// UseKubeconfig is the path to an external kubeconfig file.
	// When set, the test suite runs in "external cluster mode":
	// - Skips Kind cluster creation
	// - Validates pre-installed controllers
	// - Uses current-context from the kubeconfig
	UseKubeconfig string

	// Paths
	ClusterctlBinPath string
	ScriptsPath       string
	GenScriptPath     string

	// Timeouts
	DeploymentTimeout    time.Duration
	ASOControllerTimeout time.Duration

	// MCE (MultiClusterEngine) configuration
	// MCEAutoEnable controls whether to automatically enable MCE CAPI/CAPZ components
	// if they are not found on an external cluster. Default: true when IsExternalCluster().
	MCEAutoEnable bool
	// MCEEnablementTimeout is the timeout for waiting after MCE component enablement.
	// Controllers need time to be deployed, images pulled, and pods started.
	MCEEnablementTimeout time.Duration
}

// NewTestConfig creates a new test configuration with defaults
func NewTestConfig() *TestConfig {
	useKubeconfig := os.Getenv("USE_KUBECONFIG")

	// When using external kubeconfig, default to MCE namespaces (USE_K8S=true)
	// This triggers multicluster-engine namespace for all controllers
	if useKubeconfig != "" && os.Getenv("USE_K8S") == "" {
		os.Setenv("USE_K8S", "true")
	}

	return &TestConfig{
		// Repository defaults
		RepoURL:    GetEnvOrDefault("ARO_REPO_URL", "https://github.com/stolostron/cluster-api-installer"),
		RepoBranch: GetEnvOrDefault("ARO_REPO_BRANCH", "main"),
		RepoDir:    getDefaultRepoDir(),

		// Cluster defaults
		ManagementClusterName: GetEnvOrDefault("MANAGEMENT_CLUSTER_NAME", "capz-tests-stage"),
		WorkloadClusterName:   GetEnvOrDefault("WORKLOAD_CLUSTER_NAME", "capz-tests-cluster"),
		ClusterNamePrefix:     GetEnvOrDefault("CS_CLUSTER_NAME", fmt.Sprintf("%s-%s", GetEnvOrDefault("CAPZ_USER", DefaultCAPZUser), GetEnvOrDefault("DEPLOYMENT_ENV", DefaultDeploymentEnv))),
		OCPVersion:            GetEnvOrDefault("OCP_VERSION", "4.21"),
		Region:                GetEnvOrDefault("REGION", "uksouth"),
		AzureSubscriptionName: os.Getenv("AZURE_SUBSCRIPTION_NAME"),
		Environment:           GetEnvOrDefault("DEPLOYMENT_ENV", DefaultDeploymentEnv),
		CAPZUser:                 GetEnvOrDefault("CAPZ_USER", DefaultCAPZUser),
		WorkloadClusterNamespace: getWorkloadClusterNamespace(),
		CAPINamespace:            getControllerNamespace("CAPI_NAMESPACE", "capi-system"),
		CAPZNamespace:         getControllerNamespace("CAPZ_NAMESPACE", "capz-system"),

		// External cluster
		UseKubeconfig: useKubeconfig,

		// Paths
		ClusterctlBinPath: GetEnvOrDefault("CLUSTERCTL_BIN", "./bin/clusterctl"),
		ScriptsPath:       GetEnvOrDefault("SCRIPTS_PATH", "./scripts"),
		GenScriptPath:     GetEnvOrDefault("GEN_SCRIPT_PATH", "./doc/aro-hcp-scripts/aro-hcp-gen.sh"),

		// Timeouts
		DeploymentTimeout:    parseDeploymentTimeout(),
		ASOControllerTimeout: parseASOControllerTimeout(),

		// MCE configuration
		MCEAutoEnable:        parseMCEAutoEnable(useKubeconfig),
		MCEEnablementTimeout: parseMCEEnablementTimeout(),
	}
}

// getControllerNamespace returns the namespace for a controller based on configuration.
// If USE_K8S=true, returns "multicluster-engine" (K8S deployment mode).
// Otherwise, checks the specific env var (e.g., CAPI_NAMESPACE) and falls back to defaultNS.
func getControllerNamespace(envVar, defaultNS string) string {
	// Check if USE_K8S mode is enabled - all controllers use multicluster-engine namespace
	if os.Getenv("USE_K8S") == "true" {
		return "multicluster-engine"
	}

	// Check for specific namespace override
	if ns := os.Getenv(envVar); ns != "" {
		return ns
	}

	return defaultNS
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

// parseASOControllerTimeout parses the ASO_CONTROLLER_TIMEOUT environment variable.
// Returns the parsed duration or defaults to DefaultASOControllerTimeout.
// Logs a warning if the provided value is invalid.
func parseASOControllerTimeout() time.Duration {
	timeoutStr := os.Getenv("ASO_CONTROLLER_TIMEOUT")
	if timeoutStr == "" {
		return DefaultASOControllerTimeout
	}

	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: invalid ASO_CONTROLLER_TIMEOUT '%s', using default %v\n", timeoutStr, DefaultASOControllerTimeout)
		return DefaultASOControllerTimeout
	}
	return timeout
}

// parseMCEAutoEnable parses the MCE_AUTO_ENABLE environment variable.
// Returns true (default) when using external kubeconfig, false otherwise.
// Can be explicitly set to "false" to disable auto-enablement.
func parseMCEAutoEnable(useKubeconfig string) bool {
	envVal := os.Getenv("MCE_AUTO_ENABLE")
	if envVal != "" {
		return envVal == "true"
	}
	// Default to true only when using external kubeconfig
	return useKubeconfig != ""
}

// parseMCEEnablementTimeout parses the MCE_ENABLEMENT_TIMEOUT environment variable.
// Returns the parsed duration or defaults to DefaultMCEEnablementTimeout.
// Logs a warning if the provided value is invalid.
func parseMCEEnablementTimeout() time.Duration {
	timeoutStr := os.Getenv("MCE_ENABLEMENT_TIMEOUT")
	if timeoutStr == "" {
		return DefaultMCEEnablementTimeout
	}

	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: invalid MCE_ENABLEMENT_TIMEOUT '%s', using default %v\n", timeoutStr, DefaultMCEEnablementTimeout)
		return DefaultMCEEnablementTimeout
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

// IsExternalCluster returns true when using an external kubeconfig file
// instead of creating a local Kind cluster.
func (c *TestConfig) IsExternalCluster() bool {
	return c.UseKubeconfig != ""
}

// GetKubeContext returns the kubectl context to use for the management cluster.
// For external clusters, extracts current-context from the kubeconfig file.
// For Kind clusters, returns "kind-{ManagementClusterName}".
func (c *TestConfig) GetKubeContext() string {
	if c.IsExternalCluster() {
		return ExtractCurrentContext(c.UseKubeconfig)
	}
	return fmt.Sprintf("kind-%s", c.ManagementClusterName)
}
