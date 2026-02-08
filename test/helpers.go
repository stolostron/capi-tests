package test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// ClonedRepository represents information about a cloned git repository.
type ClonedRepository struct {
	URL    string // Repository URL (e.g., "https://github.com/RadekCap/cluster-api-installer")
	Branch string // Branch that was cloned (e.g., "ARO-ASO")
	Path   string // Local path where the repository was cloned
}

// clonedRepos stores information about all repositories cloned during tests.
// Access is protected by clonedReposMutex for thread safety.
var (
	clonedRepos      []ClonedRepository
	clonedReposMutex sync.Mutex
)

// RegisterClonedRepository records a cloned repository for tracking.
// This information is displayed in the final test output to show which
// repository versions were used during test execution.
func RegisterClonedRepository(url, branch, path string) {
	clonedReposMutex.Lock()
	defer clonedReposMutex.Unlock()

	// Check if already registered (avoid duplicates)
	for _, repo := range clonedRepos {
		if repo.URL == url && repo.Branch == branch {
			return
		}
	}

	clonedRepos = append(clonedRepos, ClonedRepository{
		URL:    url,
		Branch: branch,
		Path:   path,
	})
}

// GetClonedRepositories returns a copy of all registered cloned repositories.
func GetClonedRepositories() []ClonedRepository {
	clonedReposMutex.Lock()
	defer clonedReposMutex.Unlock()

	// Return a copy to avoid race conditions
	result := make([]ClonedRepository, len(clonedRepos))
	copy(result, clonedRepos)
	return result
}

// ClearClonedRepositories clears the list of cloned repositories.
// This is mainly useful for testing.
func ClearClonedRepositories() {
	clonedReposMutex.Lock()
	defer clonedReposMutex.Unlock()
	clonedRepos = nil
}

// CommandExists checks if a command is available in the system PATH
func CommandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// RunCommand executes a shell command and returns output and error.
// The command being executed is printed to TTY for immediate visibility.
func RunCommand(t *testing.T, name string, args ...string) (string, error) {
	t.Helper()

	// Build command string
	cmdStr := name
	if len(args) > 0 {
		cmdStr = fmt.Sprintf("%s %s", name, strings.Join(args, " "))
	}

	// Print command being executed to TTY for immediate visibility
	PrintToTTY("Running: %s\n", cmdStr)

	// Also log to test output
	t.Logf("Executing command: %s", cmdStr)

	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

// RunCommandQuiet executes a shell command without printing it to TTY.
// Use this for repeated commands in loops where printing would clutter the output.
// The command is still logged to test output for debugging purposes.
func RunCommandQuiet(t *testing.T, name string, args ...string) (string, error) {
	t.Helper()

	// Build command string for logging
	cmdStr := name
	if len(args) > 0 {
		cmdStr = fmt.Sprintf("%s %s", name, strings.Join(args, " "))
	}

	// Only log to test output (not TTY)
	t.Logf("Executing command (quiet): %s", cmdStr)

	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

// openTTY attempts to open /dev/tty for unbuffered output.
// Returns the file handle and a boolean indicating whether it should be closed.
// Falls back to os.Stderr if /dev/tty is unavailable (e.g., Windows, CI, or non-interactive).
func openTTY() (*os.File, bool) {
	tty, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		// Fallback to stderr if /dev/tty unavailable (Windows, CI, etc.)
		return os.Stderr, false
	}
	return tty, true
}

// RunCommandWithStreaming executes a shell command and streams output in real-time.
// This is useful for long-running commands where users need to see progress.
// Returns the complete output and any error that occurred.
//
// This function bypasses test framework buffering by writing directly to /dev/tty,
// ensuring output appears immediately even when run through gotestsum or go test.
func RunCommandWithStreaming(t *testing.T, name string, args ...string) (string, error) {
	t.Helper()

	// Print command being executed
	cmdStr := name
	if len(args) > 0 {
		cmdStr = fmt.Sprintf("%s %s", name, strings.Join(args, " "))
	}

	// Open TTY for unbuffered output (bypasses test framework buffering)
	tty, shouldClose := openTTY()
	if shouldClose {
		defer func() {
			if err := tty.Close(); err != nil {
				t.Logf("Warning: failed to close /dev/tty: %v", err)
			}
		}()
	}

	_, _ = fmt.Fprintf(tty, "Running (streaming): %s\n", cmdStr)
	t.Logf("Executing command (streaming): %s", cmdStr)

	cmd := exec.Command(name, args...)

	// Create pipes for stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start command: %w", err)
	}

	// Buffer to collect all output with mutex for thread-safety
	var outputBuilder strings.Builder
	var mu sync.Mutex

	// Stream output in real-time
	// Buffered channel prevents goroutine leaks if cmd.Wait() returns early
	done := make(chan bool, 2)

	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				chunk := string(buf[:n])

				// Thread-safe write to output builder
				mu.Lock()
				outputBuilder.WriteString(chunk)
				mu.Unlock()

				// Write to TTY for immediate visibility (best-effort, errors logged)
				if _, writeErr := tty.Write([]byte(chunk)); writeErr != nil {
					t.Logf("Warning: failed to write stdout to tty: %v", writeErr)
				}
			}
			if err != nil {
				break
			}
		}
		done <- true
	}()

	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stderr.Read(buf)
			if n > 0 {
				chunk := string(buf[:n])

				// Thread-safe write to output builder
				mu.Lock()
				outputBuilder.WriteString(chunk)
				mu.Unlock()

				// Write to TTY for immediate visibility (best-effort, errors logged)
				if _, writeErr := tty.Write([]byte(chunk)); writeErr != nil {
					t.Logf("Warning: failed to write stderr to tty: %v", writeErr)
				}
			}
			if err != nil {
				break
			}
		}
		done <- true
	}()

	// Wait for both readers to finish
	<-done
	<-done

	// Wait for command to complete
	cmdErr := cmd.Wait()

	// Thread-safe read of final output
	mu.Lock()
	output := strings.TrimSpace(outputBuilder.String())
	mu.Unlock()

	return output, cmdErr
}

// SetEnvVar sets an environment variable for testing
func SetEnvVar(t *testing.T, key, value string) {
	t.Helper()
	oldValue := os.Getenv(key)
	if err := os.Setenv(key, value); err != nil {
		t.Fatalf("Failed to set environment variable %s: %v", key, err)
	}
	t.Cleanup(func() {
		if oldValue == "" {
			if err := os.Unsetenv(key); err != nil {
				t.Logf("Warning: failed to unset environment variable %s: %v", key, err)
			}
		} else {
			if err := os.Setenv(key, oldValue); err != nil {
				t.Logf("Warning: failed to restore environment variable %s: %v", key, err)
			}
		}
	})
}

// FileExists checks if a file exists at the given path
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// DirExists checks if a directory exists at the given path
func DirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// GetEnvOrDefault returns environment variable value or default
func GetEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// ExtractCurrentContext reads the current-context from a kubeconfig file.
// Returns the context name or empty string if extraction fails.
func ExtractCurrentContext(kubeconfigPath string) string {
	output, err := exec.Command("kubectl", "config", "current-context",
		"--kubeconfig", kubeconfigPath).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// PrintTestHeader prints a clear test identification header to both terminal and test log.
// This helps users understand which test is running and what it does.
func PrintTestHeader(t *testing.T, testName, description string) {
	t.Helper()

	// Use openTTY helper for unbuffered output
	tty, shouldClose := openTTY()
	if shouldClose {
		defer func() {
			if err := tty.Close(); err != nil {
				t.Logf("Warning: failed to close /dev/tty: %v", err)
			}
		}()
	}

	// Print to terminal
	_, _ = fmt.Fprintf(tty, "\n=== RUN: %s ===\n", testName)
	_, _ = fmt.Fprintf(tty, "    %s\n\n", description)

	// Also log to test output
	t.Logf("=== RUN: %s ===", testName)
	t.Logf("    %s", description)
}

// PrintToTTY writes a message directly to the terminal (TTY), bypassing all
// buffering including test framework and gotestsum buffering. This ensures
// immediate visibility of output during test execution.
func PrintToTTY(format string, args ...interface{}) {
	tty, shouldClose := openTTY()
	if shouldClose {
		defer func() {
			if err := tty.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to close /dev/tty: %v\n", err)
			}
		}()
	}
	_, _ = fmt.Fprintf(tty, format, args...)
}

// ReportProgress prints progress information to TTY for real-time visibility
// and to test log for test output. This helper ensures consistent progress
// reporting across all deployment tests.
func ReportProgress(t *testing.T, iteration int, elapsed, remaining, timeout time.Duration) {
	t.Helper()
	percentage := int((float64(elapsed) / float64(timeout)) * 100)

	// Print to TTY for real-time visibility (bypasses all buffering)
	PrintToTTY("[%d] ⏳ Waiting... | Elapsed: %v | Remaining: %v | Progress: %d%%\n",
		iteration,
		elapsed.Round(time.Second),
		remaining.Round(time.Second),
		percentage)

	// Also log to test output
	t.Logf("Waiting iteration %d (elapsed: %v, remaining: %v, %d%%)",
		iteration, elapsed.Round(time.Second), remaining.Round(time.Second), percentage)
}

// IsKubectlApplySuccess checks if kubectl apply output indicates success.
// kubectl apply may return non-zero exit codes even when operations succeed,
// particularly when resources are "unchanged".
func IsKubectlApplySuccess(output string) bool {
	// Error indicators in kubectl output
	errorKeywords := []string{
		"error", "failed", "invalid", "unable to", "warning", "forbidden", "unauthorized", "not found",
	}

	lowerOutput := strings.ToLower(output)

	// Check for error keywords
	for _, keyword := range errorKeywords {
		if strings.Contains(lowerOutput, keyword) {
			return false
		}
	}

	// Check for success indicators to ensure operation actually completed
	// kubectl apply outputs "created", "configured", "unchanged" for success
	successKeywords := []string{"created", "configured", "unchanged"}
	for _, keyword := range successKeywords {
		if strings.Contains(lowerOutput, keyword) {
			return true
		}
	}

	// If output has no errors but also no success indicators, it's likely empty or unexpected
	// Return false to be conservative
	return false
}

// ExtractClusterNameFromYAML extracts the cluster name from a multi-document YAML file.
// It looks for a document with kind: Cluster (cluster.x-k8s.io/v1beta2) and returns
// its metadata.name field. This is used to get the actual provisioned cluster name
// from the generated aro.yaml file, which may differ from WORKLOAD_CLUSTER_NAME.
//
// Example YAML:
//
//	---
//	apiVersion: cluster.x-k8s.io/v1beta2
//	kind: Cluster
//	metadata:
//	  name: mveber-stage
//	  namespace: default
//
// Returns the cluster name or an error if not found.
func ExtractClusterNameFromYAML(filePath string) (string, error) {
	// Check if file exists
	if _, err := os.Stat(filePath); err != nil {
		return "", fmt.Errorf("file not accessible: %w", err)
	}

	// Read file contents
	// #nosec G304 - filePath comes from test configuration
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	// Split by document separator and parse each document
	docs := strings.Split(string(data), "---")
	for _, doc := range docs {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}

		// Parse the YAML document
		var content map[string]interface{}
		if err := yaml.Unmarshal([]byte(doc), &content); err != nil {
			// Skip documents that don't parse as objects
			continue
		}

		// Check if this is a Cluster resource
		kind, ok := content["kind"].(string)
		if !ok || kind != "Cluster" {
			continue
		}

		// Verify it's the CAPI Cluster type (cluster.x-k8s.io)
		apiVersion, ok := content["apiVersion"].(string)
		if !ok || !strings.HasPrefix(apiVersion, "cluster.x-k8s.io/") {
			continue
		}

		// Extract metadata.name
		metadata, ok := content["metadata"].(map[string]interface{})
		if !ok {
			continue
		}

		name, ok := metadata["name"].(string)
		if !ok || name == "" {
			continue
		}

		return name, nil
	}

	return "", fmt.Errorf("no Cluster resource found in %s", filePath)
}

// CheckYAMLConfigMatch verifies that existing YAML files match the current configuration.
// It extracts the cluster name from the aro.yaml file and compares it with the expected
// cluster name prefix. This is used to detect configuration mismatches that would cause
// the test to use stale YAML files with outdated values.
//
// Returns:
//   - matches: true if the existing YAML matches the expected prefix, false otherwise
//   - existingPrefix: the cluster name extracted from the existing YAML file
//
// If the file doesn't exist or cannot be parsed, returns (false, "") to trigger regeneration.
func CheckYAMLConfigMatch(t *testing.T, aroYAMLPath, expectedPrefix string) (matches bool, existingPrefix string) {
	t.Helper()

	// Extract cluster name from existing aro.yaml
	clusterName, err := ExtractClusterNameFromYAML(aroYAMLPath)
	if err != nil {
		// File doesn't exist or can't be parsed - needs regeneration
		t.Logf("Could not extract cluster name from %s: %v", aroYAMLPath, err)
		return false, ""
	}

	// Compare the extracted cluster name with expected prefix
	// The cluster name in aro.yaml should match the ClusterNamePrefix (e.g., "rcapu-stage")
	if clusterName == expectedPrefix {
		return true, clusterName
	}

	return false, clusterName
}

