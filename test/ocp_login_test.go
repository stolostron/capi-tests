package test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func isolateOCPConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, key := range []string{"USE_KUBECONFIG", "KUBECONFIG", "CLUSTER_MODE", "USE_KIND", "USE_K8S", "USE_MCE", "DEPLOY_CHARTS", "OCP_USERNAME", "OCP_PASSWORD", "OCP_SERVER", "OCP_LOGIN_TIMEOUT"} {
		t.Setenv(key, "")
	}
	t.Setenv("CAPI_TEST_CONTEXT_FILE", filepath.Join(dir, "run.json"))
	t.Setenv("ARO_REPO_DIR", filepath.Join(dir, "installer"))
	savedMCE := mceGeneratedKubeconfig
	mceGeneratedKubeconfig = ""
	t.Cleanup(func() { mceGeneratedKubeconfig = savedMCE })
	saved := configError
	configError = nil
	t.Cleanup(func() { configError = saved })
	return dir
}

func TestOCPLoginSourceSelection(t *testing.T) {
	for _, tc := range []struct {
		name, explicit, user, password, server, mode string
		external, wantError                          bool
	}{
		{name: "unset"},
		{name: "complete", user: "alice", password: "secret", server: "https://api.example.test:6443", external: true},
		{name: "missing username", password: "secret", server: "https://api.example.test", external: true, wantError: true},
		{name: "missing password", user: "alice", server: "https://api.example.test", external: true, wantError: true},
		{name: "space in password", user: "alice", password: "secret password", server: "https://api.example.test", external: true, wantError: true},
		{name: "newline password", user: "alice", password: "secret\n", server: "https://api.example.test", external: true, wantError: true},
		{name: "missing server", user: "alice", password: "secret", external: true, wantError: true},
		{name: "explicit file overrides partial credentials", explicit: "/explicit/kubeconfig", password: "secret", external: true},
		{name: "explicit file overrides complete credentials", explicit: "/explicit/kubeconfig", user: "alice", password: "secret", server: "https://api.example.test", external: true},
		{name: "ocp overrides kind mode", user: "alice", password: "secret", server: "https://api.example.test", mode: "kind", external: true},
		{name: "ocp with mce", user: "alice", password: "secret", server: "https://api.example.test", mode: "mce", external: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isolateOCPConfig(t)
			t.Setenv("USE_KUBECONFIG", tc.explicit)
			t.Setenv("OCP_USERNAME", tc.user)
			t.Setenv("OCP_PASSWORD", tc.password)
			t.Setenv("OCP_SERVER", tc.server)
			t.Setenv("CLUSTER_MODE", tc.mode)
			c := NewTestConfig()
			if c.IsExternalCluster() != tc.external {
				t.Errorf("external = %v, want %v", c.IsExternalCluster(), tc.external)
			}
			if (configError != nil) != tc.wantError {
				t.Fatalf("configError = %v, want error %v", configError, tc.wantError)
			}
			if tc.wantError && strings.Contains(*configError, "secret") {
				t.Fatal("configuration error leaked password")
			}
			if tc.explicit != "" && c.UseKubeconfig != tc.explicit {
				t.Errorf("path = %q, want explicit %q", c.UseKubeconfig, tc.explicit)
			}
			if tc.external {
				if c.mceLogin {
					t.Error("MCE authentication must not overwrite the selected source")
				}
			}
			if tc.external && c.UseKind {
				t.Error("external source must disable Kind")
			}
			if tc.external && tc.explicit == "" {
				if c.UseKubeconfig == "" {
					t.Fatal("OCP source has no isolated kubeconfig path")
				}
				if next := NewTestConfig(); next.UseKubeconfig != c.UseKubeconfig {
					t.Error("path changed between constructor calls")
				}
			}
		})
	}
}

