package test

import (
	"strings"
	"testing"
)

// TestImagePullFailureCategories_RealMessages covers kubelet and containerd
// messages as they actually appear, so the classifier is pinned against the
// wording it has to survive.
func TestImagePullFailureCategories_RealMessages(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    string
	}{
		{
			name:    "containerd unauthorized",
			message: `failed to pull and unpack image "quay.io/acm-d/ose-cluster-api-rhel9:v5.0": failed to resolve reference: unexpected status from HEAD request: 401 Unauthorized`,
			want:    categoryAuthRejected,
		},
		{
			name:    "registry denial",
			message: `Error response from daemon: denied: requested access to the resource is denied`,
			want:    categoryAccessDenied,
		},
		{
			name:    "quay rate limit",
			message: `toomanyrequests: Rate exceeded`,
			want:    categoryRateLimited,
		},
		{
			name:    "rate limit with status code",
			message: `failed to copy: httpReadSeeker: failed open: unexpected status code 429 Too Many Requests`,
			want:    categoryRateLimited,
		},
		{
			name:    "missing pull secret",
			message: `FailedToRetrieveImagePullSecret Unable to retrieve some image pull secrets (capi-manager-pull-credentials)`,
			want:    categoryPullSecretUnavailable,
		},
		{
			name:    "tag does not exist",
			message: `failed to resolve reference "quay.io/acm-d/foo:bar": manifest unknown`,
			want:    categoryImageNotFound,
		},
		{
			name:    "dns failure",
			message: `dial tcp: lookup quay.io on 10.0.0.10:53: no such host`,
			want:    categoryDNSFailed,
		},
		{
			name:    "tls failure",
			message: `x509: certificate signed by unknown authority`,
			want:    categoryTLSError,
		},
		{
			name:    "nothing recognisable",
			message: `rpc error: code = Unknown desc = something unexpected happened`,
			want:    categoryUnclassified,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := primaryImagePullCategory(imagePullFailureCategories(tt.message))
			if got != tt.want {
				t.Errorf("imagePullFailureCategories(%q) primary = %q, want %q", tt.message, got, tt.want)
			}
		})
	}
}

// TestImagePullFailureCategories_NoFalsePositives guards the failure mode that
// made CI diagnostics untrustworthy: bare three-digit runs inside digests,
// ports and byte counts were read as HTTP status codes, and "permission denied"
// from the filesystem was read as a registry denial.
func TestImagePullFailureCategories_NoFalsePositives(t *testing.T) {
	tests := []struct {
		name    string
		message string
	}{
		{
			name:    "401 inside image digest",
			message: `Failed to pull image "quay.io/acm-d/ose-cluster-api-rhel9@sha256:401bcef362fbfd199a7ee9051e2046710e1f5886e03f3ee971fce9aedc6d495d"`,
		},
		{
			name:    "429 inside image digest",
			message: `Failed to pull image "quay.io/acm-d/capz@sha256:429d68f39d641a4eccb42877fd69252708103603a6761a2405eb2b2a4e82c87a"`,
		},
		{
			name:    "403 inside a port number",
			message: `dial tcp 10.0.0.1:4035: connect: connection refused`,
		},
		{
			name:    "401 inside a byte count",
			message: `copied 401293 bytes before the connection dropped`,
		},
		{
			name:    "filesystem permission denied",
			message: `mkdir: cannot create directory 'results': Permission denied`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := imagePullFailureCategories(tt.message)
			if len(got) != 0 {
				t.Errorf("imagePullFailureCategories(%q) = %v, want no categories", tt.message, got)
			}
		})
	}
}

