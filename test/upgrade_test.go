package test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const upgradeFixture = `apiVersion: cluster.x-k8s.io/v1beta2
kind: Cluster
metadata:
  name: demo
  namespace: ns
spec:
  controlPlaneRef:
    name: demo-cp
---
apiVersion: controlplane.cluster.x-k8s.io/v1beta2
kind: AROControlPlane
metadata:
  name: demo-cp
  namespace: ns
spec:
  resources:
  - apiVersion: redhatopenshift.azure.com/v1api20251223preview
    kind: HcpOpenShiftCluster
    metadata:
      name: demo
    spec:
      properties:
        version:
          channelGroup: stable
          id: "4.20.0"
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind: AROMachinePool
metadata:
  name: demo-mp
  namespace: ns
spec:
  resources:
  - apiVersion: redhatopenshift.azure.com/v1api20251223preview
    kind: HcpOpenShiftClustersNodePool
    metadata:
      name: demo-mp
    spec:
      owner:
        name: demo
      properties:
        version:
          channelGroup: stable
          id: "4.20.0"
---
kind: ConfigMap
metadata:
  name: keep-me
data:
  version: untouched
`

func TestPrepareAROUpgrade(t *testing.T) {
	for _, tc := range []struct {
		name, input, namespace, cp, mp string
		wantErr                        bool
	}{
		{"updates both", upgradeFixture, "ns", "4.21.0", "4.21.0", false},
		{"minor CP with patch pool", upgradeFixture, "ns", "4.21", "4.21.3", false},
		{"minor versions", upgradeFixture, "ns", "4.21", "4.21", false},
		{"different pool target", upgradeFixture, "ns", "4.21.0", "4.20.1", false},
		{"idempotent", upgradeFixture, "ns", "4.20.0", "4.20.0", false},
		{"wrong namespace", upgradeFixture, "other", "4.21.0", "4.21.0", true},
		{"missing node pool version", strings.Replace(upgradeFixture, "channelGroup: stable\n          id: \"4.20.0\"\n---\nkind: ConfigMap", "channelGroup: stable\n          missing: x\n---\nkind: ConfigMap", 1), "ns", "4.21.0", "4.21.0", true},
		{"downgrade", upgradeFixture, "ns", "4.19.0", "4.19.0", true},
		{"workers newer", upgradeFixture, "ns", "4.21.0", "4.22.0", true},
		{"invalid version", upgradeFixture, "ns", "latest", "latest", true},
		{"invalid YAML", "kind: [", "ns", "4.21.0", "4.21.0", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			output, refs, err := PrepareAROUpgrade([]byte(tc.input), "demo-cp", tc.namespace, tc.cp, tc.mp)
			if (err != nil) != tc.wantErr {
				t.Fatalf("error = %v, want error %v", err, tc.wantErr)
			}
			if tc.wantErr {
				if len(output) != 0 {
					t.Fatal("failure returned mutated YAML")
				}
				return
			}
			if len(refs) != 2 {
				t.Fatalf("got %d references, want 2", len(refs))
			}
			for i, want := range []string{tc.cp, tc.mp} {
				if refs[i].Version != want || !strings.Contains(string(refs[i].Manifest), "id: \""+want+"\"") {
					t.Fatalf("resource %d did not receive target %s", i, want)
				}
			}
			if !strings.Contains(string(output), "version: untouched") || !strings.Contains(string(output), "channelGroup: stable") {
				t.Fatal("unrelated values lost")
			}
			second, _, err := PrepareAROUpgrade(output, "demo-cp", tc.namespace, tc.cp, tc.mp)
			if err != nil || string(second) != string(output) {
				t.Fatalf("not idempotent: %v", err)
			}
		})
	}
}