func TestOCPLoginAuthentication(t *testing.T) {
	for _, mode := range []string{"success", "login failure", "empty config", "invalid context", "missing oc"} {
		t.Run(mode, func(t *testing.T) {
			dir := isolateOCPConfig(t)
			t.Setenv("OCP_USERNAME", "alice")
			t.Setenv("OCP_PASSWORD", "secret-password")
			t.Setenv("OCP_SERVER", "https://api.example.test:6443")
			t.Setenv("HOME", dir)
			homeConfig := filepath.Join(dir, ".kube", "config")
			if err := os.MkdirAll(filepath.Dir(homeConfig), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(homeConfig, []byte("home-sentinel"), 0600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("KUBECONFIG", homeConfig)
			bin := filepath.Join(dir, "bin")
			if err := os.Mkdir(bin, 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin)
			t.Setenv("FAKE_OCP_MODE", mode)
			t.Setenv("FAKE_OCP_RECORD", filepath.Join(dir, "args"))
			script := `#!/bin/sh
printf '%s\n' "$@" > "$FAKE_OCP_RECORD"
config=''
while [ "$#" -gt 0 ]; do
 if [ "$1" = '--kubeconfig' ]; then shift; config="$1"; fi
 shift
done
[ -n "$config" ] || exit 90
IFS= read -r password
[ "$password" = 'secret-password' ] || exit 91
[ "$FAKE_OCP_MODE" != 'login failure' ] || { printf 'server echoed secret-password and token'; exit 1; }
[ "$FAKE_OCP_MODE" != 'empty config' ] || exit 0
printf 'fake authenticated config' > "$config"
`
			if mode != "missing oc" {
				if err := os.WriteFile(filepath.Join(bin, "oc"), []byte(script), 0700); err != nil {
					t.Fatal(err)
				}
			}
			kubectl := `#!/bin/sh
[ "$FAKE_OCP_MODE" != 'invalid context' ] || { printf 'secret-password'; exit 1; }
printf '%s\n' "$FAKE_OCP_CONTEXT"
`
			if err := os.WriteFile(filepath.Join(bin, "kubectl"), []byte(kubectl), 0700); err != nil {
				t.Fatal(err)
			}
			c := NewTestConfig()
			if mode == "success" {
				t.Setenv("FAKE_OCP_CONTEXT", "old-context")
				if c.GetKubeContext() != "old-context" {
					t.Fatal("could not populate context cache")
				}
			}
			t.Setenv("FAKE_OCP_CONTEXT", "ocp-context")
			err := AuthenticateOCP(c)
			if mode == "success" {
				if err != nil {
					t.Fatal(err)
				}
				info, err := os.Stat(c.UseKubeconfig)
				if err != nil {
					t.Fatal(err)
				}
				if info.Mode().Perm() != 0600 {
					t.Errorf("file mode = %o", info.Mode().Perm())
				}
				info, err = os.Stat(filepath.Dir(c.UseKubeconfig))
				if err != nil {
					t.Fatal(err)
				}
				if info.Mode().Perm() != 0700 {
					t.Errorf("directory mode = %o", info.Mode().Perm())
				}
				if c.GetKubeContext() != "ocp-context" {
					t.Error("external current-context not selected")
				}
			} else {
				if err == nil {
					t.Fatal("expected authentication failure")
				}
				if strings.Contains(err.Error(), "secret-password") {
					t.Fatal("error leaked password")
				}
				if FileExists(c.UseKubeconfig) {
					t.Fatal("failed login published a kubeconfig")
				}
			}
			if mode != "missing oc" {
				args, err := os.ReadFile(os.Getenv("FAKE_OCP_RECORD"))
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(string(args), "secret-password") {
					t.Fatal("password exposed in argv")
				}
			}
			homeBytes, err := os.ReadFile(homeConfig)
			if err != nil {
				t.Fatal(err)
			}
			if string(homeBytes) != "home-sentinel" {
				t.Fatal("modified user's home kubeconfig")
			}
			if os.Getenv("KUBECONFIG") != homeConfig {
				t.Fatal("authentication mutated caller KUBECONFIG")
			}
		})
	}
}

func TestOCPLoginHonorsCancellationAndPrivateDirectory(t *testing.T) {
	for _, mode := range []string{"cancelled", "symlink directory", "public directory"} {
		t.Run(mode, func(t *testing.T) {
			dir := isolateOCPConfig(t)
			t.Setenv("OCP_USERNAME", "alice")
			t.Setenv("OCP_PASSWORD", "secret")
			t.Setenv("OCP_SERVER", "https://api.example.test")
			t.Setenv("PATH", dir)
			marker := filepath.Join(dir, "called")
			t.Setenv("FAKE_MARKER", marker)
			if err := os.WriteFile(filepath.Join(dir, "oc"), []byte("#!/bin/sh\nprintf called > \"$FAKE_MARKER\"\n"), 0700); err != nil {
				t.Fatal(err)
			}
			c := NewTestConfig()
			privateDir := filepath.Dir(c.UseKubeconfig)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch mode {
			case "cancelled":
				cancel()
			case "symlink directory":
				if err := os.Symlink(dir, privateDir); err != nil {
					t.Fatal(err)
				}
			case "public directory":
				if err := os.Mkdir(privateDir, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(privateDir, 0755); err != nil {
					t.Fatal(err)
				}
			}
			if err := authenticateOCP(ctx, c); err == nil {
				t.Fatal("expected failure")
			}
			if FileExists(marker) {
				t.Fatal("oc executed despite cancellation or unsafe directory")
			}
			if FileExists(c.UseKubeconfig) {
				t.Fatal("failure published kubeconfig")
			}
		})
	}
}

func TestOCPLoginExplicitKubeconfigDoesNotAuthenticate(t *testing.T) {
	dir := isolateOCPConfig(t)
	t.Setenv("PATH", dir) // No CLI available: explicit kubeconfig needs no login.
	t.Setenv("CLUSTER_MODE", "mce")
	t.Setenv("USE_KUBECONFIG", filepath.Join(dir, "explicit"))
	t.Setenv("OCP_PASSWORD", "secret")
	c := NewTestConfig()
	if err := AuthenticateOCP(c); err != nil {
		t.Fatal(err)
	}
	t.Run("MCE phase", TestCheckDependencies_MCEAuthentication)
}

func TestOCPPasswordCommandRedaction(t *testing.T) {
	if got := redactCommand("env OCP_PASSWORD=secret oc login"); strings.Contains(got, "secret") {
		t.Fatalf("OCP_PASSWORD leaked in command: %s", got)
	}
}

func TestOCPLoginRejectsUnsafeServerAndTimeout(t *testing.T) {
	for _, tc := range []struct{ name, server, timeout string }{
		{"plaintext", "http://api.example.test", "2m"},
		{"userinfo", "https://alice:secret@api.example.test", "2m"},
		{"missing host", "https:///api", "2m"},
		{"negative timeout", "https://api.example.test", "-1s"},
		{"zero timeout", "https://api.example.test", "0"},
		{"invalid timeout", "https://api.example.test", "secret"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := isolateOCPConfig(t)
			t.Setenv("OCP_USERNAME", "alice")
			t.Setenv("OCP_PASSWORD", "secret")
			t.Setenv("OCP_SERVER", tc.server)
			t.Setenv("OCP_LOGIN_TIMEOUT", tc.timeout)
			t.Setenv("PATH", dir)
			c := NewTestConfig()
			if configError == nil {
				t.Fatal("expected configuration error")
			}
			err := AuthenticateOCP(c)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if strings.Contains(err.Error(), "secret") {
				t.Fatal("error exposes credentials")
			}
			if FileExists(c.UseKubeconfig) {
				t.Fatal("invalid credentials published kubeconfig")
			}
		})
	}
}

func TestOCPLoginTimeoutPreservesExistingConfig(t *testing.T) {
	dir := isolateOCPConfig(t)
	t.Setenv("OCP_USERNAME", "alice")
	t.Setenv("OCP_PASSWORD", "secret")
	t.Setenv("OCP_SERVER", "https://api.example.test")
	t.Setenv("OCP_LOGIN_TIMEOUT", "20ms")
	t.Setenv("PATH", dir)
	// No children: CommandContext must terminate the login process itself.
	if err := os.WriteFile(filepath.Join(dir, "oc"), []byte("#!/bin/sh\nwhile :; do :; done\n"), 0700); err != nil {
		t.Fatal(err)
	}
	c := NewTestConfig()
	if err := os.Mkdir(filepath.Dir(c.UseKubeconfig), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(c.UseKubeconfig, []byte("existing-config"), 0600); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if err := AuthenticateOCP(c); err == nil {
		t.Fatal("expected timeout")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("login exceeded configured deadline: %v", elapsed)
	}
	data, err := os.ReadFile(c.UseKubeconfig)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "existing-config" {
		t.Fatal("failed authentication overwrote existing config")
	}
	files, err := os.ReadDir(filepath.Dir(c.UseKubeconfig))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("temporary login files were not removed: %v", files)
	}
}
