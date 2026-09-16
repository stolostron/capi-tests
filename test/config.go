package test

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

var (
	// configError stores the error message if NewTestConfig() encountered a fatal error during initialization.
	// If nil, configuration succeeded. Check with (configError != nil) in first test.
	configError *string
)

const (
	// DefaultClusterDeploymentTimeout is the default timeout for the in-code polling loop
	// that waits for the workload cluster to become ready during deployment (Phase 05).
	DefaultClusterDeploymentTimeout = 60 * time.Minute

	// DefaultClusterDeletionTimeout is the default timeout for the in-code polling loop
	// that waits for the workload cluster to be fully deleted (Phase 07).
	DefaultClusterDeletionTimeout = 60 * time.Minute

	// DefaultDeploymentTimeout is the legacy default timeout for control plane deployment.
	// Deprecated: Use DefaultClusterDeploymentTimeout instead.
	DefaultDeploymentTimeout = DefaultClusterDeploymentTimeout

	// DefaultASOControllerTimeout is the default timeout for ASO controller manager to become ready.
	// ASO may take longer than other controllers due to its CRD initialization sequence:
	// scanning existing CRDs, applying missing ones, and restarting to pick up new CRDs.
	DefaultASOControllerTimeout = 10 * time.Minute

	// DefaultMCEEnablementTimeout is the default timeout for waiting after MCE component enablement.
	// MCE components need time to deploy controllers, pull images, and initialize.
	DefaultMCEEnablementTimeout = 15 * time.Minute

	// DefaultDeploymentStallTimeout is the default stall detection timeout for the infrastructure phase.
	// After infrastructure resources are fully reconciled, the timeout doubles (2x) for the
	// post-infrastructure phase where the hosted control plane provisioning is opaque.
	// Set to 0 to disable stall detection.
	DefaultDeploymentStallTimeout = 30 * time.Minute

	// DefaultNodeReadyTimeout is the default timeout for waiting for worker nodes to become available.
	// In ARO HCP, the control plane becomes ready before worker nodes are provisioned.
	// The AROMachinePool creates nodes after the HcpOpenShiftCluster is up.
	DefaultNodeReadyTimeout = 30 * time.Minute

	// DefaultCAPIUser is the default user identifier for CAPI resources.
	// Used in ClusterNamePrefix (for resource group naming) and User field.
	// Extracted to a constant to ensure consistency across all usages.
	DefaultCAPIUser = "cate"

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

// EnvVarRequirement describes a required environment variable credential.
type EnvVarRequirement struct {
	Name             string   // environment variable name (e.g., "AZURE_SUBSCRIPTION_ID")
	Desc             string   // human-readable description
	Sensitive        bool     // if true, value will be masked in output (e.g., secrets, passwords)
	RedactionAliases []string // alternate key names to redact in logs (e.g., "clientSecret" for AZURE_CLIENT_SECRET)
}

// CredentialSecretDef describes a provider's credential secret.
type CredentialSecretDef struct {
	Name           string   // secret name (e.g., "aso-controller-settings"), can use {WORKLOAD_CLUSTER_NAME} placeholder
	Namespace      string   // namespace containing the secret, can use {WORKLOAD_CLUSTER_NAMESPACE} placeholder
	RequiredFields []string // fields that must be present and non-empty in the secret (validated in Phase 05)
}

// InfraProvider defines an infrastructure provider's configuration.
// Each provider has controllers, webhooks, and optionally a credential secret.
type InfraProvider struct {
	Name               string               // provider identifier (e.g., "aro", "rosa")
	Controllers        []ControllerDef      // controllers to validate
	Webhooks           []WebhookDef         // webhooks to validate
	CredentialSecret   *CredentialSecretDef // nil if no credential secret needed
	DeploymentCharts   []string             // chart args for deploy-charts.sh
	MCEComponentName   string               // MCE component name for this provider
	RequiredTools      []string             // CLI tools required for this provider (e.g., "az" for ARO, "aws" for ROSA)
	RequiredScripts    []string             // repo-relative scripts this provider needs (validated in Phase 2)
	YAMLGenCredentials []EnvVarRequirement  // credentials required for YAML generation (Phase 04)
	ExpectedFiles      []string             // YAML files expected to be generated by gen.sh script
}

// SensitiveKeyNames returns the names of all environment variables marked as
// sensitive in this provider's YAMLGenCredentials, plus any redaction aliases.
// Used by redactCommand to build the redaction pattern from config rather than
// a hardcoded list.
func (p InfraProvider) SensitiveKeyNames() []string {
	var names []string
	for _, env := range p.YAMLGenCredentials {
		if env.Sensitive {
			names = append(names, env.Name)
			names = append(names, env.RedactionAliases...)
		}
	}
	return names
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
		// Note: ARO uses namespace-scoped AzureClusterIdentity and aso-credential secret
		// created by gen.sh script (Phase 04) in the workload cluster namespace
		CredentialSecret: &CredentialSecretDef{
			Name:      GetEnvOrDefault("ASO_CREDENTIAL_NAME", "aso-credential"),
			Namespace: "{WORKLOAD_CLUSTER_NAMESPACE}",
			RequiredFields: []string{
				"AZURE_TENANT_ID",
				"AZURE_SUBSCRIPTION_ID",
				"AZURE_CLIENT_ID",
				"AZURE_CLIENT_SECRET",
			},
		},
		DeploymentCharts: []string{"cluster-api-provider-azure"},
		MCEComponentName: "cluster-api-provider-azure-preview",
		RequiredTools:    []string{"az"},
		RequiredScripts:  []string{"scripts/deploy-charts.sh", "scripts/aro-hcp/gen.sh"},
		YAMLGenCredentials: []EnvVarRequirement{
			{Name: "AZURE_SUBSCRIPTION_ID", Desc: "Azure subscription ID", Sensitive: false},
			{Name: "AZURE_TENANT_ID", Desc: "Azure tenant ID", Sensitive: false},
			{Name: "AZURE_CLIENT_ID", Desc: "Azure service principal client ID", Sensitive: false},
			{Name: "AZURE_CLIENT_SECRET", Desc: "Azure service principal client secret", Sensitive: true, RedactionAliases: []string{"clientSecret"}},
		},
		ExpectedFiles: []string{"credentials.yaml", "aro.yaml"},
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
		// Note: ROSA uses cluster-scoped AWSClusterStaticIdentity with secret in CAPA controller namespace
		// The secret contains BOTH individual fields (AccessKeyID/SecretAccessKey for CAPA AWS sessions)
		// and INI format (credentials field with region for ROSA SDK/ROSARoleConfig)
		// Secret name is dynamic based on cluster name: ${WORKLOAD_CLUSTER_NAME}-account-creds
		// Secret namespace is CAPA controller namespace (capa-system for Kind, multicluster-engine for MCE)
		CredentialSecret: &CredentialSecretDef{
			Name:      "{WORKLOAD_CLUSTER_NAME}-account-creds",
			Namespace: "{INFRA_PROVIDER_NAMESPACE}", // CAPA controller namespace, not workload cluster namespace
			RequiredFields: []string{
				"AccessKeyID",     // Required by CAPA for AWS session creation
				"SecretAccessKey", // Required by CAPA for AWS session creation
				"credentials",     // Required by ROSA SDK (INI format with region)
			},
		},
		DeploymentCharts: []string{"cluster-api-provider-aws"},
		MCEComponentName: "cluster-api-provider-aws",
		RequiredTools:    []string{"aws"},
		RequiredScripts:  []string{"scripts/deploy-charts.sh", "scripts/rosa-hcp/gen.sh"},
		YAMLGenCredentials: []EnvVarRequirement{
			{Name: "AWS_REGION", Desc: "AWS region for deployment", Sensitive: false},
			{Name: "OCM_API_URL", Desc: "OpenShift Cluster Manager API URL", Sensitive: false},
			{Name: "OCM_CLIENT_ID", Desc: "OCM OAuth client ID", Sensitive: false},
			{Name: "AWS_ACCESS_KEY_ID", Desc: "AWS access key ID", Sensitive: false},
			{Name: "AWS_SECRET_ACCESS_KEY", Desc: "AWS secret access key", Sensitive: true, RedactionAliases: []string{"SecretAccessKey"}},
			{Name: "OCM_CLIENT_SECRET", Desc: "OCM OAuth client secret", Sensitive: true, RedactionAliases: []string{"clientSecret"}},
		},
		ExpectedFiles: []string{"secrets.yaml", "is.yaml", "rosa.yaml"},
	}
}

