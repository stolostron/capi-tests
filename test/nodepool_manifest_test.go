package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Fields mirror cluster-api-installer/scripts/aro-hcp/aro-template.yaml.
const validARONodePoolManifest = `apiVersion: cluster.x-k8s.io/v1beta2
kind: Cluster
metadata:
  name: alpha
  namespace: workloads
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind: AROMachinePool
metadata:
  name: alpha-mp1
  namespace: workloads
spec:
  resources:
    - apiVersion: redhatopenshift.azure.com/v20260901preview
      kind: HcpOpenShiftClustersNodePool
      metadata:
        name: alpha-mp1
        namespace: workloads
      spec:
        azureName: alpha-mp1
        owner:
          name: alpha
---
apiVersion: cluster.x-k8s.io/v1beta2
kind: MachinePool
metadata:
  name: alpha-mp1
  namespace: workloads
spec:
  clusterName: alpha
  template:
    spec:
      infrastructureRef:
        apiGroup: infrastructure.cluster.x-k8s.io
        kind: AROMachinePool
        name: alpha-mp1
`

func TestValidateGeneratedARONodePools(t *testing.T) {
	refStart := strings.LastIndex(validARONodePoolManifest, "        name: alpha-mp1")
	badRef := validARONodePoolManifest[:refStart] + strings.Replace(validARONodePoolManifest[refStart:], "alpha-mp1", "missing-pool", 1)
	secondPool := strings.SplitN(validARONodePoolManifest, "---", 2)[1]
	cases := []struct{ name, manifest, want string }{
		{"cluster alpha", validARONodePoolManifest, ""},
		{"cluster beta in same namespace", strings.ReplaceAll(validARONodePoolManifest, "alpha", "beta"), ""},
		{"multiple named pools", validARONodePoolManifest + "---\n" + strings.ReplaceAll(secondPool, "mp1", "workers"), ""},
		{"omitted Azure name defaults to object name", strings.Replace(validARONodePoolManifest, "        azureName: alpha-mp1\n", "", 1), ""},
		{"explicit Azure name preserved", strings.Replace(validARONodePoolManifest, "azureName: alpha-mp1", "azureName: custom-pool", 1), ""},
		{"one character Azure name allowed by schema", strings.Replace(validARONodePoolManifest, "azureName: alpha-mp1", "azureName: a", 1), ""},
		{"legacy shared ASO name", strings.Replace(validARONodePoolManifest, "        name: alpha-mp1", "        name: w-uksouth-mp-0", 1), "cluster-specific"},
		{"wrong owner", strings.Replace(validARONodePoolManifest, "          name: alpha", "          name: beta", 1), "owner"},
		{"wrong nodepool namespace", strings.Replace(validARONodePoolManifest, "        namespace: workloads", "        namespace: elsewhere", 1), "namespace"},
		{"invalid Azure name", strings.Replace(validARONodePoolManifest, "azureName: alpha-mp1", "azureName: abcdefghijkl-mp1", 1), "NAME_PREFIX"},
		{"bad Azure characters", strings.Replace(validARONodePoolManifest, "azureName: alpha-mp1", "azureName: alpha_mp1", 1), "NAME_PREFIX"},
		{"dangling infrastructure reference", badRef, "infrastructureRef"},
		{"missing nodepool", strings.Replace(validARONodePoolManifest, "kind: HcpOpenShiftClustersNodePool", "kind: OtherResource", 1), "no embedded"},
		{"duplicate ASO object", validARONodePoolManifest + "---\n" + strings.Replace(secondPool, "  name: alpha-mp1", "  name: alpha-extra", 1), "duplicate node pool Kubernetes"},
		{"missing cluster", "kind: AROMachinePool\nmetadata:\n  name: alpha-mp1\n", "Cluster"},
		{"malformed YAML", "kind: [", "decode"},
	}
	duplicatePool := strings.ReplaceAll(secondPool, "mp1", "workers")
	duplicatePool = strings.Replace(duplicatePool, "  name: alpha-workers", "  name: alpha-mp1", 1)
	duplicateRef := strings.LastIndex(duplicatePool, "        name: alpha-workers")
	duplicatePool = duplicatePool[:duplicateRef] + strings.Replace(duplicatePool[duplicateRef:], "alpha-workers", "alpha-mp1", 1)
	cases = append(cases, struct{ name, manifest, want string }{"duplicate AROMachinePool", validARONodePoolManifest + "---\n" + duplicatePool, "duplicate AROMachinePool"})
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "aro.yaml")
			if err := os.WriteFile(path, []byte(tc.manifest), 0600); err != nil {
				t.Fatal(err)
			}
			err := ValidateGeneratedARONodePools(path)
			if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
	if err := ValidateGeneratedARONodePools(filepath.Join(t.TempDir(), "missing.yaml")); err == nil {
		t.Fatal("missing manifest accepted")
	}
}
