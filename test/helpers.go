package test

import (
	"fmt"
	"io"
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
