package test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func splitTestConfig(t *testing.T) (*TestConfig, []byte) {
	t.Helper()
	config := &TestConfig{RepoDir: t.TempDir(), WorkloadClusterName: "cluster", ClusterNamePrefix: "cluster", Environment: "test", WorkloadClusterNamespace: "ns", ResourceGroupName: "rg", ClusterYAML: "aro.yaml", InfraProviders: []InfraProvider{NewAzureProvider("ns")}, AROSplitDeployment: true, AROIdentityCredentialSecret: "identity-credential", ARORoleCredentialSecret: "role-credential", ARODeploymentCredentialSecret: "aso-credential", ClusterDeploymentTimeout: time.Second}
	data, err := os.ReadFile("testdata/aro-split.yaml")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(config.RepoDir, config.GetOutputDirName())
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "aro.yaml"), data, 0600); err != nil {
		t.Fatal(err)
	}
	credentials := `apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureClusterIdentity
metadata: {name: cluster-identity, namespace: ns}
spec: {clientID: deployment-client}
---
apiVersion: v1
kind: Secret
metadata: {name: aso-credential, namespace: ns}
stringData: {AZURE_CLIENT_ID: deployment-client}
`
	if err := os.WriteFile(filepath.Join(dir, "credentials.yaml"), []byte(credentials), 0600); err != nil {
		t.Fatal(err)
	}
	return config, data
}

func resourceAnnotations(resource map[string]interface{}) map[string]interface{} {
	return resource["metadata"].(map[string]interface{})["annotations"].(map[string]interface{})
}

func TestAROSplitPartitionsAndPinsCredentials(t *testing.T) {
	config, data := splitTestConfig(t)
	plan, err := splitAROResources(data, config)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Prerequisites) != 3 || len(plan.Identities) != 1 || len(plan.Roles) != 1 || len(plan.Cluster) != 4 {
		t.Fatalf("incorrect partition counts: %+v", plan)
	}
	for _, test := range []struct {
		resource   map[string]interface{}
		credential string
	}{{plan.Prerequisites[0], "aso-credential"}, {plan.Identities[0], "identity-credential"}, {plan.Roles[0], "role-credential"}} {
		if got := resourceAnnotations(test.resource)[asoCredentialAnnotation]; got != test.credential {
			t.Errorf("credential = %v, want %s", got, test.credential)
		}
	}
	roleSpec := plan.Roles[0]["spec"].(map[string]interface{})
	if got := roleSpec["principalIdFromConfig"].(map[string]interface{})["name"]; got != "identity-map" {
		t.Errorf("lost principal reference: %v", got)
	}
	for _, document := range plan.Cluster {
		spec, _ := document["spec"].(map[string]interface{})
		resources, _ := spec["resources"].([]interface{})
		for _, item := range resources {
			resource := item.(map[string]interface{})
			annotations := resourceAnnotations(resource)
			if annotations[asoCredentialAnnotation] != "aso-credential" {
				t.Errorf("cluster bundle retained setup credential: %v", annotations)
			}
			if resource["kind"] == "UserAssignedIdentity" || resource["kind"] == "RoleAssignment" {
				if annotations[asoReconcileAnnotation] != "skip" {
					t.Errorf("privileged resource not read-only: %v", resource)
				}
			}
		}
	}
	original, err := os.ReadFile(filepath.Join(config.RepoDir, config.GetOutputDirName(), "aro.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(original) != string(data) {
		t.Fatal("original manifest was modified")
	}
}

func TestAROSplitExternalIdentitiesAreReadOnly(t *testing.T) {
	config, data := splitTestConfig(t)
	config.AROPreexistingIdentities = true
	config.AROIdentityCredentialSecret = ""
	if _, err := splitAROResources(data, config); err == nil {
		t.Fatal("accepted preexisting identity in run-owned resource group")
	}
	data = []byte(strings.Replace(string(data), "owner: {name: rg}\n        operatorSpec:", "owner: {armId: /subscriptions/sub/resourceGroups/external-identities}\n        operatorSpec:", 1))
	plan, err := splitAROResources(data, config)
	if err != nil {
		t.Fatal(err)
	}
	annotations := resourceAnnotations(plan.Identities[0])
	if annotations[asoCredentialAnnotation] != "aso-credential" || annotations[asoReconcileAnnotation] != "skip" {
		t.Fatalf("external identity is writable: %v", annotations)
	}
	if err := validateAROSplitConfig(config); err != nil {
		t.Fatal(err)
	}
}

