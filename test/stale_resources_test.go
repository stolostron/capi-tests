package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// Exercise the real detector, replacing only Azure's network boundary. Unexpected
// commands fail so these checks cannot authenticate or mutate cloud resources.
const staleAzureCLI = `#!/usr/bin/env bash
set -euo pipefail
case "$1 $2" in
  "account show")
    if [[ "${AZ_ACCOUNT_JSON:-}" != "" ]]; then echo "$AZ_ACCOUNT_JSON"; exit 0; fi
    if [[ "$*" == *"--query id"* ]]; then echo subscription-test; else echo '{"id":"subscription-test"}'; fi ;;
  "account set"|"extension show") exit 0 ;;
  "group list")
    if [[ "${AZ_LIST_ERROR:-}" != "" ]]; then echo "$AZ_LIST_ERROR" >&2; exit 1; fi
    if [[ "${AZ_GROUPS_JSON:-}" != "" ]]; then echo "$AZ_GROUPS_JSON"; exit 0; fi
    if [[ "$*" == *"createdAt:"* ]]; then
      jq '[.groups[] | select(.tags."capi-test-created-at" != null) | {name,location,createdAt:.tags."capi-test-created-at"}]' "$AZ_FIXTURE"
    else jq '.groups' "$AZ_FIXTURE"; fi ;;
  "group show")
    if [[ "${AZ_SHOW_ERROR:-}" != "" || -f "$AZ_DELETED" ]]; then
      echo "${AZ_SHOW_ERROR:-(ResourceGroupNotFound) The resource group could not be found.}" >&2; exit 3
    fi
    jq --arg state "${AZ_FINAL_STATE:-}" --arg created "${AZ_FINAL_CREATED_AT:-}" '
      .groups[0] | if $state != "" then .properties.provisioningState = $state else . end |
      if $created != "" then .tags."capi-test-created-at" = $created else . end' "$AZ_FIXTURE" ;;
  "resource list")
    if [[ "${AZ_RESOURCE_EXIT_ONLY:-}" == "true" ]]; then exit 1; fi
    if [[ "${AZ_RESOURCE_ERROR:-}" != "" ]]; then echo "$AZ_RESOURCE_ERROR" >&2; exit 1; fi
    if [[ "${AZ_DISAPPEAR_ON_LOOKUP:-}" == "true" ]]; then
      touch "$AZ_DELETED"
      echo '(ResourceGroupNotFound) The resource group could not be found.' >&2
      exit 3
    fi
    if [[ "${AZ_RESOURCES_JSON:-}" != "" ]]; then echo "$AZ_RESOURCES_JSON"; else jq '.resources // []' "$AZ_FIXTURE"; fi ;;
  "monitor activity-log")
    if [[ "${AZ_ACTIVITY_ERROR:-}" != "" ]]; then echo "$AZ_ACTIVITY_ERROR" >&2; exit 1; fi
    if [[ "$*" == *"-o json"* ]]; then
      jq '.createdAt | if . == "" then null else . end' "$AZ_FIXTURE"
    else jq -r '.createdAt // empty' "$AZ_FIXTURE"; fi ;;
  "ad app"|"ad sp") echo "${AZ_AD_JSON:-[]}" ;;
  "tag update") exit 0 ;;
  *) echo "Unexpected Azure operation: $*" >&2; exit 99 ;;
esac
`

func runStaleDetector(t *testing.T, tagged bool, state, created string, env map[string]string, jsonOutput bool) (int, string, string) {
	t.Helper()
	for _, tool := range []string{"bash", "jq"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s required for detector integration tests", tool)
		}
	}
	dir := t.TempDir()
	tags := map[string]string{}
	if tagged {
		tags["capi-test-created-at"] = created
	}
	fixture := map[string]any{
		"groups": []any{map[string]any{
			"name": "capz-tests-abc12-resgroup", "location": "uksouth", "tags": tags,
			"properties": map[string]string{"provisioningState": state},
		}},
		"createdAt": created,
	}
	data, err := json.Marshal(fixture)
	if err != nil {
		t.Fatal(err)
	}
	fixturePath := filepath.Join(dir, "fixture.json")
	if err := os.WriteFile(fixturePath, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "az"), []byte(staleAzureCLI), 0700); err != nil {
		t.Fatal(err)
	}
	args := []string{"../scripts/check-stale-resources.sh", "--max-age", "24"}
	if jsonOutput {
		args = append(args, "--json")
	}
	cmd := exec.Command("bash", args...)
	cmd.Env = append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"AZURE_SUBSCRIPTION_ID=subscription-test", "AZ_FIXTURE="+fixturePath, "AZ_DELETED="+filepath.Join(dir, "deleted"))
	for key, value := range env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	code := 0
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		} else {
			t.Fatal(err)
		}
	}
	return code, stdout.String(), stderr.String()
}

