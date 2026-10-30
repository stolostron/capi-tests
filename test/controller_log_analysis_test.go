package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func writeControllerLogFixture(t *testing.T, dir, name, contents string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAnalyzeSavedControllerLogs_CountsAndGroups(t *testing.T) {
	dir := t.TempDir()
	known := `{"level":"error","msg":"Reconciler error","error":"Operation cannot be fulfilled on clusters.cluster.x-k8s.io test: the object has been modified; please apply your changes to the latest version and try again"}`
	unknown := `{"level":"error","ts":"2026-10-30T01:02:03Z","msg":"Reconciler error","error":"permission denied"}`
	writeControllerLogFixture(t, dir, "capi-20261030_010203.log", strings.Repeat(known+"\n", 35)+strings.Repeat(unknown+"\n", 25))
	writeControllerLogFixture(t, dir, "capz-20261030_010204.log", strings.ReplaceAll(unknown, "01:02:03", "01:02:04")+"\n"+`{"level":"info","msg":"error: ignored diagnostic"}`)
	writeControllerLogFixture(t, dir, "test-output.log", "ERROR must not count")
	writeControllerLogFixture(t, dir, "capi-not-a-snapshot.log", "ERROR must not count")
	report, err := AnalyzeSavedControllerLogs(dir, []ControllerDef{{DisplayName: "CAPI"}, {DisplayName: "CAPZ"}}, "")
	if err != nil {
		t.Fatal(err)
	}
	if report.FileCount != 2 || report.ErrorCount != 61 || report.UnexpectedCount != 26 {
		t.Fatalf("report = %+v", report)
	}
	if len(report.Groups) != 2 || report.Groups[0].Known || report.Groups[0].Count != 26 || report.Groups[1].Count != 35 {
		t.Fatalf("groups = %+v", report.Groups)
	}
	for _, want := range []string{"unexpected count=26", "known count=35", "permission denied", ".log:1"} {
		if !strings.Contains(report.String(), want) {
			t.Errorf("report missing %q: %s", want, report.String())
		}
	}
}

func TestAnalyzeSavedControllerLogs_KnownRules(t *testing.T) {
	for _, tc := range []struct {
		name, rules string
		unexpected  int
		wantError   string
	}{
		{"custom and overlapping", "# custom\nretry soon\nretry.*\n", 0, ""},
		{"empty replaces defaults", "# no exceptions\n", 1, ""},
		{"invalid regex", "[\n", 0, "known error pattern 1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeControllerLogFixture(t, dir, "aso-20261030_010203.log", "ERROR retry soon\n")
			patterns := writeControllerLogFixture(t, dir, "known.txt", tc.rules)
			report, err := AnalyzeSavedControllerLogs(dir, []ControllerDef{{DisplayName: "ASO"}}, patterns)
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("error = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if report.UnexpectedCount != tc.unexpected || report.ErrorCount != 1 || len(report.Groups) != 1 || report.Groups[0].Count != 1 {
				t.Fatalf("report = %+v", report)
			}
		})
	}
}

func TestAnalyzeSavedControllerLogs_InputFailures(t *testing.T) {
	dir := t.TempDir()
	controllers := []ControllerDef{{DisplayName: "CAPI"}}
	if _, err := AnalyzeSavedControllerLogs(dir, controllers, ""); err == nil || !strings.Contains(err.Error(), "no saved controller logs") {
		t.Fatalf("empty directory: %v", err)
	}
	if _, err := AnalyzeSavedControllerLogs(filepath.Join(dir, "absent"), controllers, ""); err == nil {
		t.Fatal("missing directory accepted")
	}
	if _, err := AnalyzeSavedControllerLogs(dir, controllers, filepath.Join(dir, "absent")); err == nil || !strings.Contains(err.Error(), "read known error patterns") {
		t.Fatalf("missing rules: %v", err)
	}
	writeControllerLogFixture(t, dir, "capi-20261030_010203.log", "ERROR "+strings.Repeat("x", 128*1024))
	report, err := AnalyzeSavedControllerLogs(dir, controllers, "")
	if err != nil || report.UnexpectedCount != 1 {
		t.Fatalf("long line report=%+v err=%v", report, err)
	}
	writeControllerLogFixture(t, dir, "capi-20261030_010203.log", "INFO fine\nERROR "+strings.Repeat("x", maxControllerLogLineBytes))
	if _, err := AnalyzeSavedControllerLogs(dir, controllers, ""); err == nil || !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("oversize line: %v", err)
	}
}