func TestAROSplitRejectsInvalidBundles(t *testing.T) {
	config, data := splitTestConfig(t)
	for _, tc := range []struct{ name, old, new string }{{"wrong namespace", "namespace: ns", "namespace: other"}, {"missing identity", "kind: UserAssignedIdentity", "kind: UnknownIdentity"}, {"missing roles", "kind: RoleAssignment", "kind: UnknownRole"}, {"malformed", "spec:", "spec: ["}} {
		t.Run(tc.name, func(t *testing.T) {
			mutated := strings.ReplaceAll(string(data), tc.old, tc.new)
			if _, err := splitAROResources([]byte(mutated), config); err == nil {
				t.Fatal("invalid manifest accepted")
			}
		})
	}
}

type splitFake struct {
	t        *testing.T
	ids      map[string]string
	applied  []string
	ready    map[string]bool
	fail     string
	existing bool
}

func (fake *splitFake) commands() aroSplitCommands {
	return aroSplitCommands{
		apply: func(ctx context.Context, path string) error {
			name := strings.TrimSuffix(filepath.Base(path), ".yaml")
			fake.applied = append(fake.applied, name)
			if fake.fail == name {
				return fmt.Errorf("simulated apply failure")
			}
			fake.ready[name] = true
			return nil
		},
		run: func(ctx context.Context, args ...string) (string, error) {
			if len(args) > 2 && args[0] == "get" && args[1] == "secret" {
				return base64.StdEncoding.EncodeToString([]byte(fake.ids[args[2]])), nil
			}
			if len(args) > 2 && args[0] == "get" && args[1] == capiClusterResource {
				if fake.existing {
					return "cluster/cluster", nil
				}
				return "", nil
			}
			var path string
			for index, arg := range args {
				if arg == "-f" {
					path = args[index+1]
				}
			}
			name := strings.TrimSuffix(filepath.Base(path), ".yaml")
			if !fake.ready[name] {
				return "", fmt.Errorf("%s not ready", name)
			}
			if args[0] == "wait" {
				return "Ready", nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return "", err
			}
			documents, err := readYAMLDocuments(data)
			if err != nil {
				return "", err
			}
			items := make([]interface{}, len(documents))
			for i := range documents {
				items[i] = map[string]interface{}{"metadata": map[string]interface{}{"generation": 2}, "status": map[string]interface{}{"conditions": []interface{}{map[string]interface{}{"type": "Ready", "status": "True", "observedGeneration": 2}}}}
			}
			output, err := json.Marshal(map[string]interface{}{"items": items})
			return string(output), err
		},
	}
}
func newSplitFake(t *testing.T) *splitFake {
	return &splitFake{t: t, ids: map[string]string{"identity-credential": "identity-client", "role-credential": "role-client", "aso-credential": "deployment-client"}, ready: map[string]bool{}}
}

func TestAROSplitPhasesGateDeploymentAndResume(t *testing.T) {
	config, _ := splitTestConfig(t)
	fake := newSplitFake(t)
	for _, phase := range []string{"identities", "roles", "cluster"} {
		if err := executeAROSplitPhase(config, phase, fake.commands()); err != nil {
			t.Fatalf("%s: %v", phase, err)
		}
	}
	want := []string{"credentials", "prerequisites", "identities", "roles", "cluster"}
	if !reflect.DeepEqual(fake.applied, want) {
		t.Fatalf("applied = %v, want %v", fake.applied, want)
	}
	fake.existing = true
	fake.applied = nil
	for _, phase := range []string{"identities", "roles", "cluster"} {
		if err := executeAROSplitPhase(config, phase, fake.commands()); err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(fake.applied, []string{"credentials", "cluster"}) {
		t.Fatalf("resume reapplied privileged bundles: %v", fake.applied)
	}
}

func TestAROSplitStopsBeforeRoleOrClusterOnFailure(t *testing.T) {
	for _, scenario := range []string{"identity-fails", "roles-before-identities", "cluster-before-roles", "same-principal", "missing-principal", "wrong-deployer"} {
		t.Run(scenario, func(t *testing.T) {
			config, _ := splitTestConfig(t)
			fake := newSplitFake(t)
			phase := "identities"
			switch scenario {
			case "identity-fails":
				fake.fail = "identities"
			case "roles-before-identities":
				phase = "roles"
			case "cluster-before-roles":
				phase = "cluster"
				fake.ready["prerequisites"] = true
				fake.ready["identities"] = true
			case "same-principal":
				fake.ids["role-credential"] = "deployment-client"
			case "missing-principal":
				fake.ids["role-credential"] = ""
			case "wrong-deployer":
				fake.ids["aso-credential"] = "wrong-client"
			}
			if err := executeAROSplitPhase(config, phase, fake.commands()); err == nil {
				t.Fatal("expected phase failure")
			}
			for _, applied := range fake.applied {
				if applied == "cluster" || applied == "roles" {
					t.Fatalf("advanced after failure: %v", fake.applied)
				}
			}
		})
	}
}

func TestAROSplitReadinessRequiresCurrentGeneration(t *testing.T) {
	for _, payload := range []string{`{"metadata":{"generation":2},"status":{"conditions":[{"type":"Ready","status":"True","observedGeneration":1}]}}`, `{"metadata":{"generation":2},"status":{"conditions":[{"type":"Ready","status":"False","observedGeneration":2}]}}`, `{"items":[]}`} {
		if err := readySplitResources([]byte(payload), 1); err == nil {
			t.Errorf("accepted stale/unready resource: %s", payload)
		}
	}
}

func TestAROSplitRejectsCredentialManifestBeforeApply(t *testing.T) {
	for _, scenario := range []string{"namespace", "unexpected-kind", "wrong-identity-reference"} {
		t.Run(scenario, func(t *testing.T) {
			config, _ := splitTestConfig(t)
			fake := newSplitFake(t)
			name := "credentials.yaml"
			old := "namespace: ns"
			replacement := "namespace: other"
			if scenario == "unexpected-kind" {
				old = "kind: Secret"
				replacement = "kind: RoleAssignment"
			}
			if scenario == "wrong-identity-reference" {
				name = "aro.yaml"
				old = "name: cluster-identity"
				replacement = "name: unvalidated-admin-identity"
			}
			path := filepath.Join(config.RepoDir, config.GetOutputDirName(), name)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(strings.ReplaceAll(string(data), old, replacement)), 0600); err != nil {
				t.Fatal(err)
			}
			if err := executeAROSplitPhase(config, "identities", fake.commands()); err == nil {
				t.Fatal("unsafe manifest accepted")
			}
			if len(fake.applied) != 0 {
				t.Fatalf("applied unsafe manifest: %v", fake.applied)
			}
		})
	}
}

