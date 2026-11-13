package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"
)

var openShiftVersionSelector = regexp.MustCompile(`^[0-9]+\.[0-9]+(\.[0-9]+)?$`)

// ValidateOpenShiftVersionAvailability dispatches catalog validation by provider.
// ROSA has no catalog adapter yet; it explicitly skips rather than consulting ARO.
func ValidateOpenShiftVersionAvailability(ctx context.Context, t *testing.T, config *TestConfig) error {
	t.Helper()
	for _, provider := range config.InfraProviders {
		switch provider.Name {
		case "aro":
			if err := validateAROVersionAvailability(ctx, t, config); err != nil {
				return err
			}
		default:
			t.Logf("OpenShift version availability preflight is not implemented for provider %q; skipping catalog validation", provider.Name)
		}
	}
	return nil
}

func validateAROVersionAvailability(ctx context.Context, t *testing.T, config *TestConfig) error {
	t.Helper()
	requested := []struct{ variable, version string }{{"OCP_VERSION", config.OCPVersion}, {"OCP_VERSION_MP", config.OCPVersionMP}}
	for _, target := range requested {
		if !openShiftVersionSelector.MatchString(target.version) {
			return fmt.Errorf("invalid %s=%q: expected X.Y or X.Y.Z", target.variable, target.version)
		}
	}
	timeout, err := time.ParseDuration(config.OpenShiftVersionCheckTimeout)
	if err != nil || timeout < time.Second || timeout > 5*time.Minute {
		return fmt.Errorf("OPENSHIFT_VERSION_CHECK_TIMEOUT must be between 1s and 5m, got %q", config.OpenShiftVersionCheckTimeout)
	}
	if config.Region == "" || config.AROVersionAPIVersion == "" {
		return fmt.Errorf("ARO version preflight requires REGION and ARO_VERSION_API_VERSION")
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// az cloud show keeps catalog discovery on the active Azure cloud's ARM endpoint.
	endpoint, err := runVersionCatalogAZ(ctx, "cloud", "show", "--query", "endpoints.resourceManager", "--output", "tsv")
	if err != nil {
		return fmt.Errorf("cannot resolve Azure ARM endpoint: %w", err)
	}
	base, err := url.Parse(strings.TrimRight(endpoint, "/"))
	if err != nil || base.Scheme != "https" || base.Host == "" || base.User != nil || base.RawQuery != "" || base.Fragment != "" || base.Path != "" {
		return fmt.Errorf("invalid Azure ARM endpoint %q", endpoint)
	}
	subscription := config.AzureSubscriptionID
	if subscription == "" {
		subscription, err = runVersionCatalogAZ(ctx, "account", "show", "--query", "id", "--output", "tsv")
		if err != nil {
			return fmt.Errorf("cannot resolve Azure subscription; check az login: %w", err)
		}
	}
	if subscription == "" {
		return fmt.Errorf("ARO version preflight requires AZURE_SUBSCRIPTION_ID or an active Azure CLI subscription")
	}
	// HcpOpenShiftVersions_List in the hcpopenshiftclusters REST specification
	// (2026-09-01-preview), also used by ASO; this is the HCP catalog.
	collection := "/subscriptions/" + url.PathEscape(subscription) + "/providers/Microsoft.RedHatOpenShift/locations/" + url.PathEscape(config.Region) + "/hcpOpenShiftVersions"
	next := base.String() + collection + "?api-version=" + url.QueryEscape(config.AROVersionAPIVersion)
	seen := map[string]bool{}
	availableSet := map[string]bool{}
	for next != "" {
		if seen[next] || len(seen) >= 100 {
			return fmt.Errorf("ARO HCP version catalog pagination did not terminate")
		}
		seen[next] = true
		output, err := runVersionCatalogAZ(ctx, "rest", "--method", "get", "--url", next, "--output", "json")
		if err != nil {
			return fmt.Errorf("ARO HCP version catalog query failed for region %s; verify Azure authentication, subscription permissions and API availability: %w", config.Region, err)
		}
		var page struct {
			Value *[]struct {
				Name       string `json:"name"`
				Properties struct {
					Enabled      bool   `json:"enabled"`
					ChannelGroup string `json:"channelGroup"`
				} `json:"properties"`
			} `json:"value"`
			NextLink string `json:"nextLink"`
		}
		if err := json.Unmarshal([]byte(output), &page); err != nil {
			return fmt.Errorf("decode ARO HCP version catalog for region %s: %w", config.Region, err)
		}
		if page.Value == nil {
			return fmt.Errorf("ARO HCP version catalog for region %s: missing value array", config.Region)
		}
		for _, version := range *page.Value {
			if version.Properties.Enabled && version.Properties.ChannelGroup == "stable" && openShiftVersionSelector.MatchString(version.Name) {
				availableSet[version.Name] = true
			}
		}
		next = page.NextLink
		if next != "" {
			link, err := url.Parse(next)
			if err != nil || link.Scheme != base.Scheme || link.Host != base.Host || link.User != nil || link.Fragment != "" || link.EscapedPath() != collection {
				return fmt.Errorf("invalid ARO HCP version catalog nextLink: expected the same Azure cloud and regional collection")
			}
		}
	}
	available := make([]string, 0, len(availableSet))
	for version := range availableSet {
		available = append(available, version)
	}
	sort.Strings(available)
	if len(available) == 0 {
		return fmt.Errorf("ARO HCP version catalog returned no enabled stable versions for region %s", config.Region)
	}
	for _, target := range requested {
		match := false
		for _, version := range available {
			if version == target.version || (strings.Count(target.version, ".") == 1 && strings.HasPrefix(version, target.version+".")) {
				match = true
				break
			}
		}
		if !match {
			return fmt.Errorf("%s=%s is not available for ARO HCP in region %s; enabled stable versions: %s", target.variable, target.version, config.Region, strings.Join(available, ", "))
		}
	}
	t.Logf("ARO HCP version preflight passed in %s: OCP_VERSION=%s, OCP_VERSION_MP=%s", config.Region, config.OCPVersion, config.OCPVersionMP)
	return nil
}

// Existing RunCommand helpers have no cancellation support. This catalog-only
// runner shares one deadline across metadata discovery and all response pages.
func runVersionCatalogAZ(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "az", args...) // #nosec G204 G702 -- fixed executable, separate arguments for read-only catalog discovery
	cmd.WaitDelay = time.Second
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if err != nil {
		return "", fmt.Errorf("Azure CLI command failed: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}
