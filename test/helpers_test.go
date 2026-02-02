package test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestIsKubectlApplySuccess(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected bool
	}{
		// Success cases
		{
			name:     "resource created",
			output:   "secret/my-secret created",
			expected: true,
		},
		{
			name:     "resource configured",
			output:   "secret/aso-credential configured",
			expected: true,
		},
		{
			name:     "resource unchanged",
			output:   "secret/cluster-identity-secret unchanged",
			expected: true,
		},
		{
			name:     "multiple resources mixed",
			output:   "secret/cluster-identity-secret unchanged\nsecret/aso-credential configured",
			expected: true,
		},
		{
			name:     "uppercase success indicator",
			output:   "Secret/my-secret CREATED",
			expected: true,
		},

		// Failure cases
		{
			name:     "error message",
			output:   "Error from server: secrets \"my-secret\" not found",
			expected: false,
		},
		{
			name:     "error with colon",
			output:   "error: unable to recognize file.yaml",
			expected: false,
		},
		{
			name:     "failed operation",
			output:   "failed to apply resource",
			expected: false,
		},
		{
			name:     "invalid resource",
			output:   "invalid object spec",
			expected: false,
		},
		{
			name:     "unable to connect",
			output:   "unable to connect to server",
			expected: false,
		},
		{
			name:     "warning message",
			output:   "Warning: resource will be deleted",
			expected: false,
		},
		{
			name:     "forbidden access",
			output:   "Error: forbidden - user does not have permission",
			expected: false,
		},
		{
			name:     "unauthorized",
			output:   "Error: Unauthorized",
			expected: false,
		},
		{
			name:     "not found",
			output:   "Error: not found",
			expected: false,
		},
		{
			name:     "empty output",
			output:   "",
			expected: false,
		},
		{
			name:     "whitespace only",
			output:   "   \n\t  ",
			expected: false,
		},
		{
			name:     "unexpected output",
			output:   "some random text with no indicators",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsKubectlApplySuccess(tt.output)
			if result != tt.expected {
				t.Errorf("IsKubectlApplySuccess(%q) = %v, expected %v", tt.output, result, tt.expected)
			}
		})
	}
}

func TestExtractClusterNameFromYAML(t *testing.T) {
	// Create temporary directory for test files
	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		setupFile   func(t *testing.T) string // Returns file path
		expected    string
		expectError bool
		errorMsg    string // Substring to match in error message
	}{
		{
			name: "valid cluster resource",
			setupFile: func(t *testing.T) string {
				path := filepath.Join(tmpDir, "valid-cluster.yaml")
				content := []byte(`---
apiVersion: cluster.x-k8s.io/v1beta2
kind: Cluster
metadata:
  name: mveber-stage
  namespace: default
`)
				if err := os.WriteFile(path, content, 0644); err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
				return path
			},
			expected:    "mveber-stage",
			expectError: false,
		},
		{
			name: "multi-document YAML with Cluster resource",
			setupFile: func(t *testing.T) string {
				path := filepath.Join(tmpDir, "multi-doc.yaml")
				content := []byte(`---
apiVersion: v1
kind: Secret
metadata:
  name: some-secret
---
apiVersion: cluster.x-k8s.io/v1beta2
kind: Cluster
metadata:
  name: my-cluster
  namespace: default
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureCluster
metadata:
  name: my-cluster
`)
				if err := os.WriteFile(path, content, 0644); err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
				return path
			},
			expected:    "my-cluster",
			expectError: false,
		},
		{
			name: "no Cluster resource in file",
			setupFile: func(t *testing.T) string {
				path := filepath.Join(tmpDir, "no-cluster.yaml")
				content := []byte(`---
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config
`)
				if err := os.WriteFile(path, content, 0644); err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
				return path
			},
			expected:    "",
			expectError: true,
			errorMsg:    "no Cluster resource found",
		},
		{
			name: "wrong apiVersion for Cluster",
			setupFile: func(t *testing.T) string {
				path := filepath.Join(tmpDir, "wrong-api.yaml")
				content := []byte(`---
apiVersion: v1
kind: Cluster
metadata:
  name: wrong-cluster
`)
				if err := os.WriteFile(path, content, 0644); err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
				return path
			},
			expected:    "",
			expectError: true,
			errorMsg:    "no Cluster resource found",
		},
		{
			name: "Cluster with v1beta1 apiVersion",
			setupFile: func(t *testing.T) string {
				path := filepath.Join(tmpDir, "v1beta1-cluster.yaml")
				content := []byte(`---
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: old-cluster
  namespace: default
`)
				if err := os.WriteFile(path, content, 0644); err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
				return path
			},
			expected:    "old-cluster",
			expectError: false,
		},
		{
			name: "empty file",
			setupFile: func(t *testing.T) string {
				path := filepath.Join(tmpDir, "empty.yaml")
				if err := os.WriteFile(path, []byte{}, 0644); err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
				return path
			},
			expected:    "",
			expectError: true,
			errorMsg:    "no Cluster resource found",
		},
		{
			name: "non-existent file",
			setupFile: func(t *testing.T) string {
				return filepath.Join(tmpDir, "does-not-exist.yaml")
			},
			expected:    "",
			expectError: true,
			errorMsg:    "file not accessible",
		},
		{
			name: "Cluster without metadata.name",
			setupFile: func(t *testing.T) string {
				path := filepath.Join(tmpDir, "no-name.yaml")
				content := []byte(`---
apiVersion: cluster.x-k8s.io/v1beta2
kind: Cluster
metadata:
  namespace: default
`)
				if err := os.WriteFile(path, content, 0644); err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
				return path
			},
			expected:    "",
			expectError: true,
			errorMsg:    "no Cluster resource found",
		},
		{
			name: "Cluster with empty metadata.name",
			setupFile: func(t *testing.T) string {
				path := filepath.Join(tmpDir, "empty-name.yaml")
				content := []byte(`---
apiVersion: cluster.x-k8s.io/v1beta2
kind: Cluster
metadata:
  name: ""
  namespace: default
`)
				if err := os.WriteFile(path, content, 0644); err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
				return path
			},
			expected:    "",
			expectError: true,
			errorMsg:    "no Cluster resource found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := tt.setupFile(t)
			result, err := ExtractClusterNameFromYAML(filePath)

			if tt.expectError {
				if err == nil {
					t.Errorf("ExtractClusterNameFromYAML(%q) expected error containing %q, got nil", filePath, tt.errorMsg)
					return
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("ExtractClusterNameFromYAML(%q) error = %q, expected to contain %q", filePath, err.Error(), tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ExtractClusterNameFromYAML(%q) unexpected error: %v", filePath, err)
					return
				}
				if result != tt.expected {
					t.Errorf("ExtractClusterNameFromYAML(%q) = %q, expected %q", filePath, result, tt.expected)
				}
			}
		})
	}
}

func TestCheckYAMLConfigMatch(t *testing.T) {
	// Create temporary directory for test files
	tmpDir := t.TempDir()

	tests := []struct {
		name           string
		setupFile      func(t *testing.T) string // Returns file path
		expectedPrefix string
		wantMatch      bool
		wantExisting   string
	}{
		{
			name: "matching prefix",
			setupFile: func(t *testing.T) string {
				path := filepath.Join(tmpDir, "matching.yaml")
				content := []byte(`---
apiVersion: cluster.x-k8s.io/v1beta2
kind: Cluster
metadata:
  name: rcapu-stage
  namespace: default
`)
				if err := os.WriteFile(path, content, 0644); err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
				return path
			},
			expectedPrefix: "rcapu-stage",
			wantMatch:      true,
			wantExisting:   "rcapu-stage",
		},
		{
			name: "mismatched prefix",
			setupFile: func(t *testing.T) string {
				path := filepath.Join(tmpDir, "mismatched.yaml")
				content := []byte(`---
apiVersion: cluster.x-k8s.io/v1beta2
kind: Cluster
metadata:
  name: rcapb-stage
  namespace: default
`)
				if err := os.WriteFile(path, content, 0644); err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
				return path
			},
			expectedPrefix: "rcapu-stage",
			wantMatch:      false,
			wantExisting:   "rcapb-stage",
		},
		{
			name: "missing file returns false",
			setupFile: func(t *testing.T) string {
				return filepath.Join(tmpDir, "nonexistent.yaml")
			},
			expectedPrefix: "rcapu-stage",
			wantMatch:      false,
			wantExisting:   "",
		},
		{
			name: "file without Cluster resource returns false",
			setupFile: func(t *testing.T) string {
				path := filepath.Join(tmpDir, "no-cluster.yaml")
				content := []byte(`---
apiVersion: v1
kind: ConfigMap
metadata:
  name: some-config
`)
				if err := os.WriteFile(path, content, 0644); err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
				return path
			},
			expectedPrefix: "rcapu-stage",
			wantMatch:      false,
			wantExisting:   "",
		},
		{
			name: "different environment suffix",
			setupFile: func(t *testing.T) string {
				path := filepath.Join(tmpDir, "diff-env.yaml")
				content := []byte(`---
apiVersion: cluster.x-k8s.io/v1beta2
kind: Cluster
metadata:
  name: rcapu-prod
  namespace: default
`)
				if err := os.WriteFile(path, content, 0644); err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
				return path
			},
			expectedPrefix: "rcapu-stage",
			wantMatch:      false,
			wantExisting:   "rcapu-prod",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := tt.setupFile(t)
			gotMatch, gotExisting := CheckYAMLConfigMatch(t, filePath, tt.expectedPrefix)

			if gotMatch != tt.wantMatch {
				t.Errorf("CheckYAMLConfigMatch() match = %v, want %v", gotMatch, tt.wantMatch)
			}
			if gotExisting != tt.wantExisting {
				t.Errorf("CheckYAMLConfigMatch() existing = %q, want %q", gotExisting, tt.wantExisting)
			}
		})
	}
}