func TestAROSplitRejectsClusterAsBootstrapResource(t *testing.T) {
	config, data := splitTestConfig(t)
	changed := strings.Replace(string(data), "apiVersion: resources.azure.com/v1api20200601\n      kind: ResourceGroup", "apiVersion: redhatopenshift.azure.com/v1api20251223preview\n      kind: HcpOpenShiftCluster", 1)
	if _, err := splitAROResources([]byte(changed), config); err == nil {
		t.Fatal("cluster resource accepted in setup phase")
	}
}

func TestAROSplitWaitsForCurrentGeneration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	attempts := 0
	commands := aroSplitCommands{run: func(context.Context, ...string) (string, error) {
		attempts++
		generation := 1
		if attempts > 1 {
			generation = 2
		}
		return fmt.Sprintf(`{"metadata":{"generation":2},"status":{"conditions":[{"type":"Ready","status":"True","observedGeneration":%d}]}}`, generation), nil
	}}
	if err := waitSplitBundle(ctx, commands, "identities.yaml", 1, time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if attempts != 2 {
		t.Fatalf("readiness observations = %d, want 2", attempts)
	}
}

func TestAROSplitDeadlineBoundsCredentialRead(t *testing.T) {
	config, _ := splitTestConfig(t)
	config.ClusterDeploymentTimeout = 20 * time.Millisecond
	fake := newSplitFake(t)
	commands := fake.commands()
	commands.run = func(ctx context.Context, args ...string) (string, error) { <-ctx.Done(); return "", ctx.Err() }
	started := time.Now()
	err := executeAROSplitPhase(config, "identities", commands)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want deadline", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("credential read exceeded phase deadline: %v", elapsed)
	}
}

func TestAROSplitCommandDeadlineStopsHungProcess(t *testing.T) {
	t.Setenv("TEST_RESULTS_DIR", t.TempDir())
	script := filepath.Join(t.TempDir(), "hung-kubectl")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexec sleep 10\n"), 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := RunCommandQuietContext(ctx, t, script)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want deadline", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("hung process outlived deadline: %v", elapsed)
	}
}

func TestAROSplitRequiresControlPlaneAndNodePoolWrappers(t *testing.T) {
	config, data := splitTestConfig(t)
	plan, err := splitAROResources(data, config)
	if err != nil {
		t.Fatal(err)
	}
	var controlPlane map[string]interface{}
	var without []map[string]interface{}
	for _, document := range plan.Cluster {
		if document["kind"] == "AROControlPlane" {
			controlPlane = document
		} else {
			without = append(without, document)
		}
	}
	for _, documents := range [][]map[string]interface{}{without, append(plan.Cluster, controlPlane)} {
		if err := validateSplitIdentityReferences(config, &aroSplitPlan{Cluster: documents}, "cluster-identity"); err == nil {
			t.Fatal("accepted missing/duplicate control plane")
		}
	}
	for _, kind := range []string{"HcpOpenShiftCluster", "HcpOpenShiftClustersNodePool"} {
		changed := strings.Replace(string(data), "kind: "+kind+"\n", "kind: UnknownResource\n", 1)
		if _, err := splitAROResources([]byte(changed), config); err == nil {
			t.Fatalf("accepted missing embedded %s", kind)
		}
	}
}
