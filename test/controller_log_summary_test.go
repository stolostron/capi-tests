package test

import (
	"fmt"
	"strings"
	"testing"
)

func TestControllerSummary_CountsAndGroups(t *testing.T) {
	var logs strings.Builder
	for i := 0; i < 35; i++ {
		fmt.Fprintf(&logs, "{\"level\":\"error\",\"ts\":%d,\"msg\":\"Reconciler error\",\"err\":\"connection to the workload cluster is down\"}\n", i)
	}
	logs.WriteString("ERROR permission denied\nERROR permission denied\n")
	logs.WriteString(strings.Repeat("level=warn msg=\"WaitingForOwner\"\n", 25))
	logs.WriteString("{\"level\":\"info\",\"msg\":\"WaitingForOwner\"}\n")
	logs.WriteString("{\"level\":\"info\",\"msg\":\"previous error: recovered\"}\n")
	summary := ParseControllerLogSummary(logs.String())
	summary.Name = "CAPI"
	if summary.ErrorCount != 37 || summary.WarnCount != 25 || summary.InfoCount != 1 {
		t.Fatalf("counts = %d errors, %d warnings, %d informational; want 37,25,1", summary.ErrorCount, summary.WarnCount, summary.InfoCount)
	}
	if len(summary.Errors) != MaxSampleMessages || len(summary.Warnings) != MaxSampleMessages {
		t.Errorf("sample limits not respected: %+v", summary)
	}
	if len(summary.Groups) != 3 {
		t.Fatalf("groups = %+v; want 3", summary.Groups)
	}
	rendered := FormatControllerLogSummaries([]ControllerLogSummary{summary})
	for _, want := range []string{"Potential issues (2 occurrences)", "Transient candidates (35 occurrences)", "Informational (26 occurrences)", "[35x]", "[26x]", "permission denied", "provisioning", "not proof"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("summary missing %q:\n%s", want, rendered)
		}
	}
	if strings.Index(rendered, "Potential issues") > strings.Index(rendered, "Transient candidates") {
		t.Error("potential issues must appear first")
	}
}

func TestControllerSummary_FormatsAndClassification(t *testing.T) {
	cases := []struct {
		name, logs, label, hint string
		errors, warns, infos    int
	}{
		{"klog grouping", "E1030 12:34:56.123456 123 controller.go:42] \"Reconciler error\" err=\"broken\"\nE1030 12:34:57.123456 456 controller.go:43] \"Reconciler error\" err=\"broken\"", "Potential issues", "[2x]", 2, 0, 0},
		{"logfmt grouping", "time=2026-10-30T12:34:56Z level=error msg=\"broken\"\ntime=2026-10-30T12:34:57Z level=error msg=\"broken\"", "Potential issues", "[2x]", 2, 0, 0},
		{"TLS startup candidate", "ERROR http: TLS handshake error: EOF", "Transient candidates", "startup", 1, 0, 0},
		{"TLS expired certificate", "ERROR TLS handshake error: x509: certificate has expired", "Potential issues", "expired", 1, 0, 0},
		{"owner informational", "I1030 12:34:56.123456 123 controller.go:42] WaitingForOwner", "Informational", "dependency", 0, 0, 1},
		{"fast retry", "warning: RetryClassification: RetryFast", "Transient candidates", "retry", 0, 1, 0},
		{"plain warning", "2026-10-30T12:34:56Z WARN unknown warning", "Potential issues", "unknown warning", 0, 1, 0},
		{"unknown warning", "{\"severity\":\"WARNING\",\"message\":\"unexpected configuration\"}", "Potential issues", "unexpected configuration", 0, 1, 0},
		{"unknown error", "ERROR Azure API rate limit exceeded", "Potential issues", "rate limit", 1, 0, 0},
		{"plain info mentioning error", "INFO recovered error: previous failure", "", "", 0, 0, 0},
		{"timestamped info mentioning error", "2026-10-30T12:34:56Z INFO recovered error: previous failure", "", "", 0, 0, 0},
		{"uppercase logfmt level", "level=ERROR msg=\"failed\"", "Potential issues", "failed", 1, 0, 0},
		{"informational severity", "{\"level\":\"info\",\"msg\":\"error: TLS handshake error recovered\"}", "", "", 0, 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			summary := ParseControllerLogSummary(tc.logs)
			if summary.ErrorCount != tc.errors || summary.WarnCount != tc.warns || summary.InfoCount != tc.infos {
				t.Errorf("unexpected counts: %+v", summary)
			}
			output := FormatControllerLogSummaries([]ControllerLogSummary{summary})
			for _, want := range []string{tc.label, tc.hint} {
				if !strings.Contains(output, want) {
					t.Errorf("summary missing %q: %s", want, output)
				}
			}
		})
	}
}

func TestControllerSummary_BoundedGroups(t *testing.T) {
	var logs strings.Builder
	for i := 0; i < 50; i++ {
		fmt.Fprintf(&logs, "ERROR distinct failure %d\n", i)
	}
	logs.WriteString("ERROR connection to the workload cluster is down\n")
	summary := ParseControllerLogSummary(logs.String())
	if summary.ErrorCount != 51 {
		t.Errorf("error count = %d; want 51", summary.ErrorCount)
	}
	if len(summary.Groups) > 3*MaxSampleMessages {
		t.Errorf("unbounded groups: %d", len(summary.Groups))
	}
	rendered := FormatControllerLogSummaries([]ControllerLogSummary{summary})
	for _, want := range []string{"Potential issues (50 occurrences)", "Transient candidates (1 occurrences)", "additional occurrences"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("missing %q: %s", want, rendered)
		}
	}
}

func TestControllerSummary_DoesNotWhitelistHeuristics(t *testing.T) {
	dir := t.TempDir()
	writeControllerLogFixture(t, dir, "capi-20261030_010203.log", "ERROR TLS handshake error: EOF\nERROR WaitingForOwner\nERROR connection to the workload cluster is down\n")
	report, err := AnalyzeSavedControllerLogs(dir, []ControllerDef{{DisplayName: "CAPI"}}, "")
	if err != nil {
		t.Fatal(err)
	}
	if report.UnexpectedCount != 3 {
		t.Errorf("summary heuristics changed unexpected-error gating: %+v", report)
	}
}