func TestValidateYAMLFile(t *testing.T) {
	// Create temporary directory for test files
	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		setupFile   func(t *testing.T) string // Returns file path
		expectError bool
		errorMsg    string // Substring to match in error message
	}{
		{
			name: "valid YAML file",
			setupFile: func(t *testing.T) string {
				path := filepath.Join(tmpDir, "valid.yaml")
				content := []byte(`
apiVersion: v1
kind: Secret
metadata:
  name: test-secret
data:
  key: value
`)
				if err := os.WriteFile(path, content, 0644); err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
				return path
			},
			expectError: false,
		},
		{
			name: "valid YAML with multiple documents",
			setupFile: func(t *testing.T) string {
				path := filepath.Join(tmpDir, "multi.yaml")
				content := []byte(`
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: config1
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: config2
`)
				if err := os.WriteFile(path, content, 0644); err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
				return path
			},
			expectError: false,
		},
		{
			name: "empty file",
			setupFile: func(t *testing.T) string {
				path := filepath.Join(tmpDir, "empty.yaml")
				if err := os.WriteFile(path, []byte{}, 0644); err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
				return path
			},
			expectError: true,
			errorMsg:    "file is empty",
		},
		{
			name: "file with only whitespace",
			setupFile: func(t *testing.T) string {
				path := filepath.Join(tmpDir, "whitespace.yaml")
				content := []byte("   \n\n\t\t   \n  ")
				if err := os.WriteFile(path, content, 0644); err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
				return path
			},
			expectError: true,
			errorMsg:    "invalid YAML syntax", // Whitespace with tabs/mixed content triggers parsing error
		},
		{
			name: "file with only comments",
			setupFile: func(t *testing.T) string {
				path := filepath.Join(tmpDir, "comments.yaml")
				content := []byte(`# This is a comment
# Another comment
# No actual data
`)
				if err := os.WriteFile(path, content, 0644); err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
				return path
			},
			expectError: true,
			errorMsg:    "YAML file contains no data",
		},
		{
			name: "invalid YAML syntax - missing colon",
			setupFile: func(t *testing.T) string {
				path := filepath.Join(tmpDir, "invalid-syntax.yaml")
				content := []byte(`
apiVersion v1
kind: Secret
`)
				if err := os.WriteFile(path, content, 0644); err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
				return path
			},
			expectError: true,
			errorMsg:    "invalid YAML syntax",
		},
		{
			name: "invalid YAML syntax - bad indentation",
			setupFile: func(t *testing.T) string {
				path := filepath.Join(tmpDir, "bad-indent.yaml")
				content := []byte(`
metadata:
name: test
  namespace: default
`)
				if err := os.WriteFile(path, content, 0644); err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
				return path
			},
			expectError: true,
			errorMsg:    "invalid YAML syntax",
		},
		{
			name: "non-existent file",
			setupFile: func(t *testing.T) string {
				return filepath.Join(tmpDir, "does-not-exist.yaml")
			},
			expectError: true,
			errorMsg:    "file not accessible",
		},
		{
			name: "valid simple key-value YAML",
			setupFile: func(t *testing.T) string {
				path := filepath.Join(tmpDir, "simple.yaml")
				content := []byte(`
key1: value1
key2: value2
nested:
  subkey: subvalue
`)
				if err := os.WriteFile(path, content, 0644); err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
				return path
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := tt.setupFile(t)
			err := ValidateYAMLFile(filePath)

			if tt.expectError {
				if err == nil {
					t.Errorf("ValidateYAMLFile(%q) expected error containing %q, got nil", filePath, tt.errorMsg)
					return
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("ValidateYAMLFile(%q) error = %q, expected to contain %q", filePath, err.Error(), tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateYAMLFile(%q) unexpected error: %v", filePath, err)
				}
			}
		})
	}
}

func TestFormatAROControlPlaneConditions(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains []string // Substrings that should be in the output
	}{
		{
			name:     "empty input",
			input:    "",
			contains: []string{"(no conditions available)"},
		},
		{
			name:     "whitespace only",
			input:    "   \n\t  ",
			contains: []string{"(no conditions available)"},
		},
		{
			name:     "single condition - True status",
			input:    `[{"type":"Ready","status":"True"}]`,
			contains: []string{"✅", "Ready:", "True"},
		},
		{
			name:     "single condition - False status with reason",
			input:    `[{"type":"Ready","status":"False","reason":"Reconciling"}]`,
			contains: []string{"🔄", "Ready:", "False", "(Reconciling)"},
		},
		{
			name: "multiple conditions - mixed status",
			input: `[
				{"type":"Ready","status":"False","reason":"Reconciling"},
				{"type":"ResourceGroupReady","status":"True"},
				{"type":"VNetReady","status":"True"},
				{"type":"HcpClusterReady","status":"False","reason":"Provisioning"}
			]`,
			contains: []string{
				"Ready:", "False",
				"ResourceGroupReady:", "True", "✅",
				"VNetReady:", "True",
				"HcpClusterReady:", "False", "(Provisioning)",
			},
		},
		{
			name:     "full status object with conditions",
			input:    `{"conditions":[{"type":"Ready","status":"True"},{"type":"VNetReady","status":"True"}],"ready":true}`,
			contains: []string{"Ready:", "VNetReady:", "✅"},
		},
		{
			name:     "empty conditions array",
			input:    `[]`,
			contains: []string{"(no conditions available)"},
		},
		{
			name:     "invalid JSON",
			input:    `not valid json`,
			contains: []string{"(failed to parse conditions:"},
		},
		{
			name:     "condition with unknown status",
			input:    `[{"type":"SomeCondition","status":"Unknown"}]`,
			contains: []string{"⏳", "SomeCondition:", "Unknown"},
		},
		{
			name: "waiting condition - requires machine pool",
			input: `[{
				"type":"ExternalAuthReady",
				"status":"False",
				"reason":"ReconciliationFailed",
				"message":"external authentication requires at least one ready machine pool. Object will be requeued after 30ns"
			}]`,
			contains: []string{"⏳", "ExternalAuthReady:", "False", "(Waiting for machine pool)"},
		},
		{
			name: "waiting condition - will be requeued",
			input: `[{
				"type":"SomeCondition",
				"status":"False",
				"reason":"SomeReason",
				"message":"Resource is not ready. Will be requeued shortly."
			}]`,
			contains: []string{"⏳", "SomeCondition:", "False", "(Waiting (will retry))"},
		},
		{
			name: "waiting condition - not found",
			input: `[{
				"type":"DependencyReady",
				"status":"False",
				"reason":"ReconciliationFailed",
				"message":"Required resource not found in namespace"
			}]`,
			contains: []string{"⏳", "DependencyReady:", "False", "(Waiting for resource creation)"},
		},
		{
			name: "actual failure - no waiting pattern",
			input: `[{
				"type":"SomeCondition",
				"status":"False",
				"reason":"ActualError",
				"message":"Something went wrong unexpectedly"
			}]`,
			contains: []string{"🔄", "SomeCondition:", "False", "(ActualError)"},
		},
		{
			name: "mixed conditions - some waiting some not",
			input: `[
				{"type":"Ready","status":"False","reason":"Reconciling"},
				{"type":"ExternalAuthReady","status":"False","reason":"ReconciliationFailed","message":"requires at least one ready machine pool"},
				{"type":"HcpClusterReady","status":"True"}
			]`,
			contains: []string{
				"🔄", "Ready:", "(Reconciling)",
				"⏳", "ExternalAuthReady:", "(Waiting for machine pool)",
				"✅", "HcpClusterReady:",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatAROControlPlaneConditions(tt.input)

			for _, substr := range tt.contains {
				if !strings.Contains(result, substr) {
					t.Errorf("FormatAROControlPlaneConditions() result = %q, expected to contain %q", result, substr)
				}
			}
		})
	}
}

func TestIsWaitingCondition(t *testing.T) {
	tests := []struct {
		name            string
		condition       AROControlPlaneCondition
		expectedWaiting bool
		expectedDesc    string
	}{
		{
			name: "True status - never waiting",
			condition: AROControlPlaneCondition{
				Type:    "Ready",
				Status:  "True",
				Message: "requires at least one ready machine pool",
			},
			expectedWaiting: false,
			expectedDesc:    "",
		},
		{
			name: "False with machine pool message",
			condition: AROControlPlaneCondition{
				Type:    "ExternalAuthReady",
				Status:  "False",
				Reason:  "ReconciliationFailed",
				Message: "external authentication requires at least one ready machine pool",
			},
			expectedWaiting: true,
			expectedDesc:    "Waiting for machine pool",
		},
		{
			name: "False with requeue message",
			condition: AROControlPlaneCondition{
				Type:    "SomeCondition",
				Status:  "False",
				Reason:  "SomeReason",
				Message: "Object will be requeued after 30s",
			},
			expectedWaiting: true,
			expectedDesc:    "Waiting (will retry)",
		},
		{
			name: "False with not found message",
			condition: AROControlPlaneCondition{
				Type:    "ResourceReady",
				Status:  "False",
				Reason:  "Error",
				Message: "Dependency not found in namespace",
			},
			expectedWaiting: true,
			expectedDesc:    "Waiting for resource creation",
		},
		{
			name: "False without waiting pattern",
			condition: AROControlPlaneCondition{
				Type:    "Ready",
				Status:  "False",
				Reason:  "Error",
				Message: "Unexpected error occurred",
			},
			expectedWaiting: false,
			expectedDesc:    "",
		},
		{
			name: "False with empty message",
			condition: AROControlPlaneCondition{
				Type:   "Ready",
				Status: "False",
				Reason: "Error",
			},
			expectedWaiting: false,
			expectedDesc:    "",
		},
		{
			name: "Case insensitive matching",
			condition: AROControlPlaneCondition{
				Type:    "SomeCondition",
				Status:  "False",
				Reason:  "Error",
				Message: "WAITING FOR something to happen",
			},
			expectedWaiting: true,
			expectedDesc:    "Waiting for dependency",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isWaiting, desc := isWaitingCondition(tt.condition)
			if isWaiting != tt.expectedWaiting {
				t.Errorf("isWaitingCondition() waiting = %v, expected %v", isWaiting, tt.expectedWaiting)
			}
			if desc != tt.expectedDesc {
				t.Errorf("isWaitingCondition() desc = %q, expected %q", desc, tt.expectedDesc)
			}
		})
	}
}

func TestGetDomainPrefix(t *testing.T) {
	tests := []struct {
		name        string
		user        string
		environment string
		expected    string
	}{
		{
			name:        "short user and env",
			user:        "bob",
			environment: "dev",
			expected:    "bob-dev",
		},
		{
			name:        "longer user and env",
			user:        "radoslavcap",
			environment: "stage",
			expected:    "radoslavcap-stage",
		},
		{
			name:        "empty user",
			user:        "",
			environment: "prod",
			expected:    "-prod",
		},
		{
			name:        "empty environment",
			user:        "test",
			environment: "",
			expected:    "test-",
		},
		{
			name:        "both empty",
			user:        "",
			environment: "",
			expected:    "-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetDomainPrefix(tt.user, tt.environment)
			if result != tt.expected {
				t.Errorf("GetDomainPrefix(%q, %q) = %q, expected %q",
					tt.user, tt.environment, result, tt.expected)
			}
		})
	}
}