var (
	defaultRepoDir     string
	defaultRepoDirOnce sync.Once

	workloadClusterNamespace     string
	workloadClusterNamespaceOnce sync.Once

	clusterNamePrefix     string
	clusterNamePrefixOnce sync.Once

	resourceGroupName     string
	resourceGroupNameOnce sync.Once

	// cachedResourceTags holds tags loaded from the deployment state file on resume.
	// nil means tags were not loaded from state (fresh run or explicit CS_CLUSTER_NAME),
	// so fresh tags will be generated.
	cachedResourceTags map[string]string
)

// RunContext contains the immutable identity shared by every phase process in a
// test run. DeploymentState intentionally remains separate because it records
// mutable cleanup and controller state.
type RunContext struct {
	ClusterNamePrefix        string `json:"cluster_name_prefix"`
	ResourceGroupName        string `json:"resource_group_name"`
	WorkloadClusterName      string `json:"workload_cluster_name"`
	WorkloadClusterNamespace string `json:"workload_cluster_namespace"`
	TestRunID                string `json:"test_run_id"`
	CAPIUser                 string `json:"capi_user"`
	InfraProvider            string `json:"infra_provider"`
	DeploymentEnvironment    string `json:"deployment_environment"`
}

const (
	runContextFileName   = ".run-context.json"
	runContextLockSuffix = ".lock"
)

// runContextWorkspace returns the repository containing this package. It does
// not use the process working directory, because generation phases chdir into
// the installer repository.
func runContextWorkspace() string {
	if workspace := os.Getenv("CAPI_TEST_WORKSPACE"); workspace != "" {
		if absolute, err := filepath.Abs(workspace); err == nil {
			return absolute
		}
	}

	_, sourceFile, _, ok := runtime.Caller(0)
	if ok {
		if absolute, err := filepath.Abs(filepath.Join(filepath.Dir(sourceFile), "..")); err == nil {
			return absolute
		}
	}
	return "."
}

// RunContextFilePath returns the one absolute path used for immutable run
// identity. Relative overrides are resolved from the repository workspace.
func RunContextFilePath() string {
	path := os.Getenv("CAPI_TEST_CONTEXT_FILE")
	if path == "" {
		path = filepath.Join(runContextWorkspace(), runContextFileName)
	}
	if absolute, err := filepath.Abs(path); err == nil {
		return absolute
	}
	return path
}

func deploymentStatePath() string {
	return filepath.Join(filepath.Dir(RunContextFilePath()), ".deployment-state.json")
}

// readValidatedStateFile enforces the state-file path policy at the single
// filesystem boundary used by run identity and deployment state. Callers may
// provide a CI override, which must resolve to an absolute path. Deployment
// state additionally uses a fixed filename; traversal cannot change that name.
func readValidatedStateFile(path, expectedName string) ([]byte, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve state path %q: %w", path, err)
	}
	if !filepath.IsAbs(absolute) || (expectedName != "" && filepath.Base(filepath.Clean(absolute)) != expectedName) {
		return nil, fmt.Errorf("invalid state path %q: expected absolute filename %q", path, expectedName)
	}
	// #nosec G304 -- absolute path is validated above against a fixed state filename.
	return os.ReadFile(absolute)
}

func legacyDeploymentStatePaths(contextPath string) []string {
	paths := []string{filepath.Join(filepath.Dir(contextPath), ".deployment-state.json")}
	legacyPath := filepath.Join(getDefaultRepoDir(), ".deployment-state.json")
	if legacyPath != paths[0] {
		paths = append(paths, legacyPath)
	}
	return paths
}

