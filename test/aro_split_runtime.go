package test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type aroSplitCommands struct {
	apply func(context.Context, string) error
	run   func(context.Context, ...string) (string, error)
}

func validateAROSplitConfig(config *TestConfig) error {
	if !config.HasProvider("aro") {
		return fmt.Errorf("ARO_SPLIT_DEPLOYMENT requires the ARO provider")
	}
	secrets := []string{config.ARORoleCredentialSecret, config.ARODeploymentCredentialSecret}
	if !config.AROPreexistingIdentities {
		secrets = append(secrets, config.AROIdentityCredentialSecret)
	}
	seen := map[string]bool{}
	for _, name := range secrets {
		if name == "" {
			return fmt.Errorf("split deployment requires explicit identity and role credential secret names")
		}
		if err := ValidateRFC1123Name(name, "ARO credential secret"); err != nil {
			return err
		}
		if seen[name] {
			return fmt.Errorf("identity, role and deployment credentials must use different secrets")
		}
		seen[name] = true
	}
	return nil
}

// validateSplitCredentials checks routing identities, not their effective Azure roles.
// Authorization denial must be tested separately with operator-provided principals.
func validateSplitCredentials(ctx context.Context, config *TestConfig, credentials []map[string]interface{}, commands aroSplitCommands) error {
	ids := map[string]string{}
	names := []string{config.ARORoleCredentialSecret, config.ARODeploymentCredentialSecret}
	if !config.AROPreexistingIdentities {
		names = append(names, config.AROIdentityCredentialSecret)
	}
	for _, name := range names {
		output, err := commands.run(ctx, "get", "secret", name, "-o", "jsonpath={.data.AZURE_CLIENT_ID}")
		if err != nil {
			return fmt.Errorf("read client ID from credential secret %s: %w", name, err)
		}
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(output))
		clientID := strings.TrimSpace(string(decoded))
		if err != nil || clientID == "" {
			return fmt.Errorf("credential secret %s has no valid AZURE_CLIENT_ID", name)
		}
		for other, id := range ids {
			if strings.EqualFold(id, clientID) {
				return fmt.Errorf("credential secrets %s and %s use the same Azure client ID", other, name)
			}
		}
		ids[name] = clientID
	}
	_, clientID, err := splitCredentialIdentity(config, credentials)
	if err != nil {
		return err
	}
	if !strings.EqualFold(clientID, ids[config.ARODeploymentCredentialSecret]) {
		return fmt.Errorf("AzureClusterIdentity must use the deployment credential principal")
	}
	return nil
}

// Validate the entire credential bundle before applying any of it.
func splitCredentialIdentity(config *TestConfig, credentials []map[string]interface{}) (string, string, error) {
	var name, clientID string
	for _, document := range credentials {
		kind, _ := document["kind"].(string)
		if kind != "Secret" && kind != "AzureClusterIdentity" {
			return "", "", fmt.Errorf("credentials.yaml contains unexpected resource kind %q", kind)
		}
		metadata, _ := document["metadata"].(map[string]interface{})
		if metadata["namespace"] != config.WorkloadClusterNamespace {
			return "", "", fmt.Errorf("credential resource namespace must match workload namespace")
		}
		if kind == "AzureClusterIdentity" {
			if name != "" {
				return "", "", fmt.Errorf("credentials.yaml must contain exactly one deployment AzureClusterIdentity")
			}
			name, _ = metadata["name"].(string)
			spec, _ := document["spec"].(map[string]interface{})
			clientID, _ = spec["clientID"].(string)
			if name == "" || clientID == "" {
				return "", "", fmt.Errorf("deployment AzureClusterIdentity requires name and clientID")
			}
		}
	}
	if name == "" {
		return "", "", fmt.Errorf("credentials.yaml must contain a deployment AzureClusterIdentity")
	}
	return name, clientID, nil
}

