package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func artifactTestContext(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	t.Setenv("CAPI_TEST_CONTEXT_FILE", filepath.Join(base, "context.json"))
	t.Setenv("CAPI_TEST_OUTPUT_DIR", "")
	t.Setenv("TEST_RESULTS_DIR", "")
	t.Setenv("ARO_REPO_DIR", "")
	for _, key := range []string{"CS_CLUSTER_NAME", "RESOURCEGROUPNAME", "WORKLOAD_CLUSTER_NAME", "WORKLOAD_CLUSTER_NAMESPACE", "CAPI_USER", "INFRA_PROVIDER", "DEPLOYMENT_ENV"} {
		t.Setenv(key, "")
	}
	root := filepath.Join(base, "private-run")
	context := &RunContext{ClusterNamePrefix: "test-run", ResourceGroupName: "test-rg", WorkloadClusterNamespace: "test-ns", TestRunID: "test-id", OutputDir: root}
	if err := writeRunContext(RunContextFilePath(), context); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestArtifactsConsolidateAndSurviveWorkingDirectoryChange(t *testing.T) {
	root := artifactTestContext(t)
	if got := GetArtifactDir(); got != root {
		t.Fatalf("root = %s, want %s", got, root)
	}
	t.Chdir(t.TempDir())
	if got := getDefaultRepoDir(); got != filepath.Join(root, "cluster-api-installer-aro") {
		t.Errorf("installer path = %s", got)
	}
	if got := (&TestConfig{}).SharedTempDir(); got != root {
		t.Errorf("kubeconfig directory = %s", got)
	}
	if got := GetResultsDir(); got != filepath.Join(root, "results") {
		t.Errorf("results directory = %s", got)
	}
	info, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0700 {
		t.Errorf("private root mode = %o, want 700", info.Mode().Perm())
	}
}

func TestArtifactsPreserveExplicitOverrides(t *testing.T) {
	root := artifactTestContext(t)
	external := t.TempDir()
	t.Setenv("ARO_REPO_DIR", filepath.Join(external, "installer"))
	t.Setenv("TEST_RESULTS_DIR", filepath.Join(external, "published"))
	if got := getDefaultRepoDir(); got != filepath.Join(external, "installer") {
		t.Errorf("repo override = %s", got)
	}
	if got := GetResultsDir(); got != filepath.Join(external, "published") {
		t.Errorf("report override = %s", got)
	}
	if got := GetArtifactDir(); got != root {
		t.Errorf("private directory changed to %s", got)
	}
	t.Setenv("CAPI_TEST_OUTPUT_DIR", filepath.Join(external, "other-run"))
	context, err := readRunContext(RunContextFilePath())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := resolveArtifactDir(context); err == nil {
		t.Fatal("expected conflict for changed output root")
	}
}

func TestArtifactsReuseAcrossProcesses(t *testing.T) {
	root := artifactTestContext(t)
	GetArtifactDir()
	command := exec.Command(os.Args[0], "-test.run=^TestArtifactsProcessHelper$")
	command.Env = append(os.Environ(), "CAPI_ARTIFACT_HELPER=1")
	command.Dir = t.TempDir()
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("child process: %v: %s", err, output)
	}
	if !strings.Contains(string(output), "ARTIFACT_ROOT="+root) {
		t.Fatalf("child did not reuse root: %s", output)
	}
}

func TestArtifactsProcessHelper(t *testing.T) {
	if os.Getenv("CAPI_ARTIFACT_HELPER") != "1" {
		t.Skip("subprocess helper")
	}
	t.Log("ARTIFACT_ROOT=" + GetArtifactDir())
	// Logging requires verbose mode in the subprocess.
	println("ARTIFACT_ROOT=" + GetArtifactDir())
}

