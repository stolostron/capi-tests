package test

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// aroManifestResource describes only the identities and references needed for
// the generation guard. Embedded resources have the same shape as top-level CRs.
type aroManifestResource struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name      string `yaml:"name"`
		Namespace string `yaml:"namespace"`
	} `yaml:"metadata"`
	Spec struct {
		AzureName string `yaml:"azureName"`
		Owner     struct {
			Name string `yaml:"name"`
		} `yaml:"owner"`
		Resources   []aroManifestResource `yaml:"resources"`
		ClusterName string                `yaml:"clusterName"`
		Template    struct {
			Spec struct {
				InfrastructureRef struct {
					Kind string `yaml:"kind"`
					Name string `yaml:"name"`
				} `yaml:"infrastructureRef"`
			} `yaml:"spec"`
		} `yaml:"template"`
	} `yaml:"spec"`
}

// The bundled ASO CRD permits Azure names of 1–15 characters. This differs
// from ValidateNamePrefix's minimum because that helper includes the -mp1 suffix.
var generatedARONodePoolAzureName = regexp.MustCompile(`^[a-zA-Z]([-a-zA-Z0-9]{0,13}[a-zA-Z0-9])?$`)

// ValidateGeneratedARONodePools rejects the unscoped embedded ASO object names
// behind issue #343 before applying or reusing generated manifests. It derives
// the cluster identity from the file because gen.sh can override CS_CLUSTER_NAME
// with WORKLOAD_CLUSTER_NAME. No Kubernetes or cloud commands are run.
func ValidateGeneratedARONodePools(filePath string) error {
	// #nosec G304 -- generated manifest path comes from test configuration.
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open ARO manifest: %w", err)
	}
	defer file.Close()
	var docs []aroManifestResource
	clusterName := ""
	decoder := yaml.NewDecoder(file)
	for {
		var doc aroManifestResource
		err := decoder.Decode(&doc)
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("decode ARO manifest: %w", err)
		}
		docs = append(docs, doc)
		if doc.Kind == "Cluster" && strings.HasPrefix(doc.APIVersion, "cluster.x-k8s.io/") {
			if clusterName != "" {
				return fmt.Errorf("multiple CAPI Cluster documents in ARO manifest")
			}
			clusterName = doc.Metadata.Name
		}
	}
	if clusterName == "" {
		return fmt.Errorf("CAPI Cluster name missing from ARO manifest")
	}
	namespace := func(value string) string {
		if value == "" {
			return "default"
		}
		return value
	}
	pools := map[string]bool{}
	objects := map[string]bool{}
	azureNames := map[string]bool{}
	for _, doc := range docs {
		if doc.Kind != "AROMachinePool" {
			continue
		}
		ns := namespace(doc.Metadata.Namespace)
		poolKey := ns + "/" + doc.Metadata.Name
		if pools[poolKey] {
			return fmt.Errorf("duplicate AROMachinePool %q", poolKey)
		}
		pools[poolKey] = true
		found := false
		for _, resource := range doc.Spec.Resources {
			if resource.Kind != "HcpOpenShiftClustersNodePool" {
				continue
			}
			found = true
			name := resource.Metadata.Name
			if !strings.HasPrefix(name, clusterName+"-") || name == clusterName+"-" {
				return fmt.Errorf("node pool %q must have a cluster-specific Kubernetes name prefixed by %q; update installer templates and regenerate manifests", name, clusterName+"-")
			}
			if namespace(resource.Metadata.Namespace) != ns {
				return fmt.Errorf("node pool %q namespace does not match AROMachinePool namespace %q", name, ns)
			}
			if resource.Spec.Owner.Name != clusterName {
				return fmt.Errorf("node pool %q owner %q does not match generated cluster %q", name, resource.Spec.Owner.Name, clusterName)
			}
			key := ns + "/" + name
			if objects[key] {
				return fmt.Errorf("duplicate node pool Kubernetes object %q", key)
			}
			objects[key] = true
			azureName := resource.Spec.AzureName
			// ASO's defaultAzureName webhook uses metadata.name when azureName is omitted.
			if azureName == "" {
				azureName = name
			}
			if !generatedARONodePoolAzureName.MatchString(azureName) {
				return fmt.Errorf("node pool %q effective Azure name %q must be 1-%d characters, start with a letter, end with a letter/digit and contain only letters, digits or hyphens; set NAME_PREFIX (max %d characters for -mp1) and regenerate", name, azureName, MaxNodePoolNameLength, MaxNamePrefixLength)
			}
			if azureNames[azureName] {
				return fmt.Errorf("duplicate Azure node pool name %q for owner %q", azureName, clusterName)
			}
			azureNames[azureName] = true
		}
		if !found {
			return fmt.Errorf("AROMachinePool %q has no embedded HcpOpenShiftClustersNodePool", doc.Metadata.Name)
		}
	}
	if len(objects) == 0 {
		return fmt.Errorf("no embedded HcpOpenShiftClustersNodePool in ARO manifest")
	}
	references := 0
	for _, doc := range docs {
		if doc.Kind != "MachinePool" {
			continue
		}
		ref := doc.Spec.Template.Spec.InfrastructureRef
		if doc.Spec.ClusterName != clusterName || ref.Kind != "AROMachinePool" || !pools[namespace(doc.Metadata.Namespace)+"/"+ref.Name] {
			return fmt.Errorf("MachinePool %q infrastructureRef does not resolve to an AROMachinePool for generated cluster %q", doc.Metadata.Name, clusterName)
		}
		references++
	}
	if references == 0 {
		return fmt.Errorf("no MachinePool infrastructureRef in ARO manifest")
	}
	return nil
}
