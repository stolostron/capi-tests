package test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const versionCatalog = `{"value":[
 {"name":"4.20.17","properties":{"enabled":true,"channelGroup":"stable"}},
 {"name":"4.19.17","properties":{"enabled":true,"channelGroup":"stable"}},
 {"name":"4.21.1","properties":{"enabled":false,"channelGroup":"stable"}},
 {"name":"4.22.0","properties":{"enabled":true,"channelGroup":"candidate"}}
]}`

func versionPreflightConfig() *TestConfig {
	return &TestConfig{InfraProviders: []InfraProvider{{Name: "aro"}}, Region: "uksouth", AzureSubscriptionID: "test-subscription", OCPVersion: "4.20", OCPVersionMP: "4.20.17", AROVersionAPIVersion: "2026-09-01-preview", OpenShiftVersionCheckTimeout: "1s"}
}

func fakeVersionAZ(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	log := filepath.Join(dir, "calls")
	script := `#!/bin/sh
printf '%s\n' "$*" >> "$VERSION_CALLS"
case "$1 $2" in
 "cloud show") printf 'https://management.azure.com/' ;;
 "account show") printf 'active-subscription' ;;
 "rest --method")
  if [ -n "$VERSION_FAILURE" ]; then printf '%s' "$VERSION_FAILURE" >&2; exit 1; fi
  if [ "$VERSION_SLEEP" = yes ]; then exec sleep 30; fi
  case "$*" in
   *page=2*) printf '%s' "$VERSION_PAGE2" ;;
   *) printf '%s' "$VERSION_CATALOG" ;;
  esac ;;
 *) exit 97 ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "az"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("VERSION_CALLS", log)
	t.Setenv("VERSION_CATALOG", versionCatalog)
	t.Setenv("VERSION_PAGE2", versionCatalog)
	t.Setenv("VERSION_FAILURE", "")
	t.Setenv("VERSION_SLEEP", "")
	return log
}

func TestOpenShiftVersionPreflight(t *testing.T) {
	for _, tc := range []struct{ name, cp, mp, want string }{
		{"minor selector", "4.20", "4.20.17", ""},
		{"full patch", "4.20.17", "4.20.17", ""},
		{"independent worker version", "4.20", "4.19.17", ""},
		{"unavailable control plane", "4.21", "4.20.17", "OCP_VERSION=4.21"},
		{"unavailable worker", "4.20", "4.20.18", "OCP_VERSION_MP=4.20.18"},
		{"candidate is not stable", "4.22", "4.20.17", "OCP_VERSION=4.22"},
		{"minor boundary", "4.2", "4.20.17", "OCP_VERSION=4.2"},
		{"invalid selector", "4.20.x", "4.20.17", "invalid OCP_VERSION"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			log := fakeVersionAZ(t)
			c := versionPreflightConfig()
			c.OCPVersion = tc.cp
			c.OCPVersionMP = tc.mp
			err := ValidateOpenShiftVersionAvailability(context.Background(), t, c)
			if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else {
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("error = %v, want %q", err, tc.want)
				}
				if !strings.Contains(tc.want, "invalid") && (!strings.Contains(err.Error(), "uksouth") || !strings.Contains(err.Error(), "4.20.17")) {
					t.Fatalf("missing region or available versions: %v", err)
				}
			}
			calls, _ := os.ReadFile(log)
			if strings.Contains(string(calls), "aro get-versions") {
				t.Fatal("queried classic ARO")
			}
			if tc.want == "" && !strings.Contains(string(calls), "/locations/uksouth/hcpOpenShiftVersions?api-version=2026-09-01-preview") {
				t.Fatalf("incorrect HCP API: %s", calls)
			}
		})
	}
}

func TestOpenShiftVersionPreflightCatalogFailures(t *testing.T) {
	for _, tc := range []struct{ name, body, failure, want string }{
		{"authentication", "", "AuthorizationFailed token=do-not-expose", "catalog query failed"},
		{"malformed", "not-json", "", "decode"},
		{"wrong schema", "{}", "", "missing value"},
		{"empty", "{\"value\":[]}", "", "no enabled stable"},
		{"incomplete record", "{\"value\":[{\"name\":\"4.20.17\"}]}", "", "no enabled stable"},
		{"foreign nextLink", `{"value":[],"nextLink":"https://other.example/catalog"}`, "", "nextLink"},
		{"repeated nextLink", `{"value":[],"nextLink":"https://management.azure.com/subscriptions/test-subscription/providers/Microsoft.RedHatOpenShift/locations/uksouth/hcpOpenShiftVersions?api-version=2026-09-01-preview"}`, "", "pagination"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fakeVersionAZ(t)
			t.Setenv("VERSION_CATALOG", tc.body)
			t.Setenv("VERSION_FAILURE", tc.failure)
			err := ValidateOpenShiftVersionAvailability(context.Background(), t, versionPreflightConfig())
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
			if tc.failure != "" && strings.Contains(err.Error(), "do-not-expose") {
				t.Fatalf("raw CLI failure leaked into error: %v", err)
			}
			if tc.failure != "" && strings.Contains(err.Error(), "not available") {
				t.Fatalf("query failure misreported as unsupported: %v", err)
			}
		})
	}
}

func TestOpenShiftVersionPreflightPaginationAndSubscription(t *testing.T) {
	log := fakeVersionAZ(t)
	t.Setenv("VERSION_CATALOG", `{"value":[],"nextLink":"https://management.azure.com/subscriptions/active-subscription/providers/Microsoft.RedHatOpenShift/locations/uksouth/hcpOpenShiftVersions?api-version=2026-09-01-preview&page=2"}`)
	c := versionPreflightConfig()
	c.AzureSubscriptionID = ""
	if err := ValidateOpenShiftVersionAvailability(context.Background(), t, c); err != nil {
		t.Fatal(err)
	}
	calls, _ := os.ReadFile(log)
	if !strings.Contains(string(calls), "account show") || !strings.Contains(string(calls), "page=2") {
		t.Fatalf("missing fallback or second page: %s", calls)
	}
}

func TestOpenShiftVersionPreflightTimeout(t *testing.T) {
	fakeVersionAZ(t)
	t.Setenv("VERSION_SLEEP", "yes")
	start := time.Now()
	err := ValidateOpenShiftVersionAvailability(context.Background(), t, versionPreflightConfig())
	if err == nil || !strings.Contains(err.Error(), "deadline") {
		t.Fatalf("expected bounded timeout, got %v", err)
	}
	if time.Since(start) > 4*time.Second {
		t.Fatal("catalog command exceeded deadline")
	}
}

func TestOpenShiftVersionPreflightROSA(t *testing.T) {
	log := fakeVersionAZ(t)
	c := versionPreflightConfig()
	c.InfraProviders = []InfraProvider{{Name: "rosa"}}
	if err := ValidateOpenShiftVersionAvailability(context.Background(), t, c); err != nil {
		t.Fatal(err)
	}
	if calls, _ := os.ReadFile(log); len(calls) > 0 {
		t.Fatalf("ROSA invoked Azure: %s", calls)
	}
}

// Run the real phase in a child process so its Fatalf can be asserted without
// marking this regression test failed or reusing the parent's cached run state.
func TestOpenShiftVersionPreflightBlocksGeneration(t *testing.T) {
	fakeVersionAZ(t)
	dir := t.TempDir()
	installer := filepath.Join(dir, "installer")
	scripts := filepath.Join(installer, "scripts", "aro-hcp")
	if err := os.MkdirAll(scripts, 0700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(dir, "gen-called")
	if err := os.WriteFile(filepath.Join(scripts, "gen.sh"), []byte("#!/bin/sh\ntouch \"$GEN_MARKER\"\nexit 99\n"), 0700); err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]string{
		"ARO_REPO_DIR": installer, "INFRA_PROVIDER": "aro", "CLUSTER_MODE": "kind",
		"USE_KUBECONFIG": "", "KUBECONFIG": filepath.Join(dir, "unused-kubeconfig"),
		"CAPI_TEST_CONTEXT_FILE": filepath.Join(dir, "context.json"), "CAPI_TEST_WORKSPACE": dir,
		"CAPI_USER": "test", "DEPLOYMENT_ENV": "test", "REGION": "uksouth",
		"OCP_VERSION": "4.21", "OCP_VERSION_MP": "4.20.17", "AZURE_SUBSCRIPTION_ID": "test-subscription",
		"ARO_VERSION_API_VERSION": "2026-09-01-preview", "OPENSHIFT_VERSION_CHECK_TIMEOUT": "1s", "GEN_MARKER": marker,
	} {
		t.Setenv(key, value)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestInfrastructure_GenerateResources$", "-test.v")
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatal(ctx.Err())
	}
	if err == nil || !strings.Contains(string(output), "OCP_VERSION=4.21 is not available") || !strings.Contains(string(output), "4.20.17") {
		t.Fatalf("phase did not fail on unavailable version: %v\n%s", err, output)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("generation ran before version validation: %v", err)
	}
}