func TestArtifactsCleanupIsolatedAndRejectsUnownedPaths(t *testing.T) {
	for _, scenario := range []string{"owned", "unowned", "wrong-owner", "symlink", "nonempty"} {
		t.Run(scenario, func(t *testing.T) {
			root := artifactTestContext(t)
			outside := t.TempDir()
			sentinel := filepath.Join(outside, "other-kubeconfig.yaml")
			if err := os.WriteFile(sentinel, []byte("keep"), 0600); err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "owned":
				GetArtifactDir()
			case "unowned":
				if err := os.MkdirAll(root, 0700); err != nil {
					t.Fatal(err)
				}
			case "wrong-owner":
				GetArtifactDir()
				if err := os.WriteFile(filepath.Join(root, artifactOwnerFile), []byte("another-context"), 0600); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				if err := os.Symlink(outside, root); err != nil {
					t.Fatal(err)
				}
			case "nonempty":
				if err := os.MkdirAll(root, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, "keep"), []byte("keep"), 0600); err != nil {
					t.Fatal(err)
				}
				if err := ensureArtifactDir(root); err == nil {
					t.Fatal("claimed nonempty directory")
				}
			}
			err := removeRunArtifacts()
			if scenario == "owned" {
				if err != nil {
					t.Fatal(err)
				}
				if _, err := os.Stat(root); !os.IsNotExist(err) {
					t.Fatalf("root remains: %v", err)
				}
			} else if err == nil {
				t.Fatal("unsafe cleanup succeeded")
			}
			if _, err := os.Stat(sentinel); err != nil {
				t.Fatalf("unrelated artifact removed: %v", err)
			}
		})
	}
}

func TestArtifactsRejectDangerousRoots(t *testing.T) {
	artifactTestContext(t)
	for _, dir := range []string{"/", os.TempDir(), runContextWorkspace(), filepath.Dir(RunContextFilePath())} {
		t.Setenv("CAPI_TEST_OUTPUT_DIR", dir)
		if _, err := resolveArtifactDir(&RunContext{}); err == nil {
			t.Errorf("accepted unsafe root %s", dir)
		}
	}
}

// TestCleanRunArtifacts is an opt-in entry point for Make's local cleanup.
func TestCleanRunArtifacts(t *testing.T) {
	if os.Getenv("CAPI_CLEAN_RUN_ARTIFACTS") != "1" {
		t.Skip("requires explicit cleanup invocation")
	}
	if err := removeRunArtifacts(); err != nil {
		t.Fatal(err)
	}
}

func TestArtifactsKindKubeconfigRespectsExplicitPath(t *testing.T) {
	root := artifactTestContext(t)
	t.Setenv("CLUSTER_MODE", "kind")
	t.Setenv("USE_KIND", "true")
	t.Setenv("USE_KUBECONFIG", "")
	t.Setenv("KUBECONFIG", "")
	NewTestConfig()
	if got := os.Getenv("KUBECONFIG"); got != filepath.Join(root, "management-kubeconfig.yaml") {
		t.Errorf("Kind kubeconfig = %s", got)
	}
	explicit := filepath.Join(t.TempDir(), "external-kubeconfig")
	t.Setenv("KUBECONFIG", explicit)
	NewTestConfig()
	if got := os.Getenv("KUBECONFIG"); got != explicit {
		t.Errorf("explicit kubeconfig changed to %s", got)
	}
}

func TestArtifactsPermissionsAndMarkerSymlink(t *testing.T) {
	for _, scenario := range []string{"claim-empty", "public-root", "public-marker", "marker-symlink"} {
		t.Run(scenario, func(t *testing.T) {
			root := artifactTestContext(t)
			if scenario == "claim-empty" {
				if err := os.MkdirAll(root, 0755); err != nil {
					t.Fatal(err)
				}
				if err := ensureArtifactDir(root); err != nil {
					t.Fatal(err)
				}
				info, err := os.Stat(root)
				if err != nil {
					t.Fatal(err)
				}
				if info.Mode().Perm() != 0700 {
					t.Fatalf("claimed directory mode = %o", info.Mode().Perm())
				}
				return
			}
			GetArtifactDir()
			marker := filepath.Join(root, artifactOwnerFile)
			switch scenario {
			case "public-root":
				if err := os.Chmod(root, 0755); err != nil {
					t.Fatal(err)
				}
			case "public-marker":
				if err := os.Chmod(marker, 0644); err != nil {
					t.Fatal(err)
				}
			case "marker-symlink":
				target := filepath.Join(t.TempDir(), "marker")
				if err := os.Rename(marker, target); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, marker); err != nil {
					t.Fatal(err)
				}
			}
			if err := ensureArtifactDir(root); err == nil {
				t.Fatal("accepted unsafe directory/marker")
			}
		})
	}
}

func TestArtifactsResultsFailureDoesNotFallBack(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEST_RESULTS_DIR", filepath.Join(file, "results"))
	defer func() {
		if recover() == nil {
			t.Error("expected explicit failure creating results")
		}
	}()
	GetResultsDir()
}

