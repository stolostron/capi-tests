package test

import (
	"reflect"
	"testing"
)

func TestManagementClusterFailureDiagnosticCommands(t *testing.T) {
	context := "kind-capz-tests-stage"
	namespaces := []string{"capi-system", "capz-system", "capz-system"}
	diagnostics := managementClusterFailureDiagnosticCommands(context, namespaces)

	want := []managementClusterFailureDiagnostic{
		{
			name: "Recent events in capi-system",
			args: []string{"--context", context, "-n", "capi-system", "get", "events", "--sort-by=.lastTimestamp"},
		},
		{
			name: "Recent events in capz-system",
			args: []string{"--context", context, "-n", "capz-system", "get", "events", "--sort-by=.lastTimestamp"},
		},
		{
			name: "Configured images in all Deployments",
			args: []string{
				"--context", context,
				"get", "deployments", "--all-namespaces", "-o",
				`jsonpath={range .items[*]}{.metadata.namespace}/{.metadata.name}{"\t"}{range .spec.template.spec.initContainers[*]}{.image}{" "}{end}{range .spec.template.spec.containers[*]}{.image}{" "}{end}{"\n"}{end}`,
			},
		},
	}

	if !reflect.DeepEqual(diagnostics, want) {
		t.Errorf("managementClusterFailureDiagnosticCommands(%q, controller namespaces) = %#v, want %#v", context, diagnostics, want)
	}
}