func readRunContext(path string) (*RunContext, error) {
	data, err := readValidatedStateFile(path, "")
	if err != nil {
		return nil, err
	}
	var context RunContext
	if err := json.Unmarshal(data, &context); err != nil {
		return nil, fmt.Errorf("failed to parse run context %s: %w", path, err)
	}
	if context.ClusterNamePrefix == "" || context.ResourceGroupName == "" ||
		context.WorkloadClusterNamespace == "" || context.TestRunID == "" {
		return nil, fmt.Errorf("run context %s is missing required identity fields", path)
	}
	return &context, nil
}

func readLegacyRunContext(path string) (*RunContext, map[string]string, error) {
	data, err := readValidatedStateFile(path, ".deployment-state.json")
	if err != nil {
		return nil, nil, err
	}
	var state struct {
		ResourceGroup            string            `json:"resource_group"`
		WorkloadClusterName      string            `json:"workload_cluster_name"`
		WorkloadClusterNamespace string            `json:"workload_cluster_namespace"`
		ClusterNamePrefix        string            `json:"cluster_name_prefix"`
		TestRunID                string            `json:"test_run_id"`
		User                     string            `json:"user"`
		InfraProvider            string            `json:"infra_provider"`
		DeploymentEnvironment    string            `json:"environment"`
		ResourceTags             map[string]string `json:"resource_tags,omitempty"`
		AzureResourceTags        map[string]string `json:"azure_resource_tags,omitempty"`
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, nil, fmt.Errorf("failed to parse legacy deployment state %s: %w", path, err)
	}
	if state.ResourceTags == nil {
		state.ResourceTags = state.AzureResourceTags
	}
	return &RunContext{
		ClusterNamePrefix:        state.ClusterNamePrefix,
		ResourceGroupName:        state.ResourceGroup,
		WorkloadClusterName:      state.WorkloadClusterName,
		WorkloadClusterNamespace: state.WorkloadClusterNamespace,
		TestRunID:                state.TestRunID,
		CAPIUser:                 state.User,
		InfraProvider:            state.InfraProvider,
		DeploymentEnvironment:    state.DeploymentEnvironment,
	}, state.ResourceTags, nil
}

func explicitContextConflicts(context *RunContext) error {
	checks := []struct {
		envName string
		field   string
		value   string
		actual  string
	}{
		{"CS_CLUSTER_NAME", "ClusterNamePrefix", os.Getenv("CS_CLUSTER_NAME"), context.ClusterNamePrefix},
		{"RESOURCEGROUPNAME", "ResourceGroupName", os.Getenv("RESOURCEGROUPNAME"), context.ResourceGroupName},
		{"WORKLOAD_CLUSTER_NAME", "WorkloadClusterName", os.Getenv("WORKLOAD_CLUSTER_NAME"), context.WorkloadClusterName},
		{"WORKLOAD_CLUSTER_NAMESPACE", "WorkloadClusterNamespace", os.Getenv("WORKLOAD_CLUSTER_NAMESPACE"), context.WorkloadClusterNamespace},
		{"CAPI_USER", "CAPIUser", os.Getenv("CAPI_USER"), context.CAPIUser},
		{"INFRA_PROVIDER", "InfraProvider", os.Getenv("INFRA_PROVIDER"), context.InfraProvider},
		{"DEPLOYMENT_ENV", "DeploymentEnvironment", os.Getenv("DEPLOYMENT_ENV"), context.DeploymentEnvironment},
	}
	for _, check := range checks {
		if check.value != "" && check.actual != "" && check.value != check.actual {
			return fmt.Errorf("run context conflict for %s (%s): explicit value %q conflicts with persisted value %q", check.envName, check.field, check.value, check.actual)
		}
	}
	return nil
}

