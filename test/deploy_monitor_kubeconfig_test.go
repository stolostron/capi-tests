package test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Exercise the monitoring phase itself: a later override can break command
// execution even when SetupKubeconfig passes its own unit tests.
func TestMonitorClusterKubeconfig(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	for _, tt := range []struct {
		name      string
		external  bool
		inherited bool
	}{
		{name: "external kubeconfig takes precedence", external: true, inherited: true},
		{name: "local mode preserves inherited kubeconfig", inherited: true},
		{name: "local mode leaves default lookup to clients"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			binDir := filepath.Join(dir, "bin")
			if err := os.Mkdir(binDir, 0700); err != nil {
				t.Fatal(err)
			}
			for _, tool := range []string{"kubectl", "clusterctl"} {
				// POSIX shell builtins keep the isolated PATH free of real clients.
				script := fmt.Sprintf(`#!/bin/sh
printf '%%s\n' "$KUBECONFIG" >> "$MONITOR_KUBECONFIG_LOG.%s"
printf 'monitor-context\n'
`, tool)
				if err := os.WriteFile(filepath.Join(binDir, tool), []byte(script), 0700); err != nil {
					t.Fatal(err)
				}
			}

			inherited := ""
			if tt.inherited {
				inherited = filepath.Join(dir, "inherited.config")
			}
			external := ""
			want := inherited
			if tt.external {
				external = filepath.Join(dir, "external.config")
				want = external
			}
			logPath := filepath.Join(dir, "kubeconfig")
			t.Setenv("PATH", binDir)
			t.Setenv("ARO_REPO_DIR", dir)
			t.Setenv("CLUSTERCTL_BIN", "bin/clusterctl")
			t.Setenv("CLUSTER_MODE", "")
			t.Setenv("USE_KUBECONFIG", external)
			t.Setenv("KUBECONFIG", inherited)
			t.Setenv("CAPI_TEST_CONTEXT_FILE", filepath.Join(dir, "run-context.json"))
			t.Setenv("MONITOR_KUBECONFIG_LOG", logPath)

			// A fresh process isolates NewTestConfig's cached repository path and
			// executes only this phase, without contacting any real cluster.
			cmd := exec.Command(executable, "-test.run=^TestDeployment_MonitorCluster$", "-test.v")
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("monitoring phase failed: %v\n%s", err, output)
			}
			for _, tool := range []string{"kubectl", "clusterctl"} {
				data, err := os.ReadFile(logPath + "." + tool)
				if err != nil {
					t.Fatalf("%s did not run: %v\n%s", tool, err, output)
				}
				for _, got := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
					if got != want {
						t.Errorf("%s received KUBECONFIG = %q, want %q", tool, got, want)
					}
				}
			}
		})
	}
}