// AROControlPlaneCondition represents a condition from the AROControlPlane status
type AROControlPlaneCondition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

// WaitingPattern defines a pattern to detect waiting states that may appear as failures.
type WaitingPattern struct {
	Pattern     string // Substring to match in the message
	Description string // User-friendly description of what is being waited for
}

// waitingPatterns defines known patterns where ReconciliationFailed or similar reasons
// actually indicate a waiting/pending state rather than an actual failure.
// These patterns are matched against the condition's Message field.
var waitingPatterns = []WaitingPattern{
	{Pattern: "requires at least one ready machine pool", Description: "Waiting for machine pool"},
	{Pattern: "waiting for", Description: "Waiting for dependency"},
	{Pattern: "will be requeued", Description: "Waiting (will retry)"},
	{Pattern: "not found", Description: "Waiting for resource creation"},
	{Pattern: "is not ready", Description: "Waiting for dependency"},
	{Pattern: "not yet available", Description: "Waiting for availability"},
	{Pattern: "still being created", Description: "Waiting for creation"},
	{Pattern: "still provisioning", Description: "Waiting for provisioning"},
}

// isWaitingCondition checks if a condition with False status and a failure-like reason
// is actually just waiting for something. Returns true and a description if it's a waiting state.
func isWaitingCondition(cond AROControlPlaneCondition) (bool, string) {
	// Only check conditions that appear to be failures
	if cond.Status != "False" {
		return false, ""
	}

	// Check if the message contains any known waiting patterns
	messageLower := strings.ToLower(cond.Message)
	for _, wp := range waitingPatterns {
		if strings.Contains(messageLower, strings.ToLower(wp.Pattern)) {
			return true, wp.Description
		}
	}

	return false, ""
}

// FormatAROControlPlaneConditions formats AROControlPlane conditions for display.
// It parses the JSON output from kubectl and returns a formatted string showing
// the status of each condition with visual indicators.
func FormatAROControlPlaneConditions(jsonData string) string {
	if strings.TrimSpace(jsonData) == "" {
		return "  (no conditions available)"
	}

	// Parse the JSON - it could be a full status object or just conditions array
	var conditions []AROControlPlaneCondition

	// Try parsing as conditions array first
	if err := json.Unmarshal([]byte(jsonData), &conditions); err != nil {
		// Try parsing as status object with conditions field
		var status struct {
			Conditions []AROControlPlaneCondition `json:"conditions"`
		}
		if err := json.Unmarshal([]byte(jsonData), &status); err != nil {
			return fmt.Sprintf("  (failed to parse conditions: %v)", err)
		}
		conditions = status.Conditions
	}

	if len(conditions) == 0 {
		return "  (no conditions available)"
	}

	var result strings.Builder
	for _, cond := range conditions {
		// Check if this is a waiting condition disguised as a failure
		isWaiting, waitingDesc := isWaitingCondition(cond)

		// Determine the icon based on status and waiting detection
		icon := "⏳" // pending/unknown
		switch cond.Status {
		case "True":
			icon = "✅"
		case "False":
			if isWaiting {
				icon = "⏳" // waiting state, not a failure
			} else {
				icon = "🔄" // in-progress/retry
			}
		}

		// Format the condition line
		result.WriteString(fmt.Sprintf("  %s %s: %s", icon, cond.Type, cond.Status))

		// Add context based on whether it's a waiting condition or regular status
		if cond.Status != "True" {
			if isWaiting {
				// Show user-friendly waiting description instead of misleading "ReconciliationFailed"
				result.WriteString(fmt.Sprintf(" (%s)", waitingDesc))
			} else if cond.Reason != "" {
				// Show the original reason for non-waiting conditions
				result.WriteString(fmt.Sprintf(" (%s)", cond.Reason))
			}
		}

		result.WriteString("\n")
	}

	return result.String()
}

// EnsureAzureCredentialsSet ensures Azure credentials are available as environment variables.
// If AZURE_TENANT_ID or AZURE_SUBSCRIPTION_ID are not set, it auto-extracts them from
// the Azure CLI. This is critical for the deployment script which needs these env vars
// to configure the ASO controller credentials in the Kind cluster.
//
// Returns an error if credentials cannot be obtained (Azure CLI not logged in or failed).
func EnsureAzureCredentialsSet(t *testing.T) error {
	t.Helper()

	// Check and set AZURE_TENANT_ID
	if os.Getenv("AZURE_TENANT_ID") == "" {
		output, err := RunCommandQuiet(t, "az", "account", "show", "--query", "tenantId", "-o", "tsv")
		if err != nil {
			return fmt.Errorf("AZURE_TENANT_ID not set and could not extract from Azure CLI: %w", err)
		}
		tenantID := strings.TrimSpace(output)
		if tenantID == "" {
			return fmt.Errorf("AZURE_TENANT_ID not set and Azure CLI returned empty tenant ID")
		}
		if err := os.Setenv("AZURE_TENANT_ID", tenantID); err != nil {
			return fmt.Errorf("failed to set AZURE_TENANT_ID: %w", err)
		}
		t.Logf("AZURE_TENANT_ID auto-extracted from Azure CLI")
	}

	// Check and set AZURE_SUBSCRIPTION_ID (if neither ID nor NAME is set)
	if os.Getenv("AZURE_SUBSCRIPTION_ID") == "" && os.Getenv("AZURE_SUBSCRIPTION_NAME") == "" {
		output, err := RunCommandQuiet(t, "az", "account", "show", "--query", "id", "-o", "tsv")
		if err != nil {
			return fmt.Errorf("AZURE_SUBSCRIPTION_ID not set and could not extract from Azure CLI: %w", err)
		}
		subID := strings.TrimSpace(output)
		if subID == "" {
			return fmt.Errorf("AZURE_SUBSCRIPTION_ID not set and Azure CLI returned empty subscription ID")
		}
		if err := os.Setenv("AZURE_SUBSCRIPTION_ID", subID); err != nil {
			return fmt.Errorf("failed to set AZURE_SUBSCRIPTION_ID: %w", err)
		}
		t.Logf("AZURE_SUBSCRIPTION_ID auto-extracted from Azure CLI")
	}

	return nil
}

// PatchASOCredentialsSecret patches the aso-controller-settings secret with Azure credentials.
// The cluster-api-installer helm chart creates this secret with empty values, so we need to
// patch it with actual credentials after deployment.
//
// This function:
// 1. Gets AZURE_TENANT_ID and AZURE_SUBSCRIPTION_ID from environment (or extracts from Azure CLI)
// 2. Optionally includes AZURE_CLIENT_ID and AZURE_CLIENT_SECRET if both are set
// 3. Patches the secret in the controller namespace (capz-system or multicluster-engine)
//
// Service principal credentials (AZURE_CLIENT_ID/AZURE_CLIENT_SECRET) are optional for local
// development but required for ASO to work in Kind clusters since Kind cannot use managed
// identity or workload identity.
//
// Returns an error if credentials cannot be obtained or patching fails.
func PatchASOCredentialsSecret(t *testing.T, kubeContext string) error {
	t.Helper()

	// Ensure credentials are available
	if err := EnsureAzureCredentialsSet(t); err != nil {
		return fmt.Errorf("failed to ensure Azure credentials: %w", err)
	}

	tenantID := os.Getenv("AZURE_TENANT_ID")
	subscriptionID := os.Getenv("AZURE_SUBSCRIPTION_ID")

	if tenantID == "" || subscriptionID == "" {
		return fmt.Errorf("AZURE_TENANT_ID or AZURE_SUBSCRIPTION_ID is empty after extraction")
	}

	// Build the patch JSON with required credentials
	// Start with tenant ID and subscription ID (always required)
	patchData := map[string]string{
		"AZURE_TENANT_ID":       tenantID,
		"AZURE_SUBSCRIPTION_ID": subscriptionID,
	}

	// Add service principal credentials if available
	// These are optional for local development but required for ASO to work in Kind clusters
	clientID := os.Getenv("AZURE_CLIENT_ID")
	clientSecret := os.Getenv("AZURE_CLIENT_SECRET")
	if clientID != "" && clientSecret != "" {
		patchData["AZURE_CLIENT_ID"] = clientID
		patchData["AZURE_CLIENT_SECRET"] = clientSecret
		t.Log("Including service principal credentials in ASO secret patch")
	}

	// Build the JSON patch string
	var pairs []string
	for key, value := range patchData {
		pairs = append(pairs, fmt.Sprintf(`"%s":"%s"`, key, value))
	}
	patchJSON := fmt.Sprintf(`{"stringData":{%s}}`, strings.Join(pairs, ","))

	// Get controller namespace from config
	config := NewTestConfig()

	output, err := RunCommandQuiet(t, "kubectl", "--context", kubeContext,
		"-n", config.CAPZNamespace, "patch", "secret", "aso-controller-settings",
		"--type=merge", "-p", patchJSON)
	if err != nil {
		return fmt.Errorf("failed to patch aso-controller-settings secret: %w\nOutput: %s", err, output)
	}

	t.Log("Patched aso-controller-settings secret with Azure credentials")
	return nil
}

// MaxDomainPrefixLength is the maximum allowed length for ARO domain prefix.
// Azure/ARO enforces this limit on the AROControlPlane spec.domainPrefix field.
const MaxDomainPrefixLength = 15

// MaxExternalAuthIDLength is the maximum allowed length for ExternalAuth resource ID.
// Azure enforces this limit on the ExternalAuth resource name.
const MaxExternalAuthIDLength = 15

// ExternalAuthIDSuffix is the suffix appended to CS_CLUSTER_NAME to form the ExternalAuth ID.
// The ExternalAuth resource name is constructed as ${CS_CLUSTER_NAME}-ea.
const ExternalAuthIDSuffix = "-ea"

// MaxClusterNamePrefixLength is the maximum allowed length for CS_CLUSTER_NAME,
// calculated as MaxExternalAuthIDLength minus the length of ExternalAuthIDSuffix.
// This ensures the resulting ExternalAuth ID (${CS_CLUSTER_NAME}-ea) stays within limits.
const MaxClusterNamePrefixLength = MaxExternalAuthIDLength - len(ExternalAuthIDSuffix) // 12

// GetDomainPrefix returns the domain prefix that will be used for the ARO cluster.
// The domain prefix is derived from CAPZ_USER and DEPLOYMENT_ENV environment variables
// in the format "${CAPZ_USER}-${DEPLOYMENT_ENV}".
func GetDomainPrefix(user, environment string) string {
	return fmt.Sprintf("%s-%s", user, environment)
}

// ValidateDomainPrefix checks if the domain prefix length is within the allowed limit.
// Returns an error with a descriptive message if the prefix exceeds MaxDomainPrefixLength (15 chars).
// The domain prefix is derived from CAPZ_USER and DEPLOYMENT_ENV in the format "${CAPZ_USER}-${DEPLOYMENT_ENV}".
func ValidateDomainPrefix(user, environment string) error {
	prefix := GetDomainPrefix(user, environment)
	if len(prefix) > MaxDomainPrefixLength {
		return fmt.Errorf(
			"domain prefix '%s' (%d chars) exceeds maximum length of %d characters\n"+
				"  CAPZ_USER='%s' (%d chars) + '-' + DEPLOYMENT_ENV='%s' (%d chars) = %d chars\n"+
				"  Suggestion: Use shorter values for CAPZ_USER or DEPLOYMENT_ENV environment variables",
			prefix, len(prefix), MaxDomainPrefixLength,
			user, len(user), environment, len(environment), len(prefix))
	}
	return nil
}

// RFC1123NameRegex is a regex for RFC 1123 subdomain name validation.
// Names must consist of lowercase alphanumeric characters or '-', and must start
// and end with an alphanumeric character.
var RFC1123NameRegex = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

// ValidateRFC1123Name validates that a name complies with RFC 1123 subdomain naming.
// RFC 1123 subdomain names must:
// - Consist of lowercase alphanumeric characters or '-'
// - Start and end with an alphanumeric character
// - Not be empty
//
// This is used to validate environment variables like CAPZ_USER, CS_CLUSTER_NAME,
// and DEPLOYMENT_ENV before deployment, preventing late failures in CR deployment.
//
// Parameters:
//   - name: the value to validate
//   - varName: the environment variable name (for error messages)
//
// Returns nil if valid, or an error with remediation suggestion if invalid.
func ValidateRFC1123Name(name, varName string) error {
	if name == "" {
		return fmt.Errorf("%s is empty: must be a non-empty RFC 1123 compliant name", varName)
	}

	if RFC1123NameRegex.MatchString(name) {
		return nil
	}

	// Build detailed error message with specific issues
	var issues []string

	// Check for uppercase letters
	if strings.ToLower(name) != name {
		issues = append(issues, "contains uppercase letters")
	}

	// Check for invalid characters (not alphanumeric or hyphen)
	invalidChars := regexp.MustCompile(`[^a-z0-9-]`)
	if invalidChars.MatchString(strings.ToLower(name)) {
		issues = append(issues, "contains invalid characters (only lowercase a-z, 0-9, and '-' are allowed)")
	}

	// Check if starts with non-alphanumeric
	if len(name) > 0 && !regexp.MustCompile(`^[a-z0-9]`).MatchString(strings.ToLower(name)) {
		issues = append(issues, "must start with a lowercase alphanumeric character")
	}

	// Check if ends with non-alphanumeric
	if len(name) > 0 && !regexp.MustCompile(`[a-z0-9]$`).MatchString(strings.ToLower(name)) {
		issues = append(issues, "must end with a lowercase alphanumeric character")
	}

	// Generate suggested fix (lowercase, replace invalid chars)
	suggested := strings.ToLower(name)
	suggested = regexp.MustCompile(`[^a-z0-9-]`).ReplaceAllString(suggested, "-")
	suggested = strings.Trim(suggested, "-")
	if suggested == "" {
		suggested = "valid-name"
	}

	return fmt.Errorf(
		"%s '%s' is not RFC 1123 compliant:\n"+
			"  Issues: %s\n"+
			"  RFC 1123 requires: lowercase alphanumeric characters or '-', must start and end with alphanumeric\n"+
			"  Suggested fix: export %s=%s",
		varName, name, strings.Join(issues, "; "), varName, suggested)
}