func validateSplitIdentityReferences(config *TestConfig, plan *aroSplitPlan, identityName string) error {
	controlPlanes := 0
	for _, document := range plan.Cluster {
		if document["kind"] != "AROControlPlane" {
			continue
		}
		controlPlanes++
		spec, _ := document["spec"].(map[string]interface{})
		ref, _ := spec["identityRef"].(map[string]interface{})
		if ref["kind"] != "AzureClusterIdentity" || ref["name"] != identityName || ref["namespace"] != config.WorkloadClusterNamespace {
			return fmt.Errorf("AROControlPlane.identityRef must reference the validated deployment AzureClusterIdentity")
		}
	}
	if controlPlanes != 1 {
		return fmt.Errorf("split deployment requires exactly one AROControlPlane")
	}
	return nil
}

func readySplitResources(data []byte, expected int) error {
	var payload struct {
		Metadata struct {
			Generation int64 `json:"generation"`
		} `json:"metadata"`
		Status struct {
			Conditions []struct {
				Type, Status       string
				ObservedGeneration int64 `json:"observedGeneration"`
			} `json:"conditions"`
		} `json:"status"`
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("parse resource readiness: %w", err)
	}
	if payload.Items != nil {
		if len(payload.Items) != expected {
			return fmt.Errorf("expected %d resources, got %d", expected, len(payload.Items))
		}
		for _, item := range payload.Items {
			if err := readySplitResources(item, 1); err != nil {
				return err
			}
		}
		return nil
	}
	if expected != 1 {
		return fmt.Errorf("expected %d resources, received one", expected)
	}
	for _, condition := range payload.Status.Conditions {
		if condition.Type == "Ready" && condition.Status == "True" && condition.ObservedGeneration >= payload.Metadata.Generation && payload.Metadata.Generation > 0 {
			return nil
		}
	}
	return fmt.Errorf("resource is not Ready for its current generation; retry after ASO reconciles")
}

func waitSplitBundle(ctx context.Context, commands aroSplitCommands, path string, count int, pollInterval time.Duration) error {
	if count == 0 {
		return nil
	}
	var lastErr error
	for {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("wait for %s: %w (last observation: %v)", filepath.Base(path), err, lastErr)
		}
		data, err := commands.run(ctx, "get", "-f", path, "-o", "json")
		if err == nil {
			err = readySplitResources([]byte(data), count)
		}
		if err == nil {
			return nil
		}
		lastErr = err
		timer := time.NewTimer(pollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
		case <-timer.C:
		}
	}
}