func acquireRunContextLock(path string) (func(), error) {
	lockPath := path + runContextLockSuffix
	deadline := time.Now().Add(10 * time.Second)
	for {
		err := os.Mkdir(lockPath, 0700)
		if err == nil {
			return func() { _ = os.Remove(lockPath) }, nil
		}
		if !os.IsExist(err) {
			return nil, fmt.Errorf("failed to create run context lock %s: %w", lockPath, err)
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timed out waiting for run context lock %s", lockPath)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func writeRunContext(path string, context *RunContext) error {
	data, err := json.MarshalIndent(context, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal run context: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".run-context-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temporary run context: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("failed to set run context permissions: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("failed to write run context: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("failed to close temporary run context: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("failed to atomically persist run context: %w", err)
	}
	return nil
}

// EnsureRunContext loads or atomically creates the immutable identity for this
// run. Explicit environment values are accepted on first creation and checked
// against persisted values on every later phase invocation.
func EnsureRunContext() (*RunContext, error) {
	path := RunContextFilePath()
	if context, err := readRunContext(path); err == nil {
		if err := explicitContextConflicts(context); err != nil {
			return nil, err
		}
		return context, nil
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("failed to create run context directory: %w", err)
	}
	release, err := acquireRunContextLock(path)
	if err != nil {
		return nil, err
	}
	defer release()

	if context, err := readRunContext(path); err == nil {
		if err := explicitContextConflicts(context); err != nil {
			return nil, err
		}
		return context, nil
	}

	context := &RunContext{}
	var migratedLegacyPath string
	for _, legacyPath := range legacyDeploymentStatePaths(path) {
		legacy, tags, legacyErr := readLegacyRunContext(legacyPath)
		if legacyErr == nil {
			*context = *legacy
			cachedResourceTags = tags
			migratedLegacyPath = legacyPath
			break
		}
		if !os.IsNotExist(legacyErr) {
			return nil, legacyErr
		}
	}

	provider := GetEnvOrDefault("INFRA_PROVIDER", "aro")
	defaultWorkloadCluster := "capz-tests"
	defaultNamespacePrefix := "capz-test"
	if provider == "rosa" {
		defaultWorkloadCluster = "capa-tests"
		defaultNamespacePrefix = "capa-test"
	}
	capiUser := getCAPIUser()
	if context.CAPIUser == "" {
		context.CAPIUser = capiUser
	}
	if context.InfraProvider == "" {
		context.InfraProvider = provider
	}
	if context.DeploymentEnvironment == "" {
		context.DeploymentEnvironment = GetEnvOrDefault("DEPLOYMENT_ENV", DefaultDeploymentEnv)
	}
	if context.WorkloadClusterName == "" {
		context.WorkloadClusterName = GetEnvOrDefault("WORKLOAD_CLUSTER_NAME", defaultWorkloadCluster)
	}
	if context.ClusterNamePrefix == "" {
		if explicit := os.Getenv("CS_CLUSTER_NAME"); explicit != "" {
			context.ClusterNamePrefix = explicit
		} else {
			maxUserLen := MaxClusterNamePrefixLength - 1 - 5
			userPrefix := capiUser
			if len(userPrefix) > maxUserLen {
				userPrefix = userPrefix[:maxUserLen]
			}
			context.TestRunID = generateRunID(5)
			context.ClusterNamePrefix = fmt.Sprintf("%s-%s", userPrefix, context.TestRunID)
		}
	}
	if context.TestRunID == "" {
		if prefix := context.CAPIUser + "-"; strings.HasPrefix(context.ClusterNamePrefix, prefix) {
			context.TestRunID = strings.TrimPrefix(context.ClusterNamePrefix, prefix)
		}
		if context.TestRunID == "" {
			context.TestRunID = generateRunID(5)
		}
	}
	if context.ResourceGroupName == "" {
		if explicit := os.Getenv("RESOURCEGROUPNAME"); explicit != "" {
			context.ResourceGroupName = explicit
		} else {
			context.ResourceGroupName = fmt.Sprintf("%s-%s-resgroup", context.WorkloadClusterName, context.TestRunID)
		}
	}
	if context.WorkloadClusterNamespace == "" {
		if explicit := os.Getenv("WORKLOAD_CLUSTER_NAMESPACE"); explicit != "" {
			context.WorkloadClusterNamespace = explicit
		} else {
			prefix := GetEnvOrDefault("WORKLOAD_CLUSTER_NAMESPACE_PREFIX", defaultNamespacePrefix)
			context.WorkloadClusterNamespace = fmt.Sprintf("%s-%s", prefix, time.Now().Format("20060102-150405"))
		}
	}
	if err := explicitContextConflicts(context); err != nil {
		return nil, err
	}
	if err := writeRunContext(path, context); err != nil {
		return nil, err
	}
	if migratedLegacyPath != "" {
		if err := os.Remove(migratedLegacyPath); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to remove migrated legacy deployment state %s: %w", migratedLegacyPath, err)
		}
	}
	return context, nil
}

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

// getCAPIUser returns the user identifier from CAPI_USER env var,
// falling back to the OS username ($USER) sanitized for RFC 1123 compliance,
// then to DefaultCAPIUser as a last resort.
func getCAPIUser() string {
	if v := os.Getenv("CAPI_USER"); v != "" {
		return v
	}
	if v := os.Getenv("USER"); v != "" {
		if s := SanitizeToRFC1123(v); s != "" {
			return s
		}
	}
	return DefaultCAPIUser
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
		context, err := EnsureRunContext()
		if err != nil {
			errMsg := err.Error()
			configError = &errMsg
			return
		}
		workloadClusterNamespace = context.WorkloadClusterNamespace
	})

	return workloadClusterNamespace
}

// getClusterNamePrefix returns the CS_CLUSTER_NAME value, generating a unique one if not explicitly set.
// When CS_CLUSTER_NAME is not set, a unique prefix is generated: ${CAPI_USER}-${random5hex}
// (e.g., "cate-a1b2c"). This enables parallel test runs against the same Azure subscription
// without resource name collisions.
//
// Resolution order:
// 1. CS_CLUSTER_NAME env var (explicit override)
// 2. Existing deployment state file in RepoDir (auto-resume from previous run)
// 3. Generate unique prefix: ${CAPI_USER}-${random5hex}
//
// The auto-resume from deployment state ensures that subsequent test phases
// (run as separate go test invocations) use the same prefix as the initial phase.
func getClusterNamePrefix(capiUser string) string {
	clusterNamePrefixOnce.Do(func() {
		context, err := EnsureRunContext()
		if err != nil {
			errMsg := err.Error()
			configError = &errMsg
			return
		}
		clusterNamePrefix = context.ClusterNamePrefix
	})

	return clusterNamePrefix
}

// getResourceGroupName returns the Azure resource group name for this test run.
// When RESOURCEGROUPNAME is not set, a unique name is generated per test run
// to prevent parallel runs from interfering with each other's Azure resources.
//
// Resolution order:
// 1. RESOURCEGROUPNAME env var (explicit override)
// 2. Existing deployment state file in RepoDir (auto-resume from previous run)
// 3. Generate unique name: ${workloadClusterName}-${runID}-resgroup
func getResourceGroupName(workloadClusterName, runID string) string {
	resourceGroupNameOnce.Do(func() {
		context, err := EnsureRunContext()
		if err != nil {
			errMsg := err.Error()
			configError = &errMsg
			return
		}
		resourceGroupName = context.ResourceGroupName
	})

	return resourceGroupName
}