func TestValidateDomainPrefix(t *testing.T) {
	tests := []struct {
		name        string
		user        string
		environment string
		expectError bool
		errorMsgs   []string // Substrings to check in error message
	}{
		// Valid cases (15 chars or less)
		{
			name:        "exactly 15 chars",
			user:        "user1234567",
			environment: "dev",
			expectError: false, // "user1234567-dev" = 15 chars
		},
		{
			name:        "short prefix - 7 chars",
			user:        "bob",
			environment: "dev",
			expectError: false, // "bob-dev" = 7 chars
		},
		{
			name:        "short prefix - single chars",
			user:        "a",
			environment: "b",
			expectError: false, // "a-b" = 3 chars
		},
		{
			name:        "14 chars - just under limit",
			user:        "testuser12",
			environment: "dev",
			expectError: false, // "testuser12-dev" = 14 chars
		},

		// Invalid cases (over 15 chars)
		{
			name:        "16 chars - just over limit",
			user:        "radoslavcap",
			environment: "test",
			expectError: true, // "radoslavcap-test" = 16 chars
			errorMsgs:   []string{"exceeds maximum length", "16 chars", "15"},
		},
		{
			name:        "17 chars - original failing case",
			user:        "radoslavcap",
			environment: "stage",
			expectError: true, // "radoslavcap-stage" = 17 chars
			errorMsgs:   []string{"exceeds maximum length", "17 chars", "radoslavcap-stage"},
		},
		{
			name:        "very long prefix",
			user:        "verylongusername",
			environment: "production",
			expectError: true, // "verylongusername-production" = 27 chars
			errorMsgs:   []string{"exceeds maximum length", "27 chars", "Suggestion"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDomainPrefix(tt.user, tt.environment)

			if tt.expectError {
				if err == nil {
					prefix := GetDomainPrefix(tt.user, tt.environment)
					t.Errorf("ValidateDomainPrefix(%q, %q) expected error for prefix %q (%d chars), got nil",
						tt.user, tt.environment, prefix, len(prefix))
					return
				}
				// Check error message contains expected substrings
				for _, msg := range tt.errorMsgs {
					if !strings.Contains(err.Error(), msg) {
						t.Errorf("ValidateDomainPrefix(%q, %q) error = %q, expected to contain %q",
							tt.user, tt.environment, err.Error(), msg)
					}
				}
			} else {
				if err != nil {
					t.Errorf("ValidateDomainPrefix(%q, %q) unexpected error: %v",
						tt.user, tt.environment, err)
				}
			}
		})
	}
}

func TestValidateDomainPrefix_MaxLength(t *testing.T) {
	// Verify the MaxDomainPrefixLength constant is correct
	if MaxDomainPrefixLength != 15 {
		t.Errorf("MaxDomainPrefixLength = %d, expected 15", MaxDomainPrefixLength)
	}

	// Test boundary: exactly at the limit should pass
	// "12345678901-abc" = 15 chars (11 + 1 + 3)
	err := ValidateDomainPrefix("12345678901", "abc")
	if err != nil {
		t.Errorf("ValidateDomainPrefix at exactly 15 chars should pass, got error: %v", err)
	}

	// Test boundary: one char over should fail
	// "12345678901-abcd" = 16 chars (11 + 1 + 4)
	err = ValidateDomainPrefix("12345678901", "abcd")
	if err == nil {
		t.Error("ValidateDomainPrefix at 16 chars should fail, got nil")
	}
}

func TestValidateRFC1123Name(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		varName     string
		expectError bool
		errorMsgs   []string // Substrings to check in error message
	}{
		// Valid cases - RFC 1123 compliant names
		{
			name:        "simple lowercase name",
			value:       "rcap",
			varName:     "CAPZ_USER",
			expectError: false,
		},
		{
			name:        "name with hyphen",
			value:       "my-cluster",
			varName:     "CS_CLUSTER_NAME",
			expectError: false,
		},
		{
			name:        "name with numbers",
			value:       "cluster123",
			varName:     "CS_CLUSTER_NAME",
			expectError: false,
		},
		{
			name:        "single character",
			value:       "a",
			varName:     "CAPZ_USER",
			expectError: false,
		},
		{
			name:        "single digit",
			value:       "1",
			varName:     "DEPLOYMENT_ENV",
			expectError: false,
		},
		{
			name:        "complex valid name",
			value:       "my-test-cluster-123",
			varName:     "CS_CLUSTER_NAME",
			expectError: false,
		},
		{
			name:        "starts with number",
			value:       "123-cluster",
			varName:     "CS_CLUSTER_NAME",
			expectError: false,
		},

		// Invalid cases - RFC 1123 non-compliant names
		{
			name:        "contains uppercase - issue #288 case",
			value:       "rcapXYZ",
			varName:     "CAPZ_USER",
			expectError: true,
			errorMsgs:   []string{"not RFC 1123 compliant", "contains uppercase letters", "Suggested fix", "rcapxyz"},
		},
		{
			name:        "all uppercase",
			value:       "PRODUCTION",
			varName:     "DEPLOYMENT_ENV",
			expectError: true,
			errorMsgs:   []string{"contains uppercase letters", "production"},
		},
		{
			name:        "mixed case",
			value:       "MyCluster",
			varName:     "CS_CLUSTER_NAME",
			expectError: true,
			errorMsgs:   []string{"contains uppercase letters", "mycluster"},
		},
		{
			name:        "starts with hyphen",
			value:       "-invalid",
			varName:     "CAPZ_USER",
			expectError: true,
			errorMsgs:   []string{"must start with a lowercase alphanumeric character"},
		},
		{
			name:        "ends with hyphen",
			value:       "invalid-",
			varName:     "CAPZ_USER",
			expectError: true,
			errorMsgs:   []string{"must end with a lowercase alphanumeric character"},
		},
		{
			name:        "contains underscore",
			value:       "my_cluster",
			varName:     "CS_CLUSTER_NAME",
			expectError: true,
			errorMsgs:   []string{"contains invalid characters"},
		},
		{
			name:        "contains space",
			value:       "my cluster",
			varName:     "CS_CLUSTER_NAME",
			expectError: true,
			errorMsgs:   []string{"contains invalid characters"},
		},
		{
			name:        "contains dot",
			value:       "my.cluster",
			varName:     "CS_CLUSTER_NAME",
			expectError: true,
			errorMsgs:   []string{"contains invalid characters"},
		},
		{
			name:        "empty string",
			value:       "",
			varName:     "CAPZ_USER",
			expectError: true,
			errorMsgs:   []string{"is empty", "non-empty RFC 1123 compliant"},
		},
		{
			name:        "only hyphens",
			value:       "---",
			varName:     "DEPLOYMENT_ENV",
			expectError: true,
			errorMsgs:   []string{"must start with a lowercase alphanumeric character", "must end with a lowercase alphanumeric character"},
		},
		{
			name:        "special characters",
			value:       "test@cluster!",
			varName:     "CS_CLUSTER_NAME",
			expectError: true,
			errorMsgs:   []string{"contains invalid characters"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRFC1123Name(tt.value, tt.varName)

			if tt.expectError {
				if err == nil {
					t.Errorf("ValidateRFC1123Name(%q, %q) expected error, got nil", tt.value, tt.varName)
					return
				}
				// Check error message contains expected substrings
				for _, msg := range tt.errorMsgs {
					if !strings.Contains(err.Error(), msg) {
						t.Errorf("ValidateRFC1123Name(%q, %q) error = %q, expected to contain %q",
							tt.value, tt.varName, err.Error(), msg)
					}
				}
			} else {
				if err != nil {
					t.Errorf("ValidateRFC1123Name(%q, %q) unexpected error: %v", tt.value, tt.varName, err)
				}
			}
		})
	}
}

func TestRFC1123NameRegex(t *testing.T) {
	// Test the regex directly to ensure it matches the expected pattern
	validNames := []string{
		"a", "z", "0", "9",
		"ab", "a1", "1a", "12",
		"abc", "a-b", "a1b", "1a2",
		"my-cluster", "cluster-123", "a-b-c-d",
		"rcap-stage", "dev", "prod",
	}

	invalidNames := []string{
		"", "-", "A", "Z",
		"-a", "a-", "-ab", "ab-",
		"A-b", "a-B", "ABC",
		"a_b", "a.b", "a b", "a@b",
	}

	for _, name := range validNames {
		if !RFC1123NameRegex.MatchString(name) {
			t.Errorf("RFC1123NameRegex should match %q but didn't", name)
		}
	}

	for _, name := range invalidNames {
		if RFC1123NameRegex.MatchString(name) {
			t.Errorf("RFC1123NameRegex should not match %q but did", name)
		}
	}
}

func TestGetExternalAuthID(t *testing.T) {
	tests := []struct {
		name              string
		clusterNamePrefix string
		expected          string
	}{
		{
			name:              "short prefix",
			clusterNamePrefix: "rcap-stage",
			expected:          "rcap-stage-ea",
		},
		{
			name:              "exactly 12 chars prefix",
			clusterNamePrefix: "123456789012",
			expected:          "123456789012-ea",
		},
		{
			name:              "long prefix",
			clusterNamePrefix: "rcapxyz-stage",
			expected:          "rcapxyz-stage-ea",
		},
		{
			name:              "empty prefix",
			clusterNamePrefix: "",
			expected:          "-ea",
		},
		{
			name:              "single char prefix",
			clusterNamePrefix: "a",
			expected:          "a-ea",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetExternalAuthID(tt.clusterNamePrefix)
			if result != tt.expected {
				t.Errorf("GetExternalAuthID(%q) = %q, expected %q",
					tt.clusterNamePrefix, result, tt.expected)
			}
		})
	}
}

func TestValidateExternalAuthID(t *testing.T) {
	tests := []struct {
		name              string
		clusterNamePrefix string
		expectError       bool
		errorMsgs         []string // Substrings to check in error message
	}{
		// Valid cases (ExternalAuth ID ≤15 chars, so prefix ≤12 chars)
		{
			name:              "exactly 12 chars prefix - max valid",
			clusterNamePrefix: "123456789012",
			expectError:       false, // "123456789012-ea" = 15 chars
		},
		{
			name:              "short prefix - 10 chars",
			clusterNamePrefix: "rcap-stage",
			expectError:       false, // "rcap-stage-ea" = 13 chars
		},
		{
			name:              "single char prefix",
			clusterNamePrefix: "a",
			expectError:       false, // "a-ea" = 4 chars
		},
		{
			name:              "11 chars prefix",
			clusterNamePrefix: "12345678901",
			expectError:       false, // "12345678901-ea" = 14 chars
		},

		// Invalid cases (ExternalAuth ID >15 chars, so prefix >12 chars)
		{
			name:              "13 chars prefix - just over limit",
			clusterNamePrefix: "1234567890123",
			expectError:       true, // "1234567890123-ea" = 16 chars
			errorMsgs:         []string{"exceeds maximum length", "16 chars", "15"},
		},
		{
			name:              "original failing case - rcapxyz-stage",
			clusterNamePrefix: "rcapxyz-stage",
			expectError:       true, // "rcapxyz-stage-ea" = 16 chars
			errorMsgs:         []string{"exceeds maximum length", "16 chars", "rcapxyz-stage-ea", "Suggestion"},
		},
		{
			name:              "very long prefix",
			clusterNamePrefix: "verylongclustername",
			expectError:       true, // "verylongclustername-ea" = 22 chars
			errorMsgs:         []string{"exceeds maximum length", "22 chars", "CS_CLUSTER_NAME must be", "12"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateExternalAuthID(tt.clusterNamePrefix)

			if tt.expectError {
				if err == nil {
					externalAuthID := GetExternalAuthID(tt.clusterNamePrefix)
					t.Errorf("ValidateExternalAuthID(%q) expected error for ExternalAuth ID %q (%d chars), got nil",
						tt.clusterNamePrefix, externalAuthID, len(externalAuthID))
					return
				}
				// Check error message contains expected substrings
				for _, msg := range tt.errorMsgs {
					if !strings.Contains(err.Error(), msg) {
						t.Errorf("ValidateExternalAuthID(%q) error = %q, expected to contain %q",
							tt.clusterNamePrefix, err.Error(), msg)
					}
				}
			} else {
				if err != nil {
					t.Errorf("ValidateExternalAuthID(%q) unexpected error: %v",
						tt.clusterNamePrefix, err)
				}
			}
		})
	}
}