// GetExternalAuthID returns the ExternalAuth resource ID that will be created for the ARO cluster.
// The ExternalAuth ID is derived from CS_CLUSTER_NAME (clusterNamePrefix) with the suffix "-ea".
func GetExternalAuthID(clusterNamePrefix string) string {
	return clusterNamePrefix + ExternalAuthIDSuffix
}

// ValidateExternalAuthID checks if the ExternalAuth ID length is within the allowed limit.
// Returns an error with a descriptive message if the ID exceeds MaxExternalAuthIDLength (15 chars).
// The ExternalAuth ID is constructed as ${CS_CLUSTER_NAME}-ea.
//
// This validation catches deployment failures early in prerequisites, rather than waiting
// for the CR reconciliation phase where the error "ExternalAuth id '...' is X characters long -
// its length exceeds the maximum length allowed of 15 characters" would occur.
func ValidateExternalAuthID(clusterNamePrefix string) error {
	externalAuthID := GetExternalAuthID(clusterNamePrefix)
	if len(externalAuthID) > MaxExternalAuthIDLength {
		// Calculate a suggested shorter name
		suggestedName := clusterNamePrefix
		if len(clusterNamePrefix) > MaxClusterNamePrefixLength {
			suggestedName = clusterNamePrefix[:MaxClusterNamePrefixLength]
		}

		return fmt.Errorf(
			"ExternalAuth ID '%s' (%d chars) exceeds maximum length of %d characters\n"+
				"  CS_CLUSTER_NAME='%s' (%d chars) + '-ea' (3 chars) = %d chars\n"+
				"  CS_CLUSTER_NAME must be ≤%d characters to allow for the '-ea' suffix\n"+
				"  Suggestion: export CS_CLUSTER_NAME=%s",
			externalAuthID, len(externalAuthID), MaxExternalAuthIDLength,
			clusterNamePrefix, len(clusterNamePrefix), len(externalAuthID),
			MaxClusterNamePrefixLength,
			suggestedName)
	}
	return nil
}

// DefaultHealthCheckTimeout is the default timeout for cluster health checks
const DefaultHealthCheckTimeout = 2 * time.Minute

// DefaultApplyMaxRetries is the default maximum number of retries for kubectl apply
const DefaultApplyMaxRetries = 5

// DefaultApplyRetryDelay is the initial delay between kubectl apply retries
const DefaultApplyRetryDelay = 10 * time.Second

// WaitForClusterHealthy checks if the Kind cluster API server is responsive.
// It performs a simple kubectl get nodes command to verify connectivity.
// This function retries with exponential backoff until the cluster responds or timeout is reached.
//
// Use this before applying CRs after a long controller startup period, as the API server
// may become temporarily unresponsive due to resource exhaustion or network issues.
func WaitForClusterHealthy(t *testing.T, kubeContext string, timeout time.Duration) error {
	t.Helper()

	if timeout == 0 {
		timeout = DefaultHealthCheckTimeout
	}

	startTime := time.Now()
	attempt := 0
	baseDelay := 5 * time.Second

	PrintToTTY("\n=== Checking cluster health ===\n")
	PrintToTTY("Context: %s | Timeout: %v\n", kubeContext, timeout)
	t.Logf("Checking cluster health (context: %s, timeout: %v)", kubeContext, timeout)

	for {
		attempt++
		elapsed := time.Since(startTime)

		if elapsed > timeout {
			PrintToTTY("❌ Cluster health check timed out after %v\n\n", elapsed.Round(time.Second))
			return fmt.Errorf("cluster health check timed out after %v", elapsed.Round(time.Second))
		}

		// Try a simple kubectl command to check API server responsiveness
		PrintToTTY("[%d] Checking API server responsiveness...\n", attempt)

		_, err := RunCommandQuiet(t, "kubectl", "--context", kubeContext, "get", "nodes", "--request-timeout=10s")
		if err == nil {
			PrintToTTY("✅ Cluster is healthy and responding\n\n")
			t.Log("Cluster is healthy and responding")
			return nil
		}

		// Calculate next delay with exponential backoff (capped at 30 seconds)
		delay := baseDelay * time.Duration(attempt)
		if delay > 30*time.Second {
			delay = 30 * time.Second
		}

		remaining := timeout - elapsed
		if delay > remaining {
			delay = remaining
		}

		PrintToTTY("[%d] ⚠️  API server not responding: %v\n", attempt, err)
		PrintToTTY("[%d] ⏳ Retrying in %v (elapsed: %v, remaining: %v)\n", attempt, delay.Round(time.Second), elapsed.Round(time.Second), remaining.Round(time.Second))
		t.Logf("Cluster health check failed (attempt %d): %v, retrying in %v", attempt, err, delay.Round(time.Second))

		time.Sleep(delay)
	}
}

// ApplyWithRetry applies a YAML file using kubectl with retry logic and exponential backoff.
// This is useful when the API server may be temporarily unresponsive after long controller
// startup periods.
//
// Parameters:
//   - t: testing context
//   - kubeContext: kubectl context to use
//   - yamlPath: path to the YAML file to apply
//   - maxRetries: maximum number of retry attempts (use 0 for default of 5)
//
// Returns nil on success, or an error if all retries are exhausted.
func ApplyWithRetry(t *testing.T, kubeContext, yamlPath string, maxRetries int) error {
	t.Helper()
	// Use the configured workload cluster namespace
	config := NewTestConfig()
	return ApplyWithRetryInNamespace(t, kubeContext, config.WorkloadClusterNamespace, yamlPath, maxRetries)
}

// ApplyWithRetryInNamespace applies a YAML file with retry logic to a specific namespace.
// Parameters:
//   - kubeContext: kubectl context to use
//   - namespace: Kubernetes namespace to apply resources to
//   - yamlPath: path to the YAML file to apply
//   - maxRetries: maximum number of retry attempts (use 0 for default of 5)
//
// Returns nil on success, or an error if all retries are exhausted.
func ApplyWithRetryInNamespace(t *testing.T, kubeContext, namespace, yamlPath string, maxRetries int) error {
	t.Helper()

	if maxRetries <= 0 {
		maxRetries = DefaultApplyMaxRetries
	}

	baseDelay := DefaultApplyRetryDelay

	for attempt := 1; attempt <= maxRetries; attempt++ {
		PrintToTTY("[%d/%d] Applying %s to namespace %s...\n", attempt, maxRetries, yamlPath, namespace)
		t.Logf("Applying %s to namespace %s (attempt %d/%d)", yamlPath, namespace, attempt, maxRetries)

		output, err := RunCommandQuiet(t, "kubectl", "--context", kubeContext, "-n", namespace, "apply", "-f", yamlPath)

		// Check if apply was successful
		if err == nil || IsKubectlApplySuccess(output) {
			PrintToTTY("✅ Successfully applied %s\n", yamlPath)
			t.Logf("Successfully applied %s", yamlPath)
			return nil
		}

		// Determine if error is retryable
		if !isRetryableKubectlError(output, err) {
			PrintToTTY("❌ Non-retryable error applying %s: %v\n", yamlPath, err)
			t.Logf("Non-retryable error applying %s: %v\nOutput: %s", yamlPath, err, output)
			return fmt.Errorf("failed to apply %s: %w\nOutput: %s", yamlPath, err, output)
		}

		// Don't sleep after last attempt
		if attempt < maxRetries {
			// Exponential backoff: 10s, 20s, 40s, 60s (capped)
			delay := baseDelay * time.Duration(attempt)
			if delay > 60*time.Second {
				delay = 60 * time.Second
			}

			PrintToTTY("[%d/%d] ⚠️  Retryable error: %v\n", attempt, maxRetries, err)
			PrintToTTY("[%d/%d] ⏳ Waiting %v before retry...\n", attempt, maxRetries, delay.Round(time.Second))
			t.Logf("Apply failed (attempt %d/%d): %v, retrying in %v", attempt, maxRetries, err, delay.Round(time.Second))

			time.Sleep(delay)
		} else {
			PrintToTTY("❌ Failed to apply %s after %d attempts: %v\n", yamlPath, maxRetries, err)
			t.Logf("Failed to apply %s after %d attempts: %v\nOutput: %s", yamlPath, maxRetries, err, output)
			return fmt.Errorf("failed to apply %s after %d attempts: %w\nOutput: %s", yamlPath, maxRetries, err, output)
		}
	}

	// This should never be reached, but just in case
	return fmt.Errorf("failed to apply %s: exhausted all retries", yamlPath)
}

// isRetryableKubectlError determines if a kubectl error is retryable.
// Returns true for transient errors like connection issues, timeouts, and server unavailability.
func isRetryableKubectlError(output string, err error) bool {
	if err == nil {
		return false
	}

	// Combine error message and output for checking
	combined := strings.ToLower(output + " " + err.Error())

	// Retryable error patterns (transient issues)
	retryablePatterns := []string{
		"connection refused",
		"was refused",
		"connection reset",
		"connection lost",
		"client connection lost",
		"tls handshake timeout",
		"i/o timeout",
		"net/http",
		"context deadline exceeded",
		"server unavailable",
		"service unavailable",
		"gateway timeout",
		"too many requests",
		"internal server error",
		"http2",
		"dial tcp",
		"no such host",
		"temporary failure",
		"connection timed out",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(combined, pattern) {
			return true
		}
	}

	return false
}

// AzureErrorInfo contains information about a detected Azure error and remediation steps.
type AzureErrorInfo struct {
	ErrorType   string   // Short error type identifier (e.g., "insufficient_privileges")
	Message     string   // Human-readable error description
	Remediation []string // Steps to fix the error
}