// generateRunID creates a random hex string of the specified length.
// Uses crypto/rand for unpredictable values. Panics if crypto/rand fails,
// as this indicates a serious system issue (e.g., /dev/urandom unavailable)
// and a timestamp fallback would undermine parallel-run uniqueness.
func generateRunID(length int) string {
	bytes := make([]byte, (length+1)/2)
	if _, err := rand.Read(bytes); err != nil {
		panic(fmt.Sprintf("crypto/rand.Read failed: %v — cannot generate unique run ID for parallel safety", err))
	}
	return hex.EncodeToString(bytes)[:length]
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
	ClusterNamePrefix        string // Used as CS_CLUSTER_NAME for YAML generation
	NamePrefix               string // NAME_PREFIX used for Azure resource naming (Key Vault, node pools); passed to YAML generation
	OCPVersion               string
	OCPVersionMP             string // Full x.y.z OpenShift version for MachinePool workers (from OCP_VERSION_MP env var)
	Region                   string
	AzureSubscriptionName    string // Azure subscription name (from AZURE_SUBSCRIPTION_NAME env var)
	Environment              string
	CAPIUser                 string            // User identifier for CAPI resources (from CAPI_USER env var)
	WorkloadClusterNamespace string            // Namespace for workload cluster resources on management cluster (unique per test run)
	TestLabelPrefix          string            // Provider-specific label prefix for test namespaces (e.g., "capz-test" for ARO, "capa-test" for ROSA)
	TestRunID                string            // Unique run identifier extracted from ClusterNamePrefix (the part after CAPI_USER-). Empty when prefix does not start with CAPI_USER-.
	ResourceTags             map[string]string // Tags applied to all created cloud resources (Azure RGs, AWS stacks/VPCs) for ownership tracking and cleanup
	ResourceGroupName        string            // Azure resource group name (env: RESOURCEGROUPNAME, default: ${WorkloadClusterName}-${runID}-resgroup)
	CAPINamespace            string            // Namespace for CAPI controller (default: "capi-system", or "multicluster-engine" in K8S mode)
	CAPZNamespace            string            // Namespace for CAPZ/ASO controllers (default: "capz-system", or "multicluster-engine" in K8S mode)

	// Management cluster mode
	// ClusterMode specifies the management cluster deployment mode ("kind" or "mce").
	// - "kind": Deploy local Kind cluster (default behavior)
	// - "mce": Use existing MCE (MultiCluster Engine) cluster
	// Set via CLUSTER_MODE env var. Automatically configures UseKind and UseKubeconfig.
	ClusterMode string

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

	// UseK8S selects the multicluster-engine namespace for all controllers.
	// It is enabled explicitly with USE_K8S=true or implicitly for an external
	// kubeconfig when chart deployment is disabled.
	UseK8S bool

	// Paths
	ClusterctlBinPath string
	ScriptsPath       string
	GenScriptPath     string

	// Timeouts
	ClusterDeploymentTimeout time.Duration // CLUSTER_DEPLOYMENT_TIMEOUT: how long the deploy polling loop waits
	ClusterDeletionTimeout   time.Duration // CLUSTER_DELETION_TIMEOUT: how long the deletion polling loop waits
	DeploymentTimeout        time.Duration // Deprecated: alias for ClusterDeploymentTimeout (backward compat)
	DeploymentStallTimeout   time.Duration // 0 disables stall detection
	ASOControllerTimeout     time.Duration
	HelmInstallTimeout       time.Duration

	// Infrastructure providers
	// InfraProviderName is the selected infrastructure provider ("aro" or "rosa").
	// Set via INFRA_PROVIDER env var. Default: "aro".
	InfraProviderName string
	// InfraProviders holds the list of infrastructure provider configurations.
	// Each provider defines its controllers, webhooks, and credential secrets.
	// Initialized based on INFRA_PROVIDER env var: "aro" (CAPZ/ASO) or "rosa" (CAPA).
	InfraProviders []InfraProvider
	// ClusterYAML is the provider-specific main YAML filename.
	// For ARO: "aro.yaml", for ROSA: "rosa.yaml"
	ClusterYAML string
	// RegionEnvVar is the provider-specific region environment variable name.
	// For ARO: "REGION", for ROSA: "AWS_REGION"
	RegionEnvVar string

	// MCE (MultiClusterEngine) configuration
	// MCEAutoEnable controls whether to automatically enable MCE CAPI/CAPZ components
	// if they are not found on an external cluster. Default: true when IsExternalCluster().
	MCEAutoEnable bool
	// MCEEnablementTimeout is the timeout for waiting after MCE component enablement.
	// Controllers need time to be deployed, images pulled, and pods started.
	MCEEnablementTimeout time.Duration

	// Chart deployment configuration
	// DeployCharts controls whether to deploy Helm charts to the management cluster.
	// When true and USE_KUBECONFIG is set, deploys CAPI/provider charts to external cluster.
	// Default: false
	DeployCharts bool

	// kubeContext caches the result of GetKubeContext() so the external-cluster
	// path does not shell out to `kubectl config current-context` on every call.
	// kubeContextOnce guards the one-time computation. These are per-instance
	// (not package-level) so distinct configs with different UseKubeconfig values
	// resolve their own context correctly.
	kubeContext     string
	kubeContextOnce sync.Once
}

