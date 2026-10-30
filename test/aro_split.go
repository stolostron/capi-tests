package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const asoCredentialAnnotation = "serviceoperator.azure.com/credential-from"
const asoReconcileAnnotation = "serviceoperator.azure.com/reconcile-policy"

type aroSplitPlan struct {
	Prerequisites []map[string]interface{}
	Identities    []map[string]interface{}
	Roles         []map[string]interface{}
	Cluster       []map[string]interface{}
}

func readYAMLDocuments(data []byte) ([]map[string]interface{}, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var docs []map[string]interface{}
	for {
		var doc map[string]interface{}
		if err := decoder.Decode(&doc); err != nil {
			if err == io.EOF {
				return docs, nil
			}
			return nil, fmt.Errorf("decode manifest: %w", err)
		}
		if len(doc) > 0 {
			docs = append(docs, doc)
		}
	}
}

func copyResource(resource map[string]interface{}) (map[string]interface{}, error) {
	data, err := json.Marshal(resource)
	if err != nil {
		return nil, err
	}
	var copied map[string]interface{}
	err = json.Unmarshal(data, &copied)
	return copied, err
}

func bindASOCredential(resource map[string]interface{}, namespace, credential string, readOnly bool) error {
	metadata, ok := resource["metadata"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("%v lacks metadata", resource["kind"])
	}
	if name, _ := metadata["name"].(string); name == "" {
		return fmt.Errorf("%v lacks metadata.name", resource["kind"])
	}
	if ns, _ := metadata["namespace"].(string); ns != "" && ns != namespace {
		return fmt.Errorf("resource %v namespace %q differs from %q", metadata["name"], ns, namespace)
	}
	metadata["namespace"] = namespace
	annotations, _ := metadata["annotations"].(map[string]interface{})
	if annotations == nil {
		annotations = map[string]interface{}{}
		metadata["annotations"] = annotations
	}
	annotations[asoCredentialAnnotation] = credential
	if readOnly {
		annotations[asoReconcileAnnotation] = "skip"
	}
	return nil
}

