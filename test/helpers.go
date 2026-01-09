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

	fmt.Fprintf(tty, "Running (streaming): %s\n", cmdStr)
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
	fmt.Fprintf(tty, "\n=== RUN: %s ===\n", testName)
	fmt.Fprintf(tty, "    %s\n\n", description)

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
	fmt.Fprintf(tty, format, args...)
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

// AROControlPlaneCondition represents a condition from the AROControlPlane status
type AROControlPlaneCondition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
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
		// Determine the icon based on status
		icon := "⏳" // pending/unknown
		if cond.Status == "True" {
			icon = "✅"
		} else if cond.Status == "False" {
			icon = "🔄"
		}

		// Format the condition line
		result.WriteString(fmt.Sprintf("  %s %s: %s", icon, cond.Type, cond.Status))

		// Add reason if available and status is not True
		if cond.Status != "True" && cond.Reason != "" {
			result.WriteString(fmt.Sprintf(" (%s)", cond.Reason))
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
// 2. Patches the secret in the capz-system namespace
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

	// Patch the secret with actual values
	// The secret uses stringData, so we need to patch the data field with base64-encoded values
	patchJSON := fmt.Sprintf(`{"stringData":{"AZURE_TENANT_ID":"%s","AZURE_SUBSCRIPTION_ID":"%s"}}`,
		tenantID, subscriptionID)

	output, err := RunCommandQuiet(t, "kubectl", "--context", kubeContext,
		"-n", "capz-system", "patch", "secret", "aso-controller-settings",
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

	if maxRetries <= 0 {
		maxRetries = DefaultApplyMaxRetries
	}

	baseDelay := DefaultApplyRetryDelay

	for attempt := 1; attempt <= maxRetries; attempt++ {
		PrintToTTY("[%d/%d] Applying %s...\n", attempt, maxRetries, yamlPath)
		t.Logf("Applying %s (attempt %d/%d)", yamlPath, attempt, maxRetries)

		output, err := RunCommandQuiet(t, "kubectl", "--context", kubeContext, "apply", "-f", yamlPath)

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
	components := []struct {
		name       string
		namespace  string
		deployment string
	}{
		{"CAPZ (Cluster API Provider Azure)", "capz-system", "capz-controller-manager"},
		{"ASO (Azure Service Operator)", "capz-system", "azureserviceoperator-controller-manager"},
		{"CAPI (Cluster API)", "capi-system", "capi-controller-manager"},
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
		// Local cluster settings
		result.WriteString("\nLocal (Kind) Cluster:\n")
		result.WriteString(fmt.Sprintf("  Management Cluster: %s\n", config.ManagementClusterName))
		result.WriteString(fmt.Sprintf("  Workload Cluster:   %s\n", config.WorkloadClusterName))

		// Azure settings
		result.WriteString("\nAzure Settings:\n")
		result.WriteString(fmt.Sprintf("  Region:             %s\n", config.Region))
		if config.AzureSubscription != "" {
			result.WriteString(fmt.Sprintf("  Subscription:       %s\n", config.AzureSubscription))
		}
		result.WriteString(fmt.Sprintf("  Resource Group:     %s-resgroup\n", config.ClusterNamePrefix))
		result.WriteString(fmt.Sprintf("  OpenShift Version:  %s\n", config.OpenShiftVersion))
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

// DeploymentState holds information about the deployed test resources.
// This is written to a state file during deployment and read during cleanup
// to ensure the cleanup targets the correct Azure resources.
type DeploymentState struct {
	ResourceGroup         string `json:"resource_group"`
	ManagementClusterName string `json:"management_cluster_name"`
	WorkloadClusterName   string `json:"workload_cluster_name"`
	ClusterNamePrefix     string `json:"cluster_name_prefix"`
	Region                string `json:"region"`
	User                  string `json:"user"`
	Environment           string `json:"environment"`
}

// DeploymentStateFile is the path to the deployment state file.
// This file is written during test deployment and read during cleanup.
const DeploymentStateFile = ".deployment-state.json"

// WriteDeploymentState writes the current deployment configuration to a state file.
// This allows cleanup commands to know which Azure resources were actually created,
// regardless of current environment variables or config defaults.
func WriteDeploymentState(config *TestConfig) error {
	state := DeploymentState{
		ResourceGroup:         fmt.Sprintf("%s-resgroup", config.ClusterNamePrefix),
		ManagementClusterName: config.ManagementClusterName,
		WorkloadClusterName:   config.WorkloadClusterName,
		ClusterNamePrefix:     config.ClusterNamePrefix,
		Region:                config.Region,
		User:                  config.User,
		Environment:           config.Environment,
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
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate filename with timestamp
	filename := fmt.Sprintf("%s-%s.log", strings.ToLower(controllerName), time.Now().Format("20060102_150405"))
	logFilePath := fmt.Sprintf("%s/%s", outputDir, filename)

	// Write logs to file
	if err := os.WriteFile(logFilePath, []byte(logs), 0644); err != nil {
		return "", fmt.Errorf("failed to write log file: %w", err)
	}

	return logFilePath, nil
}

// GetAllControllerLogSummaries retrieves log summaries for all key controllers.
// Returns a slice of ControllerLogSummary for CAPI, CAPZ, and ASO controllers.
func GetAllControllerLogSummaries(t *testing.T, kubeContext string) []ControllerLogSummary {
	t.Helper()

	// Define controllers to check (same as in GetComponentVersions)
	controllers := []struct {
		name       string
		namespace  string
		deployment string
	}{
		{"CAPI", "capi-system", "capi-controller-manager"},
		{"CAPZ", "capz-system", "capz-controller-manager"},
		{"ASO", "capz-system", "azureserviceoperator-controller-manager"},
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
	controllers := []struct {
		name       string
		namespace  string
		deployment string
	}{
		{"CAPI", "capi-system", "capi-controller-manager"},
		{"CAPZ", "capz-system", "capz-controller-manager"},
		{"ASO", "capz-system", "azureserviceoperator-controller-manager"},
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
// It looks for the latest results directory, or creates one if needed.
func GetResultsDir() string {
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
	if err := os.MkdirAll(newDir, 0755); err != nil {
		// Fall back to /tmp if we can't create the directory
		return os.TempDir()
	}

	return newDir
}
