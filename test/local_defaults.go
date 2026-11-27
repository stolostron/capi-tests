package test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const localDefaultsFileName = ".capi-test-defaults.json"

// loadLocalDefaults runs once at test-process startup, before any phase or
// environment reader. Filling only absent variables also makes defaults visible
// to installer subprocesses without changing explicit (even empty) environment
// settings. Existing configuration helpers retain their empty-value semantics.
func loadLocalDefaults() error {
	path, explicit := os.LookupEnv("CAPI_TEST_DEFAULTS_FILE")
	if explicit && path == "" {
		return nil
	}
	if !explicit {
		path = localDefaultsFileName
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(runContextWorkspace(), path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if !explicit && os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read local defaults %s: %w", path, err)
	}
	var entries map[string]json.RawMessage
	if err := json.Unmarshal(data, &entries); err != nil || entries == nil {
		return fmt.Errorf("local defaults %s must be a JSON object of environment names and string values", path)
	}
	// Validate the entire document before changing the environment. Errors never
	// include values, which may contain credentials.
	values := make(map[string]string, len(entries))
	validName := regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)
	for key, raw := range entries {
		if !validName.MatchString(key) {
			return fmt.Errorf("local defaults %s contains an invalid environment name", path)
		}
		var value string
		if string(raw) == "null" || json.Unmarshal(raw, &value) != nil || strings.ContainsRune(value, 0) {
			return fmt.Errorf("local defaults %s: %s must be a string without NUL bytes", path, key)
		}
		values[key] = value
	}
	for key, value := range values {
		if _, present := os.LookupEnv(key); !present {
			if err := os.Setenv(key, value); err != nil {
				return fmt.Errorf("local defaults %s: cannot set %s", path, key)
			}
		}
	}
	return nil
}
