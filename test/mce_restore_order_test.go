package test

import (
	"errors"
	"reflect"
	"testing"
)

type mceComponentStateChange struct {
	name    string
	enabled bool
}

func TestRestoreMCEComponentStatesHonorsExclusivity(t *testing.T) {
	tests := []struct {
		name           string
		original       map[string]bool
		current        map[string]bool
		expectedChange []mceComponentStateChange
	}{
		{
			name: "disable CAPI before restoring HyperShift",
			original: map[string]bool{
				"hypershift":                         true,
				"hypershift-local-hosting":           true,
				"cluster-api":                        false,
				"cluster-api-provider-azure-preview": false,
			},
			current: map[string]bool{
				"hypershift":                         false,
				"hypershift-local-hosting":           false,
				"cluster-api":                        true,
				"cluster-api-provider-azure-preview": true,
			},
			expectedChange: []mceComponentStateChange{
				{name: "cluster-api", enabled: false},
				{name: "cluster-api-provider-azure-preview", enabled: false},
				{name: "hypershift", enabled: true},
				{name: "hypershift-local-hosting", enabled: true},
			},
		},
		{
			name: "disable HyperShift before restoring CAPI",
			original: map[string]bool{
				"hypershift":                         false,
				"hypershift-local-hosting":           false,
				"cluster-api":                        true,
				"cluster-api-provider-azure-preview": true,
			},
			current: map[string]bool{
				"hypershift":                         true,
				"hypershift-local-hosting":           true,
				"cluster-api":                        false,
				"cluster-api-provider-azure-preview": false,
			},
			expectedChange: []mceComponentStateChange{
				{name: "hypershift", enabled: false},
				{name: "hypershift-local-hosting", enabled: false},
				{name: "cluster-api", enabled: true},
				{name: "cluster-api-provider-azure-preview", enabled: true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			current := make(map[string]bool, len(tt.current))
			for component, enabled := range tt.current {
				current[component] = enabled
			}

			var changes []mceComponentStateChange
			getCurrent := func(component string) (bool, error) {
				return current[component], nil
			}
			setState := func(component string, enabled bool) error {
				previous := current[component]
				current[component] = enabled
				if hasEnabledMCEComponent(current, "hypershift", "hypershift-local-hosting") &&
					hasEnabledMCEComponent(current, "cluster-api", "cluster-api-provider-azure-preview") {
					current[component] = previous
					return errors.New("component exclusivity violation")
				}
				changes = append(changes, mceComponentStateChange{name: component, enabled: enabled})
				return nil
			}

			_, failed := restoreMCEComponentStates(tt.original, getCurrent, setState)
			if len(failed) != 0 {
				t.Fatalf("restoreMCEComponentStates() failures = %v, want none", failed)
			}
			if !reflect.DeepEqual(current, tt.original) {
				t.Errorf("restored states = %v, want %v", current, tt.original)
			}
			if !reflect.DeepEqual(changes, tt.expectedChange) {
				t.Errorf("state changes = %v, want %v", changes, tt.expectedChange)
			}
		})
	}
}

func TestRestoreMCEComponentStatesStopsEnablementAfterDisableFailure(t *testing.T) {
	original := map[string]bool{
		"hypershift":                         true,
		"hypershift-local-hosting":           true,
		"cluster-api":                        false,
		"cluster-api-provider-azure-preview": false,
	}
	current := map[string]bool{
		"hypershift":                         false,
		"hypershift-local-hosting":           false,
		"cluster-api":                        true,
		"cluster-api-provider-azure-preview": true,
	}
	var attempts []mceComponentStateChange

	_, failed := restoreMCEComponentStates(original,
		func(component string) (bool, error) {
			return current[component], nil
		},
		func(component string, enabled bool) error {
			attempts = append(attempts, mceComponentStateChange{name: component, enabled: enabled})
			if component == "cluster-api" && !enabled {
				return errors.New("simulated CAPI disable failure")
			}
			previous := current[component]
			current[component] = enabled
			if hasEnabledMCEComponent(current, "hypershift", "hypershift-local-hosting") &&
				hasEnabledMCEComponent(current, "cluster-api", "cluster-api-provider-azure-preview") {
				current[component] = previous
				return errors.New("component exclusivity violation")
			}
			return nil
		})

	if len(failed) == 0 {
		t.Fatal("restoreMCEComponentStates() failures = none, want CAPI disable failure")
	}
	for _, attempt := range attempts {
		if attempt.enabled && (attempt.name == "hypershift" || attempt.name == "hypershift-local-hosting") {
			t.Errorf("attempted to enable %s after a CAPI disable failure", attempt.name)
		}
	}
}

func hasEnabledMCEComponent(states map[string]bool, names ...string) bool {
	for _, name := range names {
		if states[name] {
			return true
		}
	}
	return false
}