func staleWorkflowStep(t *testing.T, job, name string) string {
	t.Helper()
	data, err := os.ReadFile("../.github/workflows/stale-resources.yml")
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Jobs map[string]struct {
			Steps []struct{ Name, Run string }
		}
	}
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		t.Fatal(err)
	}
	for _, step := range workflow.Jobs[job].Steps {
		if step.Name == name {
			return step.Run
		}
	}
	t.Fatalf("missing workflow step %q in %q", name, job)
	return ""
}

func TestStaleResourceWorkflowReportsInformationalFindingsAndErrors(t *testing.T) {
	for _, tt := range []struct {
		name, category, heading string
		code                    int
	}{
		{"unknown age", "unknown_age_resource_groups", "Unknown-age resource groups", 0},
		{"deleting", "deleting_resource_groups", "Resource groups being deleted", 0},
		{"disappeared", "disappeared_resource_groups", "Resource groups that disappeared during the scan", 0},
		{"lookup error", "", "Scan errors", 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			report := map[string]any{
				"stale_found": false, "total_count": 0, "scan_complete": tt.code != 2,
				"observed_at": "2026-10-26T06:07:00Z", "subscription_id": "subscription-test",
				"summary": "No confirmed stale resources", "errors": []any{},
			}
			azure := map[string]any{"resource_groups": []any{}, "ad_applications": []any{}, "service_principals": []any{}}
			for _, category := range []string{"unknown_age_resource_groups", "deleting_resource_groups", "disappeared_resource_groups"} {
				azure[category] = []any{}
			}
			if tt.category != "" {
				azure[tt.category] = []any{map[string]any{"name": "example-group", "createdAt": "unknown", "provisioningState": "Deleting"}}
			} else {
				report["errors"] = []any{map[string]any{"context": "Revalidate example-group", "message": "AuthorizationFailed"}}
			}
			report["azure"] = azure
			data, err := json.Marshal(report)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(filepath.Join(dir, "scripts"), 0700); err != nil {
				t.Fatal(err)
			}
			// Supply the detector's output at the workflow boundary, then execute
			// the actual check and summary step bodies from the YAML.
			script := fmt.Sprintf("#!/bin/bash\ncat <<'JSON'\n%s\nJSON\nexit %d\n", data, tt.code)
			if err := os.WriteFile(filepath.Join(dir, "scripts", "check-stale-resources.sh"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			outputPath := filepath.Join(dir, "outputs")
			cmd := exec.Command("bash", "-e", "-c", staleWorkflowStep(t, "check-azure", "Check for stale Azure resources"))
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "MAX_AGE_HOURS=24", "GITHUB_OUTPUT="+outputPath)
			output, err := cmd.CombinedOutput()
			if (err != nil) != (tt.code == 2) {
				t.Fatalf("unexpected step result: %v; output=%s", err, output)
			}
			outputs, err := os.ReadFile(outputPath)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(outputs), "report_found=true") || !strings.Contains(string(outputs), "stale_found=false") {
				t.Fatalf("informational/error report not passed to report job: %s", outputs)
			}
			if _, err := os.Stat(filepath.Join(dir, "azure-report.json")); err != nil {
				t.Fatalf("report artifact missing: %v", err)
			}
			summaryPath := filepath.Join(dir, "summary")
			cmd = exec.Command("bash", "-e", "-c", staleWorkflowStep(t, "report", "Generate step summary"))
			cmd.Env = append(os.Environ(), "MAX_AGE_HOURS=24", "GITHUB_STEP_SUMMARY="+summaryPath,
				"AZURE_REPORT="+string(data), "AZURE_SUMMARY=No confirmed stale resources", "AZURE_STALE=false")
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("summary failed: %v; output=%s", err, output)
			}
			summary, err := os.ReadFile(summaryPath)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{tt.heading, "example-group", "2026-10-26T06:07:00Z", "subscription-test"} {
				if !strings.Contains(string(summary), want) {
					t.Errorf("summary missing %q: %s", want, summary)
				}
			}
		})
	}
}