// DetectAzureError analyzes command output for known Azure error patterns
// and returns detailed error information with remediation steps.
// Returns nil if no known Azure error pattern is detected.
//
// This function helps provide actionable guidance when Azure operations fail,
// particularly during service principal creation, RBAC operations, and resource provisioning.
func DetectAzureError(output string) *AzureErrorInfo {
	lowerOutput := strings.ToLower(output)

	// Insufficient privileges error (service principal creation, role assignments)
	if strings.Contains(lowerOutput, "insufficient privileges") {
		return &AzureErrorInfo{
			ErrorType: "insufficient_privileges",
			Message:   "Azure operation failed due to insufficient privileges",
			Remediation: []string{
				"Verify you have the required Azure AD role to create service principals:",
				"  - Application Administrator, or",
				"  - Cloud Application Administrator, or",
				"  - Global Administrator (not recommended for production)",
				"Run: az ad signed-in-user show --query displayName -o tsv",
				"Check your role assignments: az role assignment list --assignee $(az ad signed-in-user show --query id -o tsv) --all",
				"Contact your Azure AD administrator if you need elevated permissions",
			},
		}
	}

	// Authorization failed error
	if strings.Contains(lowerOutput, "authorizationfailed") ||
		strings.Contains(lowerOutput, "authorization failed") ||
		strings.Contains(lowerOutput, "does not have authorization") {
		return &AzureErrorInfo{
			ErrorType: "authorization_failed",
			Message:   "Azure authorization failed for the requested operation",
			Remediation: []string{
				"Verify you have the required RBAC role on the subscription/resource group:",
				"  - Contributor, or",
				"  - Owner (for role assignments)",
				"Check your subscription access: az account show",
				"List your role assignments: az role assignment list --assignee $(az ad signed-in-user show --query id -o tsv) --all",
				"Ensure you're using the correct subscription: az account set --subscription <subscription-id>",
			},
		}
	}

	// Subscription not found or access denied
	if strings.Contains(lowerOutput, "subscriptionnotfound") ||
		strings.Contains(lowerOutput, "subscription not found") ||
		strings.Contains(lowerOutput, "subscription was not found") {
		return &AzureErrorInfo{
			ErrorType: "subscription_not_found",
			Message:   "The specified Azure subscription was not found or you don't have access",
			Remediation: []string{
				"Verify the subscription ID is correct: az account list -o table",
				"Ensure you have access to the subscription",
				"Try switching subscription: az account set --subscription <subscription-id>",
				"Re-login if needed: az login",
			},
		}
	}

	// Resource group not found
	if strings.Contains(lowerOutput, "resourcegroupnotfound") ||
		strings.Contains(lowerOutput, "resource group") && strings.Contains(lowerOutput, "not found") {
		return &AzureErrorInfo{
			ErrorType: "resource_group_not_found",
			Message:   "The specified resource group was not found",
			Remediation: []string{
				"Verify the resource group exists: az group list -o table",
				"Create the resource group if needed: az group create --name <name> --location <location>",
				"Check if CS_CLUSTER_NAME environment variable is set correctly",
			},
		}
	}

	// Quota exceeded
	if strings.Contains(lowerOutput, "quotaexceeded") ||
		strings.Contains(lowerOutput, "quota exceeded") ||
		strings.Contains(lowerOutput, "exceeds quota") {
		return &AzureErrorInfo{
			ErrorType: "quota_exceeded",
			Message:   "Azure resource quota exceeded",
			Remediation: []string{
				"Check your current quota usage: az vm list-usage --location <region> -o table",
				"Request a quota increase through Azure Portal:",
				"  1. Navigate to Subscriptions > Your Subscription > Usage + quotas",
				"  2. Find the resource type and click 'Request Increase'",
				"Consider using a different Azure region with available capacity",
			},
		}
	}

	// Service principal already exists
	if strings.Contains(lowerOutput, "already exists") && strings.Contains(lowerOutput, "service principal") {
		return &AzureErrorInfo{
			ErrorType: "sp_already_exists",
			Message:   "A service principal with this name already exists",
			Remediation: []string{
				"List existing service principals: az ad sp list --display-name <name> -o table",
				"Delete the existing service principal if not needed: az ad sp delete --id <sp-id>",
				"Or use a different name for the new service principal",
			},
		}
	}

	// Invalid client secret or credentials expired
	if strings.Contains(lowerOutput, "invalid_client") ||
		strings.Contains(lowerOutput, "invalid client secret") ||
		strings.Contains(lowerOutput, "credentials have expired") {
		return &AzureErrorInfo{
			ErrorType: "invalid_credentials",
			Message:   "Azure credentials are invalid or have expired",
			Remediation: []string{
				"Re-authenticate with Azure CLI: az login",
				"If using service principal, regenerate the secret:",
				"  az ad sp credential reset --id <sp-id>",
				"Update AZURE_CLIENT_SECRET environment variable if applicable",
			},
		}
	}

	// Azure CLI not logged in
	if strings.Contains(lowerOutput, "please run 'az login'") ||
		strings.Contains(lowerOutput, "not logged in") ||
		strings.Contains(lowerOutput, "no subscription found") {
		return &AzureErrorInfo{
			ErrorType: "not_logged_in",
			Message:   "Azure CLI is not logged in or session has expired",
			Remediation: []string{
				"Login to Azure CLI: az login",
				"For service principal login: az login --service-principal -u <client-id> -p <secret> --tenant <tenant-id>",
				"Verify login status: az account show",
			},
		}
	}

	return nil
}

// FormatAzureError formats an AzureErrorInfo for display.
// Returns a formatted string with the error message and remediation steps.
func FormatAzureError(info *AzureErrorInfo) string {
	if info == nil {
		return ""
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("\n=== Azure Error Detected: %s ===\n", info.Message))
	result.WriteString("\nRemediation steps:\n")
	for _, step := range info.Remediation {
		result.WriteString(fmt.Sprintf("  %s\n", step))
	}
	result.WriteString("\n")

	return result.String()
}

// GetClusterPhase retrieves the current phase of a CAPI Cluster resource.
// Returns the phase string (e.g., "Provisioning", "Provisioned", "Failed") or an error.
// This is useful for checking if a cluster is ready before attempting operations that
// require the cluster to be fully provisioned (like retrieving kubeconfig).
//
// Parameters:
//   - t: testing context
//   - kubeContext: kubectl context to use (e.g., "kind-capz-tests-stage")
//   - namespace: namespace where the Cluster resource is located
//   - clusterName: name of the Cluster resource to check
//
// Returns the phase string or an error if the cluster is not found or the phase cannot be retrieved.
func GetClusterPhase(t *testing.T, kubeContext, namespace, clusterName string) (string, error) {
	t.Helper()

	output, err := RunCommandQuiet(t, "kubectl", "--context", kubeContext, "-n", namespace, "get", "cluster",
		clusterName, "-o", "jsonpath={.status.phase}")
	if err != nil {
		return "", fmt.Errorf("failed to get cluster phase: %w", err)
	}

	phase := strings.TrimSpace(output)
	if phase == "" {
		return "", fmt.Errorf("cluster phase is empty (cluster may not have status yet)")
	}

	return phase, nil
}

// ClusterPhaseProvisioned is the phase value indicating a cluster is fully provisioned and ready.
const ClusterPhaseProvisioned = "Provisioned"

// ClusterPhaseProvisioning is the phase value indicating a cluster is still being provisioned.
const ClusterPhaseProvisioning = "Provisioning"

// ClusterPhaseFailed is the phase value indicating a cluster provisioning has failed.
const ClusterPhaseFailed = "Failed"

// IsClusterReady checks if a cluster is in the Provisioned phase.
// Returns true if the cluster is ready, false otherwise.
func IsClusterReady(t *testing.T, kubeContext, namespace, clusterName string) bool {
	t.Helper()

	phase, err := GetClusterPhase(t, kubeContext, namespace, clusterName)
	if err != nil {
		return false
	}

	return phase == ClusterPhaseProvisioned
}

// DefaultClusterReadyTimeout is the default timeout for waiting for a cluster to become ready.
const DefaultClusterReadyTimeout = 60 * time.Minute

// DefaultClusterReadyPollInterval is the default interval between cluster ready checks.
const DefaultClusterReadyPollInterval = 30 * time.Second

// WaitForClusterReady waits for a cluster to reach the Provisioned phase.
// This is useful when you need to wait for a cluster to be fully provisioned before
// performing operations that require the cluster to be ready (like retrieving kubeconfig).
//
// Parameters:
//   - t: testing context
//   - kubeContext: kubectl context to use (e.g., "kind-capz-tests-stage")
//   - namespace: namespace where the Cluster resource is located
//   - clusterName: name of the Cluster resource to check
//   - timeout: maximum time to wait for the cluster to become ready (use 0 for default of 60m)
//
// Returns nil if the cluster becomes ready, or an error if the timeout is reached or the cluster fails.
func WaitForClusterReady(t *testing.T, kubeContext, namespace, clusterName string, timeout time.Duration) error {
	t.Helper()

	if timeout == 0 {
		timeout = DefaultClusterReadyTimeout
	}

	pollInterval := DefaultClusterReadyPollInterval
	startTime := time.Now()

	PrintToTTY("\n=== Waiting for cluster to be ready ===\n")
	PrintToTTY("Cluster: %s | Namespace: %s | Timeout: %v | Poll interval: %v\n\n", clusterName, namespace, timeout, pollInterval)
	t.Logf("Waiting for cluster '%s' in namespace '%s' to be ready (timeout: %v)...", clusterName, namespace, timeout)

	iteration := 0
	for {
		elapsed := time.Since(startTime)
		remaining := timeout - elapsed

		if elapsed > timeout {
			PrintToTTY("\n❌ Timeout waiting for cluster to be ready after %v\n\n", elapsed.Round(time.Second))
			return fmt.Errorf("timeout waiting for cluster '%s' to be ready after %v", clusterName, elapsed.Round(time.Second))
		}

		iteration++

		PrintToTTY("[%d] Checking cluster phase...\n", iteration)

		phase, err := GetClusterPhase(t, kubeContext, namespace, clusterName)
		if err != nil {
			PrintToTTY("[%d] ⚠️  Failed to get cluster phase: %v\n", iteration, err)
			t.Logf("Failed to get cluster phase (iteration %d): %v", iteration, err)
		} else {
			PrintToTTY("[%d] 📊 Cluster phase: %s\n", iteration, phase)
			t.Logf("Cluster phase (iteration %d): %s", iteration, phase)

			switch phase {
			case ClusterPhaseProvisioned:
				PrintToTTY("\n✅ Cluster is ready! (took %v)\n\n", elapsed.Round(time.Second))
				t.Logf("Cluster '%s' is ready (took %v)", clusterName, elapsed.Round(time.Second))
				return nil
			case ClusterPhaseFailed:
				PrintToTTY("\n❌ Cluster provisioning failed!\n\n")
				return fmt.Errorf("cluster '%s' provisioning failed", clusterName)
			}
		}

		// Report progress
		ReportProgress(t, iteration, elapsed, remaining, timeout)

		time.Sleep(pollInterval)
	}
}

// ComponentVersion represents version information for a deployed component.
type ComponentVersion struct {
	Name    string // Component name (e.g., "CAPZ", "ASO")
	Version string // Version string (e.g., "v1.19.0")
	Image   string // Full container image reference
}

// GetDeploymentImage retrieves the container image for a deployment.
// Returns the image reference or an error if the deployment is not found.
func GetDeploymentImage(t *testing.T, kubeContext, namespace, deploymentName string) (string, error) {
	t.Helper()

	output, err := RunCommandQuiet(t, "kubectl", "--context", kubeContext,
		"-n", namespace, "get", "deployment", deploymentName,
		"-o", "jsonpath={.spec.template.spec.containers[0].image}")
	if err != nil {
		return "", fmt.Errorf("failed to get deployment image: %w", err)
	}

	image := strings.TrimSpace(output)
	if image == "" {
		return "", fmt.Errorf("deployment image is empty")
	}

	return image, nil
}

// extractVersionFromImage extracts the version tag from a container image reference.
// For example: "mcr.microsoft.com/oss/azure/capz:v1.19.0" returns "v1.19.0"
// Returns "unknown" if no version tag can be extracted.
func extractVersionFromImage(image string) string {
	// Split by @ for digest references (e.g., image@sha256:...)
	if idx := strings.Index(image, "@"); idx != -1 {
		// Digest-based reference, try to find version before @
		image = image[:idx]
	}

	// Split by : to get the tag
	parts := strings.Split(image, ":")
	if len(parts) >= 2 {
		tag := parts[len(parts)-1]
		// Validate it looks like a version (starts with v or is a number)
		if strings.HasPrefix(tag, "v") || (len(tag) > 0 && tag[0] >= '0' && tag[0] <= '9') {
			return tag
		}
	}

	return "unknown"
}

// GetComponentVersions retrieves version information for key infrastructure components.
// Returns a slice of ComponentVersion with details for each component.
// Components that cannot be queried are included with "unknown" or "not found" versions.
func GetComponentVersions(t *testing.T, kubeContext string) []ComponentVersion {
	t.Helper()

	// Define components to check - these are the key components for ARO-CAPZ deployment
	// Get namespace configuration
	config := NewTestConfig()

	components := []struct {
		name       string
		namespace  string
		deployment string
	}{
		{"CAPZ (Cluster API Provider Azure)", config.CAPZNamespace, "capz-controller-manager"},
		{"ASO (Azure Service Operator)", config.CAPZNamespace, "azureserviceoperator-controller-manager"},
		{"CAPI (Cluster API)", config.CAPINamespace, "capi-controller-manager"},
	}

	var versions []ComponentVersion

	for _, comp := range components {
		image, err := GetDeploymentImage(t, kubeContext, comp.namespace, comp.deployment)
		if err != nil {
			versions = append(versions, ComponentVersion{
				Name:    comp.name,
				Version: "not found",
				Image:   "N/A",
			})
			continue
		}

		versions = append(versions, ComponentVersion{
			Name:    comp.name,
			Version: extractVersionFromImage(image),
			Image:   image,
		})
	}

	return versions
}

// FormatComponentVersions formats a slice of ComponentVersion for display.
// Returns a formatted string suitable for logging.
// Pass nil for config to omit cluster and Azure settings.
func FormatComponentVersions(versions []ComponentVersion, config *TestConfig) string {
	var result strings.Builder
	result.WriteString("\n=== TESTED CONFIGURATION ===\n")

	if config != nil {
		// Local Kind cluster (management cluster)
		result.WriteString("\nLocal Kind Cluster:\n")
		result.WriteString(fmt.Sprintf("  Management Cluster: %s\n", config.ManagementClusterName))

		// Azure ARO cluster (workload cluster)
		result.WriteString("\nAzure ARO Cluster:\n")
		result.WriteString(fmt.Sprintf("  Workload Cluster:   %s\n", config.WorkloadClusterName))
		result.WriteString(fmt.Sprintf("  Region:             %s\n", config.Region))
		if config.AzureSubscriptionName != "" {
			result.WriteString(fmt.Sprintf("  Subscription:       %s\n", config.AzureSubscriptionName))
		}
		result.WriteString(fmt.Sprintf("  Resource Group:     %s-resgroup\n", config.ClusterNamePrefix))
		result.WriteString(fmt.Sprintf("  OpenShift Version:  %s\n", config.OCPVersion))
	}

	// Used repositories
	// First try in-memory registry (works when tests run in same process)
	repos := GetClonedRepositories()
	if len(repos) > 0 {
		result.WriteString("\n=== USED REPOSITORIES ===\n\n")
		for _, repo := range repos {
			result.WriteString(fmt.Sprintf("- %s\n", repo.URL))
			result.WriteString(fmt.Sprintf("  Branch: %s\n", repo.Branch))
		}
	} else if config != nil && config.RepoURL != "" {
		// Fallback to config values (works across separate test processes)
		result.WriteString("\n=== USED REPOSITORIES ===\n\n")
		result.WriteString(fmt.Sprintf("- %s\n", config.RepoURL))
		result.WriteString(fmt.Sprintf("  Branch: %s\n", config.RepoBranch))
	}

	// Component versions
	result.WriteString("\n=== COMPONENT VERSIONS ===\n\n")

	for _, v := range versions {
		result.WriteString(fmt.Sprintf("%s: %s\n", v.Name, v.Version))
		result.WriteString(fmt.Sprintf("  Image: %s\n", v.Image))
	}

	return result.String()
}

