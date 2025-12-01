package test

import (
	"fmt"
	"os"
	"testing"
	"time"
)

// TestDeployment_ProgressDemo demonstrates the real-time progress output
// This test simulates the waiting behavior to verify progress is shown in real-time
// Run with: go test -v ./test -run TestDeployment_ProgressDemo
func TestDeployment_ProgressDemo(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping progress demo in short mode")
	}

	// Simulate a 30-second wait with 5-second intervals (like the real test but faster)
	timeout := 30 * time.Second
	pollInterval := 5 * time.Second
	startTime := time.Now()

	fmt.Fprintf(os.Stderr, "\n=== Demo: Simulating control plane wait ===\n")
	fmt.Fprintf(os.Stderr, "Timeout: %v | Poll interval: %v\n\n", timeout, pollInterval)
	t.Logf("Starting progress demo (timeout: %v)...", timeout)

	iteration := 0
	for {
		elapsed := time.Since(startTime)
		remaining := timeout - elapsed

		if elapsed > timeout {
			fmt.Fprintf(os.Stderr, "\n❌ Demo timeout reached after %v\n\n", elapsed.Round(time.Second))
			t.Log("Demo completed - timeout reached")
			return
		}

		// Simulate checking control plane (always returns not ready for demo)
		// In real test, this would be: kubectl get kubeadmcontrolplane ...

		iteration++
		percentage := int((float64(elapsed) / float64(timeout)) * 100)

		// Print progress to stderr for real-time visibility
		fmt.Fprintf(os.Stderr, "[%d] ⏳ Waiting... | Elapsed: %v | Remaining: %v | Progress: %d%%\n",
			iteration,
			elapsed.Round(time.Second),
			remaining.Round(time.Second),
			percentage)

		// Also log to test output
		t.Logf("Demo iteration %d (elapsed: %v, remaining: %v, %d%%)",
			iteration, elapsed.Round(time.Second), remaining.Round(time.Second), percentage)

		// For demo, complete after 15 seconds
		if elapsed > 15*time.Second {
			fmt.Fprintf(os.Stderr, "\n✅ Demo complete! (took %v)\n\n", elapsed.Round(time.Second))
			t.Log("Demo completed successfully")
			return
		}

		time.Sleep(pollInterval)
	}
}
