package test

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

type imagePullContainerStatus struct {
	Name  string `json:"name"`
	State struct {
		Waiting struct {
			Reason  string `json:"reason"`
			Message string `json:"message"`
		} `json:"waiting"`
	} `json:"state"`
}

type imagePullPod struct {
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Spec struct {
		ImagePullSecrets []struct {
			Name string `json:"name"`
		} `json:"imagePullSecrets"`
	} `json:"spec"`
	Status struct {
		ContainerStatuses     []imagePullContainerStatus `json:"containerStatuses"`
		InitContainerStatuses []imagePullContainerStatus `json:"initContainerStatuses"`
	} `json:"status"`
}

type imagePullEvent struct {
	Reason         string `json:"reason"`
	Message        string `json:"message"`
	InvolvedObject struct {
		Name string `json:"name"`
		Kind string `json:"kind"`
	} `json:"involvedObject"`
}

// Image pull failure categories. These fixed strings are the only text ever
// derived from a kubelet message: the messages themselves can contain registry
// responses, tokens or URLs, so they must never be logged verbatim.
const (
	categoryPullSecretUnavailable = "image pull secret could not be retrieved"
	categoryImageNotFound         = "image repository or tag not found"
	categoryAuthRejected          = "registry authentication rejected"
	categoryCredentialsUnparsable = "image pull credentials could not be parsed"
	categoryAccessDenied          = "registry access denied or repository unavailable"
	categoryDNSFailed             = "registry DNS lookup failed"
	categoryTLSError              = "registry TLS certificate error"
	categoryConnectionTimeout     = "registry connection timed out"
	categoryRateLimited           = "registry rate limited the pull"
	categoryUnclassified          = "unclassified image pull error"
)

var (
	// Digests and other long hex runs embed arbitrary digit sequences, so a
	// digest such as sha256:401b... would otherwise read as an HTTP 401.
	imagePullHexRunPattern = regexp.MustCompile(`sha256:[0-9a-f]+|\b[0-9a-f]{12,}\b`)

	// An HTTP status code only counts as evidence when it carries status
	// context or its standard reason phrase. A bare three-digit run is not
	// evidence: it also matches ports, byte counts and image tags.
	imagePullStatus401Pattern = regexp.MustCompile(`\b401\s+unauthorized\b|\b(?:status|status code|statuscode|http|response|code)\b[^0-9]{0,16}401\b`)
	imagePullStatus403Pattern = regexp.MustCompile(`\b403\s+forbidden\b|\b(?:status|status code|statuscode|http|response|code)\b[^0-9]{0,16}403\b`)
	imagePullStatus429Pattern = regexp.MustCompile(`\b429\s+too\s+many\s+requests\b|\b(?:status|status code|statuscode|http|response|code)\b[^0-9]{0,16}429\b`)
)

// normalizeImagePullMessage lowercases the message and removes substrings known
// to produce false positives.
func normalizeImagePullMessage(message string) string {
	message = strings.ToLower(message)
	message = imagePullHexRunPattern.ReplaceAllString(message, " ")
	// "permission denied" is a filesystem error. Registry denials say
	// "denied: requested access ..." instead.
	message = strings.ReplaceAll(message, "permission denied", " ")
	return message
}

// imagePullFailureCategories returns every category the message matches, most
// specific first. Returning all of them rather than the first match keeps an
// ambiguous message visibly ambiguous instead of presenting one guess as fact.
func imagePullFailureCategories(message string) []string {
	message = normalizeImagePullMessage(message)

	var categories []string
	appendIf := func(matched bool, category string) {
		if matched {
			categories = append(categories, category)
		}
	}

	appendIf(strings.Contains(message, "failedtoretrieveimagepullsecret") ||
		strings.Contains(message, "unable to retrieve some image pull secrets") ||
		(strings.Contains(message, "image pull secret") && strings.Contains(message, "not found")),
		categoryPullSecretUnavailable)
	appendIf(strings.Contains(message, "manifest unknown") ||
		strings.Contains(message, "name unknown") ||
		strings.Contains(message, "404 not found"),
		categoryImageNotFound)
	appendIf(strings.Contains(message, "unauthorized") ||
		strings.Contains(message, "authentication required") ||
		strings.Contains(message, "no basic auth credentials") ||
		strings.Contains(message, "invalid username/password") ||
		imagePullStatus401Pattern.MatchString(message),
		categoryAuthRejected)
	appendIf(strings.Contains(message, "invalid auth config") ||
		strings.Contains(message, "error parsing credentials") ||
		strings.Contains(message, "failed to decode"),
		categoryCredentialsUnparsable)
	appendIf(strings.Contains(message, "denied") ||
		strings.Contains(message, "forbidden") ||
		imagePullStatus403Pattern.MatchString(message),
		categoryAccessDenied)
	appendIf(strings.Contains(message, "no such host") ||
		strings.Contains(message, "server misbehaving"),
		categoryDNSFailed)
	appendIf(strings.Contains(message, "x509") ||
		strings.Contains(message, "certificate verify failed"),
		categoryTLSError)
	appendIf(strings.Contains(message, "i/o timeout") ||
		strings.Contains(message, "connection timed out") ||
		strings.Contains(message, "context deadline exceeded"),
		categoryConnectionTimeout)
	appendIf(strings.Contains(message, "toomanyrequests") ||
		strings.Contains(message, "too many requests") ||
		imagePullStatus429Pattern.MatchString(message),
		categoryRateLimited)

	return categories
}

