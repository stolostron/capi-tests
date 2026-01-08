package test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
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
