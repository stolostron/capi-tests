package test

import (
	"encoding/json"
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

// imagePullFailureCategory returns only fixed diagnostic text. Kubelet messages
// can contain registry responses or URLs, so they must never be logged verbatim.
func imagePullFailureCategory(message string) string {
	message = strings.ToLower(message)
	switch {
	case strings.Contains(message, "failedtoretrieveimagepullsecret"),
		strings.Contains(message, "unable to retrieve some image pull secrets"),
		strings.Contains(message, "image pull secret") && strings.Contains(message, "not found"):
		return "image pull secret could not be retrieved"
	case strings.Contains(message, "manifest unknown"),
		strings.Contains(message, "name unknown"),
		strings.Contains(message, "404 not found"):
		return "image repository or tag not found"
	case strings.Contains(message, "unauthorized"),
		strings.Contains(message, "authentication required"),
		strings.Contains(message, "no basic auth credentials"),
		strings.Contains(message, "invalid username/password"),
		strings.Contains(message, "401"):
		return "registry authentication rejected"
	case strings.Contains(message, "invalid auth config"),
		strings.Contains(message, "error parsing credentials"),
		strings.Contains(message, "failed to decode"):
		return "image pull credentials could not be parsed"
	case strings.Contains(message, "denied"),
		strings.Contains(message, "forbidden"),
		strings.Contains(message, "403"):
		return "registry access denied or repository unavailable"
	case strings.Contains(message, "no such host"),
		strings.Contains(message, "server misbehaving"):
		return "registry DNS lookup failed"
	case strings.Contains(message, "x509"),
		strings.Contains(message, "certificate verify failed"):
		return "registry TLS certificate error"
	case strings.Contains(message, "i/o timeout"),
		strings.Contains(message, "connection timed out"),
		strings.Contains(message, "context deadline exceeded"):
		return "registry connection timed out"
	case strings.Contains(message, "too many requests"),
		strings.Contains(message, "429"):
		return "registry rate limited the pull"
	default:
		return "unclassified image pull error"
	}
}

func isImagePullWaitingReason(reason string) bool {
	return reason == "ErrImagePull" || reason == "ImagePullBackOff"
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
			PrintToTTY("  Kubelet status category: %s\n",
				imagePullFailureCategory(status.State.Waiting.Message))
			for _, event := range eventList.Items {
				if event.InvolvedObject.Kind != "Pod" || event.InvolvedObject.Name != pod.Metadata.Name {
					continue
				}
				if event.Reason != "Failed" && event.Reason != "BackOff" &&
					event.Reason != "FailedToRetrieveImagePullSecret" {
					continue
				}
				PrintToTTY("  Kubelet event category: %s\n",
					imagePullFailureCategory(event.Reason+" "+event.Message))
			}
		}
	}
}