// executeAROSplitPhase is shared by the full lifecycle and independent Make phases.
// Only identities applies credentials.yaml; later phases require completed setup.
func executeAROSplitPhase(config *TestConfig, phase string, commands aroSplitCommands) error {
	timeout := config.ClusterDeploymentTimeout
	if timeout <= 0 {
		timeout = DefaultClusterDeploymentTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := validateAROSplitConfig(config); err != nil {
		return err
	}
	if phase != "identities" && phase != "roles" && phase != "cluster" {
		return fmt.Errorf("unknown ARO split phase %q", phase)
	}
	outputDir := filepath.Join(config.RepoDir, config.GetOutputDirName())
	clusterData, err := os.ReadFile(filepath.Join(outputDir, config.ClusterYAML))
	if err != nil {
		return err
	}
	plan, err := splitAROResources(clusterData, config)
	if err != nil {
		return err
	}
	credentialPath := filepath.Join(outputDir, "credentials.yaml")
	credentialData, err := os.ReadFile(credentialPath)
	if err != nil {
		return err
	}
	credentials, err := readYAMLDocuments(credentialData)
	if err != nil {
		return err
	}
	identityName, _, err := splitCredentialIdentity(config, credentials)
	if err != nil {
		return err
	}
	if err := validateSplitIdentityReferences(config, plan, identityName); err != nil {
		return err
	}
	if phase == "identities" {
		if err := commands.apply(ctx, credentialPath); err != nil {
			return err
		}
	}
	if err := validateSplitCredentials(ctx, config, credentials, commands); err != nil {
		return err
	}
	bundles := []struct {
		name      string
		resources []map[string]interface{}
	}{
		{"prerequisites", plan.Prerequisites}, {"identities", plan.Identities}, {"roles", plan.Roles}, {"cluster", plan.Cluster},
	}
	paths := map[string]string{}
	for _, bundle := range bundles {
		path, err := writeSplitBundle(filepath.Join(outputDir, "split-deployment"), bundle.name, bundle.resources)
		if err != nil {
			return err
		}
		paths[bundle.name] = path
	}
	existing, err := commands.run(ctx, "get", capiClusterResource, config.GetProvisionedClusterName(), "--ignore-not-found", "-o", "name")
	if err != nil {
		return err
	}
	started := strings.TrimSpace(existing) != ""
	for _, bundle := range bundles[:3] {
		shouldApply := phase == "identities" && (bundle.name == "prerequisites" || bundle.name == "identities") || phase == "roles" && bundle.name == "roles"
		if shouldApply && !started && len(bundle.resources) > 0 {
			if err := commands.apply(ctx, paths[bundle.name]); err != nil {
				return err
			}
		}
		// Every phase proves its prerequisites exist and reconciled before proceeding.
		if phase == "identities" && bundle.name == "roles" {
			break
		}
		if err := waitSplitBundle(ctx, commands, paths[bundle.name], len(bundle.resources), 2*time.Second); err != nil {
			return fmt.Errorf("%s phase blocked by %s: %w", phase, bundle.name, err)
		}
	}
	if phase == "cluster" {
		return commands.apply(ctx, paths["cluster"])
	}
	return nil
}

func runAROSplitPhase(t *testing.T, phase string) {
	t.Helper()
	config := NewTestConfig()
	if !config.AROSplitDeployment {
		t.Skip("ARO_SPLIT_DEPLOYMENT is disabled")
	}
	SetupKubeconfig(t, config)
	kubeContext := ""
	commands := aroSplitCommands{
		run: func(ctx context.Context, args ...string) (string, error) {
			if kubeContext == "" {
				if config.IsExternalCluster() {
					var err error
					kubeContext, err = RunCommandQuietContext(ctx, t, "kubectl", "config", "current-context", "--kubeconfig", config.UseKubeconfig)
					if err != nil {
						return "", err
					}
					if kubeContext == "" {
						return "", fmt.Errorf("external kubeconfig has no current context")
					}
				} else {
					kubeContext = config.GetKubeContext()
				}
			}
			return RunCommandQuietContext(ctx, t, "kubectl", append([]string{"--context", kubeContext, "-n", config.WorkloadClusterNamespace, "--request-timeout=30s"}, args...)...)
		},
	}
	commands.apply = func(ctx context.Context, path string) error {
		var output string
		policy := RetryPolicy{MaxAttempts: DefaultApplyMaxRetries, InitialDelay: DefaultApplyRetryDelay, MaxDelay: 30 * time.Second,
			Sleep: func(delay time.Duration) {
				timer := time.NewTimer(delay)
				defer timer.Stop()
				select {
				case <-ctx.Done():
				case <-timer.C:
				}
			},
		}
		return policy.Execute(func() error {
			if err := ctx.Err(); err != nil {
				return err
			}
			var err error
			output, err = commands.run(ctx, "apply", "--validate=warn", "-f", path)
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if err == nil || IsKubectlApplySuccess(output) {
				return nil
			}
			return err
		}, func(err error) bool { return ctx.Err() == nil && isRetryableKubectlError(output, err) })
	}
	if err := executeAROSplitPhase(config, phase, commands); err != nil {
		t.Fatalf("ARO %s phase: %v", phase, err)
	}
	t.Logf("ARO %s phase complete; credential routing checked, effective Azure privileges require separate authorization testing", phase)
}
