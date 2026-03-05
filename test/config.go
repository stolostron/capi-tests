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

	// DefaultNodeReadyTimeout is the default timeout for waiting for worker nodes to become available.
	// In ARO HCP, the control plane becomes ready before worker nodes are provisioned.
	// The AROMachinePool creates nodes after the HcpOpenShiftCluster is up.
	DefaultNodeReadyTimeout = 30 * time.Minute

	// DefaultCAPZUser is the default user identifier for CAPZ resources.
	// Used in ClusterNamePrefix (for resource group naming) and User field.
	// Extracted to a constant to ensure consistency across all usages.
	DefaultCAPZUser = "rcapd"

	// DefaultDeploymentEnv is the default deployment environment identifier.
	// Used in ClusterNamePrefix and Environment field.
	DefaultDeploymentEnv = "stage"

	// MCE component names as used in mce.spec.overrides.components
	MCEComponentCAPI = "cluster-api"

	// DefaultHelmInstallTimeout is the default timeout for Helm install operations
	// (e.g., cert-manager installation during Kind cluster setup).
	DefaultHelmInstallTimeout = 10 * time.Minute

	// DefaultControllerTimeout is the default timeout for waiting for a controller to become ready.
	DefaultControllerTimeout = 10 * time.Minute

	// CAPI core constants (provider-independent)

	// CAPIControllerDeployment is the CAPI core controller deployment name.
	CAPIControllerDeployment = "capi-controller-manager"

	// CAPIWebhookService is the CAPI core webhook service name.
	CAPIWebhookService = "capi-webhook-service"

	// CAPIWebhookPort is the CAPI core webhook service port.
	CAPIWebhookPort = 443

	// CAPIPodSelector is the label selector for CAPI core pods.
	CAPIPodSelector = "cluster.x-k8s.io/provider=cluster-api"

	// CAPIDeploymentChartName is the Helm chart argument for CAPI core.
	CAPIDeploymentChartName = "cluster-api"
)

// ControllerDef describes a controller deployment to validate.
type ControllerDef struct {
	DisplayName    string        // human-readable name (e.g., "CAPZ", "ASO")
	Namespace      string        // Kubernetes namespace (e.g., "capz-system")
	DeploymentName string        // deployment name (e.g., "capz-controller-manager")
	PodSelector    string        // label selector for pods (e.g., "cluster.x-k8s.io/provider=infrastructure-azure")
	Timeout        time.Duration // readiness timeout (0 = DefaultControllerTimeout)
}

// WebhookDef describes a webhook service to validate.
type WebhookDef struct {
	DisplayName string // human-readable name (e.g., "CAPZ", "ASO")
	Namespace   string // Kubernetes namespace
	ServiceName string // Kubernetes service name (e.g., "capz-webhook-service")
	Port        int    // service port (e.g., 443)
}

// CredentialSecretDef describes a provider's credential secret.
type CredentialSecretDef struct {
	Name            string   // secret name (e.g., "aso-controller-settings")
	Namespace       string   // namespace containing the secret
	RequiredFields  []string // fields that must be present and non-empty in the secret
	RequiredEnvVars []string // env vars that must be set for this check to run (skip if missing)
}

// InfraProvider defines an infrastructure provider's configuration.
// Each provider has controllers, webhooks, and optionally a credential secret.
type InfraProvider struct {
	Name             string               // provider identifier (e.g., "aro", "rosa")
	Controllers      []ControllerDef      // controllers to validate
	Webhooks         []WebhookDef         // webhooks to validate
	CredentialSecret *CredentialSecretDef // nil if no credential secret needed
	DeploymentCharts []string             // chart args for deploy-charts.sh
	MCEComponentName string               // MCE component name for this provider
	RequiredTools    []string             // CLI tools required for this provider (e.g., "az" for ARO, "aws" for ROSA)
	RequiredScripts  []string             // repo-relative scripts this provider needs (validated in Phase 2)
}