// primaryImagePullCategory returns the most specific category found, or the
// unclassified placeholder when nothing matched.
func primaryImagePullCategory(categories []string) string {
	if len(categories) == 0 {
		return categoryUnclassified
	}
	return categories[0]
}

// formatImagePullCategories renders every signal found in one message. More than
// one means the message is ambiguous, and saying so is the point: a single
// confident label on an ambiguous message is what misleads a reader.
func formatImagePullCategories(categories []string) string {
	if len(categories) <= 1 {
		return primaryImagePullCategory(categories)
	}
	return categories[0] + " (ambiguous, also matched: " + strings.Join(categories[1:], "; ") + ")"
}

func isImagePullWaitingReason(reason string) bool {
	return reason == "ErrImagePull" || reason == "ImagePullBackOff"
}

// isDiagnosticImagePullEventReason reports whether an event reason carries a
// registry response worth classifying.
//
// BackOff is deliberately excluded. Its message is only
// `Back-off pulling image "repo@sha256:..."`, which contains no error at all,
// so classifying it produces a verdict derived from an image reference rather
// than from anything the registry said.
func isDiagnosticImagePullEventReason(reason string) bool {
	return reason == "Failed" || reason == "FailedToRetrieveImagePullSecret"
}

// printImagePullFailureDiagnostics emits a narrow summary from pod status and
// events. Do not print kubectl's raw JSON, pod descriptions, or event messages:
// those fields may contain credentials or other sensitive registry responses.
func printImagePullFailureDiagnostics(t *testing.T, kubeContext, namespace string) {
	t.Helper()
	PrintToTTY("\n=== Image pull diagnostics in %s (secret values omitted) ===\n", namespace)

	podOutput, err := RunCommandQuiet(t, "kubectl", "--context", kubeContext,
		"-n", namespace, "--request-timeout=10s", "get", "pods", "-o", "json")
	if err != nil {
		PrintToTTY("Could not retrieve pod status: %v\n", err)
		return
	}
	var podList struct {
		Items []imagePullPod `json:"items"`
	}
	if err := json.Unmarshal([]byte(podOutput), &podList); err != nil {
		PrintToTTY("Could not parse pod status JSON\n")
		return
	}

	eventOutput, eventErr := RunCommandQuiet(t, "kubectl", "--context", kubeContext,
		"-n", namespace, "--request-timeout=10s", "get", "events", "-o", "json")
	var eventList struct {
		Items []imagePullEvent `json:"items"`
	}
	if eventErr == nil {
		if err := json.Unmarshal([]byte(eventOutput), &eventList); err != nil {
			eventErr = err
		}
	}
	if eventErr != nil {
		PrintToTTY("Could not retrieve or parse pod events\n")
	}

	for _, pod := range podList.Items {
		statuses := append(pod.Status.ContainerStatuses, pod.Status.InitContainerStatuses...)
		for _, status := range statuses {
			if !isImagePullWaitingReason(status.State.Waiting.Reason) {
				continue
			}
			PrintToTTY("Pod %s, container %s: %s\n",
				pod.Metadata.Name, status.Name, status.State.Waiting.Reason)
			if len(pod.Spec.ImagePullSecrets) == 0 {
				PrintToTTY("  Pod has no imagePullSecrets reference\n")
			} else {
				for _, secret := range pod.Spec.ImagePullSecrets {
					PrintToTTY("  imagePullSecrets reference: %s\n", secret.Name)
				}
			}
			statusCategories := imagePullFailureCategories(status.State.Waiting.Message)
			PrintToTTY("  Kubelet status category: %s\n",
				formatImagePullCategories(statusCategories))
			backOffEvents := 0
			for _, event := range eventList.Items {
				if event.InvolvedObject.Kind != "Pod" || event.InvolvedObject.Name != pod.Metadata.Name {
					continue
				}
				if event.Reason == "BackOff" {
					backOffEvents++
					continue
				}
				if !isDiagnosticImagePullEventReason(event.Reason) {
					continue
				}
				eventCategories := imagePullFailureCategories(event.Reason + " " + event.Message)
				// Event reasons are a fixed kubelet vocabulary, so naming the
				// reason is safe and tells the reader which event each category
				// came from.
				PrintToTTY("  Kubelet event %s: %s\n",
					event.Reason, formatImagePullCategories(eventCategories))
				// The event carries the registry's own response, so it is the
				// more reliable of the two. When it disagrees with the pod
				// status, say so rather than printing both as fact.
				if primaryImagePullCategory(statusCategories) != primaryImagePullCategory(eventCategories) {
					PrintToTTY("  Status and event categories disagree; prefer the event\n")
				}
			}
			if backOffEvents > 0 {
				PrintToTTY("  Kubelet BackOff events: %d (retry notices, no registry response)\n",
					backOffEvents)
			}
		}
	}
}