// NewTestConfig creates a new test configuration with defaults
func NewTestConfig() *TestConfig {
	useKubeconfig := os.Getenv("USE_KUBECONFIG")
	deployCharts := parseDeployCharts()

	// Handle CLUSTER_MODE: auto-configure based on cluster mode
	clusterMode := GetEnvOrDefault("CLUSTER_MODE", "")
	if clusterMode == "kind" {
		// Set USE_KIND for Kind mode (tests depend on this)
		_ = os.Setenv("USE_KIND", "true") // #nosec G104
	} else if clusterMode == "mce" {
		// Step 1: Create temporary kubeconfig file if not already set
		if useKubeconfig == "" {
			// Pattern: /tmp/cluster-api-installer-aro.*.kubeconfig
			repoDir := GetEnvOrDefault("ARO_REPO_DIR", "/tmp/cluster-api-installer-aro")
			repoBaseName := filepath.Base(repoDir)

			// Check if kubeconfig already exists from a previous run
			pattern := filepath.Join("/tmp", repoBaseName+".*.kubeconfig")
			matches, err := filepath.Glob(pattern)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to glob for existing kubeconfig: %v\n", err)
			}
			if len(matches) > 0 {
				// Use most recent kubeconfig (sort by modification time)
				var latestFile string
				var latestTime time.Time
				for _, match := range matches {
					info, err := os.Stat(match)
					if err == nil && info.ModTime().After(latestTime) {
						latestTime = info.ModTime()
						latestFile = match
					}
				}
				if latestFile != "" {
					useKubeconfig = latestFile
				} else {
					useKubeconfig = matches[0]
				}
			} else {
				// Create new temporary kubeconfig file (empty, will be populated by oc login)
				file, err := os.CreateTemp("/tmp", repoBaseName+".*.kubeconfig")
				if err != nil {
					errMsg := fmt.Sprintf("Failed to create temporary kubeconfig file: %v", err)
					configError = &errMsg
					useKubeconfig = "" // Clear to prevent partial config state
				} else {
					tempPath := file.Name()
					if err := file.Close(); err != nil {
						errMsg := fmt.Sprintf("Failed to close temporary kubeconfig file: %v", err)
						configError = &errMsg
						useKubeconfig = "" // Clear to prevent partial config state
					} else if err := os.Chmod(tempPath, 0600); err != nil {
						errMsg := fmt.Sprintf("Failed to set permissions on temporary kubeconfig file: %v", err)
						configError = &errMsg
						useKubeconfig = "" // Clear to prevent partial config state
					} else {
						// All operations succeeded - set the kubeconfig path
						useKubeconfig = tempPath
					}
				}
			}
		}

		// Step 2: Set KUBECONFIG environment variable
		if useKubeconfig != "" {
			_ = os.Setenv("USE_KUBECONFIG", useKubeconfig) // #nosec G104
			_ = os.Setenv("KUBECONFIG", useKubeconfig)     // #nosec G104
		}

		// Step 3: Set MCE mode flags
		_ = os.Setenv("USE_KIND", "false") // #nosec G104
		_ = os.Setenv("USE_MCE", "true")   // #nosec G104

		// MCE mode disables chart deployment (controllers are pre-installed)
		if os.Getenv("DEPLOY_CHARTS") == "" {
			_ = os.Setenv("DEPLOY_CHARTS", "false") // #nosec G104
		}
	}

	// An external kubeconfig without chart deployment uses the namespaces where
	// MCE provides the controllers. Keep this derived state in the config rather
	// than mutating USE_K8S in the caller's environment.
	useK8S := os.Getenv("USE_K8S") == "true"
	if useKubeconfig != "" && !deployCharts && os.Getenv("USE_K8S") == "" {
		useK8S = true
	}

	// Initialize/read the immutable identity before resolving provider-specific
	// defaults. Separate phase processes must use the persisted provider,
	// environment, and user as well as the cluster identity fields.
	runContext, contextErr := EnsureRunContext()
	if contextErr != nil {
		errMsg := contextErr.Error()
		configError = &errMsg
	}

	// Determine infrastructure provider
	infraProviderName := GetEnvOrDefault("INFRA_PROVIDER", "aro")
	if runContext != nil && runContext.InfraProvider != "" {
		infraProviderName = runContext.InfraProvider
	}

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
	var clusterYAML string
	var defaultRegion string
	var regionEnvVar string

	switch infraProviderName {
	case "rosa":
		providerNamespace = getControllerNamespace(useK8S, "CAPA_NAMESPACE", "capa-system")
		infraProviders = []InfraProvider{NewAWSProvider(providerNamespace)}
		defaultGenScriptPath = "./scripts/rosa-hcp/gen.sh"
		defaultMgmtCluster = "capa-tests-stage"
		defaultWorkloadCluster = "capa-tests"
		testLabelPrefix = "capa-test"
		clusterYAML = "rosa.yaml"
		regionEnvVar = "AWS_REGION"
		defaultRegion = "us-east-1"
	default: // "aro"
		infraProviderName = "aro" // normalize unknown values
		providerNamespace = getControllerNamespace(useK8S, "CAPZ_NAMESPACE", "capz-system")
		azureProvider := NewAzureProvider(providerNamespace)
		for i := range azureProvider.Controllers {
			if azureProvider.Controllers[i].DisplayName == "ASO" {
				azureProvider.Controllers[i].Timeout = asoTimeout
			}
		}
		infraProviders = []InfraProvider{azureProvider}
		defaultGenScriptPath = "./scripts/aro-hcp/gen.sh"
		defaultMgmtCluster = "capz-tests-stage"
		defaultWorkloadCluster = "capz-tests"
		testLabelPrefix = "capz-test"
		clusterYAML = "aro.yaml"
		regionEnvVar = "REGION"
		defaultRegion = "uksouth"
	}

	// Resolve immutable identity values from the persisted context when one
	// exists. EnsureRunContext has already checked explicit environment values
	// for conflicts, so omitted values safely inherit the original run.
	capiUser := getCAPIUser()
	environment := GetEnvOrDefault("DEPLOYMENT_ENV", DefaultDeploymentEnv)
	if runContext != nil {
		if runContext.CAPIUser != "" {
			capiUser = runContext.CAPIUser
		}
		if runContext.DeploymentEnvironment != "" {
			environment = runContext.DeploymentEnvironment
		}
	}

	// Resolve CS_CLUSTER_NAME with auto-uniqueness for parallel runs
	prefix := getClusterNamePrefix(capiUser)

	// Extract run ID from the generated prefix (the part after the user prefix)
	testRunID := ""
	if userPrefix := capiUser + "-"; strings.HasPrefix(prefix, userPrefix) {
		testRunID = strings.TrimPrefix(prefix, userPrefix)
	}

	// Resolve workload cluster name and resource group name
	workloadClusterName := GetEnvOrDefault("WORKLOAD_CLUSTER_NAME", defaultWorkloadCluster)
	rgName := getResourceGroupName(workloadClusterName, testRunID)
	namespace := getWorkloadClusterNamespace(testLabelPrefix)
	if runContext != nil {
		prefix = runContext.ClusterNamePrefix
		testRunID = runContext.TestRunID
		workloadClusterName = runContext.WorkloadClusterName
		rgName = runContext.ResourceGroupName
		namespace = runContext.WorkloadClusterNamespace
	}

	// Build resource tags for cleanup and ownership tracking (used for both Azure and AWS).
	// On resume, use cached tags from the deployment state to preserve the original created-at timestamp.
	resourceTags := cachedResourceTags
	if resourceTags == nil {
		resourceTags = map[string]string{
			"capi-test-user":       capiUser,
			"capi-test-env":        environment,
			"capi-test-run-id":     prefix,
			"capi-test-created-at": time.Now().Format(time.RFC3339),
		}
	}

	clusterDeployTimeout := parseClusterDeploymentTimeout()

	return &TestConfig{
		// Repository defaults
		RepoURL:    GetEnvOrDefault("ARO_REPO_URL", "https://github.com/stolostron/cluster-api-installer"),
		RepoBranch: GetEnvOrDefault("ARO_REPO_BRANCH", "main"),
		RepoDir:    getDefaultRepoDir(),

		// Cluster defaults
		ManagementClusterName:    GetEnvOrDefault("MANAGEMENT_CLUSTER_NAME", defaultMgmtCluster),
		WorkloadClusterName:      workloadClusterName,
		ClusterNamePrefix:        prefix,
		NamePrefix:               GetEnvOrDefault("NAME_PREFIX", ""),
		OCPVersion:               GetEnvOrDefault("OCP_VERSION", "4.20"),
		OCPVersionMP:             GetEnvOrDefault("OCP_VERSION_MP", "4.20.17"),
		Region:                   GetEnvOrDefault(regionEnvVar, defaultRegion),
		AzureSubscriptionName:    os.Getenv("AZURE_SUBSCRIPTION_NAME"),
		Environment:              environment,
		CAPIUser:                 capiUser,
		WorkloadClusterNamespace: namespace,
		TestLabelPrefix:          testLabelPrefix,
		TestRunID:                testRunID,
		ResourceTags:             resourceTags,
		ResourceGroupName:        rgName,
		CAPINamespace:            getControllerNamespace(useK8S, "CAPI_NAMESPACE", "capi-system"),
		CAPZNamespace:            providerNamespace,

		// Management cluster mode
		ClusterMode: clusterMode,

		// External cluster
		UseKubeconfig: useKubeconfig,

		// Kind mode
		UseKind: os.Getenv("USE_KIND") == "true",
		UseK8S:  useK8S,

		// Paths
		ClusterctlBinPath: GetEnvOrDefault("CLUSTERCTL_BIN", "./bin/clusterctl"),
		ScriptsPath:       GetEnvOrDefault("SCRIPTS_PATH", "./scripts"),
		GenScriptPath:     GetEnvOrDefault("GEN_SCRIPT_PATH", defaultGenScriptPath),

		// Timeouts
		ClusterDeploymentTimeout: clusterDeployTimeout,
		ClusterDeletionTimeout:   parseClusterDeletionTimeout(),
		DeploymentTimeout:        clusterDeployTimeout, // backward compat alias
		DeploymentStallTimeout:   parseDeploymentStallTimeout(),
		ASOControllerTimeout:     asoTimeout,
		HelmInstallTimeout:       parseHelmInstallTimeout(),

		// Infrastructure providers
		InfraProviderName: infraProviderName,
		InfraProviders:    infraProviders,
		ClusterYAML:       clusterYAML,
		RegionEnvVar:      regionEnvVar,

		// MCE configuration
		MCEAutoEnable:        parseMCEAutoEnable(useKubeconfig),
		MCEEnablementTimeout: parseMCEEnablementTimeout(),

		// Chart deployment
		DeployCharts: deployCharts,
	}
}

