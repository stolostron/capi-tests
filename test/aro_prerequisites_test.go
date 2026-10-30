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

func TestAROPrerequisiteInventory(t *testing.T) {
	for _, missing := range []string{"", "ResourceGroup", "VirtualNetwork", "VirtualNetworksSubnet", "UserAssignedIdentity"} {
		t.Run("missing-"+missing, func(t *testing.T) {
			config, data := splitTestConfig(t)
			plan, err := splitAROResources(data, config)
			if err != nil {
				t.Fatal(err)
			}
			for _, resources := range []*[]map[string]interface{}{&plan.Prerequisites, &plan.Identities} {
				var retained []map[string]interface{}
				for _, resource := range *resources {
					if resource["kind"] != missing {
						retained = append(retained, resource)
					}
				}
				*resources = retained
			}
			err = validateAROPrerequisiteInventory(plan)
			if missing == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), missing) {
				t.Fatalf("missing %s: %v", missing, err)
			}
		})
	}
}

func TestAROPrerequisiteInventoryBlocksApply(t *testing.T) {
	config, data := splitTestConfig(t)
	// A whole kind omitted from the manifest would otherwise never be waited on.
	data = []byte(strings.Replace(string(data), "kind: VirtualNetwork\n", "kind: NetworkSecurityGroup\n", 1))
	path := filepath.Join(config.RepoDir, config.GetOutputDirName(), config.ClusterYAML)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	fake := newSplitFake(t)
	err := executeAROSplitPhase(config, "identities", fake.commands())
	if err == nil || !strings.Contains(err.Error(), "VirtualNetwork") || len(fake.applied) != 0 {
		t.Fatalf("error=%v applied=%v", err, fake.applied)
	}
}

func TestAROMissingInfrastructureDiagnostic(t *testing.T) {
	for _, tc := range []struct {
		reason, message, status string
		generation              int
		want                    bool
	}{
		{"InfrastructureNotFound", "AROCluster cluster not found", "False", 2, true},
		{"InfrastructureNotFound", "AROCluster cluster not found", "False", 1, false},
		{"InfrastructureNotFound", "AROCluster cluster not found", "True", 2, false},
		{"Waiting", "", "False", 2, false},
		{"NotFound", "Secret cluster missing", "False", 2, false},
		{"NotFound", "AROCluster other missing", "False", 2, false},
	} {
		data := fmt.Sprintf(`{"metadata":{"generation":2},"status":{"conditions":[{"type":"Ready","status":%q,"reason":%q,"message":%q,"observedGeneration":%d}]}}`, tc.status, tc.reason, tc.message, tc.generation)
		diagnostic, err := missingInfrastructureDiagnostic(data, "cluster")
		if err != nil || (diagnostic != "") != tc.want {
			t.Errorf("%+v: diagnostic=%q error=%v", tc, diagnostic, err)
		}
	}
	if _, err := missingInfrastructureDiagnostic("not json", "cluster"); err == nil {
		t.Fatal("accepted malformed status")
	}
}

// Command fixtures prove the test harness enforces its assertions, not that a
// real CAPZ controller emits the expected missing-infrastructure condition.
func TestAROMissingInfrastructureScenario(t *testing.T) {
	for _, tc := range []struct {
		name                              string
		occupied, hcp, silent, apiFailure bool
		want                              string
	}{
		{name: "explicit diagnostic"},
		{name: "occupied namespace", occupied: true, want: "empty workload namespace"},
		{name: "unexpected HCP", hcp: true, want: "unexpected hcpopenshiftclusters"},
		{name: "silent controller", silent: true, want: "before deadline"},
		{name: "API failure", apiFailure: true, want: "preflight"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config, data := splitTestConfig(t)
			plan, err := splitAROResources(data, config)
			if err != nil {
				t.Fatal(err)
			}
			applied := false
			commands := aroSplitCommands{
				apply: func(_ context.Context, path string) error {
					data, err := os.ReadFile(path)
					if err != nil {
						return err
					}
					docs, err := readYAMLDocuments(data)
					if err != nil {
						return err
					}
					if len(docs) != 2 {
						return fmt.Errorf("negative bundle has %d documents", len(docs))
					}
					for _, doc := range docs {
						if doc["kind"] != "Cluster" && doc["kind"] != "AROControlPlane" {
							return fmt.Errorf("unexpected applied kind %v", doc["kind"])
						}
					}
					applied = true
					return nil
				},
				run: func(_ context.Context, args ...string) (string, error) {
					if tc.apiFailure {
						return "", errors.New("forbidden")
					}
					if !applied {
						if tc.occupied {
							return "cluster/existing", nil
						}
						return "", nil
					}
					if args[1] == "hcpopenshiftclusters.redhatopenshift.azure.com" && tc.hcp {
						return "hcpopenshiftcluster/cluster", nil
					}
					if args[1] == "arocontrolplanes.controlplane.cluster.x-k8s.io" {
						if tc.silent {
							return `{"metadata":{"generation":2}}`, nil
						}
						return `{"metadata":{"generation":2},"status":{"conditions":[{"type":"Ready","status":"False","reason":"InfrastructureNotFound","message":"AROCluster cluster not found","observedGeneration":2}]}}`, nil
					}
					return "", nil
				},
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
			defer cancel()
			diagnostic, err := runAROMissingInfrastructureScenario(ctx, config, plan, commands, t.TempDir(), time.Millisecond)
			if tc.want == "" {
				if err != nil || diagnostic == "" {
					t.Fatalf("diagnostic=%q error=%v", diagnostic, err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v want %q", err, tc.want)
			}
			if (tc.occupied || tc.apiFailure) && applied {
				t.Fatal("applied resources after failed preflight")
			}
		})
	}
}

func TestAROPrerequisiteReadinessBlocksHCP(t *testing.T) {
	for _, payload := range []string{
		`{"items":[]}`,
		`{"items":[{"metadata":{"generation":2},"status":{"conditions":[{"type":"Ready","status":"False","observedGeneration":2}]}},{},{}]}`,
		`{"items":[{"metadata":{"generation":2},"status":{"conditions":[{"type":"Ready","status":"True","observedGeneration":1}]}},{},{}]}`,
	} {
		config, _ := splitTestConfig(t)
		config.ClusterDeploymentTimeout = 20 * time.Millisecond
		fake := newSplitFake(t)
		commands := fake.commands()
		original := commands.run
		commands.run = func(ctx context.Context, args ...string) (string, error) {
			if len(args) > 2 && args[1] == "-f" && filepath.Base(args[2]) == "prerequisites.yaml" {
				return payload, nil
			}
			return original(ctx, args...)
		}
		err := executeAROSplitPhase(config, "cluster", commands)
		if err == nil || !strings.Contains(err.Error(), "blocked by prerequisites") || len(fake.applied) > 0 {
			t.Fatalf("error=%v applied=%v", err, fake.applied)
		}
	}
}
