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