// getControllerNamespace returns the namespace for a controller based on configuration.
// If useK8S is true, returns "multicluster-engine" (K8S deployment mode).
// Otherwise, checks the specific env var (e.g., CAPI_NAMESPACE) and falls back to defaultNS.
func getControllerNamespace(useK8S bool, envVar, defaultNS string) string {
	// K8S mode uses the multicluster-engine namespace for all controllers.
	if useK8S {
		return "multicluster-engine"
	}

	// Check for specific namespace override
	if ns := os.Getenv(envVar); ns != "" {
		return ns
	}

	return defaultNS
}

// parseClusterDeploymentTimeout parses the CLUSTER_DEPLOYMENT_TIMEOUT environment variable.
// Controls how long the deployment polling loop waits for the cluster to become ready.
//
// Resolution order:
//  1. CLUSTER_DEPLOYMENT_TIMEOUT (new, preferred)
//  2. DEPLOYMENT_TIMEOUT (legacy, backward compat)
//  3. DefaultClusterDeploymentTimeout (60m)
func parseClusterDeploymentTimeout() time.Duration {
	if timeoutStr := os.Getenv("CLUSTER_DEPLOYMENT_TIMEOUT"); timeoutStr != "" {
		timeout, err := time.ParseDuration(timeoutStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: invalid CLUSTER_DEPLOYMENT_TIMEOUT '%s', using default %v\n", timeoutStr, DefaultClusterDeploymentTimeout)
			return DefaultClusterDeploymentTimeout
		}
		return timeout
	}

	// Fall back to legacy DEPLOYMENT_TIMEOUT
	if timeoutStr := os.Getenv("DEPLOYMENT_TIMEOUT"); timeoutStr != "" {
		timeout, err := time.ParseDuration(timeoutStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: invalid DEPLOYMENT_TIMEOUT '%s', using default %v\n", timeoutStr, DefaultClusterDeploymentTimeout)
			return DefaultClusterDeploymentTimeout
		}
		return timeout
	}

	return DefaultClusterDeploymentTimeout
}

// parseClusterDeletionTimeout parses the CLUSTER_DELETION_TIMEOUT environment variable.
// Controls how long the deletion polling loop waits for the cluster to be deleted.
//
// Resolution order:
//  1. CLUSTER_DELETION_TIMEOUT (new, preferred)
//  2. DEPLOYMENT_TIMEOUT (legacy, backward compat)
//  3. DefaultClusterDeletionTimeout (60m)
func parseClusterDeletionTimeout() time.Duration {
	if timeoutStr := os.Getenv("CLUSTER_DELETION_TIMEOUT"); timeoutStr != "" {
		timeout, err := time.ParseDuration(timeoutStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: invalid CLUSTER_DELETION_TIMEOUT '%s', using default %v\n", timeoutStr, DefaultClusterDeletionTimeout)
			return DefaultClusterDeletionTimeout
		}
		return timeout
	}

	// Fall back to legacy DEPLOYMENT_TIMEOUT
	if timeoutStr := os.Getenv("DEPLOYMENT_TIMEOUT"); timeoutStr != "" {
		timeout, err := time.ParseDuration(timeoutStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: invalid DEPLOYMENT_TIMEOUT '%s', using default %v\n", timeoutStr, DefaultClusterDeletionTimeout)
			return DefaultClusterDeletionTimeout
		}
		return timeout
	}

	return DefaultClusterDeletionTimeout
}

// parseDeploymentTimeout parses the DEPLOYMENT_TIMEOUT environment variable.
// Deprecated: Use parseClusterDeploymentTimeout instead.
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

