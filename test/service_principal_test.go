package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateServicePrincipalExists(t *testing.T) {
	const clientID = "11111111-1111-1111-1111-111111111111"
	cases := []struct {
		name, output, status, want string
		missing                    bool
	}{
		{name: "exists", status: "0"},
		{name: "missing Graph resource", output: "ERROR: (Request_ResourceNotFound) Resource does not exist.", status: "3", want: "does not exist in Azure AD", missing: true},
		{name: "missing CLI principal", output: "ERROR: Service principal '" + clientID + "' doesn't exist", status: "3", want: "does not exist in Azure AD", missing: true},
		{name: "permission denied", output: "ERROR: Authorization_RequestDenied: Insufficient privileges", status: "1", want: "could not verify"},
		{name: "unauthenticated", output: "ERROR: Please run 'az login'", status: "1", want: "could not verify"},
		{name: "network failure", output: "ERROR: Failed to establish a new connection", status: "1", want: "could not verify"},
		{name: "unknown failure", output: "ERROR: unexpected failure", status: "1", want: "could not verify"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			binDir := t.TempDir()
			script := `#!/bin/sh
[ "$*" = "ad sp show --id ` + clientID + ` --output none" ] || exit 90
printf '%s\n' "$SP_TEST_OUTPUT"
exit "$SP_TEST_STATUS"
`
			if err := os.WriteFile(filepath.Join(binDir, "az"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", binDir)
			t.Setenv("SP_TEST_OUTPUT", tc.output)
			t.Setenv("SP_TEST_STATUS", tc.status)
			err := ValidateServicePrincipalExists(t, clientID)
			if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
			if got := strings.Contains(err.Error(), "az ad sp create-for-rbac"); got != tc.missing {
				t.Fatalf("recreation advice present = %v, want %v: %v", got, tc.missing, err)
			}
			if !tc.missing && strings.Contains(err.Error(), "does not exist") {
				t.Fatalf("unconfirmed absence reported: %v", err)
			}
		})
	}
}

// Run the real phase test in a subprocess so expected failures do not fail this test.
func TestAzureAuthenticationServicePrincipalPreflight(t *testing.T) {
	cases := []struct {
		name, clientID, secret, tenant, skipLogin, lookupStatus, wantCalls string
		wantFailure                                                        bool
	}{
		{name: "SP login before lookup", clientID: "client", secret: "secret", tenant: "tenant", lookupStatus: "0", wantCalls: "login\nad sp show\n"},
		{name: "lookup failure blocks phase", clientID: "client", secret: "secret", tenant: "tenant", lookupStatus: "1", wantCalls: "login\nad sp show\n", wantFailure: true},
		{name: "existing session avoids login", clientID: "client", secret: "secret", tenant: "tenant", skipLogin: "true", lookupStatus: "0", wantCalls: "account show\nad sp show\n"},
		{name: "CLI mode avoids lookup", lookupStatus: "1", wantCalls: "account show\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			binDir := t.TempDir()
			calls := filepath.Join(binDir, "calls")
			script := `#!/bin/sh
case "$1 $2 $3" in
    "login --service-principal -u") printf 'login\n' >> "$SP_TEST_CALLS" ;;
    "account show ") printf 'account show\n' >> "$SP_TEST_CALLS" ;;
    "ad sp show") printf 'ad sp show\n' >> "$SP_TEST_CALLS"; exit "$SP_TEST_STATUS" ;;
    *) exit 90 ;;
esac
`
			if err := os.WriteFile(filepath.Join(binDir, "az"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", binDir)
			t.Setenv("SP_TEST_CALLS", calls)
			t.Setenv("SP_TEST_STATUS", tc.lookupStatus)
			t.Setenv("AZURE_CLIENT_ID", tc.clientID)
			t.Setenv("AZURE_CLIENT_SECRET", tc.secret)
			t.Setenv("AZURE_TENANT_ID", tc.tenant)
			t.Setenv("SKIP_AZURE_LOGIN", tc.skipLogin)
			t.Setenv("CI", "false")
			t.Setenv("GITHUB_ACTIONS", "false")
			t.Setenv("INFRA_PROVIDER", "aro")
			t.Setenv("CAPI_TEST_CONTEXT_FILE", filepath.Join(binDir, "context.json"))
			t.Setenv("ARO_REPO_DIR", filepath.Join(binDir, "installer"))
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(executable, "-test.run=^TestCheckDependencies_AzureAuthentication$", "-test.v")
			output, err := cmd.CombinedOutput()
			if (err != nil) != tc.wantFailure {
				t.Fatalf("phase error = %v, want failure %v: %s", err, tc.wantFailure, output)
			}
			got, err := os.ReadFile(calls)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.wantCalls {
				t.Errorf("calls = %q, want %q", got, tc.wantCalls)
			}
		})
	}
}
