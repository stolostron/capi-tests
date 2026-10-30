package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalDefaultsStartup(t *testing.T) {
	if os.Getenv("CAPI_LOCAL_DEFAULTS_CHILD") == "1" {
		if got := GetEnvOrDefault("REGION", "builtin"); got != os.Getenv("CAPI_EXPECT_REGION") {
			t.Fatalf("region = %q, want %q", got, os.Getenv("CAPI_EXPECT_REGION"))
		}
		if got := os.Getenv("USE_KIND"); got != "true" {
			t.Fatalf("direct environment reader got USE_KIND=%q, want true", got)
		}
		return
	}
	for _, tc := range []struct {
		name, explicit, expected string
		set                      bool
	}{
		{name: "local defaults", expected: "local-region"},
		{name: "environment wins", explicit: "environment-region", expected: "environment-region", set: true},
		{name: "explicit empty suppresses local default", expected: "builtin", set: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, ".capi-test-defaults.json")
			if err := os.WriteFile(path, []byte(`{"REGION":"local-region","USE_KIND":"true"}`), 0600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(os.Args[0], "-test.run=^TestLocalDefaultsStartup$", "-test.count=1")
			cmd.Env = []string{"CAPI_LOCAL_DEFAULTS_CHILD=1", "CAPI_EXPECT_REGION=" + tc.expected, "CAPI_TEST_WORKSPACE=" + dir}
			if tc.set {
				cmd.Env = append(cmd.Env, "REGION="+tc.explicit)
			}
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("child failed: %v\n%s", err, output)
			}
		})
	}
}

func TestLocalDefaultsStartupErrors(t *testing.T) {
	for _, tc := range []struct {
		name, data, path, want string
		implicit, disabled     bool
	}{
		{name: "missing explicit file", path: "missing.json", want: "read local defaults"},
		{name: "unreadable file", path: ".", want: "read local defaults"},
		{name: "malformed", data: `{"SECRET":"private-value",`, want: "must be a JSON object"},
		{name: "array", data: `[]`, want: "must be a JSON object"},
		{name: "null object", data: `null`, want: "must be a JSON object"},
		{name: "numeric value", data: `{"USE_KIND":42}`, want: "USE_KIND must be a string"},
		{name: "null value", data: `{"USE_KIND":null}`, want: "USE_KIND must be a string"},
		{name: "invalid name", data: `{"BAD=NAME":"private-value"}`, want: "invalid environment name"},
		{name: "nul value", data: `{"USE_KIND":"\u0000"}`, want: "USE_KIND must be a string"},
		{name: "disabled ignores malformed default", data: `invalid`, disabled: true},
		{name: "absent implicit file", implicit: true},
		{name: "valid relative file", data: `{"REGION":"literal-$(touch nope)-$HOME"}`, path: localDefaultsFileName},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, localDefaultsFileName)
			if tc.data != "" {
				if err := os.WriteFile(path, []byte(tc.data), 0600); err != nil {
					t.Fatal(err)
				}
			}
			cmd := exec.Command(os.Args[0], "-test.run=^$", "-test.count=1")
			cmd.Env = []string{"CAPI_TEST_WORKSPACE=" + dir}
			switch {
			case tc.disabled:
				cmd.Env = append(cmd.Env, "CAPI_TEST_DEFAULTS_FILE=")
			case tc.implicit:
			default:
				selected := tc.path
				if selected == "" {
					selected = path
				}
				cmd.Env = append(cmd.Env, "CAPI_TEST_DEFAULTS_FILE="+selected)
			}
			output, err := cmd.CombinedOutput()
			if tc.want == "" {
				if err != nil {
					t.Fatalf("unexpected failure: %v\n%s", err, output)
				}
			} else if err == nil || !strings.Contains(string(output), tc.want) {
				t.Fatalf("error=%v output=%s, want %q", err, output, tc.want)
			}
			if strings.Contains(string(output), "private-value") {
				t.Fatal("error leaked a configuration value")
			}
		})
	}
}

func TestLocalDefaultsConfigAndContext(t *testing.T) {
	if os.Getenv("CAPI_LOCAL_DEFAULTS_CONFIG_CHILD") == "1" {
		if os.Getenv("CAPI_EXPECT_CONFLICT") == "1" {
			if _, err := EnsureRunContext(); err == nil || !strings.Contains(err.Error(), "INFRA_PROVIDER") {
				t.Fatalf("expected immutable provider conflict, got %v", err)
			}
			return
		}
		config := NewTestConfig()
		if configError != nil {
			t.Fatal(*configError)
		}
		if config.InfraProviderName != "rosa" || config.Region != "eu-west-1" || !config.UseKind || config.Environment != "dev" || config.CAPIUser != "alice" || config.ClusterDeploymentTimeout.String() != "3h0m0s" {
			t.Fatalf("local defaults not reflected in config: provider=%s region=%s kind=%t env=%s user=%s timeout=%s", config.InfraProviderName, config.Region, config.UseKind, config.Environment, config.CAPIUser, config.ClusterDeploymentTimeout)
		}
		if os.Getenv("NAME_PREFIX") != "literal-$(touch nope)-$HOME" {
			t.Fatal("local value was expanded")
		}
		return
	}
	dir := t.TempDir()
	path := filepath.Join(dir, localDefaultsFileName)
	data := `{"INFRA_PROVIDER":"rosa","AWS_REGION":"eu-west-1","USE_KIND":"true","CAPI_USER":"alice","DEPLOYMENT_ENV":"dev","CLUSTER_DEPLOYMENT_TIMEOUT":"180m","NAME_PREFIX":"literal-$(touch nope)-$HOME"}`
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"initial", "environment conflict", "local conflict"} {
		cmd := exec.Command(os.Args[0], "-test.run=^TestLocalDefaultsConfigAndContext$", "-test.count=1")
		cmd.Env = []string{"CAPI_TEST_DEFAULTS_FILE=" + path, "CAPI_TEST_WORKSPACE=" + dir, "CAPI_LOCAL_DEFAULTS_CONFIG_CHILD=1", "ARO_REPO_DIR=" + dir}
		if mode == "environment conflict" {
			cmd.Env = append(cmd.Env, "INFRA_PROVIDER=aro", "CAPI_EXPECT_CONFLICT=1")
		}
		if mode == "local conflict" {
			if err := os.WriteFile(path, []byte(strings.Replace(data, `"rosa"`, `"aro"`, 1)), 0600); err != nil {
				t.Fatal(err)
			}
			cmd.Env = append(cmd.Env, "CAPI_EXPECT_CONFLICT=1")
		}
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s child failed: %v\n%s", mode, err, output)
		}
	}
}