// NewAzureProvider returns the InfraProvider configuration for Azure (CAPZ/ASO).
// The namespace parameter is the resolved namespace for CAPZ/ASO controllers
// (e.g., "capz-system" for Kind mode, "multicluster-engine" for MCE mode).
func NewAzureProvider(namespace string) InfraProvider {
	return InfraProvider{
		Name: "aro",
		Controllers: []ControllerDef{
			{
				DisplayName:    "CAPZ",
				Namespace:      namespace,
				DeploymentName: "capz-controller-manager",
				PodSelector:    "cluster.x-k8s.io/provider=infrastructure-azure",
			},
			{
				DisplayName:    "ASO",
				Namespace:      namespace,
				DeploymentName: "azureserviceoperator-controller-manager",
				PodSelector:    "app.kubernetes.io/name=azure-service-operator",
			},
		},
		Webhooks: []WebhookDef{
			{DisplayName: "CAPZ", Namespace: namespace, ServiceName: "capz-webhook-service", Port: 443},
			{DisplayName: "ASO", Namespace: namespace, ServiceName: "azureserviceoperator-webhook-service", Port: 443},
		},
		CredentialSecret: &CredentialSecretDef{
			Name:      "aso-controller-settings",
			Namespace: namespace,
			RequiredFields: []string{
				"AZURE_TENANT_ID",
				"AZURE_SUBSCRIPTION_ID",
				"AZURE_CLIENT_ID",
				"AZURE_CLIENT_SECRET",
			},
			RequiredEnvVars: []string{
				"AZURE_CLIENT_ID",
				"AZURE_CLIENT_SECRET",
			},
		},
		DeploymentCharts: []string{"cluster-api-provider-azure"},
		MCEComponentName: "cluster-api-provider-azure-preview",
		RequiredTools:    []string{"az"},
		RequiredScripts:  []string{"scripts/deploy-charts.sh", "scripts/aro-hcp/gen.sh"},
	}
}

// NewAWSProvider returns the InfraProvider configuration for AWS (CAPA).
// The namespace parameter is the resolved namespace for the CAPA controller
// (e.g., "capa-system" for Kind mode, "multicluster-engine" for MCE mode).
func NewAWSProvider(namespace string) InfraProvider {
	return InfraProvider{
		Name: "rosa",
		Controllers: []ControllerDef{
			{
				DisplayName:    "CAPA",
				Namespace:      namespace,
				DeploymentName: "capa-controller-manager",
				PodSelector:    "cluster.x-k8s.io/provider=infrastructure-aws",
			},
		},
		Webhooks: []WebhookDef{
			{DisplayName: "CAPA", Namespace: namespace, ServiceName: "capa-webhook-service", Port: 443},
		},
		CredentialSecret: &CredentialSecretDef{
			Name:            "capa-manager-bootstrap-credentials",
			Namespace:       namespace,
			RequiredFields:  []string{"credentials"},
			RequiredEnvVars: []string{"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY"},
		},
		DeploymentCharts: []string{"cluster-api-provider-aws"},
		MCEComponentName: "cluster-api-provider-aws",
		RequiredTools:    []string{"aws"},
		RequiredScripts:  []string{"scripts/deploy-charts.sh", "scripts/rosa-hcp/gen.sh"},
	}
}

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
// Format: {prefix}-{YYYYMMDD-HHMMSS} (e.g., "capz-test-20260203-140812" or "capa-test-20260203-140812")
// This namespace is passed as $NAMESPACE to the YAML generation script and used for
// all resource checks.
//
// The defaultPrefix parameter is provider-specific: "capz-test" for ARO, "capa-test" for ROSA.
//
// Resolution order:
// 1. WORKLOAD_CLUSTER_NAMESPACE env var (explicit override for resume scenarios)
// 2. Existing deployment state file in RepoDir (auto-resume from previous run)
// 3. Generate unique namespace using WORKLOAD_CLUSTER_NAMESPACE_PREFIX (default: provider-specific prefix)
//
// The auto-resume from deployment state ensures that subsequent test phases
// (run as separate go test invocations) use the same namespace as YAML generation.
func getWorkloadClusterNamespace(defaultPrefix string) string {
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
		// #nosec G304 - path constructed from repo directory and fixed filename (.deployment-state.json)
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
		prefix := GetEnvOrDefault("WORKLOAD_CLUSTER_NAMESPACE_PREFIX", defaultPrefix)
		timestamp := time.Now().Format("20060102-150405")
		workloadClusterNamespace = fmt.Sprintf("%s-%s", prefix, timestamp)
	})

	return workloadClusterNamespace
}

