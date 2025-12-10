package test

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// CommandExists checks if a command is available in the system PATH
func CommandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// RunCommand executes a shell command and returns output and error.
// In verbose mode (-v flag), the command being executed is printed to stderr
// for immediate visibility.
func RunCommand(t *testing.T, name string, args ...string) (string, error) {
	t.Helper()

	// Print command being executed to stderr when in verbose mode
	cmdStr := name
	if len(args) > 0 {
		cmdStr = fmt.Sprintf("%s %s", name, strings.Join(args, " "))
	}
	if testing.Verbose() {
		fmt.Fprintf(os.Stderr, "Running: %s\n", cmdStr)
	}

	// Also log to test output
	t.Logf("Executing command: %s", cmdStr)

	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

// RunCommandWithStreaming executes a shell command and streams output in real-time.
// This is useful for long-running commands where users need to see progress.
// Returns the complete output and any error that occurred.
func RunCommandWithStreaming(t *testing.T, name string, args ...string) (string, error) {
	t.Helper()

	// Print command being executed
	cmdStr := name
	if len(args) > 0 {
		cmdStr = fmt.Sprintf("%s %s", name, strings.Join(args, " "))
	}
	if testing.Verbose() {
		fmt.Fprintf(os.Stderr, "Running (streaming): %s\n", cmdStr)
	}

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

	// Buffer to collect all output
	var outputBuilder strings.Builder

	// Stream output in real-time
	done := make(chan bool)
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				chunk := string(buf[:n])
				outputBuilder.WriteString(chunk)
				// Print to stderr for real-time visibility
				fmt.Fprint(os.Stderr, chunk)
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
				outputBuilder.WriteString(chunk)
				// Print to stderr for real-time visibility
				fmt.Fprint(os.Stderr, chunk)
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

	output := strings.TrimSpace(outputBuilder.String())
	return output, cmdErr
}

// SetEnvVar sets an environment variable for testing
func SetEnvVar(t *testing.T, key, value string) {
	t.Helper()
	oldValue := os.Getenv(key)
	os.Setenv(key, value)
	t.Cleanup(func() {
		if oldValue == "" {
			os.Unsetenv(key)
		} else {
			os.Setenv(key, oldValue)
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

// ReportProgress prints progress information to stderr for real-time visibility
// and to test log for test output. This helper ensures consistent progress
// reporting across all deployment tests.
func ReportProgress(t *testing.T, w io.Writer, iteration int, elapsed, remaining, timeout time.Duration) {
	t.Helper()
	percentage := int((float64(elapsed) / float64(timeout)) * 100)

	// Print to stderr for real-time visibility (unbuffered)
	fmt.Fprintf(w, "[%d] ⏳ Waiting... | Elapsed: %v | Remaining: %v | Progress: %d%%\n",
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
