package test

import (
	"testing"
	"time"
)

func TestDetectStallPhase(t *testing.T) {
	const baseTimeout = 30 * time.Minute

	tests := []struct {
		name             string
		enabled          bool
		stallTimeout     time.Duration
		stallDuration    time.Duration
		progress         stallProgressState
		wantStalled      bool
		wantPhase        string
		wantEffective    time.Duration
	}{
		{
			name:    "disabled returns not stalled",
			enabled: false,
			progress: stallProgressState{
				infraComplete: true,
			},
			wantStalled: false,
		},
		{
			name:          "infra phase within timeout",
			enabled:       true,
			stallTimeout:  baseTimeout,
			stallDuration: 20 * time.Minute,
			progress: stallProgressState{
				infraComplete: false,
				cpReady:       false,
			},
			wantStalled:   false,
			wantPhase:     "infrastructure",
			wantEffective: baseTimeout,
		},
		{
			name:          "infra phase exceeded timeout",
			enabled:       true,
			stallTimeout:  baseTimeout,
			stallDuration: 35 * time.Minute,
			progress: stallProgressState{
				infraComplete: false,
				cpReady:       false,
			},
			wantStalled:   true,
			wantPhase:     "infrastructure",
			wantEffective: baseTimeout,
		},
		{
			name:          "post-infra phase within 2x timeout",
			enabled:       true,
			stallTimeout:  baseTimeout,
			stallDuration: 45 * time.Minute,
			progress: stallProgressState{
				infraComplete:       true,
				cpReady:             false,
				infraTotalResources: 46,
				infraResourceReady:  46,
			},
			wantStalled:   false,
			wantPhase:     "post-infrastructure",
			wantEffective: 2 * baseTimeout,
		},
		{
			name:          "post-infra phase exceeded 2x timeout",
			enabled:       true,
			stallTimeout:  baseTimeout,
			stallDuration: 65 * time.Minute,
			progress: stallProgressState{
				infraComplete:       true,
				cpReady:             false,
				infraTotalResources: 46,
				infraResourceReady:  46,
			},
			wantStalled:   true,
			wantPhase:     "post-infrastructure",
			wantEffective: 2 * baseTimeout,
		},
		{
			name:          "both infra and cp ready uses base timeout",
			enabled:       true,
			stallTimeout:  baseTimeout,
			stallDuration: 35 * time.Minute,
			progress: stallProgressState{
				infraComplete: true,
				cpReady:       true,
			},
			wantStalled:   true,
			wantPhase:     "infrastructure",
			wantEffective: baseTimeout,
		},
		{
			name:          "zero stall duration never triggers",
			enabled:       true,
			stallTimeout:  baseTimeout,
			stallDuration: 0,
			progress: stallProgressState{
				infraComplete: false,
				cpReady:       false,
			},
			wantStalled:   false,
			wantPhase:     "infrastructure",
			wantEffective: baseTimeout,
		},
		{
			name:          "exactly at infra timeout does not trigger",
			enabled:       true,
			stallTimeout:  baseTimeout,
			stallDuration: baseTimeout,
			progress: stallProgressState{
				infraComplete: false,
				cpReady:       false,
			},
			wantStalled:   false,
			wantPhase:     "infrastructure",
			wantEffective: baseTimeout,
		},
		{
			name:          "exactly at post-infra timeout does not trigger",
			enabled:       true,
			stallTimeout:  baseTimeout,
			stallDuration: 2 * baseTimeout,
			progress: stallProgressState{
				infraComplete: true,
				cpReady:       false,
			},
			wantStalled:   false,
			wantPhase:     "post-infrastructure",
			wantEffective: 2 * baseTimeout,
		},
		{
			name:          "one nanosecond past infra timeout triggers",
			enabled:       true,
			stallTimeout:  baseTimeout,
			stallDuration: baseTimeout + 1,
			progress: stallProgressState{
				infraComplete: false,
				cpReady:       false,
			},
			wantStalled:   true,
			wantPhase:     "infrastructure",
			wantEffective: baseTimeout,
		},
		{
			name:          "one nanosecond past post-infra timeout triggers",
			enabled:       true,
			stallTimeout:  baseTimeout,
			stallDuration: 2*baseTimeout + 1,
			progress: stallProgressState{
				infraComplete: true,
				cpReady:       false,
			},
			wantStalled:   true,
			wantPhase:     "post-infrastructure",
			wantEffective: 2 * baseTimeout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectStallPhase(tt.enabled, tt.stallTimeout, tt.stallDuration, tt.progress)

			if result.stalled != tt.wantStalled {
				t.Errorf("stalled = %v, want %v", result.stalled, tt.wantStalled)
			}

			if !tt.enabled {
				if result.phase != "" {
					t.Errorf("phase = %q, want empty when disabled", result.phase)
				}
				if result.effectiveTimeout != 0 {
					t.Errorf("effectiveTimeout = %v, want 0 when disabled", result.effectiveTimeout)
				}
				return
			}

			if result.phase != tt.wantPhase {
				t.Errorf("phase = %q, want %q", result.phase, tt.wantPhase)
			}
			if result.effectiveTimeout != tt.wantEffective {
				t.Errorf("effectiveTimeout = %v, want %v", result.effectiveTimeout, tt.wantEffective)
			}
			if result.stallDuration != tt.stallDuration {
				t.Errorf("stallDuration = %v, want %v", result.stallDuration, tt.stallDuration)
			}
		})
	}
}
