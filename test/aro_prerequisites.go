package test

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// validateAROPrerequisiteInventory prevents an incomplete generated plan from
// bypassing the Ready checks simply because an entire required kind is absent.
func validateAROPrerequisiteInventory(plan *aroSplitPlan) error {
	required := []string{"resources.azure.com/ResourceGroup", "network.azure.com/VirtualNetwork", "network.azure.com/VirtualNetworksSubnet", "managedidentity.azure.com/UserAssignedIdentity"}
	found := make(map[string]bool)
	for _, resources := range [][]map[string]interface{}{plan.Prerequisites, plan.Identities} {
		for _, resource := range resources {
			api, _ := resource["apiVersion"].(string)
			kind, _ := resource["kind"].(string)
			found[strings.SplitN(api, "/", 2)[0]+"/"+kind] = true
		}
	}
	var missing []string
	for _, kind := range required {
		if !found[kind] {
			missing = append(missing, kind)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("incomplete Azure prerequisite inventory: missing %s", strings.Join(missing, ", "))
	}
	return nil
}

// missingInfrastructureDiagnostic accepts only a current, explicit CAPZ failure
// condition. AROControlPlane v1beta2 uses []metav1.Condition, including
// observedGeneration. A silent wait, generic Ready=False or stale error does not pass.
func missingInfrastructureDiagnostic(data string, infraName string) (string, error) {
	var object struct {
		Metadata struct {
			Generation int64 `json:"generation"`
		} `json:"metadata"`
		Status struct {
			Conditions []struct {
				Type, Status, Reason, Message string
				ObservedGeneration            int64 `json:"observedGeneration"`
			} `json:"conditions"`
		} `json:"status"`
	}
	if err := json.Unmarshal([]byte(data), &object); err != nil {
		return "", fmt.Errorf("parse AROControlPlane status: %w", err)
	}
	for _, condition := range object.Status.Conditions {
		if condition.Status != "False" || condition.Reason == "" || condition.Message == "" || object.Metadata.Generation <= 0 || condition.ObservedGeneration < object.Metadata.Generation {
			continue
		}
		text := strings.ToLower(condition.Reason + " " + condition.Message)
		missing := strings.Contains(text, "notfound") || strings.Contains(text, "not found") || strings.Contains(text, "missing")
		if missing && strings.Contains(text, "arocluster") && strings.Contains(condition.Message, infraName) {
			return fmt.Sprintf("%s=False reason=%s: %s", condition.Type, condition.Reason, condition.Message), nil
		}
	}
	return "", nil
}

// runAROMissingInfrastructureScenario deliberately omits AROCluster, so CAPZ
// cannot provision the infrastructure embedded in that object. It never runs as
// part of normal deployment and requires a fresh, explicitly prepared namespace.
func runAROMissingInfrastructureScenario(ctx context.Context, config *TestConfig, plan *aroSplitPlan, commands aroSplitCommands, dir string, poll time.Duration) (string, error) {
	var documents []map[string]interface{}
	var infraName, controlPlaneName string
	for _, document := range plan.Cluster {
		spec, _ := document["spec"].(map[string]interface{})
		metadata, _ := document["metadata"].(map[string]interface{})
		switch document["kind"] {
		case "Cluster":
			ref, _ := spec["infrastructureRef"].(map[string]interface{})
			infraName, _ = ref["name"].(string)
			if ref["kind"] != "AROCluster" {
				return "", fmt.Errorf("negative scenario requires Cluster.infrastructureRef.kind=AROCluster")
			}
			documents = append(documents, document)
		case "AROControlPlane":
			controlPlaneName, _ = metadata["name"].(string)
			documents = append(documents, document)
		}
	}
	if len(documents) != 2 || infraName == "" || controlPlaneName == "" {
		return "", fmt.Errorf("negative scenario requires exactly one named Cluster and AROControlPlane with an infrastructure reference")
	}
	path, err := writeSplitBundle(filepath.Join(dir, "missing-infrastructure"), "cluster-controlplane", documents)
	if err != nil {
		return "", err
	}
	// Refuse to modify existing workload resources or run alongside a deployment.
	for _, resource := range []string{capiClusterResource, "aroclusters.infrastructure.cluster.x-k8s.io", "arocontrolplanes.controlplane.cluster.x-k8s.io", "hcpopenshiftclusters.redhatopenshift.azure.com"} {
		output, err := commands.run(ctx, "get", resource, "-o", "name")
		if err != nil {
			return "", fmt.Errorf("negative scenario preflight %s: %w", resource, err)
		}
		if strings.TrimSpace(output) != "" {
			return "", fmt.Errorf("negative scenario requires an empty workload namespace: found %s", resource)
		}
	}
	if err := commands.apply(ctx, path); err != nil {
		return "", fmt.Errorf("apply negative scenario: %w", err)
	}
	for {
		if err := ctx.Err(); err != nil {
			return "", fmt.Errorf("CAPZ did not report a current missing-AROCluster diagnostic before deadline: %w", err)
		}
		// A successful diagnostic must never conceal an incorrectly created HCP.
		for _, resource := range []string{"aroclusters.infrastructure.cluster.x-k8s.io", "hcpopenshiftclusters.redhatopenshift.azure.com"} {
			output, err := commands.run(ctx, "get", resource, "-o", "name")
			if err != nil {
				return "", fmt.Errorf("observe negative scenario %s: %w", resource, err)
			}
			if strings.TrimSpace(output) != "" {
				return "", fmt.Errorf("unexpected %s appeared while infrastructure was deliberately omitted", resource)
			}
		}
		output, err := commands.run(ctx, "get", "arocontrolplanes.controlplane.cluster.x-k8s.io", controlPlaneName, "-o", "json")
		if err != nil {
			return "", fmt.Errorf("observe AROControlPlane: %w", err)
		}
		diagnostic, err := missingInfrastructureDiagnostic(output, infraName)
		if err != nil {
			return "", err
		}
		if diagnostic != "" {
			return diagnostic, nil
		}
		timer := time.NewTimer(poll)
		select {
		case <-ctx.Done():
			timer.Stop()
		case <-timer.C:
		}
	}
}