// ValidateYAMLFile validates that a file contains valid YAML.
// Returns an error if the file is empty, unreadable, or contains invalid YAML syntax.
// This is more robust than just checking file size, as it verifies YAML structure.
func ValidateYAMLFile(filePath string) error {
	// Check if file exists
	info, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("file not accessible: %w", err)
	}

	// Check if file is empty
	if info.Size() == 0 {
		return fmt.Errorf("file is empty")
	}

	// Read file contents
	// #nosec G304 - filePath is validated via os.Stat above and comes from test configuration
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Parse YAML to validate syntax
	var content interface{}
	if err := yaml.Unmarshal(data, &content); err != nil {
		return fmt.Errorf("invalid YAML syntax: %w", err)
	}

	// Ensure YAML actually contains data (not just whitespace/comments)
	if content == nil {
		return fmt.Errorf("YAML file contains no data")
	}

	return nil
}

// ExtractNamespaceFromYAML extracts the namespace from the first Kubernetes resource in a YAML file.
// This is used to check if existing generated YAMLs match the current namespace configuration.
func ExtractNamespaceFromYAML(filePath string) (string, error) {
	// #nosec G304 - filePath comes from test configuration
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	// Use regex to find first namespace: value in the YAML
	// This handles both single and multi-document YAML files
	re := regexp.MustCompile(`(?m)^\s*namespace:\s*(\S+)`)
	matches := re.FindSubmatch(content)
	if len(matches) < 2 {
		return "", fmt.Errorf("no namespace found in %s", filePath)
	}
	return string(matches[1]), nil
}

// DeploymentState holds information about the deployed test resources.
// This is written to a state file during deployment and read during cleanup
// to ensure the cleanup targets the correct Azure resources.
type DeploymentState struct {
	ResourceGroup            string `json:"resource_group"`
	ManagementClusterName    string `json:"management_cluster_name"`
	WorkloadClusterName      string `json:"workload_cluster_name"`
	WorkloadClusterNamespace string `json:"workload_cluster_namespace"`
	ClusterNamePrefix        string `json:"cluster_name_prefix"`
	Region                   string `json:"region"`
	User                     string `json:"user"`
	Environment              string `json:"environment"`
}

// DeploymentStateFile is the path to the deployment state file.
// This file is written during test deployment and read during cleanup.
const DeploymentStateFile = ".deployment-state.json"

// WriteDeploymentState writes the current deployment configuration to a state file.
// This allows cleanup commands to know which Azure resources were actually created,
// regardless of current environment variables or config defaults.
func WriteDeploymentState(config *TestConfig) error {
	state := DeploymentState{
		ResourceGroup:            fmt.Sprintf("%s-resgroup", config.ClusterNamePrefix),
		ManagementClusterName:    config.ManagementClusterName,
		WorkloadClusterName:      config.WorkloadClusterName,
		WorkloadClusterNamespace: config.WorkloadClusterNamespace,
		ClusterNamePrefix:        config.ClusterNamePrefix,
		Region:                   config.Region,
		User:                     config.CAPZUser,
		Environment:              config.Environment,
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal deployment state: %w", err)
	}

	if err := os.WriteFile(DeploymentStateFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write deployment state file: %w", err)
	}

	return nil
}

// ReadDeploymentState reads the deployment state from the state file.
// Returns nil if the file doesn't exist (no deployment has been recorded).
func ReadDeploymentState() (*DeploymentState, error) {
	data, err := os.ReadFile(DeploymentStateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No state file, return nil without error
		}
		return nil, fmt.Errorf("failed to read deployment state file: %w", err)
	}

	var state DeploymentState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse deployment state file: %w", err)
	}

	return &state, nil
}

// DeleteDeploymentState removes the deployment state file.
// Called after successful cleanup to indicate no active deployment.
func DeleteDeploymentState() error {
	err := os.Remove(DeploymentStateFile)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete deployment state file: %w", err)
	}
	return nil
}

// ControllerLogSummary holds summarized log information for a controller.
type ControllerLogSummary struct {
	Name       string   // Controller name (e.g., "CAPZ", "ASO", "CAPI")
	Namespace  string   // Namespace where the controller runs
	Deployment string   // Deployment name
	ErrorCount int      // Number of error log lines
	WarnCount  int      // Number of warning log lines
	Errors     []string // Sample error messages (limited)
	Warnings   []string // Sample warning messages (limited)
	LogFile    string   // Path to saved complete log file
}

// MaxSampleMessages is the maximum number of error/warning messages to keep in summary.
const MaxSampleMessages = 10

// GetControllerLogs retrieves logs from a controller deployment.
// Returns the log output or an error if the logs cannot be retrieved.
func GetControllerLogs(t *testing.T, kubeContext, namespace, deploymentName string, tailLines int) (string, error) {
	t.Helper()

	if tailLines <= 0 {
		tailLines = 1000 // Default to last 1000 lines
	}

	output, err := RunCommandQuiet(t, "kubectl", "--context", kubeContext,
		"-n", namespace, "logs",
		fmt.Sprintf("deployment/%s", deploymentName),
		"--all-containers=true",
		fmt.Sprintf("--tail=%d", tailLines))
	if err != nil {
		return "", fmt.Errorf("failed to get logs for %s: %w", deploymentName, err)
	}

	return output, nil
}

// ParseControllerLogs parses log output and counts errors and warnings.
// It looks for common patterns in controller logs to identify issues.
func ParseControllerLogs(logs string) (errors []string, warnings []string) {
	lines := strings.Split(logs, "\n")

	for _, line := range lines {
		lowerLine := strings.ToLower(line)

		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Check for error patterns
		// Common patterns: "level=error", "ERROR", '"level":"error"', "error:"
		if strings.Contains(lowerLine, "level=error") ||
			strings.Contains(lowerLine, `"level":"error"`) ||
			strings.Contains(lowerLine, "\"level\": \"error\"") ||
			(strings.Contains(lowerLine, " error ") && !strings.Contains(lowerLine, "error=nil")) ||
			strings.Contains(lowerLine, "error:") {
			if len(errors) < MaxSampleMessages*2 { // Keep more samples initially, trim later
				errors = append(errors, line)
			}
			continue
		}

		// Check for warning patterns
		// Common patterns: "level=warn", "WARN", '"level":"warn"', "warning:"
		if strings.Contains(lowerLine, "level=warn") ||
			strings.Contains(lowerLine, `"level":"warn"`) ||
			strings.Contains(lowerLine, "\"level\": \"warn\"") ||
			strings.Contains(lowerLine, " warn ") ||
			strings.Contains(lowerLine, "warning:") {
			if len(warnings) < MaxSampleMessages*2 { // Keep more samples initially, trim later
				warnings = append(warnings, line)
			}
		}
	}

	return errors, warnings
}

// SummarizeControllerLogs retrieves and summarizes logs from a controller.
// It returns a ControllerLogSummary with counts and sample messages.
func SummarizeControllerLogs(t *testing.T, kubeContext, namespace, deploymentName, controllerName string) ControllerLogSummary {
	t.Helper()

	summary := ControllerLogSummary{
		Name:       controllerName,
		Namespace:  namespace,
		Deployment: deploymentName,
	}

	logs, err := GetControllerLogs(t, kubeContext, namespace, deploymentName, 5000)
	if err != nil {
		t.Logf("Warning: Could not retrieve logs for %s: %v", controllerName, err)
		return summary
	}

	errors, warnings := ParseControllerLogs(logs)
	summary.ErrorCount = len(errors)
	summary.WarnCount = len(warnings)

	// Keep only MaxSampleMessages samples
	if len(errors) > MaxSampleMessages {
		summary.Errors = errors[:MaxSampleMessages]
	} else {
		summary.Errors = errors
	}

	if len(warnings) > MaxSampleMessages {
		summary.Warnings = warnings[:MaxSampleMessages]
	} else {
		summary.Warnings = warnings
	}

	return summary
}

// SaveControllerLogs saves the complete logs from a controller to a file.
// Returns the path to the saved log file or an error.
func SaveControllerLogs(t *testing.T, kubeContext, namespace, deploymentName, controllerName, outputDir string) (string, error) {
	t.Helper()

	// Get full logs (larger tail for complete history)
	logs, err := GetControllerLogs(t, kubeContext, namespace, deploymentName, 10000)
	if err != nil {
		return "", err
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0750); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate filename with timestamp
	filename := fmt.Sprintf("%s-%s.log", strings.ToLower(controllerName), time.Now().Format("20060102_150405"))
	logFilePath := fmt.Sprintf("%s/%s", outputDir, filename)

	// Write logs to file
	if err := os.WriteFile(logFilePath, []byte(logs), 0600); err != nil {
		return "", fmt.Errorf("failed to write log file: %w", err)
	}

	return logFilePath, nil
}

// GetAllControllerLogSummaries retrieves log summaries for all key controllers.
// Returns a slice of ControllerLogSummary for CAPI, CAPZ, and ASO controllers.
func GetAllControllerLogSummaries(t *testing.T, kubeContext string) []ControllerLogSummary {
	t.Helper()

	// Define controllers to check (same as in GetComponentVersions)
	// Get namespace configuration
	config := NewTestConfig()

	controllers := []struct {
		name       string
		namespace  string
		deployment string
	}{
		{"CAPI", config.CAPINamespace, "capi-controller-manager"},
		{"CAPZ", config.CAPZNamespace, "capz-controller-manager"},
		{"ASO", config.CAPZNamespace, "azureserviceoperator-controller-manager"},
	}

	var summaries []ControllerLogSummary

	for _, ctrl := range controllers {
		summary := SummarizeControllerLogs(t, kubeContext, ctrl.namespace, ctrl.deployment, ctrl.name)
		summaries = append(summaries, summary)
	}

	return summaries
}

// FormatControllerLogSummaries formats controller log summaries for display.
// Returns a human-readable summary string.
func FormatControllerLogSummaries(summaries []ControllerLogSummary) string {
	var result strings.Builder

	result.WriteString("\n=== CONTROLLER LOG SUMMARY ===\n\n")

	totalErrors := 0
	totalWarnings := 0

	for _, s := range summaries {
		totalErrors += s.ErrorCount
		totalWarnings += s.WarnCount

		// Status indicator
		icon := "✅"
		if s.ErrorCount > 0 {
			icon = "❌"
		} else if s.WarnCount > 0 {
			icon = "⚠️"
		}

		result.WriteString(fmt.Sprintf("%s %s Controller:\n", icon, s.Name))
		result.WriteString(fmt.Sprintf("   Errors: %d | Warnings: %d\n", s.ErrorCount, s.WarnCount))

		if s.LogFile != "" {
			result.WriteString(fmt.Sprintf("   Log file: %s\n", s.LogFile))
		}

		// Show sample error messages
		if len(s.Errors) > 0 {
			result.WriteString("   Sample errors:\n")
			for i, err := range s.Errors {
				if i >= 3 { // Show only first 3 in summary
					result.WriteString(fmt.Sprintf("   ... and %d more errors\n", len(s.Errors)-3))
					break
				}
				// Truncate long lines
				errLine := err
				if len(errLine) > 200 {
					errLine = errLine[:200] + "..."
				}
				result.WriteString(fmt.Sprintf("     - %s\n", errLine))
			}
		}

		result.WriteString("\n")
	}

	// Overall summary
	result.WriteString("─────────────────────────────\n")
	result.WriteString(fmt.Sprintf("Total: %d errors, %d warnings across all controllers\n", totalErrors, totalWarnings))

	if totalErrors > 0 {
		result.WriteString("⚠️  Review controller logs for details on errors.\n")
	} else if totalWarnings > 0 {
		result.WriteString("ℹ️  Some warnings found but no errors.\n")
	} else {
		result.WriteString("✅ All controllers running without errors or warnings.\n")
	}

	return result.String()
}