func TestValidateExternalAuthID_Constants(t *testing.T) {
	// Verify the constants are correctly defined
	if MaxExternalAuthIDLength != 15 {
		t.Errorf("MaxExternalAuthIDLength = %d, expected 15", MaxExternalAuthIDLength)
	}

	if ExternalAuthIDSuffix != "-ea" {
		t.Errorf("ExternalAuthIDSuffix = %q, expected \"-ea\"", ExternalAuthIDSuffix)
	}

	if MaxClusterNamePrefixLength != 12 {
		t.Errorf("MaxClusterNamePrefixLength = %d, expected 12 (15 - 3)", MaxClusterNamePrefixLength)
	}

	// Verify the relationship: MaxClusterNamePrefixLength + len(suffix) == MaxExternalAuthIDLength
	if MaxClusterNamePrefixLength+len(ExternalAuthIDSuffix) != MaxExternalAuthIDLength {
		t.Errorf("MaxClusterNamePrefixLength (%d) + len(ExternalAuthIDSuffix) (%d) != MaxExternalAuthIDLength (%d)",
			MaxClusterNamePrefixLength, len(ExternalAuthIDSuffix), MaxExternalAuthIDLength)
	}

	// Test boundary: exactly at the limit should pass
	// Prefix of 12 chars + "-ea" (3 chars) = 15 chars
	err := ValidateExternalAuthID("123456789012")
	if err != nil {
		t.Errorf("ValidateExternalAuthID with 12 char prefix should pass, got error: %v", err)
	}

	// Test boundary: one char over should fail
	// Prefix of 13 chars + "-ea" (3 chars) = 16 chars
	err = ValidateExternalAuthID("1234567890123")
	if err == nil {
		t.Error("ValidateExternalAuthID with 13 char prefix should fail, got nil")
	}
}

func TestIsRetryableKubectlError(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		err      error
		expected bool
	}{
		// Retryable errors - connection issues (from issue #265)
		{
			name:     "http2 client connection lost",
			output:   `error when retrieving current configuration: Get "https://127.0.0.1:51396/api/v1/namespaces/default/secrets/cluster-identity-secret": http2: client connection lost`,
			err:      fmt.Errorf("exit status 1"),
			expected: true,
		},
		{
			name:     "TLS handshake timeout",
			output:   `error when retrieving current configuration: Get "https://127.0.0.1:51396/api/v1/namespaces/default/secrets/aso-credential": net/http: TLS handshake timeout`,
			err:      fmt.Errorf("exit status 1"),
			expected: true,
		},
		{
			name:     "connection refused",
			output:   `The connection to the server localhost:8443 was refused - did you specify the right host or port?`,
			err:      fmt.Errorf("exit status 1"),
			expected: true,
		},
		{
			name:     "connection reset by peer",
			output:   `connection reset by peer`,
			err:      fmt.Errorf("exit status 1"),
			expected: true,
		},
		{
			name:     "i/o timeout",
			output:   `dial tcp 127.0.0.1:51396: i/o timeout`,
			err:      fmt.Errorf("exit status 1"),
			expected: true,
		},
		{
			name:     "context deadline exceeded",
			output:   `context deadline exceeded`,
			err:      fmt.Errorf("context deadline exceeded"),
			expected: true,
		},
		{
			name:     "server unavailable",
			output:   `Error from server: Service Unavailable`,
			err:      fmt.Errorf("exit status 1"),
			expected: true,
		},
		{
			name:     "gateway timeout",
			output:   `Error from server: gateway timeout`,
			err:      fmt.Errorf("exit status 1"),
			expected: true,
		},
		{
			name:     "too many requests",
			output:   `Error from server: too many requests`,
			err:      fmt.Errorf("exit status 1"),
			expected: true,
		},
		{
			name:     "internal server error",
			output:   `Error from server: internal server error`,
			err:      fmt.Errorf("exit status 1"),
			expected: true,
		},
		{
			name:     "dial tcp error",
			output:   `dial tcp: lookup kubernetes.default.svc.cluster.local: no such host`,
			err:      fmt.Errorf("exit status 1"),
			expected: true,
		},
		{
			name:     "temporary failure in name resolution",
			output:   `temporary failure in name resolution`,
			err:      fmt.Errorf("exit status 1"),
			expected: true,
		},
		{
			name:     "connection timed out",
			output:   `dial tcp 10.96.0.1:443: connection timed out`,
			err:      fmt.Errorf("exit status 1"),
			expected: true,
		},

		// Non-retryable errors - resource/validation issues
		{
			name:     "resource not found",
			output:   `Error from server (NotFound): secrets "my-secret" not found`,
			err:      fmt.Errorf("exit status 1"),
			expected: false,
		},
		{
			name:     "invalid YAML",
			output:   `error: error parsing yaml: error converting YAML to JSON`,
			err:      fmt.Errorf("exit status 1"),
			expected: false,
		},
		{
			name:     "forbidden - no permission",
			output:   `Error from server (Forbidden): secrets is forbidden: User "system:anonymous" cannot create resource`,
			err:      fmt.Errorf("exit status 1"),
			expected: false,
		},
		{
			name:     "already exists",
			output:   `Error from server (AlreadyExists): secrets "my-secret" already exists`,
			err:      fmt.Errorf("exit status 1"),
			expected: false,
		},
		{
			name:     "validation failed",
			output:   `The Secret "my-secret" is invalid: metadata.name: Invalid value`,
			err:      fmt.Errorf("exit status 1"),
			expected: false,
		},
		{
			name:     "generic error without patterns",
			output:   `something went wrong`,
			err:      fmt.Errorf("exit status 1"),
			expected: false,
		},

		// Edge cases
		{
			name:     "nil error",
			output:   `some output`,
			err:      nil,
			expected: false,
		},
		{
			name:     "empty output with error",
			output:   ``,
			err:      fmt.Errorf("connection refused"),
			expected: true, // Error message itself contains retryable pattern
		},
		{
			name:     "case insensitive - CONNECTION REFUSED",
			output:   `CONNECTION REFUSED`,
			err:      fmt.Errorf("error"),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRetryableKubectlError(tt.output, tt.err)
			if result != tt.expected {
				t.Errorf("isRetryableKubectlError(%q, %v) = %v, expected %v",
					tt.output, tt.err, result, tt.expected)
			}
		})
	}
}

func TestRetryConstants(t *testing.T) {
	// Verify retry constants have sensible values
	if DefaultHealthCheckTimeout < 30*time.Second {
		t.Errorf("DefaultHealthCheckTimeout = %v, expected at least 30s", DefaultHealthCheckTimeout)
	}

	if DefaultApplyMaxRetries < 3 {
		t.Errorf("DefaultApplyMaxRetries = %d, expected at least 3", DefaultApplyMaxRetries)
	}

	if DefaultApplyRetryDelay < 5*time.Second {
		t.Errorf("DefaultApplyRetryDelay = %v, expected at least 5s", DefaultApplyRetryDelay)
	}
}

func TestClusterPhaseConstants(t *testing.T) {
	// Verify cluster phase constants are correctly defined
	if ClusterPhaseProvisioned != "Provisioned" {
		t.Errorf("ClusterPhaseProvisioned = %q, expected \"Provisioned\"", ClusterPhaseProvisioned)
	}

	if ClusterPhaseProvisioning != "Provisioning" {
		t.Errorf("ClusterPhaseProvisioning = %q, expected \"Provisioning\"", ClusterPhaseProvisioning)
	}

	if ClusterPhaseFailed != "Failed" {
		t.Errorf("ClusterPhaseFailed = %q, expected \"Failed\"", ClusterPhaseFailed)
	}
}

func TestClusterReadyConstants(t *testing.T) {
	// Verify cluster ready timeout constants have sensible values
	if DefaultClusterReadyTimeout < 30*time.Minute {
		t.Errorf("DefaultClusterReadyTimeout = %v, expected at least 30m", DefaultClusterReadyTimeout)
	}

	if DefaultClusterReadyPollInterval < 10*time.Second {
		t.Errorf("DefaultClusterReadyPollInterval = %v, expected at least 10s", DefaultClusterReadyPollInterval)
	}

	// Verify poll interval is less than timeout
	if DefaultClusterReadyPollInterval >= DefaultClusterReadyTimeout {
		t.Errorf("DefaultClusterReadyPollInterval (%v) should be less than DefaultClusterReadyTimeout (%v)",
			DefaultClusterReadyPollInterval, DefaultClusterReadyTimeout)
	}
}

func TestExtractVersionFromImage(t *testing.T) {
	tests := []struct {
		name     string
		image    string
		expected string
	}{
		{
			name:     "standard image with version tag",
			image:    "mcr.microsoft.com/oss/azure/capz:v1.19.0",
			expected: "v1.19.0",
		},
		{
			name:     "image with numeric version",
			image:    "registry.k8s.io/cluster-api/cluster-api-controller:1.8.4",
			expected: "1.8.4",
		},
		{
			name:     "image with digest",
			image:    "mcr.microsoft.com/oss/azure/capz:v1.19.0@sha256:abc123",
			expected: "v1.19.0",
		},
		{
			name:     "image with only digest (no tag)",
			image:    "mcr.microsoft.com/oss/azure/capz@sha256:abc123",
			expected: "unknown",
		},
		{
			name:     "image with latest tag",
			image:    "registry.example.com/controller:latest",
			expected: "unknown",
		},
		{
			name:     "image without tag",
			image:    "mcr.microsoft.com/oss/azure/capz",
			expected: "unknown",
		},
		{
			name:     "image with port and version",
			image:    "localhost:5000/myimage:v2.3.4",
			expected: "v2.3.4",
		},
		{
			name:     "empty image",
			image:    "",
			expected: "unknown",
		},
		{
			name:     "image with pre-release version",
			image:    "registry.io/app:v1.2.3-alpha.1",
			expected: "v1.2.3-alpha.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractVersionFromImage(tt.image)
			if result != tt.expected {
				t.Errorf("extractVersionFromImage(%q) = %q, expected %q", tt.image, result, tt.expected)
			}
		})
	}
}

