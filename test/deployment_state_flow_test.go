package test

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDeploymentStateFlowRoundTrip(t *testing.T) {
	// Run in a fresh process so repository-path caching from other tests cannot
	// redirect legacy state cleanup outside this fixture.
	if os.Getenv("CAPI_STATE_FLOW_ROUNDTRIP") != "1" {
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(executable, "-test.run=^TestDeploymentStateFlowRoundTrip$", "-test.v")
		cmd.Env = append(os.Environ(), "CAPI_STATE_FLOW_ROUNDTRIP=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("isolated state round trip: %v\n%s", err, output)
		}
		return
	}
	workspace := t.TempDir()
	t.Chdir(workspace)
	t.Setenv("DEPLOYMENT_STATE_FILE", filepath.Join(workspace, "state.json"))
	t.Setenv("CAPI_TEST_CONTEXT_FILE", filepath.Join(workspace, "context.json"))
	// Keep legacy cleanup candidates inside this fixture too.
	t.Setenv("ARO_REPO_DIR", workspace)
	config := &TestConfig{
		ResourceGroupName: "alice-stage-resgroup", ManagementClusterName: "alice-management",
		WorkloadClusterName: "alice-workload", WorkloadClusterNamespace: "alice-namespace",
		ClusterNamePrefix: "alice-stage", Region: "uksouth", CAPIUser: "alice",
		Environment: "stage", TestRunID: "run-one", ResourceTags: map[string]string{"owner": "alice"},
	}
	if err := WriteDeploymentState(config); err != nil {
		t.Fatal(err)
	}
	// A subsequent phase replaces stale deployment metadata.
	config.Region = "eastus"
	if err := WriteDeploymentState(config); err != nil {
		t.Fatal(err)
	}
	want := &DeploymentState{
		ResourceGroup: config.ResourceGroupName, ManagementClusterName: config.ManagementClusterName,
		WorkloadClusterName: config.WorkloadClusterName, WorkloadClusterNamespace: config.WorkloadClusterNamespace,
		ClusterNamePrefix: config.ClusterNamePrefix, Region: config.Region, User: config.CAPIUser,
		Environment: config.Environment, TestRunID: config.TestRunID, ResourceTags: config.ResourceTags,
	}
	got, err := ReadDeploymentState()
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadDeploymentState() = %+v, %v; want %+v", got, err, want)
	}
	info, err := os.Stat(deploymentStateFilePath())
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("state file permissions: info=%v, err=%v; want 0600", info, err)
	}
	for range 2 {
		if err := DeleteDeploymentState(); err != nil {
			t.Fatalf("idempotent deletion failed: %v", err)
		}
	}
	if state, err := ReadDeploymentState(); err != nil || state != nil {
		t.Fatalf("state after deletion = %+v, %v; want nil, nil", state, err)
	}
}

func TestDeploymentStateFlowErrors(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("DEPLOYMENT_STATE_FILE", "state.json")
	if err := os.Mkdir("state.json", 0700); err != nil {
		t.Fatal(err)
	}
	// A directory at the filename gives a deterministic write error even as root.
	if err := WriteDeploymentState(&TestConfig{}); err == nil || !strings.Contains(err.Error(), "failed to write deployment state file") {
		t.Fatalf("write error = %v; want contextual state-file error", err)
	}
	if _, err := ReadDeploymentState(); err == nil || !strings.Contains(err.Error(), "failed to read deployment state file") {
		t.Fatalf("read error = %v; want contextual state-file error", err)
	}
	if err := os.Remove("state.json"); err != nil {
		t.Fatal(err)
	}
	writeStateFlowFile(t, "state.json", "{invalid", 0600)
	if _, err := ReadDeploymentState(); err == nil || !strings.Contains(err.Error(), "failed to parse deployment state file") {
		t.Fatalf("parse error = %v; want contextual state-file error", err)
	}
}

func TestDeploymentStateFlowWritePermissionError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can bypass directory write permissions")
	}
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("DEPLOYMENT_STATE_FILE", "state.json")
	if err := os.Chmod(dir, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(dir, 0700); err != nil {
			t.Errorf("restore fixture directory permissions: %v", err)
		}
	})
	err := WriteDeploymentState(&TestConfig{})
	if err == nil || !errors.Is(err, os.ErrPermission) || !strings.Contains(err.Error(), "failed to write deployment state file") {
		t.Fatalf("write error = %v; want contextual permission error", err)
	}
}