func TestControllerErrorMessage(t *testing.T) {
	for _, tc := range []struct {
		line string
		want bool
	}{
		{`E1030 12:34:56.123456 123 controller.go:42] "Reconciler error" err="denied"`, true},
		{`I1030 12:34:56.123456 123 controller.go:42] "ready"`, false},
		{`I1030 12:34:56.123456 123 controller.go:42] "previous error recovered"`, false},
		{"2026-10-30T12:34:56Z\tERROR\tReconciler failed", true},
		{`{"severity" : "ERROR", "message":"failed"}`, true},
		{`{"level":"info","msg":"error: retry diagnostic"}`, false},
		{"level=error msg=failed", true},
		{"warning: retry", false},
		{"ERROR", true},
		{"", false},
	} {
		if _, got := controllerErrorMessage(tc.line); got != tc.want {
			t.Errorf("classify %q = %v, want %v", tc.line, got, tc.want)
		}
	}
}

func TestAnalyzeSavedControllerLogs_CleanAndKnownOnly(t *testing.T) {
	for _, line := range []string{
		"",
		"INFO controller ready\nwarning: retry\n",
		"ERROR Operation cannot be fulfilled on clusters test: the object has been modified; please apply your changes to the latest version and try again\n",
	} {
		dir := t.TempDir()
		writeControllerLogFixture(t, dir, "capi-20261030_010203.log", line)
		report, err := AnalyzeSavedControllerLogs(dir, []ControllerDef{{DisplayName: "CAPI"}}, "")
		if err != nil || report.UnexpectedCount != 0 {
			t.Fatalf("line=%q report=%+v error=%v", line, report, err)
		}
	}
}

// Exercise the actual phase in a separate test process so its expected failures
// do not fail this regression test or touch the caller's persisted run context.
func TestControllerLogAnalysisEntryPoint(t *testing.T) {
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, content, enabled, output string
		wantFailure                    bool
	}{
		{"known only", "ERROR Operation cannot be fulfilled on clusters test: the object has been modified; please apply your changes to the latest version and try again\n", "true", "1 errors, 0 unexpected", false},
		{"unexpected", "ERROR permission denied\n", "true", "Found 1 unexpected controller error occurrences", true},
		{"disabled", "ERROR permission denied\n", "false", "--- SKIP:", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeControllerLogFixture(t, dir, "capi-20261030_010203.log", tc.content)
			marker := filepath.Join(dir, "kubectl-called")
			// Use shell builtins only: PATH contains no real cluster tools.
			fake := "#!/bin/sh\n: > \"" + marker + "\"\nexit 99\n"
			if err := os.WriteFile(filepath.Join(dir, "kubectl"), []byte(fake), 0700); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(binary, "-test.run=^TestVerification_ControllerLogAnalysis$", "-test.v") // #nosec G204 -- current test binary, fixed arguments
			cmd.Env = []string{
				"PATH=" + dir,
				"CAPI_USER=tester", "INFRA_PROVIDER=aro",
				"CAPI_TEST_CONTEXT_FILE=" + filepath.Join(dir, "run-context.json"),
				"ARO_REPO_DIR=" + filepath.Join(dir, "installer"),
				"CONTROLLER_LOG_DIR=" + dir,
				"ANALYZE_CONTROLLER_LOGS=" + tc.enabled,
			}
			output, err := cmd.CombinedOutput()
			if (err != nil) != tc.wantFailure || !strings.Contains(string(output), tc.output) {
				t.Fatalf("error=%v, want failure=%v and output %q; got:\n%s", err, tc.wantFailure, tc.output, output)
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatalf("kubectl was invoked (marker stat: %v)", err)
			}
		})
	}
}