func TestFormatComponentVersions(t *testing.T) {
	tests := []struct {
		name     string
		versions []ComponentVersion
		checks   []string // Strings that should be present in output
	}{
		{
			name: "single component",
			versions: []ComponentVersion{
				{Name: "CAPZ", Version: "v1.19.0", Image: "mcr.microsoft.com/capz:v1.19.0"},
			},
			checks: []string{"CAPZ", "v1.19.0", "COMPONENT VERSIONS"},
		},
		{
			name: "multiple components",
			versions: []ComponentVersion{
				{Name: "CAPZ", Version: "v1.19.0", Image: "mcr.microsoft.com/capz:v1.19.0"},
				{Name: "ASO", Version: "v2.10.0", Image: "mcr.microsoft.com/aso:v2.10.0"},
			},
			checks: []string{"CAPZ", "v1.19.0", "ASO", "v2.10.0"},
		},
		{
			name: "component not found",
			versions: []ComponentVersion{
				{Name: "CAPI", Version: "not found", Image: "N/A"},
			},
			checks: []string{"CAPI", "not found"},
		},
		{
			name:     "empty versions",
			versions: []ComponentVersion{},
			checks:   []string{"COMPONENT VERSIONS"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatComponentVersions(tt.versions, nil)

			for _, check := range tt.checks {
				if !strings.Contains(result, check) {
					t.Errorf("FormatComponentVersions() output should contain %q, got:\n%s", check, result)
				}
			}
		})
	}

	// Test with full config
	t.Run("with config", func(t *testing.T) {
		versions := []ComponentVersion{
			{Name: "CAPZ", Version: "v1.19.0", Image: "mcr.microsoft.com/capz:v1.19.0"},
		}
		config := &TestConfig{
			ManagementClusterName: "test-mgmt",
			WorkloadClusterName:   "test-workload",
			Region:                "eastus",
			ClusterNamePrefix:     "test-prefix",
			OpenShiftVersion:      "4.21",
		}
		result := FormatComponentVersions(versions, config)
		checks := []string{
			"test-mgmt",
			"test-workload",
			"eastus",
			"test-prefix-resgroup",
			"4.21",
		}
		for _, check := range checks {
			if !strings.Contains(result, check) {
				t.Errorf("FormatComponentVersions() should contain %q, got:\n%s", check, result)
			}
		}
	})
}

func TestComponentVersionStruct(t *testing.T) {
	// Test that ComponentVersion struct can be properly created and used
	cv := ComponentVersion{
		Name:    "Test Component",
		Version: "v1.0.0",
		Image:   "test.io/image:v1.0.0",
	}

	if cv.Name != "Test Component" {
		t.Errorf("ComponentVersion.Name = %q, expected %q", cv.Name, "Test Component")
	}
	if cv.Version != "v1.0.0" {
		t.Errorf("ComponentVersion.Version = %q, expected %q", cv.Version, "v1.0.0")
	}
	if cv.Image != "test.io/image:v1.0.0" {
		t.Errorf("ComponentVersion.Image = %q, expected %q", cv.Image, "test.io/image:v1.0.0")
	}
}

func TestParseControllerLogs(t *testing.T) {
	tests := []struct {
		name             string
		logs             string
		expectedErrors   int
		expectedWarnings int
	}{
		{
			name: "no errors or warnings",
			logs: `info msg="Starting controller"
info msg="Controller started successfully"`,
			expectedErrors:   0,
			expectedWarnings: 0,
		},
		{
			name: "logrus style error",
			logs: `level=info msg="Starting controller"
level=error msg="Failed to connect"
level=info msg="Retrying..."`,
			expectedErrors:   1,
			expectedWarnings: 0,
		},
		{
			name: "JSON style error",
			logs: `{"level":"info","msg":"Starting controller"}
{"level":"error","msg":"Failed to connect"}
{"level":"info","msg":"Retrying..."}`,
			expectedErrors:   1,
			expectedWarnings: 0,
		},
		{
			name: "logrus style warning",
			logs: `level=info msg="Starting controller"
level=warn msg="Deprecated feature used"
level=info msg="Continuing..."`,
			expectedErrors:   0,
			expectedWarnings: 1,
		},
		{
			name: "JSON style warning",
			logs: `{"level":"info","msg":"Starting"}
{"level":"warn","msg":"Deprecated feature used"}`,
			expectedErrors:   0,
			expectedWarnings: 1,
		},
		{
			name: "mixed errors and warnings",
			logs: `level=info msg="Starting"
level=error msg="Error 1"
level=warn msg="Warning 1"
level=error msg="Error 2"
level=warn msg="Warning 2"
level=info msg="Done"`,
			expectedErrors:   2,
			expectedWarnings: 2,
		},
		{
			name: "error: prefix",
			logs: `info: Starting controller
error: Failed to connect
info: Retrying`,
			expectedErrors:   1,
			expectedWarnings: 0,
		},
		{
			name: "warning: prefix",
			logs: `info: Starting controller
warning: Deprecated feature
info: Continuing`,
			expectedErrors:   0,
			expectedWarnings: 1,
		},
		{
			name:             "empty logs",
			logs:             "",
			expectedErrors:   0,
			expectedWarnings: 0,
		},
		{
			name: "error=nil should not count as error",
			logs: `level=info msg="Completed" error=nil
level=info msg="Result" error=nil`,
			expectedErrors:   0,
			expectedWarnings: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors, warnings := ParseControllerLogs(tt.logs)
			if len(errors) != tt.expectedErrors {
				t.Errorf("errors count = %d, expected %d\nErrors: %v", len(errors), tt.expectedErrors, errors)
			}
			if len(warnings) != tt.expectedWarnings {
				t.Errorf("warnings count = %d, expected %d\nWarnings: %v", len(warnings), tt.expectedWarnings, warnings)
			}
		})
	}
}

func TestControllerLogSummaryStruct(t *testing.T) {
	summary := ControllerLogSummary{
		Name:       "CAPZ",
		Namespace:  "capz-system",
		Deployment: "capz-controller-manager",
		ErrorCount: 5,
		WarnCount:  10,
		Errors:     []string{"error 1", "error 2"},
		Warnings:   []string{"warning 1"},
		LogFile:    "/tmp/capz.log",
	}

	if summary.Name != "CAPZ" {
		t.Errorf("Name = %q, expected %q", summary.Name, "CAPZ")
	}
	if summary.ErrorCount != 5 {
		t.Errorf("ErrorCount = %d, expected %d", summary.ErrorCount, 5)
	}
	if len(summary.Errors) != 2 {
		t.Errorf("Errors length = %d, expected %d", len(summary.Errors), 2)
	}
}

func TestFormatControllerLogSummaries(t *testing.T) {
	summaries := []ControllerLogSummary{
		{
			Name:       "CAPI",
			ErrorCount: 0,
			WarnCount:  0,
		},
		{
			Name:       "CAPZ",
			ErrorCount: 2,
			WarnCount:  5,
			Errors:     []string{"error line 1", "error line 2"},
		},
		{
			Name:       "ASO",
			ErrorCount: 0,
			WarnCount:  3,
		},
	}

	output := FormatControllerLogSummaries(summaries)

	// Check header present
	if !strings.Contains(output, "CONTROLLER LOG SUMMARY") {
		t.Error("Output should contain 'CONTROLLER LOG SUMMARY' header")
	}

	// Check each controller is listed
	if !strings.Contains(output, "CAPI") {
		t.Error("Output should contain CAPI controller")
	}
	if !strings.Contains(output, "CAPZ") {
		t.Error("Output should contain CAPZ controller")
	}
	if !strings.Contains(output, "ASO") {
		t.Error("Output should contain ASO controller")
	}

	// Check totals
	if !strings.Contains(output, "2 errors") {
		t.Error("Output should show 2 errors total")
	}
	if !strings.Contains(output, "8 warnings") {
		t.Error("Output should show 8 warnings total")
	}
}

func TestFormatControllerLogSummaries_NoIssues(t *testing.T) {
	summaries := []ControllerLogSummary{
		{Name: "CAPI", ErrorCount: 0, WarnCount: 0},
		{Name: "CAPZ", ErrorCount: 0, WarnCount: 0},
	}

	output := FormatControllerLogSummaries(summaries)

	if !strings.Contains(output, "0 errors") {
		t.Error("Output should show 0 errors")
	}
	if !strings.Contains(output, "without errors or warnings") {
		t.Error("Output should indicate no issues found")
	}
}

func TestGetResultsDir(t *testing.T) {
	// This is a basic test - the function should always return a valid path
	dir := GetResultsDir()

	if dir == "" {
		t.Error("GetResultsDir should not return empty string")
	}

	// Should be a valid path format
	if !filepath.IsAbs(dir) && !strings.HasPrefix(dir, "results/") {
		t.Errorf("GetResultsDir returned unexpected path format: %s", dir)
	}
}

