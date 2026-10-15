package test

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestParseGoVersion(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    goVersion
		wantErr bool
	}{
		{name: "go version output", input: "go1.26.9 linux/amd64", want: goVersion{major: 1, minor: 26, patch: 9}},
		{name: "full go version output", input: "go version go1.26.9 linux/amd64", want: goVersion{major: 1, minor: 26, patch: 9}},
		{name: "go version with build suffix", input: "go1.26.9-X:nodwarf5 linux/amd64", want: goVersion{major: 1, minor: 26, patch: 9}},
		{name: "go.mod directive", input: "1.26.0", want: goVersion{major: 1, minor: 26}},
		{name: "missing minor version", input: "go1", wantErr: true},
		{name: "invalid version", input: "not-a-version", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseGoVersion(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseGoVersion(%q) error = %v, wantErr %t", tt.input, err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Errorf("parseGoVersion(%q) = %+v, want %+v", tt.input, got, tt.want)
			}
		})
	}
}

func TestMinimumGoVersionFromGoMod(t *testing.T) {
	got, err := minimumGoVersionFromGoMod("module example.com/test\n\ngo 1.26.0\n")
	if err != nil {
		t.Fatalf("minimumGoVersionFromGoMod() error = %v", err)
	}
	want := goVersion{major: 1, minor: 26}
	if got != want {
		t.Errorf("minimumGoVersionFromGoMod() = %+v, want %+v", got, want)
	}

	if _, err := minimumGoVersionFromGoMod("module example.com/test\n"); err == nil {
		t.Error("minimumGoVersionFromGoMod() error = nil, want missing go directive error")
	}
}

func TestGoVersionMeetsMinimum(t *testing.T) {
	minimum := goVersion{major: 1, minor: 26}
	tests := []struct {
		name    string
		current goVersion
		want    bool
	}{
		{name: "below minimum", current: goVersion{major: 1, minor: 25}, want: false},
		{name: "equal to minimum", current: minimum, want: true},
		{name: "above minimum", current: goVersion{major: 1, minor: 27}, want: true},
		{name: "newer major", current: goVersion{major: 2}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := goVersionMeetsMinimum(tt.current, minimum); got != tt.want {
				t.Errorf("goVersionMeetsMinimum(%+v, %+v) = %t, want %t", tt.current, minimum, got, tt.want)
			}
		})
	}
}

// TestCheckDependencies_GoVersion verifies that the installed Go toolchain
// meets the minimum version declared by this repository's go.mod.
func TestCheckDependencies_GoVersion(t *testing.T) {
	const goCommand = "go"
	if !CommandExists(goCommand) {
		t.Fatalf("Go is not installed or not in PATH. Install Go at the minimum version declared in go.mod or newer.")
	}

	versionOutput, err := RunCommand(t, goCommand, "version")
	if err != nil {
		t.Fatalf("Failed to get Go version: %v", err)
	}
	currentVersion, err := parseGoVersion(versionOutput)
	if err != nil {
		t.Fatalf("Could not parse Go version from %q: %v", strings.TrimSpace(versionOutput), err)
	}

	goModPath := filepath.Join("..", "go.mod")
	goModContents, err := os.ReadFile(goModPath)
	if err != nil {
		t.Fatalf("Failed to read minimum Go version from %s: %v", goModPath, err)
	}
	minimumVersion, err := minimumGoVersionFromGoMod(string(goModContents))
	if err != nil {
		t.Fatalf("Failed to read minimum Go version from %s: %v", goModPath, err)
	}

	if !goVersionMeetsMinimum(currentVersion, minimumVersion) {
		t.Fatalf("Installed Go version %s is below the repository minimum Go version %s from %s. Install Go %s or newer.",
			formatGoVersion(currentVersion), formatGoVersion(minimumVersion), goModPath, formatGoVersion(minimumVersion))
	}

	t.Logf("Detected Go version %s; repository minimum is %s", strings.TrimSpace(versionOutput), formatGoVersion(minimumVersion))
}

type goVersion struct {
	major int
	minor int
	patch int
}

func parseGoVersion(version string) (goVersion, error) {
	fields := strings.Fields(strings.TrimSpace(version))
	if len(fields) == 0 {
		return goVersion{}, fmt.Errorf("version is empty")
	}

	versionToken := ""
	for _, field := range fields {
		candidate := strings.TrimPrefix(field, "go")
		if len(candidate) > 0 && candidate[0] >= '0' && candidate[0] <= '9' {
			versionToken = candidate
			break
		}
	}
	if versionToken == "" {
		return goVersion{}, fmt.Errorf("no Go version token found in %q", strings.TrimSpace(version))
	}
	if hyphen := strings.IndexByte(versionToken, '-'); hyphen >= 0 {
		versionToken = versionToken[:hyphen]
	}

	parts := strings.Split(versionToken, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return goVersion{}, fmt.Errorf("expected major.minor[.patch], got %q", versionToken)
	}

	parsed := make([]int, len(parts))
	for i, part := range parts {
		value, err := strconv.Atoi(part)
		if err != nil || value < 0 {
			return goVersion{}, fmt.Errorf("invalid version component %q", part)
		}
		parsed[i] = value
	}

	result := goVersion{major: parsed[0], minor: parsed[1]}
	if len(parsed) == 3 {
		result.patch = parsed[2]
	}
	return result, nil
}

func minimumGoVersionFromGoMod(contents string) (goVersion, error) {
	for lineNumber, line := range strings.Split(contents, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] != "go" {
			continue
		}
		if len(fields) < 2 {
			return goVersion{}, fmt.Errorf("go directive on line %d has no version", lineNumber+1)
		}
		version, err := parseGoVersion(fields[1])
		if err != nil {
			return goVersion{}, fmt.Errorf("invalid go directive on line %d: %w", lineNumber+1, err)
		}
		return version, nil
	}

	return goVersion{}, fmt.Errorf("go directive not found")
}

func goVersionMeetsMinimum(current, minimum goVersion) bool {
	if current.major != minimum.major {
		return current.major > minimum.major
	}
	if current.minor != minimum.minor {
		return current.minor > minimum.minor
	}
	return current.patch >= minimum.patch
}

func formatGoVersion(version goVersion) string {
	return fmt.Sprintf("%d.%d.%d", version.major, version.minor, version.patch)
}