// splitAROResources operates on real embedded ASO resources from installer aro.yaml.
// It preserves owner/ConfigMap references and the original manifest on disk.
func splitAROResources(data []byte, config *TestConfig) (*aroSplitPlan, error) {
	documents, err := readYAMLDocuments(data)
	if err != nil {
		return nil, err
	}
	plan := &aroSplitPlan{Cluster: documents}
	clusters, controlPlanes, machinePools, capiClusters := 0, 0, 0, 0
	for _, document := range documents {
		kind, _ := document["kind"].(string)
		api, _ := document["apiVersion"].(string)
		allowed := map[string]string{"Cluster": "cluster.x-k8s.io", "MachinePool": "cluster.x-k8s.io", "AROCluster": "infrastructure.cluster.x-k8s.io", "AROMachinePool": "infrastructure.cluster.x-k8s.io", "AROControlPlane": "controlplane.cluster.x-k8s.io"}
		if group, ok := allowed[kind]; !ok || strings.SplitN(api, "/", 2)[0] != group {
			return nil, fmt.Errorf("unsupported top-level resource %s %s", api, kind)
		}
		metadata, _ := document["metadata"].(map[string]interface{})
		if metadata["namespace"] != config.WorkloadClusterNamespace {
			return nil, fmt.Errorf("%s namespace must match workload namespace", kind)
		}
		if kind == "Cluster" {
			capiClusters++
		}
		if kind == "AROControlPlane" {
			controlPlanes++
		}
		if kind == "AROMachinePool" {
			machinePools++
		}
		if kind != "AROCluster" && kind != "AROControlPlane" && kind != "AROMachinePool" {
			continue
		}
		if kind == "AROCluster" {
			clusters++
		}
		spec, ok := document["spec"].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("%s lacks spec", kind)
		}
		resources, ok := spec["resources"].([]interface{})
		if !ok || len(resources) == 0 {
			return nil, fmt.Errorf("%s lacks embedded ASO resources", kind)
		}
		hcpResources, nodePoolResources := 0, 0
		for _, item := range resources {
			resource, ok := item.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid embedded resource in %s", kind)
			}
			api, _ := resource["apiVersion"].(string)
			resourceKind, _ := resource["kind"].(string)
			group := strings.SplitN(api, "/", 2)[0]
			if resourceKind == "HcpOpenShiftCluster" && group == "redhatopenshift.azure.com" {
				hcpResources++
			}
			if resourceKind == "HcpOpenShiftClustersNodePool" && group == "redhatopenshift.azure.com" {
				nodePoolResources++
			}
			if !strings.HasSuffix(group, ".azure.com") {
				return nil, fmt.Errorf("unsupported embedded resource API %q", api)
			}
			identity := resourceKind == "UserAssignedIdentity" && group == "managedidentity.azure.com"
			role := resourceKind == "RoleAssignment" && group == "authorization.azure.com"
			if (resourceKind == "RoleAssignment" && !role) || (resourceKind == "UserAssignedIdentity" && !identity) {
				return nil, fmt.Errorf("unexpected API group for %s", resourceKind)
			}
			if (identity || role) && kind != "AROCluster" {
				return nil, fmt.Errorf("%s must be in AROCluster resources for staged deployment", resourceKind)
			}
			if kind == "AROCluster" {
				prerequisites := map[string]bool{"resources.azure.com/ResourceGroup": true, "network.azure.com/VirtualNetwork": true, "network.azure.com/NetworkSecurityGroup": true, "network.azure.com/VirtualNetworksSubnet": true, "keyvault.azure.com/Vault": true}
				if !identity && !role && !prerequisites[group+"/"+resourceKind] {
					return nil, fmt.Errorf("unsupported ARO prerequisite %s/%s", group, resourceKind)
				}
				bootstrap, err := copyResource(resource)
				if err != nil {
					return nil, err
				}
				credential := config.ARODeploymentCredentialSecret
				readOnly := false
				if identity {
					credential = config.AROIdentityCredentialSecret
					if config.AROPreexistingIdentities {
						credential = config.ARODeploymentCredentialSecret
						readOnly = true
						identitySpec, _ := resource["spec"].(map[string]interface{})
						owner, _ := identitySpec["owner"].(map[string]interface{})
						armID, _ := owner["armId"].(string)
						if armID == "" || strings.HasSuffix(strings.ToLower(armID), "/resourcegroups/"+strings.ToLower(config.ResourceGroupName)) {
							return nil, fmt.Errorf("preexisting identity must reference an external resource group with owner.armId")
						}
					}
				} else if role {
					credential = config.ARORoleCredentialSecret
				}
				if err := bindASOCredential(bootstrap, config.WorkloadClusterNamespace, credential, readOnly); err != nil {
					return nil, err
				}
				switch {
				case identity:
					plan.Identities = append(plan.Identities, bootstrap)
				case role:
					plan.Roles = append(plan.Roles, bootstrap)
				default:
					plan.Prerequisites = append(plan.Prerequisites, bootstrap)
				}
			}
			// After setup, the deployment principal only reads identity/RBAC resources.
			if err := bindASOCredential(resource, config.WorkloadClusterNamespace, config.ARODeploymentCredentialSecret, identity || role); err != nil {
				return nil, err
			}
		}
		if kind == "AROControlPlane" && hcpResources != 1 {
			return nil, fmt.Errorf("AROControlPlane requires exactly one embedded HcpOpenShiftCluster")
		}
		if kind == "AROMachinePool" && nodePoolResources != 1 {
			return nil, fmt.Errorf("AROMachinePool requires exactly one embedded HcpOpenShiftClustersNodePool")
		}
	}
	if controlPlanes != 1 || machinePools == 0 || capiClusters != 1 {
		return nil, fmt.Errorf("split deployment requires one Cluster, one AROControlPlane, and at least one AROMachinePool")
	}
	if clusters != 1 || len(plan.Identities) == 0 || len(plan.Roles) == 0 {
		return nil, fmt.Errorf("split deployment requires one AROCluster with UserAssignedIdentity and RoleAssignment resources")
	}
	return plan, nil
}

func writeSplitBundle(dir, name string, documents []map[string]interface{}) (string, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	var data bytes.Buffer
	encoder := yaml.NewEncoder(&data)
	for _, document := range documents {
		if err := encoder.Encode(document); err != nil {
			return "", err
		}
	}
	if err := encoder.Close(); err != nil {
		return "", err
	}
	path := filepath.Join(dir, name+".yaml")
	if err := os.WriteFile(path, data.Bytes(), 0600); err != nil {
		return "", err
	}
	return path, nil
}