// parseDeploymentStallTimeout parses the DEPLOYMENT_STALL_TIMEOUT environment variable.
// Returns the parsed duration or defaults to DefaultDeploymentStallTimeout.
// Set to "0" to disable stall detection entirely.
func parseDeploymentStallTimeout() time.Duration {
	timeoutStr := os.Getenv("DEPLOYMENT_STALL_TIMEOUT")
	if timeoutStr == "" {
		return DefaultDeploymentStallTimeout
	}

	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: invalid DEPLOYMENT_STALL_TIMEOUT '%s', using default %v\n", timeoutStr, DefaultDeploymentStallTimeout)
		return DefaultDeploymentStallTimeout
	}
	if timeout < 0 {
		fmt.Fprintf(os.Stderr, "Warning: negative DEPLOYMENT_STALL_TIMEOUT '%s' treated as disabled (0)\n", timeoutStr)
		return 0
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

// parseDeployCharts parses the DEPLOY_CHARTS environment variable.
// Returns true if DEPLOY_CHARTS=true, false otherwise.
// Default: false
func parseDeployCharts() bool {
	return GetEnvOrDefault("DEPLOY_CHARTS", "false") == "true"
}

// GetOutputDirName returns the output directory name for generated infrastructure files
func (c *TestConfig) GetOutputDirName() string {
	return fmt.Sprintf("%s-%s", c.WorkloadClusterName, c.Environment)
}

// GetProvisionedClusterName returns the actual cluster name from the generated cluster YAML file.
// This is the name defined in the Cluster resource's metadata.name field, which may differ
// from WorkloadClusterName (the local configuration). Use this when interacting with
// the provisioned cluster via kubectl commands.
//
// Returns the extracted cluster name or WorkloadClusterName as fallback if cluster YAML
// doesn't exist yet (e.g., before YAML generation phase).
func (c *TestConfig) GetProvisionedClusterName() string {
	clusterYAMLPath := fmt.Sprintf("%s/%s/%s", c.RepoDir, c.GetOutputDirName(), c.ClusterYAML)

	name, err := ExtractClusterNameFromYAML(clusterYAMLPath)
	if err != nil {
		// Fall back to WorkloadClusterName if cluster YAML doesn't exist or can't be parsed
		// This allows earlier phases (before YAML generation) to still work
		return c.WorkloadClusterName
	}

	return name
}

// GetProvisionedControlPlaneName returns the actual control plane resource name
// from the generated cluster YAML file by reading the Cluster's spec.controlPlaneRef.name.
// This works for both ARO (AROControlPlane) and ROSA (ROSAControlPlane).
// Falls back to GetProvisionedClusterName() + "-control-plane" if cluster YAML
// doesn't exist or doesn't contain a controlPlaneRef.
func (c *TestConfig) GetProvisionedControlPlaneName() string {
	clusterYAMLPath := fmt.Sprintf("%s/%s/%s", c.RepoDir, c.GetOutputDirName(), c.ClusterYAML)

	name, err := ExtractControlPlaneRefFromYAML(clusterYAMLPath)
	if err != nil {
		return c.GetProvisionedClusterName() + "-control-plane"
	}

	return name
}

// GetProvisionedMachinePoolName returns the actual MachinePool resource name
// from the generated cluster YAML file. Falls back to GetProvisionedClusterName() + "-pool"
// if cluster YAML doesn't exist or doesn't contain a MachinePool resource.
func (c *TestConfig) GetProvisionedMachinePoolName() string {
	clusterYAMLPath := fmt.Sprintf("%s/%s/%s", c.RepoDir, c.GetOutputDirName(), c.ClusterYAML)

	name, err := ExtractMachinePoolNameFromYAML(clusterYAMLPath)
	if err != nil {
		return c.GetProvisionedClusterName() + "-pool"
	}

	return name
}

// GetClusterYAMLPath returns the path to the generated cluster YAML file.
// For ARO: {outputDir}/aro.yaml, for ROSA: {outputDir}/rosa.yaml
func (c *TestConfig) GetClusterYAMLPath() string {
	path := fmt.Sprintf("%s/%s/%s", c.RepoDir, c.GetOutputDirName(), c.ClusterYAML)
	// Validate the path stays within the expected directory to prevent path traversal
	absPath, err := filepath.Abs(path)
	if err != nil {
		return path // fallback to original
	}
	absRepo, err := filepath.Abs(c.RepoDir)
	if err != nil {
		return path
	}
	if !strings.HasPrefix(absPath, absRepo) {
		// Path traversal detected - return safe default
		return fmt.Sprintf("%s/%s/cluster.yaml", c.RepoDir, c.GetOutputDirName())
	}
	return path
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
// For ARO: credentials.yaml and aro.yaml
// For ROSA: secrets.yaml, is.yaml, and rosa.yaml
func (c *TestConfig) GetExpectedFiles() []string {
	if len(c.InfraProviders) > 0 {
		return c.InfraProviders[0].ExpectedFiles
	}
	// Fallback to defaults
	return []string{
		"credentials.yaml",
		"aro.yaml",
	}
}

// SharedTempDir returns a directory suitable for storing temporary files that
// must persist across CI steps. In Prow, SHARED_DIR is a volume shared between
// all step containers. Outside Prow, falls back to os.TempDir().
func (c *TestConfig) SharedTempDir() string {
	if dir := GetEnvOrDefault("SHARED_DIR", ""); dir != "" {
		return dir
	}
	return os.TempDir()
}

// GetKubeContext returns the kubectl context to use for the management cluster.
// For external clusters, extracts current-context from the kubeconfig file.
// For Kind clusters, returns "kind-{ManagementClusterName}".
//
// The result is computed once and cached: the external-cluster path shells out to
// `kubectl config current-context`, and this method is called dozens of times across
// test phases, so caching avoids repeatedly spawning that subprocess.
func (c *TestConfig) GetKubeContext() string {
	c.kubeContextOnce.Do(func() {
		if c.IsExternalCluster() {
			c.kubeContext = ExtractCurrentContext(c.UseKubeconfig)
		} else {
			c.kubeContext = fmt.Sprintf("kind-%s", c.ManagementClusterName)
		}
	})
	// If the external-cluster lookup failed (transient kubectl error), the Once
	// has fired but cached "". Fall back to a fresh lookup so callers can retry
	// rather than getting a permanently poisoned empty context.
	if c.IsExternalCluster() && c.kubeContext == "" {
		return ExtractCurrentContext(c.UseKubeconfig)
	}
	return c.kubeContext
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