// SaveAllControllerLogs saves complete logs for all controllers to the specified directory.
// Updates the ControllerLogSummary slice with the saved log file paths.
func SaveAllControllerLogs(t *testing.T, kubeContext, outputDir string, summaries []ControllerLogSummary) []ControllerLogSummary {
	t.Helper()

	// Define controllers (same list as in GetAllControllerLogSummaries)
	// Get namespace configuration
	config := NewTestConfig()

	controllers := []struct {
		name       string
		namespace  string
		deployment string
	}{
		{"CAPI", config.CAPINamespace, "capi-controller-manager"},
		{"CAPZ", config.CAPZNamespace, "capz-controller-manager"},
		{"ASO", config.CAPZNamespace, "azureserviceoperator-controller-manager"},
	}

	// Create a map for quick lookup
	controllerMap := make(map[string]struct{ namespace, deployment string })
	for _, c := range controllers {
		controllerMap[c.name] = struct{ namespace, deployment string }{c.namespace, c.deployment}
	}

	// Update summaries with log file paths
	for i := range summaries {
		if ctrl, ok := controllerMap[summaries[i].Name]; ok {
			logFile, err := SaveControllerLogs(t, kubeContext, ctrl.namespace, ctrl.deployment, summaries[i].Name, outputDir)
			if err != nil {
				t.Logf("Warning: Failed to save logs for %s: %v", summaries[i].Name, err)
			} else {
				summaries[i].LogFile = logFile
			}
		}
	}

	return summaries
}

// GetResultsDir returns the appropriate results directory for saving logs.
// It checks TEST_RESULTS_DIR env var first (set by Makefile), then falls back
// to looking for the latest results directory, or creates one if needed.
func GetResultsDir() string {
	// Check for environment variable set by Makefile
	if envDir := os.Getenv("TEST_RESULTS_DIR"); envDir != "" {
		// Ensure directory exists
		if err := os.MkdirAll(envDir, 0750); err == nil {
			return envDir
		}
	}

	// Check for latest results directory first
	latestDir := "results/latest"
	if DirExists(latestDir) {
		return latestDir
	}

	// Look for any timestamped results directory
	entries, err := os.ReadDir("results")
	if err == nil && len(entries) > 0 {
		// Find the most recent directory
		var latestTimestamp string
		for _, e := range entries {
			if e.IsDir() && e.Name() != "latest" {
				if e.Name() > latestTimestamp {
					latestTimestamp = e.Name()
				}
			}
		}
		if latestTimestamp != "" {
			return fmt.Sprintf("results/%s", latestTimestamp)
		}
	}

	// Create a new results directory with current timestamp
	timestamp := time.Now().Format("20060102_150405")
	newDir := fmt.Sprintf("results/%s", timestamp)
	if err := os.MkdirAll(newDir, 0750); err != nil {
		// Fall back to /tmp if we can't create the directory
		return os.TempDir()
	}

	return newDir
}

// AzureAuthMode represents the method of Azure authentication being used.
type AzureAuthMode string

const (
	// AzureAuthModeServicePrincipal indicates authentication via service principal credentials
	// (AZURE_CLIENT_ID + AZURE_CLIENT_SECRET + AZURE_TENANT_ID).
	AzureAuthModeServicePrincipal AzureAuthMode = "service-principal"

	// AzureAuthModeCLI indicates authentication via Azure CLI (az login).
	AzureAuthModeCLI AzureAuthMode = "cli"

	// AzureAuthModeNone indicates no valid authentication is available.
	AzureAuthModeNone AzureAuthMode = "none"
)

// DetectAzureAuthMode determines which authentication method is available.
// It checks for service principal credentials first (preferred for CI/automation),
// then falls back to Azure CLI authentication.
//
// Service principal authentication requires:
// - AZURE_CLIENT_ID
// - AZURE_CLIENT_SECRET
// - AZURE_TENANT_ID
//
// CLI authentication requires:
// - Successful "az account show" command
func DetectAzureAuthMode(t *testing.T) AzureAuthMode {
	t.Helper()

	// Check for service principal credentials first
	clientID := os.Getenv("AZURE_CLIENT_ID")
	clientSecret := os.Getenv("AZURE_CLIENT_SECRET")
	tenantID := os.Getenv("AZURE_TENANT_ID")

	if clientID != "" && clientSecret != "" && tenantID != "" {
		t.Log("Service principal credentials detected (AZURE_CLIENT_ID, AZURE_CLIENT_SECRET, AZURE_TENANT_ID)")
		return AzureAuthModeServicePrincipal
	}

	// Check if Azure CLI is logged in
	_, err := RunCommandQuiet(t, "az", "account", "show")
	if err == nil {
		t.Log("Azure CLI authentication detected")
		return AzureAuthModeCLI
	}

	return AzureAuthModeNone
}

// HasServicePrincipalCredentials returns true if service principal environment variables are set.
// This is a quick check without validating the credentials.
func HasServicePrincipalCredentials() bool {
	return os.Getenv("AZURE_CLIENT_ID") != "" &&
		os.Getenv("AZURE_CLIENT_SECRET") != "" &&
		os.Getenv("AZURE_TENANT_ID") != ""
}

// ValidateServicePrincipalCredentials validates that service principal credentials can authenticate.
// This performs an actual Azure CLI login with the service principal to verify credentials work.
// Returns an error if authentication fails.
func ValidateServicePrincipalCredentials(t *testing.T) error {
	t.Helper()

	clientID := os.Getenv("AZURE_CLIENT_ID")
	clientSecret := os.Getenv("AZURE_CLIENT_SECRET")
	tenantID := os.Getenv("AZURE_TENANT_ID")

	if clientID == "" || clientSecret == "" || tenantID == "" {
		return fmt.Errorf("missing service principal credentials: AZURE_CLIENT_ID=%t, AZURE_CLIENT_SECRET=%t, AZURE_TENANT_ID=%t",
			clientID != "", clientSecret != "", tenantID != "")
	}

	// Test login with service principal (using --allow-no-subscriptions in case SP has no subscription access)
	t.Log("Validating service principal credentials...")
	_, err := RunCommandQuiet(t, "az", "login",
		"--service-principal",
		"-u", clientID,
		"-p", clientSecret,
		"--tenant", tenantID,
		"--allow-no-subscriptions")
	if err != nil {
		return fmt.Errorf("service principal authentication failed: %w\n\n"+
			"Please verify your service principal credentials:\n"+
			"  - AZURE_CLIENT_ID is correct\n"+
			"  - AZURE_CLIENT_SECRET is valid and not expired\n"+
			"  - AZURE_TENANT_ID is correct\n\n"+
			"To regenerate the secret:\n"+
			"  az ad sp credential reset --id <client-id>", err)
	}

	t.Log("Service principal credentials validated successfully")
	return nil
}

// GetAzureAuthDescription returns a human-readable description of the current auth mode.
func GetAzureAuthDescription(mode AzureAuthMode) string {
	switch mode {
	case AzureAuthModeServicePrincipal:
		return "service principal (AZURE_CLIENT_ID/AZURE_CLIENT_SECRET)"
	case AzureAuthModeCLI:
		return "Azure CLI (az login)"
	default:
		return "no authentication"
	}
}

// DeletionResourceStatus holds the status of resources being deleted.
type DeletionResourceStatus struct {
	ClusterExists         bool
	ClusterPhase          string
	ClusterFinalizers     []string
	AROControlPlaneCount  int
	MachinePoolCount      int
	AzureResourceGroup    string
	AzureRGExists         bool
	AzureRGProvisionState string
}

// GetDeletionResourceStatus retrieves the current status of all resources being deleted.
// This provides a comprehensive view of the deletion progress.
func GetDeletionResourceStatus(t *testing.T, kubeContext, namespace, clusterName, resourceGroup string) DeletionResourceStatus {
	t.Helper()

	status := DeletionResourceStatus{
		AzureResourceGroup: resourceGroup,
	}

	// Check if cluster still exists and get its phase
	output, err := RunCommandQuiet(t, "kubectl", "--context", kubeContext, "-n", namespace,
		"get", "cluster", clusterName, "-o", "jsonpath={.status.phase}", "--ignore-not-found")
	if err == nil && strings.TrimSpace(output) != "" {
		status.ClusterExists = true
		status.ClusterPhase = strings.TrimSpace(output)
	} else {
		// Double check - try to get the cluster without jsonpath
		checkOutput, checkErr := RunCommandQuiet(t, "kubectl", "--context", kubeContext, "-n", namespace,
			"get", "cluster", clusterName, "--ignore-not-found")
		status.ClusterExists = checkErr == nil && strings.TrimSpace(checkOutput) != ""
	}

	// Get cluster finalizers if cluster exists
	if status.ClusterExists {
		finalizersOutput, err := RunCommandQuiet(t, "kubectl", "--context", kubeContext, "-n", namespace,
			"get", "cluster", clusterName, "-o", "jsonpath={.metadata.finalizers}")
		if err == nil && strings.TrimSpace(finalizersOutput) != "" {
			// Parse JSON array of finalizers
			finalizers := strings.Trim(finalizersOutput, "[]\"")
			if finalizers != "" {
				status.ClusterFinalizers = strings.Split(finalizers, "\",\"")
			}
		}
	}

	// Count AROControlPlane resources
	output, err = RunCommandQuiet(t, "kubectl", "--context", kubeContext, "-n", namespace,
		"get", "arocontrolplane", "--ignore-not-found", "-o", "jsonpath={.items[*].metadata.name}")
	if err == nil && strings.TrimSpace(output) != "" {
		status.AROControlPlaneCount = len(strings.Fields(output))
	}

	// Count MachinePool resources
	output, err = RunCommandQuiet(t, "kubectl", "--context", kubeContext, "-n", namespace,
		"get", "machinepool", "--ignore-not-found", "-o", "jsonpath={.items[*].metadata.name}")
	if err == nil && strings.TrimSpace(output) != "" {
		status.MachinePoolCount = len(strings.Fields(output))
	}

	// Check Azure resource group status if az CLI is available
	if CommandExists("az") && resourceGroup != "" {
		output, err = RunCommandQuiet(t, "az", "group", "show", "--name", resourceGroup,
			"--query", "properties.provisioningState", "-o", "tsv")
		if err == nil && strings.TrimSpace(output) != "" {
			status.AzureRGExists = true
			status.AzureRGProvisionState = strings.TrimSpace(output)
		}
	}

	return status
}

// FormatDeletionProgress formats the deletion status as a human-readable progress report.
func FormatDeletionProgress(status DeletionResourceStatus) string {
	var sb strings.Builder

	// Box width: 61 characters inside the borders
	// Emoji takes 2 visual cells but 4 bytes, so we add 2 to valueWidth for padding
	const labelWidth = 18 // "AROControlPlane:" padded

	// Helper to format a row with emoji, label, and value
	formatRow := func(emoji, label, value string) string {
		// Layout: "│ " + emoji(2) + " " + label(18) + value(38) + " │"
		// Total: 2 + 2 + 1 + 18 + 38 + 2 = 63, but emoji is 4 bytes so Go sees 65
		const valueWidth = 38
		if len(value) > valueWidth {
			value = value[:valueWidth-3] + "..."
		}
		return fmt.Sprintf("│ %s %-*s%-*s │\n", emoji, labelWidth, label+":", valueWidth, value)
	}

	sb.WriteString("┌─────────────────────────────────────────────────────────────┐\n")
	sb.WriteString("│                     DELETION PROGRESS                       │\n")
	sb.WriteString("├─────────────────────────────────────────────────────────────┤\n")

	// Cluster status
	if status.ClusterExists {
		phase := status.ClusterPhase
		if phase == "" {
			phase = "unknown"
		}
		sb.WriteString(formatRow("🔄", "Cluster", fmt.Sprintf("Deleting (phase: %s)", phase)))
	} else {
		sb.WriteString(formatRow("✅", "Cluster", "Deleted"))
	}

	// Finalizers
	if len(status.ClusterFinalizers) > 0 {
		sb.WriteString(formatRow("🔒", "Finalizers", fmt.Sprintf("%d active", len(status.ClusterFinalizers))))
		for _, f := range status.ClusterFinalizers {
			// Truncate long finalizer names to fit in the box
			if len(f) > 53 {
				f = f[:50] + "..."
			}
			sb.WriteString(fmt.Sprintf("│      - %-53s│\n", f))
		}
	}

	// AROControlPlane
	if status.AROControlPlaneCount > 0 {
		sb.WriteString(formatRow("🔄", "AROControlPlane", fmt.Sprintf("%d remaining", status.AROControlPlaneCount)))
	} else {
		sb.WriteString(formatRow("✅", "AROControlPlane", "Deleted"))
	}

	// MachinePool
	if status.MachinePoolCount > 0 {
		sb.WriteString(formatRow("🔄", "MachinePool", fmt.Sprintf("%d remaining", status.MachinePoolCount)))
	} else {
		sb.WriteString(formatRow("✅", "MachinePool", "Deleted"))
	}

	// Azure resource group
	if status.AzureResourceGroup != "" {
		if status.AzureRGExists {
			stateInfo := status.AzureRGProvisionState
			if stateInfo == "" {
				stateInfo = "exists"
			}
			sb.WriteString(formatRow("🔄", "Azure RG", fmt.Sprintf("%s (%s)", status.AzureResourceGroup, stateInfo)))
		} else {
			sb.WriteString(formatRow("✅", "Azure RG", "Deleted"))
		}
	}

	sb.WriteString("└─────────────────────────────────────────────────────────────┘\n")

	return sb.String()
}