func TestArtifactsExplicitReportsDoNotInitializePrivateRoot(t *testing.T) {
	base := t.TempDir()
	contextPath := filepath.Join(base, "unused-context")
	t.Setenv("CAPI_TEST_CONTEXT_FILE", contextPath)
	t.Setenv("CAPI_TEST_OUTPUT_DIR", "/")
	reports := filepath.Join(base, "reports")
	t.Setenv("TEST_RESULTS_DIR", reports)
	if got := GetResultsDir(); got != reports {
		t.Fatalf("reports = %s", got)
	}
	if _, err := os.Stat(contextPath); !os.IsNotExist(err) {
		t.Fatalf("initialized unused context: %v", err)
	}
}

func TestArtifactsMakePhaseFlow(t *testing.T) {
	if _, err := exec.LookPath("make"); err != nil {
		t.Skip("make unavailable")
	}
	root := artifactTestContext(t)
	base := t.TempDir()
	mock := filepath.Join(base, "gotestsum")
	script := `#!/bin/bash
set -eu
printf '%s|%s\n' "$CAPI_TEST_OUTPUT_DIR" "$TEST_RESULTS_DIR" >> "$ARTIFACT_PHASE_LOG"
for arg in "$@"; do
 case "$arg" in --junitfile=*) printf '<testsuites/>\n' > "${arg#--junitfile=}";; esac
done
`
	if err := os.WriteFile(mock, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	phaseLog := filepath.Join(base, "phases")
	t.Setenv("ARTIFACT_PHASE_LOG", phaseLog)
	t.Setenv("PATH", base+string(os.PathListSeparator)+os.Getenv("PATH"))
	reports := filepath.Join(base, "published-reports")
	for _, phase := range []string{"_check-dep", "_setup"} {
		command := exec.Command("make", "--no-print-directory", phase, "GOFLAGS=-mod=mod", "GOTESTSUM_BIN="+mock, "RESULTS_DIR="+reports, "LATEST_RESULTS_DIR="+filepath.Join(base, "latest"))
		command.Dir = runContextWorkspace()
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("make %s: %v: %s", phase, err, output)
		}
	}
	data, err := os.ReadFile(phaseLog)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Repeat(root+"|"+reports+"\n", 2)
	if string(data) != want {
		t.Fatalf("phase paths = %q, want %q", data, want)
	}
	for _, name := range []string{"junit-check-dep.xml", "junit-setup.xml"} {
		if _, err := os.Stat(filepath.Join(reports, name)); err != nil {
			t.Error(err)
		}
	}
	if _, err := os.Stat(filepath.Join(reports, artifactOwnerFile)); !os.IsNotExist(err) {
		t.Fatalf("private marker leaked to reports: %v", err)
	}
}

func TestArtifactsFreshFixedIdentityUsesUniqueDirectories(t *testing.T) {
	base := t.TempDir()
	t.Setenv("CAPI_TEST_OUTPUT_DIR", "")
	t.Setenv("SHARED_DIR", base)
	t.Setenv("ARO_REPO_DIR", filepath.Join(base, "legacy-installer"))
	t.Setenv("CS_CLUSTER_NAME", "cate-fixed")
	t.Setenv("CAPI_USER", "cate")
	t.Setenv("RESOURCEGROUPNAME", "fixed-rg")
	t.Setenv("WORKLOAD_CLUSTER_NAMESPACE", "fixed-ns")
	var first string
	for _, name := range []string{"first.json", "second.json"} {
		t.Setenv("CAPI_TEST_CONTEXT_FILE", filepath.Join(base, name))
		context, err := EnsureRunContext()
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(context.OutputDir, base+string(filepath.Separator)) {
			t.Fatalf("output escaped shared parent: %s", context.OutputDir)
		}
		if first == "" {
			first = context.OutputDir
		} else if context.OutputDir == first {
			t.Fatal("new run reused another run's output directory")
		}
	}
}

func TestArtifactsLegacyContextDirectoriesAreIsolated(t *testing.T) {
	base := t.TempDir()
	t.Setenv("CAPI_TEST_OUTPUT_DIR", "")
	context := &RunContext{TestRunID: "fixed"}
	var first string
	for _, dir := range []string{"one", "two"} {
		t.Setenv("CAPI_TEST_CONTEXT_FILE", filepath.Join(base, dir, "context.json"))
		root, err := resolveArtifactDir(context)
		if err != nil {
			t.Fatal(err)
		}
		if first == "" {
			first = root
		} else if first == root {
			t.Fatal("legacy contexts share an artifact directory")
		}
	}
}
