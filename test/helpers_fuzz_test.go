package test

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTempFile(t *testing.T, content []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fuzz.yaml")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	return path
}

// --- Validation functions ---

func FuzzValidateRFC1123Name(f *testing.F) {
	f.Add("cate", "CAPI_USER")
	f.Add("my-cluster", "CS_CLUSTER_NAME")
	f.Add("stage", "DEPLOYMENT_ENV")
	f.Add("", "VAR")
	f.Add("UPPERCASE", "VAR")
	f.Add("-leading-dash", "VAR")
	f.Add("trailing-dash-", "VAR")
	f.Add("has space", "VAR")
	f.Add("has.dot", "VAR")
	f.Add("a", "VAR")
	f.Add("0", "VAR")
	f.Add("valid-name-123", "VAR")

	f.Fuzz(func(t *testing.T, name, varName string) {
		err := ValidateRFC1123Name(name, varName)

		if name == "" && err == nil {
			t.Error("empty name must return error")
		}
		if err != nil && err.Error() == "" {
			t.Error("non-nil error must have non-empty message")
		}
	})
}

func FuzzValidateDomainPrefix(f *testing.F) {
	f.Add("cate", "stage")
	f.Add("ab", "cd")
	f.Add("", "")
	f.Add("longusername", "longenvironment")
	f.Add("a", "b")
	f.Add("abcdefghijklmno", "x")

	f.Fuzz(func(t *testing.T, user, environment string) {
		err := ValidateDomainPrefix(user, environment)

		prefix := GetDomainPrefix(user, environment)
		if len(prefix) > MaxDomainPrefixLength && err == nil {
			t.Errorf("prefix %q (%d chars) exceeds max %d but no error returned",
				prefix, len(prefix), MaxDomainPrefixLength)
		}
		if len(prefix) <= MaxDomainPrefixLength && err != nil {
			t.Errorf("prefix %q (%d chars) within limit but got error: %v",
				prefix, len(prefix), err)
		}
	})
}

func FuzzValidateNamePrefix(f *testing.F) {
	f.Add("")
	f.Add("workers")
	f.Add("a")
	f.Add("abcdefghijk")
	f.Add("abcdefghijkl")
	f.Add("1invalid")
	f.Add("-invalid")
	f.Add("UPPER")

	f.Fuzz(func(t *testing.T, namePrefix string) {
		err := ValidateNamePrefix(namePrefix)

		if namePrefix == "" && err != nil {
			t.Error("empty namePrefix must return nil")
		}
		if err != nil && err.Error() == "" {
			t.Error("non-nil error must have non-empty message")
		}
	})
}

func FuzzValidateExternalAuthID(f *testing.F) {
	f.Add("cate-stage")
	f.Add("")
	f.Add("abcdefghijkl")
	f.Add("abcdefghijklm")
	f.Add("a")
	f.Add("exactly12ch")

	f.Fuzz(func(t *testing.T, clusterNamePrefix string) {
		err := ValidateExternalAuthID(clusterNamePrefix)

		id := GetExternalAuthID(clusterNamePrefix)
		if len(id) > MaxExternalAuthIDLength && err == nil {
			t.Errorf("ExternalAuth ID %q (%d chars) exceeds max %d but no error",
				id, len(id), MaxExternalAuthIDLength)
		}
		if len(id) <= MaxExternalAuthIDLength && err != nil {
			t.Errorf("ExternalAuth ID %q (%d chars) within limit but got error: %v",
				id, len(id), err)
		}
	})
}

// --- YAML parsing functions ---

func FuzzExtractClusterNameFromYAML(f *testing.F) {
	f.Add([]byte(""))
	f.Add([]byte("not yaml at all {{{"))
	f.Add([]byte("key: value"))
	f.Add([]byte("---\nkind: Cluster\napiVersion: cluster.x-k8s.io/v1beta1\nmetadata:\n  name: my-cluster\n"))
	f.Add([]byte("---\nkind: Deployment\napiVersion: apps/v1\nmetadata:\n  name: nginx\n"))
	f.Add([]byte("---\nkind: Cluster\napiVersion: other.io/v1\nmetadata:\n  name: wrong-api\n"))
	f.Add([]byte("---\n---\n---\n"))

	f.Fuzz(func(t *testing.T, content []byte) {
		path := writeTempFile(t, content)
		_, _ = ExtractClusterNameFromYAML(path)
	})
}

func FuzzExtractControlPlaneRefFromYAML(f *testing.F) {
	f.Add([]byte(""))
	f.Add([]byte("key: value"))
	f.Add([]byte("---\nkind: Cluster\napiVersion: cluster.x-k8s.io/v1beta1\nspec:\n  controlPlaneRef:\n    name: my-cp\n"))
	f.Add([]byte("---\nkind: Cluster\napiVersion: cluster.x-k8s.io/v1beta1\nmetadata:\n  name: test\n"))

	f.Fuzz(func(t *testing.T, content []byte) {
		path := writeTempFile(t, content)
		_, _ = ExtractControlPlaneRefFromYAML(path)
	})
}

func FuzzExtractMachinePoolNameFromYAML(f *testing.F) {
	f.Add([]byte(""))
	f.Add([]byte("key: value"))
	f.Add([]byte("---\nkind: MachinePool\napiVersion: cluster.x-k8s.io/v1beta1\nmetadata:\n  name: my-pool\n"))

	f.Fuzz(func(t *testing.T, content []byte) {
		path := writeTempFile(t, content)
		_, _ = ExtractMachinePoolNameFromYAML(path)
	})
}

func FuzzExtractNamespaceFromYAML(f *testing.F) {
	f.Add([]byte(""))
	f.Add([]byte("namespace: default"))
	f.Add([]byte("  namespace: kube-system\n"))
	f.Add([]byte("key: value\nno-namespace-here: true"))
	f.Add([]byte("---\nmetadata:\n  namespace: capz-test-20260101\n"))

	f.Fuzz(func(t *testing.T, content []byte) {
		path := writeTempFile(t, content)
		_, _ = ExtractNamespaceFromYAML(path)
	})
}

func FuzzValidateYAMLFile(f *testing.F) {
	f.Add([]byte("key: value"))
	f.Add([]byte(""))
	f.Add([]byte("   \n\t\n  "))
	f.Add([]byte("# just a comment"))
	f.Add([]byte("{invalid yaml: ["))
	f.Add([]byte("---\na: 1\n---\nb: 2\n"))
	f.Add([]byte("null"))

	f.Fuzz(func(t *testing.T, content []byte) {
		path := writeTempFile(t, content)
		_ = ValidateYAMLFile(path)
	})
}

// --- Log parsing ---

func FuzzParseControllerLogs(f *testing.F) {
	f.Add("")
	f.Add("normal log line without issues")
	f.Add("level=error msg=\"something failed\"")
	f.Add("level=warn msg=\"something suspicious\"")
	f.Add("{\"level\":\"error\",\"msg\":\"json error\"}")
	f.Add("{\"level\":\"warn\",\"msg\":\"json warn\"}")
	f.Add("error: connection refused")
	f.Add("warning: deprecated API")
	f.Add("error=nil status=ok")
	f.Add("line1\nline2\nlevel=error msg=fail\nline4")

	f.Fuzz(func(t *testing.T, logs string) {
		errors, warnings := ParseControllerLogs(logs)

		for _, e := range errors {
			if e == "" {
				t.Error("error entry must not be empty")
			}
		}
		for _, w := range warnings {
			if w == "" {
				t.Error("warning entry must not be empty")
			}
		}
	})
}