func TestDetectAzureError(t *testing.T) {
	tests := []struct {
		name           string
		output         string
		expectedType   string
		expectNil      bool
		checkRemediate bool
	}{
		// Insufficient privileges error (from issue #223)
		{
			name: "insufficient privileges - service principal creation",
			output: `Creating SP for RBAC with name rcap-sp-149357424, with role Custom-Owner (Block Billing and Subscription deletion) and in scopes /subscriptions/b23756f7-4594-40a3-980f-10bb6168fc20
Insufficient privileges to complete the operation.`,
			expectedType:   "insufficient_privileges",
			expectNil:      false,
			checkRemediate: true,
		},
		{
			name:           "insufficient privileges - lowercase",
			output:         "error: insufficient privileges to perform this action",
			expectedType:   "insufficient_privileges",
			expectNil:      false,
			checkRemediate: true,
		},

		// Authorization failed errors
		{
			name:           "authorization failed - camelcase",
			output:         "AuthorizationFailed: The client does not have authorization to perform action",
			expectedType:   "authorization_failed",
			expectNil:      false,
			checkRemediate: true,
		},
		{
			name:           "authorization failed - spaces",
			output:         "Error: Authorization failed for subscription",
			expectedType:   "authorization_failed",
			expectNil:      false,
			checkRemediate: true,
		},
		{
			name:           "does not have authorization",
			output:         "The user does not have authorization to perform this operation",
			expectedType:   "authorization_failed",
			expectNil:      false,
			checkRemediate: true,
		},

		// Subscription not found
		{
			name:           "subscription not found - camelcase",
			output:         "SubscriptionNotFound: The subscription 'xyz' could not be found",
			expectedType:   "subscription_not_found",
			expectNil:      false,
			checkRemediate: true,
		},
		{
			name:           "subscription not found - spaces",
			output:         "Error: subscription not found in tenant",
			expectedType:   "subscription_not_found",
			expectNil:      false,
			checkRemediate: true,
		},
		{
			name:           "subscription was not found",
			output:         "The subscription was not found or you don't have access",
			expectedType:   "subscription_not_found",
			expectNil:      false,
			checkRemediate: true,
		},

		// Resource group not found
		{
			name:           "resource group not found - camelcase",
			output:         "ResourceGroupNotFound: Resource group 'mygroup' could not be found",
			expectedType:   "resource_group_not_found",
			expectNil:      false,
			checkRemediate: true,
		},
		{
			name:           "resource group not found - spaces",
			output:         "Error: The resource group 'test-rg' was not found",
			expectedType:   "resource_group_not_found",
			expectNil:      false,
			checkRemediate: true,
		},

		// Quota exceeded
		{
			name:           "quota exceeded - camelcase",
			output:         "QuotaExceeded: Operation could not be completed as it exceeds quota",
			expectedType:   "quota_exceeded",
			expectNil:      false,
			checkRemediate: true,
		},
		{
			name:           "quota exceeded - spaces",
			output:         "Error: quota exceeded for resource type in region",
			expectedType:   "quota_exceeded",
			expectNil:      false,
			checkRemediate: true,
		},
		{
			name:           "exceeds quota",
			output:         "The operation exceeds quota limits for VM cores",
			expectedType:   "quota_exceeded",
			expectNil:      false,
			checkRemediate: true,
		},

		// Service principal already exists
		{
			name:           "service principal already exists",
			output:         "A service principal with this name already exists in the directory",
			expectedType:   "sp_already_exists",
			expectNil:      false,
			checkRemediate: true,
		},

		// Invalid credentials
		{
			name:           "invalid client secret",
			output:         "AADSTS7000215: Invalid client secret provided",
			expectedType:   "invalid_credentials",
			expectNil:      false,
			checkRemediate: true,
		},
		{
			name:           "invalid_client error code",
			output:         "error: invalid_client - the client credentials are not valid",
			expectedType:   "invalid_credentials",
			expectNil:      false,
			checkRemediate: true,
		},
		{
			name:           "credentials have expired",
			output:         "The credentials have expired. Please re-authenticate.",
			expectedType:   "invalid_credentials",
			expectNil:      false,
			checkRemediate: true,
		},

		// Not logged in
		{
			name:           "please run az login",
			output:         "ERROR: Please run 'az login' to setup account.",
			expectedType:   "not_logged_in",
			expectNil:      false,
			checkRemediate: true,
		},
		{
			name:           "not logged in",
			output:         "Error: You are not logged in to Azure CLI",
			expectedType:   "not_logged_in",
			expectNil:      false,
			checkRemediate: true,
		},
		{
			name:           "no subscription found",
			output:         "ERROR: No subscription found. Run 'az account set' to select a subscription.",
			expectedType:   "not_logged_in",
			expectNil:      false,
			checkRemediate: true,
		},

		// No error detected
		{
			name:         "no azure error - success output",
			output:       "Successfully created resource group 'test-rg' in location 'eastus'",
			expectedType: "",
			expectNil:    true,
		},
		{
			name:         "no azure error - empty output",
			output:       "",
			expectedType: "",
			expectNil:    true,
		},
		{
			name:         "no azure error - generic error",
			output:       "Some random error occurred",
			expectedType: "",
			expectNil:    true,
		},
		{
			name:         "no azure error - kubernetes error",
			output:       "Error from server (NotFound): pods not found",
			expectedType: "",
			expectNil:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := DetectAzureError(tc.output)

			if tc.expectNil {
				if result != nil {
					t.Errorf("Expected nil but got error type: %s", result.ErrorType)
				}
				return
			}

			if result == nil {
				t.Errorf("Expected error type '%s' but got nil", tc.expectedType)
				return
			}

			if result.ErrorType != tc.expectedType {
				t.Errorf("Expected error type '%s' but got '%s'", tc.expectedType, result.ErrorType)
			}

			if result.Message == "" {
				t.Error("Expected non-empty error message")
			}

			if tc.checkRemediate && len(result.Remediation) == 0 {
				t.Error("Expected non-empty remediation steps")
			}
		})
	}
}

func TestFormatAzureError(t *testing.T) {
	tests := []struct {
		name          string
		info          *AzureErrorInfo
		expectEmpty   bool
		expectedParts []string
	}{
		{
			name:        "nil input returns empty string",
			info:        nil,
			expectEmpty: true,
		},
		{
			name: "formats insufficient privileges error",
			info: &AzureErrorInfo{
				ErrorType: "insufficient_privileges",
				Message:   "Azure operation failed due to insufficient privileges",
				Remediation: []string{
					"Verify you have the required Azure AD role",
					"Contact your Azure AD administrator",
				},
			},
			expectEmpty: false,
			expectedParts: []string{
				"Azure Error Detected",
				"insufficient privileges",
				"Remediation steps:",
				"Azure AD role",
				"administrator",
			},
		},
		{
			name: "formats error with empty remediation",
			info: &AzureErrorInfo{
				ErrorType:   "test_error",
				Message:     "Test error message",
				Remediation: []string{},
			},
			expectEmpty: false,
			expectedParts: []string{
				"Azure Error Detected",
				"Test error message",
				"Remediation steps:",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := FormatAzureError(tc.info)

			if tc.expectEmpty {
				if result != "" {
					t.Errorf("Expected empty string but got: %s", result)
				}
				return
			}

			if result == "" {
				t.Error("Expected non-empty formatted string")
				return
			}

			for _, part := range tc.expectedParts {
				if !strings.Contains(result, part) {
					t.Errorf("Expected formatted output to contain '%s', got: %s", part, result)
				}
			}
		})
	}
}

// TestHasServicePrincipalCredentials tests the HasServicePrincipalCredentials function.
func TestHasServicePrincipalCredentials(t *testing.T) {
	tests := []struct {
		name         string
		clientID     string
		clientSecret string
		tenantID     string
		expected     bool
	}{
		{
			name:         "all credentials set",
			clientID:     "test-client-id",
			clientSecret: "test-secret",
			tenantID:     "test-tenant-id",
			expected:     true,
		},
		{
			name:         "missing client id",
			clientID:     "",
			clientSecret: "test-secret",
			tenantID:     "test-tenant-id",
			expected:     false,
		},
		{
			name:         "missing client secret",
			clientID:     "test-client-id",
			clientSecret: "",
			tenantID:     "test-tenant-id",
			expected:     false,
		},
		{
			name:         "missing tenant id",
			clientID:     "test-client-id",
			clientSecret: "test-secret",
			tenantID:     "",
			expected:     false,
		},
		{
			name:         "all empty",
			clientID:     "",
			clientSecret: "",
			tenantID:     "",
			expected:     false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Save original values
			origClientID := os.Getenv("AZURE_CLIENT_ID")
			origClientSecret := os.Getenv("AZURE_CLIENT_SECRET")
			origTenantID := os.Getenv("AZURE_TENANT_ID")

			// Restore original values after test
			t.Cleanup(func() {
				_ = os.Setenv("AZURE_CLIENT_ID", origClientID)
				_ = os.Setenv("AZURE_CLIENT_SECRET", origClientSecret)
				_ = os.Setenv("AZURE_TENANT_ID", origTenantID)
			})

			// Set test values
			if tc.clientID != "" {
				_ = os.Setenv("AZURE_CLIENT_ID", tc.clientID)
			} else {
				_ = os.Unsetenv("AZURE_CLIENT_ID")
			}
			if tc.clientSecret != "" {
				_ = os.Setenv("AZURE_CLIENT_SECRET", tc.clientSecret)
			} else {
				_ = os.Unsetenv("AZURE_CLIENT_SECRET")
			}
			if tc.tenantID != "" {
				_ = os.Setenv("AZURE_TENANT_ID", tc.tenantID)
			} else {
				_ = os.Unsetenv("AZURE_TENANT_ID")
			}

			result := HasServicePrincipalCredentials()
			if result != tc.expected {
				t.Errorf("HasServicePrincipalCredentials() = %v, expected %v", result, tc.expected)
			}
		})
	}
}

// TestGetAzureAuthDescription tests the GetAzureAuthDescription function.
func TestGetAzureAuthDescription(t *testing.T) {
	tests := []struct {
		mode     AzureAuthMode
		expected string
	}{
		{
			mode:     AzureAuthModeServicePrincipal,
			expected: "service principal (AZURE_CLIENT_ID/AZURE_CLIENT_SECRET)",
		},
		{
			mode:     AzureAuthModeCLI,
			expected: "Azure CLI (az login)",
		},
		{
			mode:     AzureAuthModeNone,
			expected: "no authentication",
		},
		{
			mode:     AzureAuthMode("unknown"),
			expected: "no authentication",
		},
	}

	for _, tc := range tests {
		t.Run(string(tc.mode), func(t *testing.T) {
			result := GetAzureAuthDescription(tc.mode)
			if result != tc.expected {
				t.Errorf("GetAzureAuthDescription(%q) = %q, expected %q", tc.mode, result, tc.expected)
			}
		})
	}
}

// TestAzureAuthModeConstants validates the authentication mode constants are correctly defined.
func TestAzureAuthModeConstants(t *testing.T) {
	// Verify constant values are as expected
	if AzureAuthModeServicePrincipal != "service-principal" {
		t.Errorf("AzureAuthModeServicePrincipal = %q, expected 'service-principal'", AzureAuthModeServicePrincipal)
	}
	if AzureAuthModeCLI != "cli" {
		t.Errorf("AzureAuthModeCLI = %q, expected 'cli'", AzureAuthModeCLI)
	}
	if AzureAuthModeNone != "none" {
		t.Errorf("AzureAuthModeNone = %q, expected 'none'", AzureAuthModeNone)
	}
}

// TestClonedRepositoryTracking tests the cloned repository tracking functionality.
func TestClonedRepositoryTracking(t *testing.T) {
	// Clear any existing repos first
	ClearClonedRepositories()

	// Initially should be empty
	repos := GetClonedRepositories()
	if len(repos) != 0 {
		t.Errorf("Expected 0 repos after clear, got %d", len(repos))
	}

	// Register a repository
	RegisterClonedRepository("https://github.com/test/repo1", "main", "/tmp/repo1")
	repos = GetClonedRepositories()
	if len(repos) != 1 {
		t.Fatalf("Expected 1 repo, got %d", len(repos))
	}
	if repos[0].URL != "https://github.com/test/repo1" {
		t.Errorf("Expected URL 'https://github.com/test/repo1', got %q", repos[0].URL)
	}
	if repos[0].Branch != "main" {
		t.Errorf("Expected branch 'main', got %q", repos[0].Branch)
	}

	// Register same repo again - should not create duplicate
	RegisterClonedRepository("https://github.com/test/repo1", "main", "/tmp/repo1")
	repos = GetClonedRepositories()
	if len(repos) != 1 {
		t.Errorf("Expected 1 repo (no duplicate), got %d", len(repos))
	}

	// Register a different repo
	RegisterClonedRepository("https://github.com/test/repo2", "dev", "/tmp/repo2")
	repos = GetClonedRepositories()
	if len(repos) != 2 {
		t.Errorf("Expected 2 repos, got %d", len(repos))
	}

	// Clear and verify
	ClearClonedRepositories()
	repos = GetClonedRepositories()
	if len(repos) != 0 {
		t.Errorf("Expected 0 repos after clear, got %d", len(repos))
	}
}