func TestAROUpgradeResourceReady(t *testing.T) {
	good := `{"metadata":{"generation":2},"spec":{"properties":{"version":{"id":"4.21"}}},"status":{"properties":{"version":{"id":"4.21"}},"conditions":[{"type":"Ready","status":"True","observedGeneration":2}]}}`
	for _, tc := range []struct {
		name, input string
		want        bool
	}{
		{"ready", good, true},
		{"stale readiness", strings.Replace(good, "observedGeneration\":2", "observedGeneration\":1", 1), false},
		{"old desired version", strings.Replace(good, "4.21", "4.20", 1), false},
		{"old reported version", strings.ReplaceAll(good, "4.21", "4.20"), false},
		{"not ready", strings.Replace(good, "True", "False", 1), false},
		{"malformed", "{", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := AROUpgradeResourceReady([]byte(tc.input), "4.21"); got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestAROUpgradeClusterVersionReady(t *testing.T) {
	good := `{"status":{"desired":{"version":"4.21.3"},"history":[{"version":"4.21.3","state":"Completed"}],"conditions":[{"type":"Available","status":"True"},{"type":"Progressing","status":"False"},{"type":"Failing","status":"False"}]}}`
	for _, tc := range []struct {
		name, input, target string
		want                bool
	}{
		{"minor target", good, "4.21", true}, {"exact target", good, "4.21.3", true},
		{"wrong patch", good, "4.21.2", false}, {"wrong minor", good, "4.2", false},
		{"in progress", strings.Replace(good, "Completed", "Partial", 1), "4.21", false},
		{"unavailable", strings.Replace(good, "True", "False", 1), "4.21", false},
		{"missing status", "{}", "4.21", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := AROUpgradeClusterVersionReady([]byte(tc.input), tc.target); got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestValidateAROUpgradeLiveVersion(t *testing.T) {
	for _, tc := range []struct {
		current, target string
		wantErr         bool
	}{
		{"4.20.0", "4.21.0", false}, {"4.21.0", "4.21.0", false}, {"4.22.0", "4.21.0", true}, {"", "4.21", true}, {"4.21", "latest", true},
	} {
		t.Run(tc.current+" to "+tc.target, func(t *testing.T) {
			err := ValidateAROUpgradeLiveVersion([]byte(fmt.Sprintf(`{"spec":{"properties":{"version":{"id":%q}}}}`, tc.current)), tc.target)
			if (err != nil) != tc.wantErr {
				t.Fatalf("error=%v want error %v", err, tc.wantErr)
			}
		})
	}
}

func TestWaitForAROUpgrade(t *testing.T) {
	t.Run("transient recovery", func(t *testing.T) {
		attempts := 0
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		err := WaitForAROUpgrade(ctx, time.Millisecond, func() error {
			attempts++
			if attempts < 3 {
				return fmt.Errorf("not ready")
			}
			return nil
		})
		if err != nil || attempts != 3 {
			t.Fatalf("error=%v attempts=%d", err, attempts)
		}
	})
	t.Run("timeout preserves cause", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
		defer cancel()
		err := WaitForAROUpgrade(ctx, time.Millisecond, func() error { return fmt.Errorf("node pool old version") })
		if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "node pool old version") {
			t.Fatalf("error=%v", err)
		}
	})
	t.Run("already cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := WaitForAROUpgrade(ctx, time.Millisecond, func() error { t.Fatal("check invoked after cancellation"); return nil })
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error=%v", err)
		}
	})
}

func TestRunUpgradeCommand(t *testing.T) {
	dir := t.TempDir()
	script := `#!/bin/sh
case "$1" in
fail) echo "synthetic failure"; exit 1 ;;
hang) exec sleep 10 ;;
*) cat ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "kubectl"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Run("stdin", func(t *testing.T) {
		output, err := RunUpgradeCommand(t, context.Background(), []byte("manifest"), "apply", "-f", "-")
		if err != nil || output != "manifest" {
			t.Fatalf("output=%q err=%v", output, err)
		}
	})
	t.Run("failure", func(t *testing.T) {
		_, err := RunUpgradeCommand(t, context.Background(), nil, "fail")
		if err == nil || !strings.Contains(err.Error(), "synthetic failure") {
			t.Fatalf("error=%v", err)
		}
	})
	t.Run("bounded subprocess", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		start := time.Now()
		_, err := RunUpgradeCommand(t, ctx, nil, "hang")
		if err == nil || time.Since(start) > 2*time.Second {
			t.Fatalf("error=%v elapsed=%v", err, time.Since(start))
		}
	})
}

func TestAROUpgradeConfig(t *testing.T) {
	t.Setenv("CAPI_TEST_CONTEXT_FILE", filepath.Join(t.TempDir(), "run-context.json"))
	t.Setenv("ARO_UPGRADE_VERSION", "")
	t.Setenv("ARO_UPGRADE_NODE_POOL_VERSION", "")
	cfg := NewTestConfig()
	if cfg.AROUpgradeVersion != "" || cfg.AROUpgradeNodePoolVersion != "" {
		t.Fatal("upgrade enabled by default")
	}
	t.Setenv("ARO_UPGRADE_VERSION", "4.21")
	t.Setenv("ARO_UPGRADE_NODE_POOL_VERSION", "4.20")
	cfg = NewTestConfig()
	if cfg.AROUpgradeVersion != "4.21" || cfg.AROUpgradeNodePoolVersion != "4.20" {
		t.Fatal("upgrade configuration ignored")
	}
}

func TestPrepareAROUpgradeMultiplePools(t *testing.T) {
	docs := strings.Split(upgradeFixture, "---\n")
	extra := strings.ReplaceAll(docs[2], "demo-mp", "demo-mp-two")
	unrelated := strings.ReplaceAll(strings.ReplaceAll(docs[2], "demo-mp", "other-mp"), "name: demo\n", "name: other\n")
	input := upgradeFixture + "---\n" + extra + "---\n" + unrelated
	output, refs, err := PrepareAROUpgrade([]byte(input), "demo-cp", "ns", "4.21", "4.21")
	if err != nil || len(refs) != 3 {
		t.Fatalf("resources=%d error=%v", len(refs), err)
	}
	if strings.Count(string(output), `id: "4.21"`) != 3 || strings.Count(string(output), `id: "4.20.0"`) != 1 {
		t.Fatalf("versions not scoped to selected cluster: %s", output)
	}
}

func TestReconcileAROUpgrade(t *testing.T) {
	for _, scenario := range []string{"success", "missing pool", "save failed", "CP apply failed", "CP not ready", "CVO incomplete", "pool apply failed"} {
		t.Run(scenario, func(t *testing.T) {
			resources := []AROUpgradeResource{
				{Kind: "HcpOpenShiftCluster", Name: "cp", Version: "4.21", Manifest: []byte("cp")},
				{Kind: "HcpOpenShiftClustersNodePool", Name: "pool", Version: "4.21", Manifest: []byte("pool")},
			}
			var events []string
			cpApplied, poolApplied, cpCompleted := false, false, false
			runner := func(ctx context.Context, stdin []byte, args ...string) (string, error) {
				command := strings.Join(args, " ")
				if strings.Contains(command, "apply") {
					events = append(events, "apply "+string(stdin))
					if string(stdin) == "cp" {
						cpApplied = true
						if scenario == "CP apply failed" {
							return "", fmt.Errorf("apply failed")
						}
					} else {
						poolApplied = true
						if !cpCompleted {
							t.Fatal("pool changed before control plane completion")
						}
						if scenario == "pool apply failed" {
							return "", fmt.Errorf("pool apply failed")
						}
					}
					return "", nil
				}
				if strings.Contains(command, "clusterversion") {
					events = append(events, "check CVO")
					if scenario == "CVO incomplete" {
						return `{}`, nil
					}
					cpCompleted = true
					return `{"status":{"desired":{"version":"4.21.3"},"history":[{"version":"4.21.3","state":"Completed"}],"conditions":[{"type":"Available","status":"True"},{"type":"Progressing","status":"False"},{"type":"Failing","status":"False"}]}}`, nil
				}
				pool := strings.Contains(command, "NodePool")
				if pool && scenario == "missing pool" {
					return "", fmt.Errorf("not found")
				}
				if !cpApplied {
					events = append(events, "preflight")
					return `{"spec":{"properties":{"version":{"id":"4.20"}}}}`, nil
				}
				if !pool && scenario == "CP not ready" {
					return `{}`, nil
				}
				return `{"metadata":{"generation":2},"spec":{"properties":{"version":{"id":"4.21"}}},"status":{"properties":{"version":{"id":"4.21"}},"conditions":[{"type":"Ready","status":"True","observedGeneration":2}]}}`, nil
			}
			saved := false
			save := func() error {
				saved = true
				events = append(events, "save")
				if scenario == "save failed" {
					return fmt.Errorf("disk full")
				}
				return nil
			}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Millisecond)
			defer cancel()
			err := ReconcileAROUpgrade(ctx, resources, []string{"--context", "management", "--namespace", "ns"}, "workload", time.Millisecond, runner, save)
			if (err == nil) != (scenario == "success") {
				t.Fatalf("error=%v events=%v", err, events)
			}
			if scenario == "missing pool" && (saved || cpApplied) {
				t.Fatalf("mutated after preflight failure: %v", events)
			}
			if scenario == "save failed" && cpApplied {
				t.Fatalf("applied after failed save: %v", events)
			}
			if strings.HasPrefix(scenario, "CP") || scenario == "CVO incomplete" {
				if poolApplied {
					t.Fatalf("pool changed after CP failure: %v", events)
				}
			}
			if scenario == "success" && !poolApplied {
				t.Fatalf("pool never applied: %v", events)
			}
		})
	}
}