// ReportDeletionProgress prints the current deletion status to TTY and test log.
func ReportDeletionProgress(t *testing.T, iteration int, elapsed, remaining time.Duration, status DeletionResourceStatus) {
	t.Helper()

	percentage := 0
	total := elapsed + remaining
	if total > 0 {
		percentage = int((float64(elapsed) / float64(total)) * 100)
	}

	PrintToTTY("\n[%d] ⏳ Elapsed: %v | Remaining: %v | Progress: %d%%\n",
		iteration, elapsed.Round(time.Second), remaining.Round(time.Second), percentage)
	PrintToTTY("%s", FormatDeletionProgress(status))

	// Log summary for test output
	t.Logf("Deletion progress: cluster=%v, arocp=%d, mp=%d, azureRG=%v",
		status.ClusterExists, status.AROControlPlaneCount, status.MachinePoolCount, status.AzureRGExists)
}

// ============================================================================
// Configuration Validation Functions
// ============================================================================
//
// These functions provide fail-fast validation for configuration values,
// ensuring issues are caught early (in phase 1) rather than during deployment.

// ValidateAzureSubscriptionAccess validates that the Azure subscription is accessible.
// Returns nil if the subscription can be accessed, or an error with remediation guidance.
// This validation ensures the subscription exists and the current credentials have access.
func ValidateAzureSubscriptionAccess(t *testing.T, subscriptionID string) error {
	t.Helper()

	if subscriptionID == "" {
		return fmt.Errorf(
			"AZURE_SUBSCRIPTION_ID is empty\n" +
				"  The subscription ID is required to access Azure resources.\n\n" +
				"  To fix this:\n" +
				"    Option 1: export AZURE_SUBSCRIPTION_ID=<your-subscription-id>\n" +
				"    Option 2: export AZURE_SUBSCRIPTION_ID=$(az account show --query id -o tsv)\n" +
				"    Option 3: Run 'az login' and let the test auto-extract credentials")
	}

	// Check if az CLI is available
	if !CommandExists("az") {
		// Can't validate subscription without az CLI, skip validation
		t.Log("Azure CLI not available, skipping subscription access validation")
		return nil
	}

	// Try to access the subscription
	output, err := RunCommandQuiet(t, "az", "account", "show", "--subscription", subscriptionID, "--query", "state", "-o", "tsv")
	if err != nil {
		// Detect specific Azure error patterns
		if azureErr := DetectAzureError(output + err.Error()); azureErr != nil {
			return fmt.Errorf(
				"azure subscription '%s' is not accessible\n"+
					"  %s\n\n"+
					"  Remediation:\n%s",
				subscriptionID, azureErr.Message, formatRemediationSteps(azureErr.Remediation))
		}

		return fmt.Errorf(
			"azure subscription '%s' is not accessible\n"+
				"  Error: %v\n\n"+
				"  To fix this:\n"+
				"    1. Verify the subscription ID is correct:\n"+
				"       az account list -o table\n"+
				"    2. Ensure you have access to the subscription:\n"+
				"       az account set --subscription %s\n"+
				"    3. Re-authenticate if needed:\n"+
				"       az login",
			subscriptionID, err, subscriptionID)
	}

	state := strings.TrimSpace(output)
	if state != "Enabled" {
		return fmt.Errorf(
			"azure subscription '%s' is in state '%s' (expected: Enabled)\n"+
				"  The subscription must be in 'Enabled' state to create resources.\n\n"+
				"  To fix this:\n"+
				"    1. Check subscription status in Azure Portal\n"+
				"    2. Ensure billing is up to date\n"+
				"    3. Contact your Azure administrator if the subscription is disabled",
			subscriptionID, state)
	}

	return nil
}

// formatRemediationSteps formats a slice of remediation steps as indented lines.
func formatRemediationSteps(steps []string) string {
	var result strings.Builder
	for _, step := range steps {
		result.WriteString(fmt.Sprintf("    %s\n", step))
	}
	return result.String()
}

// azureRegions contains the list of valid Azure regions.
// This is a subset of commonly used regions; the full list is validated via Azure CLI.
var azureRegions = map[string]bool{
	// Americas
	"eastus": true, "eastus2": true, "westus": true, "westus2": true, "westus3": true,
	"centralus": true, "northcentralus": true, "southcentralus": true, "westcentralus": true,
	"canadacentral": true, "canadaeast": true,
	"brazilsouth": true, "brazilsoutheast": true,
	// Europe
	"northeurope": true, "westeurope": true,
	"uksouth": true, "ukwest": true,
	"francecentral": true, "francesouth": true,
	"germanywestcentral": true, "germanynorth": true,
	"switzerlandnorth": true, "switzerlandwest": true,
	"norwayeast": true, "norwaywest": true,
	"swedencentral": true, "swedensouth": true,
	"polandcentral": true,
	// Asia Pacific
	"eastasia": true, "southeastasia": true,
	"australiaeast": true, "australiasoutheast": true, "australiacentral": true,
	"japaneast": true, "japanwest": true,
	"koreacentral": true, "koreasouth": true,
	"centralindia": true, "southindia": true, "westindia": true,
	// Middle East & Africa
	"uaenorth": true, "uaecentral": true,
	"southafricanorth": true, "southafricawest": true,
	"qatarcentral":  true,
	"israelcentral": true,
}

// ValidateAzureRegion validates that the specified Azure region is valid.
// Returns nil if the region is valid, or an error with remediation guidance.
func ValidateAzureRegion(t *testing.T, region string) error {
	t.Helper()

	if region == "" {
		return fmt.Errorf(
			"REGION is empty\n" +
				"  An Azure region is required for resource deployment.\n\n" +
				"  To fix this:\n" +
				"    export REGION=<azure-region>\n\n" +
				"  Common regions: eastus, westus2, uksouth, westeurope, eastasia")
	}

	// Normalize to lowercase for comparison
	normalizedRegion := strings.ToLower(region)

	// Quick check against known regions
	if azureRegions[normalizedRegion] {
		return nil
	}

	// If not in known list, validate via Azure CLI if available
	if CommandExists("az") {
		output, err := RunCommandQuiet(t, "az", "account", "list-locations", "--query", "[?name=='"+normalizedRegion+"'].name", "-o", "tsv")
		if err == nil && strings.TrimSpace(output) == normalizedRegion {
			return nil
		}

		// Get list of available regions for error message
		regionsOutput, _ := RunCommandQuiet(t, "az", "account", "list-locations", "--query", "[].name", "-o", "tsv")
		availableRegions := strings.Split(strings.TrimSpace(regionsOutput), "\n")

		// Find similar regions for suggestions
		suggestions := findSimilarRegions(normalizedRegion, availableRegions)
		suggestionText := ""
		if len(suggestions) > 0 {
			suggestionText = fmt.Sprintf("\n  Did you mean: %s?", strings.Join(suggestions, ", "))
		}

		return fmt.Errorf(
			"REGION '%s' is not a valid Azure region\n"+
				"  The specified region was not found in available Azure locations.%s\n\n"+
				"  To fix this:\n"+
				"    1. List available regions: az account list-locations --query '[].name' -o tsv\n"+
				"    2. Set a valid region: export REGION=<valid-region>\n\n"+
				"  Common regions: eastus, westus2, uksouth, westeurope, eastasia",
			region, suggestionText)
	}

	// Can't validate via CLI, check against known list
	return fmt.Errorf(
		"REGION '%s' is not a recognized Azure region\n"+
			"  The region was not found in the list of known Azure regions.\n\n"+
			"  To fix this:\n"+
			"    1. Verify the region name (case-sensitive, no spaces)\n"+
			"    2. List available regions: az account list-locations --query '[].name' -o tsv\n"+
			"    3. Set a valid region: export REGION=<valid-region>\n\n"+
			"  Common regions: eastus, westus2, uksouth, westeurope, eastasia",
		region)
}

// findSimilarRegions finds regions that are similar to the given input.
// Used to provide "did you mean?" suggestions in error messages.
func findSimilarRegions(input string, regions []string) []string {
	var similar []string
	for _, region := range regions {
		// Simple substring matching
		if strings.Contains(region, input) || strings.Contains(input, region) {
			similar = append(similar, region)
		}
	}
	// Limit to 3 suggestions
	if len(similar) > 3 {
		similar = similar[:3]
	}
	return similar
}

// Timeout validation constants
const (
	// MinDeploymentTimeout is the minimum allowed deployment timeout.
	// Deployments typically take at least 15 minutes, so shorter timeouts are likely errors.
	MinDeploymentTimeout = 15 * time.Minute

	// MaxDeploymentTimeout is the maximum allowed deployment timeout.
	// Timeouts over 3 hours are likely configuration errors.
	MaxDeploymentTimeout = 3 * time.Hour

	// MinASOControllerTimeout is the minimum allowed ASO controller timeout.
	MinASOControllerTimeout = 2 * time.Minute

	// MaxASOControllerTimeout is the maximum allowed ASO controller timeout.
	MaxASOControllerTimeout = 30 * time.Minute
)

// ValidateTimeout validates that a timeout duration is within reasonable bounds.
// Returns nil if the timeout is valid, or an error with remediation guidance.
//
// Parameters:
//   - name: The environment variable name (for error messages)
//   - timeout: The timeout duration to validate
//   - min: Minimum allowed timeout
//   - max: Maximum allowed timeout
func ValidateTimeout(name string, timeout, min, max time.Duration) error {
	if timeout < min {
		return fmt.Errorf(
			"%s '%v' is too short (minimum: %v)\n"+
				"  Timeout values that are too short may cause premature failures\n\n"+
				"  To fix this:\n"+
				"    export %s=%v\n\n"+
				"  The timeout must be at least %v to allow sufficient time for operations",
			name, timeout, min, name, min, min)
	}

	if timeout > max {
		return fmt.Errorf(
			"%s '%v' is too long (maximum: %v)\n"+
				"  Extremely long timeouts may indicate a configuration error\n\n"+
				"  To fix this:\n"+
				"    export %s=%v\n\n"+
				"  If you need a longer timeout, consider investigating why operations are taking so long",
			name, timeout, max, name, max)
	}

	return nil
}

// ValidateDeploymentTimeout validates the DEPLOYMENT_TIMEOUT configuration.
// Returns nil if the timeout is within acceptable bounds, or an error with remediation guidance.
func ValidateDeploymentTimeout(timeout time.Duration) error {
	return ValidateTimeout("DEPLOYMENT_TIMEOUT", timeout, MinDeploymentTimeout, MaxDeploymentTimeout)
}

// ValidateASOControllerTimeout validates the ASO_CONTROLLER_TIMEOUT configuration.
// Returns nil if the timeout is within acceptable bounds, or an error with remediation guidance.
func ValidateASOControllerTimeout(timeout time.Duration) error {
	return ValidateTimeout("ASO_CONTROLLER_TIMEOUT", timeout, MinASOControllerTimeout, MaxASOControllerTimeout)
}

// ConfigValidationResult holds the results of a configuration validation.
type ConfigValidationResult struct {
	Variable   string // Environment variable name
	Value      string // Current value (may be masked for secrets)
	IsValid    bool   // Whether the validation passed
	Error      error  // Validation error (nil if valid)
	IsCritical bool   // Whether this is a critical validation (blocks deployment)
	SkipReason string // Reason if validation was skipped
}

// ValidateAllConfigurations performs comprehensive configuration validation.
// This is designed to be called once during phase 1 (Check Dependencies) to catch
// all configuration issues early, before any Azure resources are created.
//
// Returns a slice of ConfigValidationResult with details about each validation.
func ValidateAllConfigurations(t *testing.T, config *TestConfig) []ConfigValidationResult {
	t.Helper()

	var results []ConfigValidationResult

	// Validate RFC 1123 naming compliance
	for _, item := range []struct {
		name  string
		value string
	}{
		{"CAPZ_USER", config.CAPZUser},
		{"DEPLOYMENT_ENV", config.Environment},
		{"CS_CLUSTER_NAME", config.ClusterNamePrefix},
		{"WORKLOAD_CLUSTER_NAMESPACE", config.WorkloadClusterNamespace},
	} {
		result := ConfigValidationResult{
			Variable:   item.name,
			Value:      item.value,
			IsCritical: true,
		}
		if err := ValidateRFC1123Name(item.value, item.name); err != nil {
			result.IsValid = false
			result.Error = err
		} else {
			result.IsValid = true
		}
		results = append(results, result)
	}

	// Validate domain prefix length
	domainPrefix := GetDomainPrefix(config.CAPZUser, config.Environment)
	result := ConfigValidationResult{
		Variable:   "Domain Prefix (CAPZ_USER-DEPLOYMENT_ENV)",
		Value:      domainPrefix,
		IsCritical: true,
	}
	if err := ValidateDomainPrefix(config.CAPZUser, config.Environment); err != nil {
		result.IsValid = false
		result.Error = err
	} else {
		result.IsValid = true
	}
	results = append(results, result)

	// Validate ExternalAuth ID length
	externalAuthID := GetExternalAuthID(config.ClusterNamePrefix)
	result = ConfigValidationResult{
		Variable:   "ExternalAuth ID (CS_CLUSTER_NAME-ea)",
		Value:      externalAuthID,
		IsCritical: true,
	}
	if err := ValidateExternalAuthID(config.ClusterNamePrefix); err != nil {
		result.IsValid = false
		result.Error = err
	} else {
		result.IsValid = true
	}
	results = append(results, result)

	// Validate Azure region
	result = ConfigValidationResult{
		Variable:   "REGION",
		Value:      config.Region,
		IsCritical: true,
	}
	if err := ValidateAzureRegion(t, config.Region); err != nil {
		result.IsValid = false
		result.Error = err
	} else {
		result.IsValid = true
	}
	results = append(results, result)

	// Validate timeout values
	result = ConfigValidationResult{
		Variable:   "DEPLOYMENT_TIMEOUT",
		Value:      config.DeploymentTimeout.String(),
		IsCritical: false, // Not critical, deployment will just timeout
	}
	if err := ValidateDeploymentTimeout(config.DeploymentTimeout); err != nil {
		result.IsValid = false
		result.Error = err
	} else {
		result.IsValid = true
	}
	results = append(results, result)

	result = ConfigValidationResult{
		Variable:   "ASO_CONTROLLER_TIMEOUT",
		Value:      config.ASOControllerTimeout.String(),
		IsCritical: false,
	}
	if err := ValidateASOControllerTimeout(config.ASOControllerTimeout); err != nil {
		result.IsValid = false
		result.Error = err
	} else {
		result.IsValid = true
	}
	results = append(results, result)

	return results
}