func TestStaleResourceDetection(t *testing.T) {
	old := time.Now().UTC().Add(-72 * time.Hour).Format(time.RFC3339)
	recent := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	tests := []struct {
		name, state, created, category string
		tagged                         bool
		env                            map[string]string
		code, stale                    int
	}{
		{name: "confirmed stale", tagged: true, state: "Succeeded", created: old, code: 1, stale: 1},
		{name: "recent", tagged: true, state: "Succeeded", created: recent},
		{name: "convention stale", state: "Succeeded", created: old, code: 1, stale: 1},
		{name: "convention recent", state: "Succeeded", created: recent},
		{name: "unknown empty group", state: "Succeeded", category: "unknown_age_resource_groups"},
		{name: "unknown nonempty group", state: "Succeeded", env: map[string]string{"AZ_RESOURCES_JSON": `[{"name":"vnet","createdTime":null}]`}, category: "unknown_age_resource_groups"},
		{name: "invalid ownership timestamp", tagged: true, state: "Succeeded", created: "not-a-date", category: "unknown_age_resource_groups"},
		{name: "deleting", tagged: true, state: "Deleting", created: old, category: "deleting_resource_groups"},
		{name: "deletion starts during scan", state: "Succeeded", created: old, env: map[string]string{"AZ_FINAL_STATE": "Deleting"}, category: "deleting_resource_groups"},
		{name: "ownership timestamp changes", tagged: true, state: "Succeeded", created: old, env: map[string]string{"AZ_FINAL_CREATED_AT": recent}},
		{name: "tagged disappeared", tagged: true, created: old, env: map[string]string{"AZ_SHOW_ERROR": "(ResourceGroupNotFound) Gone"}, category: "disappeared_resource_groups"},
		{name: "disappeared during resource lookup", state: "Succeeded", env: map[string]string{"AZ_DISAPPEAR_ON_LOOKUP": "true"}, category: "disappeared_resource_groups"},
		{name: "forbidden group lookup", tagged: true, created: old, env: map[string]string{"AZ_SHOW_ERROR": "(AuthorizationFailed) Forbidden"}, code: 2},
		{name: "throttled resource lookup", state: "Succeeded", env: map[string]string{"AZ_RESOURCE_ERROR": "(TooManyRequests) Throttled"}, code: 2},
		{name: "resource lookup fails without stderr", state: "Succeeded", env: map[string]string{"AZ_RESOURCE_EXIT_ONLY": "true"}, code: 2},
		{name: "failed activity lookup", state: "Succeeded", env: map[string]string{"AZ_ACTIVITY_ERROR": "(AuthorizationFailed) Forbidden"}, code: 2},
		{name: "invalid activity timestamp", state: "Succeeded", created: "invalid response", code: 2},
		{name: "failed discovery", env: map[string]string{"AZ_LIST_ERROR": "(AuthenticationFailed) Expired"}, code: 2},
		{name: "invalid account response", env: map[string]string{"AZ_ACCOUNT_JSON": "not-json"}, code: 2},
		{name: "invalid resources response", state: "Succeeded", env: map[string]string{"AZ_RESOURCES_JSON": "{}"}, code: 2},
		{name: "invalid discovery member", env: map[string]string{"AZ_GROUPS_JSON": `[{"name":null}]`}, code: 2},
		{name: "invalid resource member", state: "Succeeded", env: map[string]string{"AZ_RESOURCES_JSON": `["not-an-object"]`}, code: 2},
		{name: "invalid directory response", tagged: true, state: "Succeeded", created: recent, env: map[string]string{"AZ_AD_JSON": "null"}, code: 2},
		{name: "invalid directory member", tagged: true, state: "Succeeded", created: recent, env: map[string]string{"AZ_AD_JSON": `[{"appId":"app","displayName":"app","tags":null}]`}, code: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, jsonOutput := range []bool{true, false} {
				t.Run(fmt.Sprintf("json=%t", jsonOutput), func(t *testing.T) {
					code, output, stderr := runStaleDetector(t, tt.tagged, tt.state, tt.created, tt.env, jsonOutput)
					if code != tt.code {
						t.Fatalf("exit=%d, want %d; stdout=%s; stderr=%s", code, tt.code, output, stderr)
					}
					if !jsonOutput {
						if !strings.Contains(output, "subscription-test") || !strings.Contains(output, "Observed at:") {
							t.Errorf("missing scan context: %s", output)
						}
						if tt.category != "" && !strings.Contains(output, "capz-tests-abc12-resgroup") {
							t.Errorf("missing classified group: %s", output)
						}
						if tt.code == 2 && strings.Contains(output, "No stale resources found") {
							t.Errorf("incomplete scan reported clean: %s", output)
						}
						if tt.category == "unknown_age_resource_groups" && strings.Contains(output, "older than 24h") {
							t.Errorf("unknown age claimed above threshold: %s", output)
						}
						return
					}
					var report struct {
						StaleFound   bool                       `json:"stale_found"`
						ScanComplete bool                       `json:"scan_complete"`
						Total        int                        `json:"total_count"`
						ObservedAt   string                     `json:"observed_at"`
						Subscription string                     `json:"subscription_id"`
						Errors       []json.RawMessage          `json:"errors"`
						Azure        map[string]json.RawMessage `json:"azure"`
					}
					if err := json.Unmarshal([]byte(output), &report); err != nil {
						t.Fatalf("invalid report: %v; stdout=%s; stderr=%s", err, output, stderr)
					}
					if report.StaleFound != (tt.stale > 0) || report.Total != tt.stale || report.ScanComplete != (tt.code != 2) {
						t.Errorf("wrong finding counts or completion: %s", output)
					}
					if _, err := time.Parse(time.RFC3339, report.ObservedAt); err != nil || report.Subscription != "subscription-test" {
						t.Errorf("missing scan context: %s", output)
					}
					if tt.code == 2 && len(report.Errors) == 0 {
						t.Errorf("missing lookup error: %s", output)
					}
					if tt.category != "" {
						var groups []struct{ Name string }
						if err := json.Unmarshal(report.Azure[tt.category], &groups); err != nil || len(groups) != 1 || groups[0].Name != "capz-tests-abc12-resgroup" {
							t.Errorf("wrong classification %s: %s", tt.category, output)
						}
					}
				})
			}
		})
	}
}