// TestConfig holds configuration for CAPI tests
type TestConfig struct {
	// Repository configuration
	RepoURL    string
	RepoBranch string
	RepoDir    string

	// Cluster configuration
	ManagementClusterName    string
	WorkloadClusterName      string
	ClusterNamePrefix        string // Used as CS_CLUSTER_NAME for YAML generation; resource group becomes ${ClusterNamePrefix}-resgroup
	OCPVersion               string
	Region                   string
	AzureSubscriptionName    string // Azure subscription name (from AZURE_SUBSCRIPTION_NAME env var)
	Environment              string
	CAPZUser                 string // User identifier for CAPZ resources (from CAPZ_USER env var)
	WorkloadClusterNamespace string // Namespace for workload cluster resources on management cluster (unique per test run)
	TestLabelPrefix          string // Provider-specific label prefix for test namespaces (e.g., "capz-test" for ARO, "capa-test" for ROSA)
	CAPINamespace            string // Namespace for CAPI controller (default: "capi-system", or "multicluster-engine" when USE_K8S=true)
	CAPZNamespace            string // Namespace for CAPZ/ASO controllers (default: "capz-system", or "multicluster-engine" when USE_K8S=true)

	// External cluster configuration
	// UseKubeconfig is the path to an external kubeconfig file.
	// When set, the test suite runs in "external cluster mode":
	// - Skips Kind cluster creation
	// - Validates pre-installed controllers
	// - Uses current-context from the kubeconfig
	UseKubeconfig string

	// UseKind enables Kind deployment mode (USE_KIND=true).
	// When true, creates a local Kind management cluster with CAPI/CAPZ/ASO controllers.
	UseKind bool

	// Paths
	ClusterctlBinPath string
	ScriptsPath       string
	GenScriptPath     string

	// Timeouts
	DeploymentTimeout    time.Duration
	ASOControllerTimeout time.Duration
	HelmInstallTimeout   time.Duration

	// Infrastructure providers
	// InfraProviderName is the selected infrastructure provider ("aro" or "rosa").
	// Set via INFRA_PROVIDER env var. Default: "aro".
	InfraProviderName string
	// InfraProviders holds the list of infrastructure provider configurations.
	// Each provider defines its controllers, webhooks, and credential secrets.
	// Initialized based on INFRA_PROVIDER env var: "aro" (CAPZ/ASO) or "rosa" (CAPA).
	InfraProviders []InfraProvider

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
		_ = os.Setenv("USE_K8S", "true") // #nosec G104 - os.Setenv with fixed key/value cannot fail in practice
	}

	// Determine infrastructure provider
	infraProviderName := GetEnvOrDefault("INFRA_PROVIDER", "aro")

	// Parse ASO controller timeout unconditionally so that
	// ASOControllerTimeout is always a valid duration (used by ValidateAllConfigurations).
	asoTimeout := parseASOControllerTimeout()

	// Resolve provider-specific namespace, cluster names, and build provider config
	var providerNamespace string
	var infraProviders []InfraProvider
	var defaultGenScriptPath string
	var defaultMgmtCluster string
	var defaultWorkloadCluster string
	var testLabelPrefix string

	switch infraProviderName {
	case "rosa":
		providerNamespace = getControllerNamespace("CAPA_NAMESPACE", "capa-system")
		infraProviders = []InfraProvider{NewAWSProvider(providerNamespace)}
		defaultGenScriptPath = "./scripts/rosa-hcp/gen.sh"
		defaultMgmtCluster = "capa-tests-stage"
		defaultWorkloadCluster = "capa-tests-cluster"
		testLabelPrefix = "capa-test"
	default: // "aro"
		infraProviderName = "aro" // normalize unknown values
		providerNamespace = getControllerNamespace("CAPZ_NAMESPACE", "capz-system")
		azureProvider := NewAzureProvider(providerNamespace)
		for i := range azureProvider.Controllers {
			if azureProvider.Controllers[i].DisplayName == "ASO" {
				azureProvider.Controllers[i].Timeout = asoTimeout
			}
		}
		infraProviders = []InfraProvider{azureProvider}
		defaultGenScriptPath = "./scripts/aro-hcp/gen.sh"
		defaultMgmtCluster = "capz-tests-stage"
		defaultWorkloadCluster = "capz-tests-cluster"
		testLabelPrefix = "capz-test"
	}

	return &TestConfig{
		// Repository defaults
		RepoURL:    GetEnvOrDefault("ARO_REPO_URL", "https://github.com/marek-veber/cluster-api-installer"),
		RepoBranch: GetEnvOrDefault("ARO_REPO_BRANCH", "capi-tests"),
		RepoDir:    getDefaultRepoDir(),

		// Cluster defaults
		ManagementClusterName:    GetEnvOrDefault("MANAGEMENT_CLUSTER_NAME", defaultMgmtCluster),
		WorkloadClusterName:      GetEnvOrDefault("WORKLOAD_CLUSTER_NAME", defaultWorkloadCluster),
		ClusterNamePrefix:        GetEnvOrDefault("CS_CLUSTER_NAME", fmt.Sprintf("%s-%s", GetEnvOrDefault("CAPZ_USER", DefaultCAPZUser), GetEnvOrDefault("DEPLOYMENT_ENV", DefaultDeploymentEnv))),
		OCPVersion:               GetEnvOrDefault("OCP_VERSION", "4.20"),
		Region:                   GetEnvOrDefault("REGION", "uksouth"),
		AzureSubscriptionName:    os.Getenv("AZURE_SUBSCRIPTION_NAME"),
		Environment:              GetEnvOrDefault("DEPLOYMENT_ENV", DefaultDeploymentEnv),
		CAPZUser:                 GetEnvOrDefault("CAPZ_USER", DefaultCAPZUser),
		WorkloadClusterNamespace: getWorkloadClusterNamespace(testLabelPrefix),
		TestLabelPrefix:          testLabelPrefix,
		CAPINamespace:            getControllerNamespace("CAPI_NAMESPACE", "capi-system"),
		CAPZNamespace:            providerNamespace,

		// External cluster
		UseKubeconfig: useKubeconfig,

		// Kind mode
		UseKind: os.Getenv("USE_KIND") == "true",

		// Paths
		ClusterctlBinPath: GetEnvOrDefault("CLUSTERCTL_BIN", "./bin/clusterctl"),
		ScriptsPath:       GetEnvOrDefault("SCRIPTS_PATH", "./scripts"),
		GenScriptPath:     GetEnvOrDefault("GEN_SCRIPT_PATH", defaultGenScriptPath),

		// Timeouts
		DeploymentTimeout:    parseDeploymentTimeout(),
		ASOControllerTimeout: asoTimeout,
		HelmInstallTimeout:   parseHelmInstallTimeout(),

		// Infrastructure providers
		InfraProviderName: infraProviderName,
		InfraProviders:    infraProviders,

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

// parseHelmInstallTimeout parses the HELM_INSTALL_TIMEOUT environment variable.
// Returns the parsed duration or defaults to DefaultHelmInstallTimeout.
// This timeout is passed to deploy scripts for Helm install operations (e.g., cert-manager).
func parseHelmInstallTimeout() time.Duration {
	timeoutStr := os.Getenv("HELM_INSTALL_TIMEOUT")
	if timeoutStr == "" {
		return DefaultHelmInstallTimeout
	}

	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: invalid HELM_INSTALL_TIMEOUT '%s', using default %v\n", timeoutStr, DefaultHelmInstallTimeout)
		return DefaultHelmInstallTimeout
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

// GetProvisionedAROControlPlaneName returns the actual AROControlPlane resource name
// from the generated aro.yaml file. Falls back to GetProvisionedClusterName() + "-control-plane"
// if aro.yaml doesn't exist or doesn't contain an AROControlPlane resource.
func (c *TestConfig) GetProvisionedAROControlPlaneName() string {
	aroYAMLPath := fmt.Sprintf("%s/%s/aro.yaml", c.RepoDir, c.GetOutputDirName())

	name, err := ExtractAROControlPlaneNameFromYAML(aroYAMLPath)
	if err != nil {
		return c.GetProvisionedClusterName() + "-control-plane"
	}

	return name
}

// GetProvisionedMachinePoolName returns the actual MachinePool resource name
// from the generated aro.yaml file. Falls back to GetProvisionedClusterName() + "-pool"
// if aro.yaml doesn't exist or doesn't contain a MachinePool resource.
func (c *TestConfig) GetProvisionedMachinePoolName() string {
	aroYAMLPath := fmt.Sprintf("%s/%s/aro.yaml", c.RepoDir, c.GetOutputDirName())

	name, err := ExtractMachinePoolNameFromYAML(aroYAMLPath)
	if err != nil {
		return c.GetProvisionedClusterName() + "-pool"
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

// IsKindMode returns true when Kind deployment mode is enabled (USE_KIND=true).
func (c *TestConfig) IsKindMode() bool {
	return c.UseKind
}

// GetExpectedFiles returns the list of expected YAML files for infrastructure deployment.
// The generation script produces credentials.yaml (Azure credentials for CAPZ/ASO) and
// aro.yaml (Cluster, AROControlPlane, AROCluster with ASO resources, MachinePool).
// Infrastructure resources are embedded in AROCluster.spec.resources[] within aro.yaml.
func (c *TestConfig) GetExpectedFiles() []string {
	return []string{
		"credentials.yaml",
		"aro.yaml",
	}
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

// AllControllers returns all infrastructure controllers across all providers,
// prepended with the CAPI core controller. Used for version queries, log collection,
// and readiness checks that need to iterate over every controller.
func (c *TestConfig) AllControllers() []ControllerDef {
	controllers := []ControllerDef{
		{DisplayName: "CAPI", Namespace: c.CAPINamespace, DeploymentName: CAPIControllerDeployment, PodSelector: CAPIPodSelector},
	}
	for _, p := range c.InfraProviders {
		controllers = append(controllers, p.Controllers...)
	}
	return controllers
}

// AllWebhooks returns all webhooks across all providers,
// prepended with the CAPI core webhook.
func (c *TestConfig) AllWebhooks() []WebhookDef {
	webhooks := []WebhookDef{
		{DisplayName: "CAPI", Namespace: c.CAPINamespace, ServiceName: CAPIWebhookService, Port: CAPIWebhookPort},
	}
	for _, p := range c.InfraProviders {
		webhooks = append(webhooks, p.Webhooks...)
	}
	return webhooks
}

// AllNamespaces returns deduplicated namespaces across CAPI core and all providers.
func (c *TestConfig) AllNamespaces() []string {
	seen := map[string]bool{c.CAPINamespace: true}
	namespaces := []string{c.CAPINamespace}
	for _, p := range c.InfraProviders {
		for _, ctrl := range p.Controllers {
			if !seen[ctrl.Namespace] {
				seen[ctrl.Namespace] = true
				namespaces = append(namespaces, ctrl.Namespace)
			}
		}
	}
	return namespaces
}

// DeploymentChartArgs returns all chart arguments for deploy-charts.sh,
// starting with CAPI core and appending each provider's charts.
func (c *TestConfig) DeploymentChartArgs() []string {
	args := []string{CAPIDeploymentChartName}
	for _, p := range c.InfraProviders {
		args = append(args, p.DeploymentCharts...)
	}
	return args
}

// HasProvider returns true if the named infrastructure provider is in the active provider list.
// Use this to guard provider-specific test logic (e.g., config.HasProvider("aro")).
func (c *TestConfig) HasProvider(name string) bool {
	for _, p := range c.InfraProviders {
		if p.Name == name {
			return true
		}
	}
	return false
}

// AllRequiredTools returns deduplicated CLI tools required across all providers.
func (c *TestConfig) AllRequiredTools() []string {
	seen := map[string]bool{}
	var tools []string
	for _, p := range c.InfraProviders {
		for _, tool := range p.RequiredTools {
			if !seen[tool] {
				seen[tool] = true
				tools = append(tools, tool)
			}
		}
	}
	return tools
}

// AllRequiredScripts returns deduplicated repo-relative scripts required across all providers.
func (c *TestConfig) AllRequiredScripts() []string {
	seen := map[string]bool{}
	var scripts []string
	for _, p := range c.InfraProviders {
		for _, script := range p.RequiredScripts {
			if !seen[script] {
				seen[script] = true
				scripts = append(scripts, script)
			}
		}
	}
	return scripts
}