// FormatValidationResults formats validation results for display.
// Returns a formatted string suitable for logging or printing to TTY.
func FormatValidationResults(results []ConfigValidationResult) string {
	var sb strings.Builder

	sb.WriteString("\n=== CONFIGURATION VALIDATION RESULTS ===\n\n")

	var criticalErrors, warnings int

	for _, r := range results {
		icon := "✅"
		if !r.IsValid {
			if r.IsCritical {
				icon = "❌"
				criticalErrors++
			} else {
				icon = "⚠️"
				warnings++
			}
		} else if r.SkipReason != "" {
			icon = "⏭️"
		}

		sb.WriteString(fmt.Sprintf("%s %s: %s\n", icon, r.Variable, r.Value))

		if r.SkipReason != "" {
			sb.WriteString(fmt.Sprintf("   Skipped: %s\n", r.SkipReason))
		}

		if r.Error != nil {
			// Indent error message
			errLines := strings.Split(r.Error.Error(), "\n")
			for _, line := range errLines {
				sb.WriteString(fmt.Sprintf("   %s\n", line))
			}
		}
	}

	sb.WriteString("\n─────────────────────────────────────────\n")
	if criticalErrors > 0 {
		sb.WriteString(fmt.Sprintf("❌ %d critical error(s) found - deployment will fail!\n", criticalErrors))
	}
	if warnings > 0 {
		sb.WriteString(fmt.Sprintf("⚠️  %d warning(s) found - review recommended\n", warnings))
	}
	if criticalErrors == 0 && warnings == 0 {
		sb.WriteString("✅ All configuration validations passed\n")
	}

	return sb.String()
}

// =============================================================================
// Cluster Resource Detection Functions
// =============================================================================

// GetExistingClusterNames returns names of all Cluster CRs in the specified namespace.
// Returns an empty slice if no clusters are found or if the Cluster CRD is not installed.
func GetExistingClusterNames(t *testing.T, kubeContext, namespace string) ([]string, error) {
	t.Helper()

	// Get all Cluster resources in the namespace
	output, err := RunCommandQuiet(t, "kubectl", "--context", kubeContext,
		"-n", namespace, "get", "cluster", "-o", "jsonpath={.items[*].metadata.name}")

	if err != nil {
		// Check if the error is because CRD doesn't exist (expected on fresh clusters)
		if strings.Contains(output, "the server doesn't have a resource type") ||
			strings.Contains(output, "No resources found") ||
			strings.Contains(err.Error(), "NotFound") {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to list Cluster resources: %w", err)
	}

	// Parse the space-separated list of cluster names
	output = strings.TrimSpace(output)
	if output == "" {
		return []string{}, nil
	}

	names := strings.Fields(output)
	return names, nil
}

// CheckForMismatchedClusters checks if any existing Cluster CRs don't match the expected prefix.
// Returns a list of cluster names that don't start with the expected prefix.
// This is used to detect stale Cluster resources from previous configurations (e.g., different CAPZ_USER).
func CheckForMismatchedClusters(t *testing.T, kubeContext, namespace, expectedPrefix string) ([]string, error) {
	t.Helper()

	existingClusters, err := GetExistingClusterNames(t, kubeContext, namespace)
	if err != nil {
		return nil, err
	}

	var mismatched []string
	for _, name := range existingClusters {
		// Check if the cluster name starts with the expected prefix
		if !strings.HasPrefix(name, expectedPrefix) {
			mismatched = append(mismatched, name)
		}
	}

	return mismatched, nil
}

// FormatMismatchedClustersError formats a user-friendly error message for mismatched clusters.
// This provides clear guidance on how to clean up stale Cluster resources.
func FormatMismatchedClustersError(mismatched []string, expectedPrefix, namespace string) string {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	sb.WriteString("❌ EXISTING CLUSTER RESOURCES DETECTED\n")
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	sb.WriteString("Found existing Cluster CRs that don't match current configuration:\n\n")
	for _, name := range mismatched {
		sb.WriteString(fmt.Sprintf("  • %s\n", name))
	}

	sb.WriteString(fmt.Sprintf("\nCurrent config expects cluster names starting with: %s\n\n", expectedPrefix))

	sb.WriteString("This typically happens when CAPZ_USER was changed without cleaning up\n")
	sb.WriteString("the previous cluster resources. Deploying new clusters alongside old ones\n")
	sb.WriteString("can cause conflicts and unexpected behavior.\n\n")

	sb.WriteString("TO CLEAN UP:\n\n")

	// Single cluster cleanup
	if len(mismatched) == 1 {
		sb.WriteString(fmt.Sprintf("  kubectl delete cluster %s -n %s\n\n", mismatched[0], namespace))
	} else {
		// Multiple clusters
		sb.WriteString("  # Delete specific cluster:\n")
		sb.WriteString(fmt.Sprintf("  kubectl delete cluster %s -n %s\n\n", mismatched[0], namespace))
		sb.WriteString("  # Or delete all clusters in namespace:\n")
		sb.WriteString(fmt.Sprintf("  kubectl delete cluster --all -n %s\n\n", namespace))
	}

	sb.WriteString("  # Or use make clean for complete cleanup:\n")
	sb.WriteString("  make clean\n\n")

	sb.WriteString("After cleanup, re-run the tests.\n")
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	return sb.String()
}

// =============================================================================
// MCE (MultiClusterEngine) Helper Functions
// =============================================================================

// IsMCECluster checks if the external cluster has MCE (MultiClusterEngine) installed.
// Returns true if the 'multiclusterengine' resource exists, false otherwise.
func IsMCECluster(t *testing.T, kubeContext string) bool {
	t.Helper()
	_, err := RunCommandQuiet(t, "kubectl", "--context", kubeContext,
		"get", "mce", "multiclusterengine", "-o", "name")
	return err == nil
}

// MCEComponentStatus represents the status of an MCE component
type MCEComponentStatus struct {
	Name    string
	Enabled bool
	Exists  bool
}

// GetMCEComponentStatus retrieves the enabled status of a specific MCE component.
// Returns the component status or an error if the MCE resource cannot be queried.
func GetMCEComponentStatus(t *testing.T, kubeContext, componentName string) (*MCEComponentStatus, error) {
	t.Helper()

	// Query component enabled status using jsonpath
	output, err := RunCommandQuiet(t, "kubectl", "--context", kubeContext,
		"get", "mce", "multiclusterengine", "-o",
		fmt.Sprintf("jsonpath={.spec.overrides.components[?(@.name=='%s')].enabled}", componentName))

	if err != nil {
		return nil, fmt.Errorf("failed to query MCE component status: %w", err)
	}

	status := &MCEComponentStatus{
		Name:   componentName,
		Exists: output != "",
	}

	if output == "true" {
		status.Enabled = true
	}

	return status, nil
}

// SetMCEComponentState sets the enabled state of a specific MCE component.
// This uses jq to transform the components array while preserving other settings.
func SetMCEComponentState(t *testing.T, kubeContext, componentName string, enabled bool) error {
	t.Helper()

	action := "Disabling"
	if enabled {
		action = "Enabling"
	}

	PrintToTTY("%s MCE component: %s\n", action, componentName)
	t.Logf("%s MCE component: %s", action, componentName)

	// Get current MCE resource as JSON
	currentOutput, err := RunCommandQuiet(t, "kubectl", "--context", kubeContext,
		"get", "mce", "multiclusterengine", "-o", "json")
	if err != nil {
		return fmt.Errorf("failed to get MCE resource: %w", err)
	}

	// Build the jq expression to update the specific component
	jqExpr := fmt.Sprintf(
		`.spec.overrides.components | map(if .name == "%s" then .enabled = %t else . end)`,
		componentName, enabled)

	// Use jq to transform the components array
	// #nosec G204 - jq binary with expression built from validated MCE component name, not user input
	jqCmd := exec.Command("jq", "-c", jqExpr)
	jqCmd.Stdin = strings.NewReader(currentOutput)
	transformedBytes, err := jqCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to transform MCE components with jq: %w", err)
	}
	transformed := strings.TrimSpace(string(transformedBytes))

	// Build the patch JSON
	patchJSON := fmt.Sprintf(`{"spec":{"overrides":{"components":%s}}}`, transformed)

	// Apply the patch
	output, err := RunCommand(t, "kubectl", "--context", kubeContext,
		"patch", "mce", "multiclusterengine", "--type=merge", "-p", patchJSON)
	if err != nil {
		return fmt.Errorf("failed to patch MCE resource: %w\nOutput: %s", err, output)
	}

	stateStr := "disabled"
	if enabled {
		stateStr = "enabled"
	}
	PrintToTTY("✅ MCE component %s %s successfully\n", componentName, stateStr)
	t.Logf("MCE component %s %s successfully", componentName, stateStr)
	return nil
}

// EnableMCEComponent enables a specific MCE component by patching the multiclusterengine resource.
// This uses jq to transform the components array while preserving other settings.
func EnableMCEComponent(t *testing.T, kubeContext, componentName string) error {
	t.Helper()

	PrintToTTY("Enabling MCE component: %s\n", componentName)
	t.Logf("Enabling MCE component: %s", componentName)

	// Get current MCE resource as JSON
	currentOutput, err := RunCommandQuiet(t, "kubectl", "--context", kubeContext,
		"get", "mce", "multiclusterengine", "-o", "json")
	if err != nil {
		return fmt.Errorf("failed to get MCE resource: %w", err)
	}

	// Build the jq expression to update the specific component
	jqExpr := fmt.Sprintf(
		`.spec.overrides.components | map(if .name == "%s" then .enabled = true else . end)`,
		componentName)

	// Use jq to transform the components array
	// #nosec G204 - jq binary with expression built from validated MCE component name, not user input
	jqCmd := exec.Command("jq", "-c", jqExpr)
	jqCmd.Stdin = strings.NewReader(currentOutput)
	transformedBytes, err := jqCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to transform MCE components with jq: %w", err)
	}
	transformed := strings.TrimSpace(string(transformedBytes))

	// Build the patch JSON
	patchJSON := fmt.Sprintf(`{"spec":{"overrides":{"components":%s}}}`, transformed)

	// Apply the patch
	output, err := RunCommand(t, "kubectl", "--context", kubeContext,
		"patch", "mce", "multiclusterengine", "--type=merge", "-p", patchJSON)
	if err != nil {
		return fmt.Errorf("failed to patch MCE resource: %w\nOutput: %s", err, output)
	}

	PrintToTTY("✅ MCE component %s enabled successfully\n", componentName)
	t.Logf("MCE component %s enabled successfully", componentName)
	return nil
}

// WaitForMCEController waits for a controller deployment to become available after MCE enablement.
// Returns nil when the controller is available, or an error if timeout is reached.
func WaitForMCEController(t *testing.T, kubeContext, namespace, deploymentName string, timeout time.Duration) error {
	t.Helper()

	if timeout == 0 {
		timeout = DefaultMCEEnablementTimeout
	}

	pollInterval := 15 * time.Second
	startTime := time.Now()

	PrintToTTY("\n=== Waiting for MCE controller: %s ===\n", deploymentName)
	PrintToTTY("Namespace: %s | Timeout: %v\n\n", namespace, timeout)

	iteration := 0
	for {
		elapsed := time.Since(startTime)

		if elapsed > timeout {
			return fmt.Errorf("timeout waiting for MCE controller %s after %v", deploymentName, elapsed.Round(time.Second))
		}

		iteration++

		// Check if deployment exists and is available
		output, err := RunCommandQuiet(t, "kubectl", "--context", kubeContext,
			"-n", namespace, "get", "deployment", deploymentName,
			"-o", "jsonpath={.status.conditions[?(@.type=='Available')].status}")

		if err != nil {
			PrintToTTY("[%d] Deployment %s not found yet, waiting...\n", iteration, deploymentName)
		} else if strings.TrimSpace(output) == "True" {
			PrintToTTY("✅ MCE controller %s is available! (took %v)\n", deploymentName, elapsed.Round(time.Second))
			return nil
		} else {
			PrintToTTY("[%d] Deployment %s status: %s\n", iteration, deploymentName, strings.TrimSpace(output))
		}

		time.Sleep(pollInterval)
	}
}