// TestFormatComponentVersions_WithRepositories tests that FormatComponentVersions includes repository info.
func TestFormatComponentVersions_WithRepositories(t *testing.T) {
	// Clear and set up test repositories
	ClearClonedRepositories()
	RegisterClonedRepository("https://github.com/RadekCap/cluster-api-installer", "ARO-ASO", "/tmp/capi")

	versions := []ComponentVersion{
		{Name: "CAPZ", Version: "v1.19.0", Image: "mcr.microsoft.com/capz:v1.19.0"},
	}

	result := FormatComponentVersions(versions, nil)

	// Check for repository section
	if !strings.Contains(result, "USED REPOSITORIES") {
		t.Error("Output should contain USED REPOSITORIES section")
	}
	if !strings.Contains(result, "https://github.com/RadekCap/cluster-api-installer") {
		t.Error("Output should contain repository URL")
	}
	if !strings.Contains(result, "Branch: ARO-ASO") {
		t.Error("Output should contain branch name")
	}

	// Clean up
	ClearClonedRepositories()
}

// TestFormatComponentVersions_ConfigFallback tests that FormatComponentVersions
// falls back to config values when no repositories are registered in memory.
// This is critical for cross-process test execution where each phase runs
// in a separate go test process.
func TestFormatComponentVersions_ConfigFallback(t *testing.T) {
	// Clear any registered repositories to simulate fresh process
	ClearClonedRepositories()

	// Create config with repository info (as would be available in verification phase)
	config := &TestConfig{
		RepoURL:               "https://github.com/RadekCap/cluster-api-installer",
		RepoBranch:            "ARO-ASO",
		ManagementClusterName: "test-cluster",
		WorkloadClusterName:   "workload-cluster",
		Region:                "eastus",
		ClusterNamePrefix:     "test",
		OpenShiftVersion:      "4.21",
	}

	versions := []ComponentVersion{
		{Name: "CAPZ", Version: "v1.19.0", Image: "mcr.microsoft.com/capz:v1.19.0"},
	}

	result := FormatComponentVersions(versions, config)

	// Check for repository section from config fallback
	if !strings.Contains(result, "USED REPOSITORIES") {
		t.Error("Output should contain USED REPOSITORIES section from config fallback")
	}
	if !strings.Contains(result, "https://github.com/RadekCap/cluster-api-installer") {
		t.Error("Output should contain repository URL from config")
	}
	if !strings.Contains(result, "Branch: ARO-ASO") {
		t.Error("Output should contain branch name from config")
	}
}

// ============================================================================
// Configuration Validation Tests (Issue #396)
// ============================================================================

// TestValidateAzureRegion tests the Azure region validation function.
func TestValidateAzureRegion(t *testing.T) {
	tests := []struct {
		name        string
		region      string
		expectError bool
		errorMsgs   []string // Substrings to check in error message
	}{
		// Valid regions (from the known list)
		{
			name:        "valid region - eastus",
			region:      "eastus",
			expectError: false,
		},
		{
			name:        "valid region - westeurope",
			region:      "westeurope",
			expectError: false,
		},
		{
			name:        "valid region - uksouth",
			region:      "uksouth",
			expectError: false,
		},
		{
			name:        "valid region - uppercase (normalized)",
			region:      "EASTUS",
			expectError: false, // Should be normalized to lowercase
		},
		{
			name:        "valid region - mixed case",
			region:      "EastUS",
			expectError: false, // Should be normalized to lowercase
		},
		{
			name:        "valid region - australiaeast",
			region:      "australiaeast",
			expectError: false,
		},

		// Invalid regions
		{
			name:        "empty region",
			region:      "",
			expectError: true,
			errorMsgs:   []string{"REGION is empty", "To fix this"},
		},
		{
			name:        "invalid region - typo",
			region:      "eastuss",
			expectError: true,
			errorMsgs:   []string{"not a valid Azure region", "To fix this"},
		},
		{
			name:        "invalid region - made up",
			region:      "neverland",
			expectError: true,
			errorMsgs:   []string{"not a valid Azure region", "Common regions"},
		},
		{
			name:        "invalid region - with space",
			region:      "east us",
			expectError: true,
			errorMsgs:   []string{"not a valid Azure region", "To fix this"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAzureRegion(t, tt.region)

			if tt.expectError {
				if err == nil {
					t.Errorf("ValidateAzureRegion(t, %q) expected error, got nil", tt.region)
					return
				}
				for _, msg := range tt.errorMsgs {
					if !strings.Contains(err.Error(), msg) {
						t.Errorf("ValidateAzureRegion(t, %q) error = %q, expected to contain %q",
							tt.region, err.Error(), msg)
					}
				}
			} else {
				if err != nil {
					t.Errorf("ValidateAzureRegion(t, %q) unexpected error: %v", tt.region, err)
				}
			}
		})
	}
}

// TestAzureRegionsMap tests that the azureRegions map contains expected regions.
func TestAzureRegionsMap(t *testing.T) {
	// Verify some expected regions are in the map
	expectedRegions := []string{
		"eastus", "eastus2", "westus", "westus2",
		"northeurope", "westeurope", "uksouth",
		"eastasia", "southeastasia", "australiaeast",
	}

	for _, region := range expectedRegions {
		if !azureRegions[region] {
			t.Errorf("Expected region '%s' to be in azureRegions map", region)
		}
	}

	// Verify map is not empty
	if len(azureRegions) < 20 {
		t.Errorf("Expected at least 20 regions in azureRegions map, got %d", len(azureRegions))
	}
}

// TestFindSimilarRegions tests the region suggestion function.
func TestFindSimilarRegions(t *testing.T) {
	regions := []string{"eastus", "eastus2", "westus", "westus2", "westeurope", "northeurope"}

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "find regions containing east",
			input:    "east",
			expected: []string{"eastus", "eastus2"},
		},
		{
			name:     "find regions containing europe",
			input:    "europe",
			expected: []string{"westeurope", "northeurope"},
		},
		{
			name:     "no matches",
			input:    "xyz",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findSimilarRegions(tt.input, regions)

			// Check that expected results are found
			for _, exp := range tt.expected {
				found := false
				for _, res := range result {
					if res == exp {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("findSimilarRegions(%q, regions) expected to contain %q, got %v",
						tt.input, exp, result)
				}
			}
		})
	}

	// Test that results are limited to 3
	t.Run("limits to 3 suggestions", func(t *testing.T) {
		manyRegions := []string{"eus1", "eus2", "eus3", "eus4", "eus5"}
		result := findSimilarRegions("eus", manyRegions)
		if len(result) > 3 {
			t.Errorf("findSimilarRegions should return at most 3 suggestions, got %d", len(result))
		}
	})
}

// TestValidateTimeout tests the generic timeout validation function.
func TestValidateTimeout(t *testing.T) {
	tests := []struct {
		name        string
		timeout     time.Duration
		min         time.Duration
		max         time.Duration
		expectError bool
		errorMsgs   []string
	}{
		// Valid cases
		{
			name:        "within range",
			timeout:     30 * time.Minute,
			min:         15 * time.Minute,
			max:         3 * time.Hour,
			expectError: false,
		},
		{
			name:        "at minimum",
			timeout:     15 * time.Minute,
			min:         15 * time.Minute,
			max:         3 * time.Hour,
			expectError: false,
		},
		{
			name:        "at maximum",
			timeout:     3 * time.Hour,
			min:         15 * time.Minute,
			max:         3 * time.Hour,
			expectError: false,
		},

		// Invalid cases
		{
			name:        "below minimum",
			timeout:     5 * time.Minute,
			min:         15 * time.Minute,
			max:         3 * time.Hour,
			expectError: true,
			errorMsgs:   []string{"too short", "minimum"},
		},
		{
			name:        "above maximum",
			timeout:     5 * time.Hour,
			min:         15 * time.Minute,
			max:         3 * time.Hour,
			expectError: true,
			errorMsgs:   []string{"too long", "maximum"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTimeout("TEST_TIMEOUT", tt.timeout, tt.min, tt.max)

			if tt.expectError {
				if err == nil {
					t.Errorf("ValidateTimeout expected error, got nil")
					return
				}
				for _, msg := range tt.errorMsgs {
					if !strings.Contains(err.Error(), msg) {
						t.Errorf("ValidateTimeout error = %q, expected to contain %q",
							err.Error(), msg)
					}
				}
			} else {
				if err != nil {
					t.Errorf("ValidateTimeout unexpected error: %v", err)
				}
			}
		})
	}
}

// TestValidateDeploymentTimeout tests the deployment timeout validation.
func TestValidateDeploymentTimeout(t *testing.T) {
	tests := []struct {
		name        string
		timeout     time.Duration
		expectError bool
	}{
		{"valid - 45 minutes", 45 * time.Minute, false},
		{"valid - 1 hour", 1 * time.Hour, false},
		{"valid - 2 hours", 2 * time.Hour, false},
		{"valid - at minimum", MinDeploymentTimeout, false},
		{"valid - at maximum", MaxDeploymentTimeout, false},
		{"invalid - 5 minutes", 5 * time.Minute, true},
		{"invalid - 10 minutes", 10 * time.Minute, true},
		{"invalid - 5 hours", 5 * time.Hour, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDeploymentTimeout(tt.timeout)
			if tt.expectError && err == nil {
				t.Errorf("ValidateDeploymentTimeout(%v) expected error, got nil", tt.timeout)
			}
			if !tt.expectError && err != nil {
				t.Errorf("ValidateDeploymentTimeout(%v) unexpected error: %v", tt.timeout, err)
			}
		})
	}
}

// TestValidateASOControllerTimeout tests the ASO controller timeout validation.
func TestValidateASOControllerTimeout(t *testing.T) {
	tests := []struct {
		name        string
		timeout     time.Duration
		expectError bool
	}{
		{"valid - 5 minutes", 5 * time.Minute, false},
		{"valid - 10 minutes", 10 * time.Minute, false},
		{"valid - at minimum", MinASOControllerTimeout, false},
		{"valid - at maximum", MaxASOControllerTimeout, false},
		{"invalid - 1 minute", 1 * time.Minute, true},
		{"invalid - 1 hour", 1 * time.Hour, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateASOControllerTimeout(tt.timeout)
			if tt.expectError && err == nil {
				t.Errorf("ValidateASOControllerTimeout(%v) expected error, got nil", tt.timeout)
			}
			if !tt.expectError && err != nil {
				t.Errorf("ValidateASOControllerTimeout(%v) unexpected error: %v", tt.timeout, err)
			}
		})
	}
}

// TestTimeoutConstants tests that timeout constants have correct values.
func TestTimeoutConstants(t *testing.T) {
	// Verify minimum/maximum relationship
	if MinDeploymentTimeout >= MaxDeploymentTimeout {
		t.Errorf("MinDeploymentTimeout (%v) should be less than MaxDeploymentTimeout (%v)",
			MinDeploymentTimeout, MaxDeploymentTimeout)
	}

	if MinASOControllerTimeout >= MaxASOControllerTimeout {
		t.Errorf("MinASOControllerTimeout (%v) should be less than MaxASOControllerTimeout (%v)",
			MinASOControllerTimeout, MaxASOControllerTimeout)
	}

	// Verify sensible values
	if MinDeploymentTimeout < 10*time.Minute {
		t.Errorf("MinDeploymentTimeout (%v) seems too short", MinDeploymentTimeout)
	}

	if MaxDeploymentTimeout > 6*time.Hour {
		t.Errorf("MaxDeploymentTimeout (%v) seems too long", MaxDeploymentTimeout)
	}
}