// TestImagePullFailureCategories_Ambiguous checks that a message carrying more
// than one signal reports all of them. Collapsing such a message to a single
// confident label is what previously pointed an investigation at rate limiting
// when the real cause was a stale credential.
func TestImagePullFailureCategories_Ambiguous(t *testing.T) {
	message := `unauthorized: access to the requested resource is not authorized; retry later: 429 Too Many Requests`

	got := imagePullFailureCategories(message)
	if len(got) != 2 {
		t.Fatalf("imagePullFailureCategories(...) = %v, want 2 categories", got)
	}
	if got[0] != categoryAuthRejected {
		t.Errorf("primary category = %q, want %q", got[0], categoryAuthRejected)
	}
	if got[1] != categoryRateLimited {
		t.Errorf("secondary category = %q, want %q", got[1], categoryRateLimited)
	}

	formatted := formatImagePullCategories(got)
	if !strings.Contains(formatted, "ambiguous") {
		t.Errorf("formatImagePullCategories(%v) = %q, want it to flag ambiguity", got, formatted)
	}
	if !strings.Contains(formatted, categoryRateLimited) {
		t.Errorf("formatImagePullCategories(%v) = %q, want it to name every match", got, formatted)
	}
}

func TestFormatImagePullCategories(t *testing.T) {
	tests := []struct {
		name       string
		categories []string
		want       string
	}{
		{
			name:       "no match",
			categories: nil,
			want:       categoryUnclassified,
		},
		{
			name:       "single match is reported plainly",
			categories: []string{categoryAuthRejected},
			want:       categoryAuthRejected,
		},
		{
			name:       "multiple matches are flagged",
			categories: []string{categoryAuthRejected, categoryRateLimited},
			want:       categoryAuthRejected + " (ambiguous, also matched: " + categoryRateLimited + ")",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatImagePullCategories(tt.categories); got != tt.want {
				t.Errorf("formatImagePullCategories(%v) = %q, want %q", tt.categories, got, tt.want)
			}
		})
	}
}

// TestImagePullFailureCategories_NeverLeaksMessage asserts the security
// contract of this file: only fixed category strings are returned, never any
// fragment of the kubelet message, which can embed registry responses or tokens.
func TestImagePullFailureCategories_NeverLeaksMessage(t *testing.T) {
	const secret = "Basic ZGVmaW5pdGVseS1hLXNlY3JldA=="
	message := `unauthorized: authentication required; Authorization: ` + secret

	allowed := map[string]bool{
		categoryPullSecretUnavailable: true,
		categoryImageNotFound:         true,
		categoryAuthRejected:          true,
		categoryCredentialsUnparsable: true,
		categoryAccessDenied:          true,
		categoryDNSFailed:             true,
		categoryTLSError:              true,
		categoryConnectionTimeout:     true,
		categoryRateLimited:           true,
	}

	for _, category := range imagePullFailureCategories(message) {
		if !allowed[category] {
			t.Errorf("classifier returned %q, which is not a fixed category", category)
		}
	}
	if formatted := formatImagePullCategories(imagePullFailureCategories(message)); strings.Contains(formatted, secret) {
		t.Errorf("formatted output leaked the message: %q", formatted)
	}
}

// TestIsDiagnosticImagePullEventReason pins which event reasons are worth
// classifying. BackOff must stay excluded: its message is only a retry notice
// naming the image, so classifying it derives a verdict from a digest.
func TestIsDiagnosticImagePullEventReason(t *testing.T) {
	tests := []struct {
		reason string
		want   bool
	}{
		{reason: "Failed", want: true},
		{reason: "FailedToRetrieveImagePullSecret", want: true},
		{reason: "BackOff", want: false},
		{reason: "Pulling", want: false},
		{reason: "Pulled", want: false},
		{reason: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.reason, func(t *testing.T) {
			if got := isDiagnosticImagePullEventReason(tt.reason); got != tt.want {
				t.Errorf("isDiagnosticImagePullEventReason(%q) = %v, want %v", tt.reason, got, tt.want)
			}
		})
	}
}

// TestImagePullFailureCategories_BackOffMessage documents why BackOff events
// are not classified: the message carries no registry response, only an image
// reference whose digest can contain any digit sequence.
func TestImagePullFailureCategories_BackOffMessage(t *testing.T) {
	message := `Back-off pulling image "quay.io/acm-d/ose-cluster-api-rhel9@sha256:429bcef362fbfd199a7ee9051e2046710e1f5886e03f3ee971fce9aedc6d495d"`

	if got := imagePullFailureCategories(message); len(got) != 0 {
		t.Errorf("imagePullFailureCategories(back-off message) = %v, want no categories", got)
	}
}
