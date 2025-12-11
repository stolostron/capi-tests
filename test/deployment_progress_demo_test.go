package test

import (
	"fmt"
	"os"
	"testing"
	"time"
)

// TestDeployment_ProgressDemo demonstrates the real-time progress output
// This test simulates the waiting behavior to verify progress is shown in real-time
// Run with: RUN_DEMO_TESTS=1 go test -v ./test -run TestDeployment_ProgressDemo
func TestDeployment_ProgressDemo(t *testing.T) {
	if os.Getenv("RUN_DEMO_TESTS") != "1" {
		t.Skip("Skipping demo test (set RUN_DEMO_TESTS=1 to run)")
	}

	// Simulate a wait with a 30-second timeout and 5-second intervals
	// Demo completes after 15 seconds to show the success case
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

		// Report progress using helper function
		ReportProgress(t, iteration, elapsed, remaining, timeout)

		// For demo, complete after 15 seconds
		if elapsed > 15*time.Second {
			fmt.Fprintf(os.Stderr, "\n✅ Demo complete! (took %v)\n\n", elapsed.Round(time.Second))
			t.Log("Demo completed successfully")
			return
		}

		time.Sleep(pollInterval)
	}
}