// This executes the real clean-all recipe in a disposable repository. Kind and
// cloud cleanup are fakes; every path the recipe can remove is under t.TempDir.
func TestDeploymentStateFlowMakeCleanup(t *testing.T) {
	makefile, err := os.ReadFile("../Makefile")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name         string
		pathMode     string
		partial      bool
		missing      bool
		alreadyGone  bool
		cloudFails   bool
		relativeRepo bool
		commandLine  bool
	}{
		{name: "legacy state survives changed environment"},
		{name: "run scoped state wins over stale legacy", pathMode: "scoped"},
		{name: "explicit absolute path", pathMode: "absolute"},
		{name: "explicit relative path", pathMode: "relative"},
		{name: "relative installer with explicit path", pathMode: "relative", relativeRepo: true},
		{name: "relative installer with default state", relativeRepo: true},
		{name: "relative installer with scoped state", pathMode: "scoped", relativeRepo: true},
		{name: "command line absolute path", pathMode: "absolute", commandLine: true},
		{name: "command line relative path", pathMode: "relative", commandLine: true},
		{name: "partial state falls back per field", partial: true},
		{name: "missing state falls back to environment", missing: true},
		{name: "already deleted cluster is harmless", alreadyGone: true},
		{name: "failed cloud cleanup retains state", cloudFails: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			workspace := t.TempDir()
			repoDir := filepath.Join(workspace, "installer")
			binDir := filepath.Join(workspace, "bin")
			for _, dir := range []string{repoDir, binDir, filepath.Join(repoDir, "nested"), filepath.Join(workspace, "test"), filepath.Join(workspace, "scripts")} {
				if err := os.MkdirAll(dir, 0700); err != nil {
					t.Fatal(err)
				}
			}
			writeStateFlowFile(t, filepath.Join(workspace, "Makefile"), string(makefile), 0600)
			writeStateFlowFile(t, filepath.Join(workspace, "test", "config.go"), "const DefaultCAPIUser = \"fallback\"\n", 0600)
			writeStateFlowFile(t, filepath.Join(binDir, "kind"), `#!/bin/bash
echo "kind $*" >> "$CALLS"
case "$*" in
  'get clusters') printf '%s\n' "$MOCK_CLUSTERS" ;;
  'delete cluster --name alice-management'|'delete cluster --name bob-management') ;;
  *) exit 99 ;;
esac
`, 0700)
			writeStateFlowFile(t, filepath.Join(workspace, "scripts", "cleanup-azure-resources.sh"), "#!/bin/bash\necho \"azure $*\" >> \"$CALLS\"\nexit \"$MOCK_CLOUD_EXIT\"\n", 0700)
			// Unexpected calls to actual cloud tools are forbidden in this fixture.
			for _, tool := range []string{"az", "aws", "kubectl"} {
				writeStateFlowFile(t, filepath.Join(binDir, tool), "#!/bin/bash\nexit 99\n", 0700)
			}
			for _, key := range []string{"DEPLOYMENT_STATE_FILE", "RESOURCEGROUPNAME", "CS_CLUSTER_NAME", "MAKEFLAGS", "MFLAGS", "MAKEOVERRIDES"} {
				t.Setenv(key, "")
			}
			t.Setenv("PATH", binDir+":/usr/bin:/bin")
			t.Setenv("TMPDIR", workspace)
			t.Setenv("ARO_REPO_DIR", repoDir)
			if tc.relativeRepo {
				t.Setenv("ARO_REPO_DIR", "installer")
			}
			t.Setenv("CAPI_TEST_CONTEXT_FILE", filepath.Join(workspace, "context.json"))
			t.Setenv("CAPI_USER", "bob")
			t.Setenv("DEPLOYMENT_ENV", "prod")
			t.Setenv("MANAGEMENT_CLUSTER_NAME", "bob-management")
			t.Setenv("AZURE_RESOURCE_GROUP", "bob-resgroup")
			t.Setenv("INFRA_PROVIDER", "aro")
			t.Setenv("CALLS", filepath.Join(workspace, "calls"))
			t.Setenv("MOCK_CLUSTERS", "alice-management\nbob-management")
			t.Setenv("MOCK_CLOUD_EXIT", "0")
			if tc.alreadyGone {
				t.Setenv("MOCK_CLUSTERS", "")
			}
			if tc.cloudFails {
				t.Setenv("MOCK_CLOUD_EXIT", "1")
			}
			statePath := filepath.Join(repoDir, legacyDeploymentStateFile)
			switch tc.pathMode {
			case "scoped":
				t.Setenv("RESOURCEGROUPNAME", "alice-resgroup")
				statePath = filepath.Join(repoDir, deploymentStateFilePath())
				writeStateFlowFile(t, filepath.Join(repoDir, legacyDeploymentStateFile), "{\n  \"resource_group\": \"stale-resgroup\"\n}\n", 0600)
			case "absolute":
				statePath = filepath.Join(workspace, "explicit-state.json")
				t.Setenv("DEPLOYMENT_STATE_FILE", statePath)
			case "relative":
				statePath = filepath.Join(repoDir, "nested", "state.json")
				t.Setenv("DEPLOYMENT_STATE_FILE", "nested/state.json")
			}
			if !tc.missing {
				if tc.partial {
					data, marshalErr := json.MarshalIndent(map[string]string{"resource_group": "alice-resgroup"}, "", "  ")
					if marshalErr != nil {
						t.Fatal(marshalErr)
					}
					writeStateFlowFile(t, statePath, string(data), 0600)
				} else {
					// Match the deployment phase's working directory when writing state.
					t.Chdir(repoDir)
					if err := WriteDeploymentState(&TestConfig{ResourceGroupName: "alice-resgroup", ManagementClusterName: "alice-management", ClusterNamePrefix: "alice-stage", CAPIUser: "alice", Environment: "stage"}); err != nil {
						t.Fatal(err)
					}
				}
			}
			t.Setenv("CS_CLUSTER_NAME", "bob-prod")
			writeStateFlowFile(t, filepath.Join(workspace, "context.json"), "{}", 0600)
			args := []string{"--no-print-directory", "clean-all", "GOBIN=" + binDir, "GOTESTSUM_BIN=unused"}
			if tc.commandLine {
				args = append(args, "DEPLOYMENT_STATE_FILE="+os.Getenv("DEPLOYMENT_STATE_FILE"))
				t.Setenv("DEPLOYMENT_STATE_FILE", "")
			}
			cmd := exec.Command("make", args...)
			cmd.Dir = workspace
			output, err := cmd.CombinedOutput()
			if (err != nil) != tc.cloudFails {
				t.Fatalf("make clean-all error = %v; cloudFails=%v\n%s", err, tc.cloudFails, output)
			}
			calls, err := os.ReadFile(filepath.Join(workspace, "calls"))
			if err != nil {
				t.Fatal(err)
			}
			group, management, prefix := "alice-resgroup", "alice-management", "alice-stage"
			if tc.missing {
				group = "bob-resgroup"
			}
			if tc.missing || tc.partial {
				management, prefix = "bob-management", "bob-prod"
			}
			wantCalls := "kind get clusters\n"
			if !tc.alreadyGone {
				wantCalls += "kind delete cluster --name " + management + "\n"
			}
			wantCalls += "azure --resource-group " + group + " --prefix " + prefix + " --match-mode contains --force\n"
			if string(calls) != wantCalls {
				t.Errorf("cleanup calls:\n%s\nwant:\n%s", calls, wantCalls)
			}
			for _, path := range []string{statePath, filepath.Join(workspace, "context.json")} {
				_, err := os.Stat(path)
				if tc.cloudFails && err != nil {
					t.Errorf("failed cleanup did not retain %s: %v", path, err)
				} else if !tc.cloudFails && !os.IsNotExist(err) {
					t.Errorf("successful cleanup left %s: %v", path, err)
				}
			}
		})
	}
}

func writeStateFlowFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatalf("write fixture %s: %v", path, err)
	}
}