// TestConfigValidationResult tests the ConfigValidationResult struct.
func TestConfigValidationResult(t *testing.T) {
	result := ConfigValidationResult{
		Variable:   "TEST_VAR",
		Value:      "test-value",
		IsValid:    true,
		Error:      nil,
		IsCritical: true,
		SkipReason: "",
	}

	if result.Variable != "TEST_VAR" {
		t.Errorf("Variable = %q, expected %q", result.Variable, "TEST_VAR")
	}
	if result.Value != "test-value" {
		t.Errorf("Value = %q, expected %q", result.Value, "test-value")
	}
	if !result.IsValid {
		t.Error("IsValid should be true")
	}
	if result.Error != nil {
		t.Errorf("Error should be nil, got %v", result.Error)
	}
	if !result.IsCritical {
		t.Error("IsCritical should be true")
	}
}

// TestFormatValidationResults tests the validation results formatter.
func TestFormatValidationResults(t *testing.T) {
	tests := []struct {
		name    string
		results []ConfigValidationResult
		checks  []string
	}{
		{
			name: "all valid",
			results: []ConfigValidationResult{
				{Variable: "VAR1", Value: "val1", IsValid: true, IsCritical: true},
				{Variable: "VAR2", Value: "val2", IsValid: true, IsCritical: false},
			},
			checks: []string{"CONFIGURATION VALIDATION", "VAR1", "VAR2", "✅", "All configuration validations passed"},
		},
		{
			name: "critical error",
			results: []ConfigValidationResult{
				{Variable: "VAR1", Value: "val1", IsValid: true, IsCritical: true},
				{Variable: "VAR2", Value: "bad", IsValid: false, IsCritical: true, Error: fmt.Errorf("invalid value")},
			},
			checks: []string{"VAR1", "VAR2", "❌", "critical error"},
		},
		{
			name: "warning only",
			results: []ConfigValidationResult{
				{Variable: "VAR1", Value: "val1", IsValid: true, IsCritical: true},
				{Variable: "VAR2", Value: "warn", IsValid: false, IsCritical: false, Error: fmt.Errorf("warning")},
			},
			checks: []string{"VAR1", "VAR2", "⚠️", "warning"},
		},
		{
			name: "mixed errors and warnings",
			results: []ConfigValidationResult{
				{Variable: "VAR1", Value: "ok", IsValid: true, IsCritical: true},
				{Variable: "VAR2", Value: "bad", IsValid: false, IsCritical: true, Error: fmt.Errorf("error")},
				{Variable: "VAR3", Value: "warn", IsValid: false, IsCritical: false, Error: fmt.Errorf("warning")},
			},
			checks: []string{"VAR1", "VAR2", "VAR3", "❌", "⚠️", "critical error", "warning"},
		},
		{
			name:    "empty results",
			results: []ConfigValidationResult{},
			checks:  []string{"CONFIGURATION VALIDATION", "All configuration validations passed"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatValidationResults(tt.results)

			for _, check := range tt.checks {
				if !strings.Contains(result, check) {
					t.Errorf("FormatValidationResults() output should contain %q, got:\n%s", check, result)
				}
			}
		})
	}
}

// TestValidateAllConfigurations tests the comprehensive configuration validation.
func TestValidateAllConfigurations(t *testing.T) {
	// Create a valid config
	config := &TestConfig{
		CAPZUser:             "rcap",
		Environment:          "stage",
		ClusterNamePrefix:    "rcap-stage",
		TestNamespace:        "default",
		Region:               "uksouth",
		DeploymentTimeout:    45 * time.Minute,
		ASOControllerTimeout: 10 * time.Minute,
	}

	results := ValidateAllConfigurations(t, config)

	// Should have results for all validations
	if len(results) == 0 {
		t.Error("ValidateAllConfigurations should return non-empty results")
	}

	// All should be valid with this config
	for _, r := range results {
		if !r.IsValid {
			t.Errorf("Validation for %s failed unexpectedly: %v", r.Variable, r.Error)
		}
	}
}

// TestValidateAllConfigurations_InvalidConfig tests validation with invalid config.
func TestValidateAllConfigurations_InvalidConfig(t *testing.T) {
	// Create an invalid config (RFC 1123 violation - uppercase)
	config := &TestConfig{
		CAPZUser:             "RCAP", // Invalid - uppercase
		Environment:          "stage",
		ClusterNamePrefix:    "RCAP-stage", // Invalid - uppercase
		TestNamespace:        "default",
		Region:               "uksouth",
		DeploymentTimeout:    45 * time.Minute,
		ASOControllerTimeout: 10 * time.Minute,
	}

	results := ValidateAllConfigurations(t, config)

	// Should have at least one invalid result
	hasInvalid := false
	for _, r := range results {
		if !r.IsValid {
			hasInvalid = true
			break
		}
	}

	if !hasInvalid {
		t.Error("ValidateAllConfigurations should detect invalid config (uppercase in RFC 1123 names)")
	}
}

// TestFormatRemediationSteps tests the remediation steps formatter.
func TestFormatRemediationSteps(t *testing.T) {
	steps := []string{
		"Step 1: Do this",
		"Step 2: Do that",
		"Step 3: Finish up",
	}

	result := formatRemediationSteps(steps)

	for _, step := range steps {
		if !strings.Contains(result, step) {
			t.Errorf("formatRemediationSteps should contain %q, got: %s", step, result)
		}
	}

	// Should have indentation
	if !strings.Contains(result, "    ") {
		t.Error("formatRemediationSteps should indent steps")
	}
}

// TestFormatRemediationSteps_Empty tests with empty steps.
func TestFormatRemediationSteps_Empty(t *testing.T) {
	result := formatRemediationSteps([]string{})
	if result != "" {
		t.Errorf("formatRemediationSteps([]) should return empty string, got: %s", result)
	}
}

// ============================================================================
// Cluster Resource Detection Tests (Issue #433 - Fail-fast mismatch detection)
// ============================================================================

// TestCheckForMismatchedClusters_Logic tests the mismatch detection logic.
// Note: This tests the pure logic without needing a real cluster.
func TestCheckForMismatchedClusters_Logic(t *testing.T) {
	tests := []struct {
		name             string
		existingClusters []string
		expectedPrefix   string
		wantMismatched   []string
	}{
		{
			name:             "no clusters - no mismatch",
			existingClusters: []string{},
			expectedPrefix:   "rcapk-stage",
			wantMismatched:   nil,
		},
		{
			name:             "all match prefix",
			existingClusters: []string{"rcapk-stage", "rcapk-stage-2"},
			expectedPrefix:   "rcapk-stage",
			wantMismatched:   nil,
		},
		{
			name:             "one mismatch",
			existingClusters: []string{"rcapb-stage", "rcapk-stage"},
			expectedPrefix:   "rcapk-stage",
			wantMismatched:   []string{"rcapb-stage"},
		},
		{
			name:             "all mismatch",
			existingClusters: []string{"rcapb-stage", "other-cluster"},
			expectedPrefix:   "rcapk-stage",
			wantMismatched:   []string{"rcapb-stage", "other-cluster"},
		},
		{
			name:             "similar but not matching prefix",
			existingClusters: []string{"rcapk", "rcapk-prod"},
			expectedPrefix:   "rcapk-stage",
			wantMismatched:   []string{"rcapk", "rcapk-prod"},
		},
		{
			name:             "exact match only",
			existingClusters: []string{"rcapk-stage"},
			expectedPrefix:   "rcapk-stage",
			wantMismatched:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the prefix matching logic directly
			var mismatched []string
			for _, name := range tt.existingClusters {
				if !strings.HasPrefix(name, tt.expectedPrefix) {
					mismatched = append(mismatched, name)
				}
			}

			// Compare results
			if len(mismatched) != len(tt.wantMismatched) {
				t.Errorf("mismatched count = %d, want %d", len(mismatched), len(tt.wantMismatched))
				t.Logf("got: %v, want: %v", mismatched, tt.wantMismatched)
				return
			}

			for i, name := range mismatched {
				if name != tt.wantMismatched[i] {
					t.Errorf("mismatched[%d] = %q, want %q", i, name, tt.wantMismatched[i])
				}
			}
		})
	}
}

// TestFormatMismatchedClustersError tests the error message formatting.
func TestFormatMismatchedClustersError(t *testing.T) {
	tests := []struct {
		name           string
		mismatched     []string
		expectedPrefix string
		namespace      string
		wantContains   []string
	}{
		{
			name:           "single cluster",
			mismatched:     []string{"rcapb-stage"},
			expectedPrefix: "rcapk-stage",
			namespace:      "default",
			wantContains: []string{
				"EXISTING CLUSTER RESOURCES DETECTED",
				"rcapb-stage",
				"rcapk-stage",
				"kubectl delete cluster rcapb-stage -n default",
				"make clean",
			},
		},
		{
			name:           "multiple clusters",
			mismatched:     []string{"rcapb-stage", "old-cluster"},
			expectedPrefix: "rcapk-stage",
			namespace:      "test-ns",
			wantContains: []string{
				"rcapb-stage",
				"old-cluster",
				"kubectl delete cluster --all -n test-ns",
				"CAPZ_USER was changed",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatMismatchedClustersError(tt.mismatched, tt.expectedPrefix, tt.namespace)

			for _, want := range tt.wantContains {
				if !strings.Contains(result, want) {
					t.Errorf("FormatMismatchedClustersError() should contain %q", want)
					t.Logf("Got:\n%s", result)
				}
			}
		})
	}
}

// TestFormatMismatchedClustersError_HasInstructions verifies the error message provides actionable guidance.
func TestFormatMismatchedClustersError_HasInstructions(t *testing.T) {
	result := FormatMismatchedClustersError(
		[]string{"old-cluster"},
		"new-prefix",
		"default",
	)

	// Must have clear header
	if !strings.Contains(result, "━") {
		t.Error("Error message should have visual separator")
	}

	// Must explain the problem
	if !strings.Contains(result, "don't match current configuration") {
		t.Error("Error message should explain what the problem is")
	}

	// Must provide cleanup commands
	if !strings.Contains(result, "kubectl delete") {
		t.Error("Error message should provide kubectl delete command")
	}

	// Must mention make clean alternative
	if !strings.Contains(result, "make clean") {
		t.Error("Error message should mention make clean as alternative")
	}

	// Must provide context about why this happens
	if !strings.Contains(result, "CAPZ_USER") {
		t.Error("Error message should explain CAPZ_USER change scenario")
	}
}
