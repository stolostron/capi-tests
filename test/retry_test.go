package test

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestRetryPolicyRetriesWithExponentialBackoff(t *testing.T) {
	var attempts int
	var delays []time.Duration

	policy := RetryPolicy{
		MaxAttempts:    3,
		InitialDelay:   100 * time.Millisecond,
		MaxDelay:       time.Second,
		JitterFraction: 0,
		Sleep: func(delay time.Duration) {
			delays = append(delays, delay)
		},
	}

	err := policy.Execute(func() error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary failure")
		}
		return nil
	}, func(error) bool {
		return true
	})

	if err != nil {
		t.Fatalf("Execute() returned unexpected error: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("Execute() made %d attempts, want 3", attempts)
	}
	if want := []time.Duration{100 * time.Millisecond, 200 * time.Millisecond}; !reflect.DeepEqual(delays, want) {
		t.Fatalf("sleep delays = %v, want %v", delays, want)
	}
}

func TestRetryPolicyStopsOnTerminalError(t *testing.T) {
	terminalErr := errors.New("invalid configuration")
	var attempts int
	var slept bool

	policy := RetryPolicy{
		MaxAttempts:  3,
		InitialDelay: time.Second,
		Sleep: func(time.Duration) {
			slept = true
		},
	}

	err := policy.Execute(func() error {
		attempts++
		return terminalErr
	}, func(error) bool {
		return false
	})

	if !errors.Is(err, terminalErr) {
		t.Fatalf("Execute() error = %v, want %v", err, terminalErr)
	}
	if attempts != 1 {
		t.Fatalf("Execute() made %d attempts, want 1", attempts)
	}
	if slept {
		t.Fatal("Execute() slept after a terminal error")
	}
}

func TestRetryPolicyReturnsLastErrorAfterExhaustion(t *testing.T) {
	firstErr := errors.New("first failure")
	lastErr := errors.New("last failure")
	var attempts int

	policy := RetryPolicy{
		MaxAttempts: 2,
		Sleep:       func(time.Duration) {},
	}

	err := policy.Execute(func() error {
		attempts++
		if attempts == 1 {
			return firstErr
		}
		return lastErr
	}, func(error) bool {
		return true
	})

	if !errors.Is(err, lastErr) {
		t.Fatalf("Execute() error = %v, want %v", err, lastErr)
	}
	if attempts != 2 {
		t.Fatalf("Execute() made %d attempts, want 2", attempts)
	}
}

func TestRetryPolicyCapsBackoffAndAppliesJitter(t *testing.T) {
	var delays []time.Duration

	policy := RetryPolicy{
		MaxAttempts:    3,
		InitialDelay:   100 * time.Millisecond,
		MaxDelay:       150 * time.Millisecond,
		JitterFraction: 0.5,
		Random: func() float64 {
			return 1
		},
		Sleep: func(delay time.Duration) {
			delays = append(delays, delay)
		},
	}

	_ = policy.Execute(func() error {
		return errors.New("temporary failure")
	}, func(error) bool {
		return true
	})

	if want := []time.Duration{150 * time.Millisecond, 150 * time.Millisecond}; !reflect.DeepEqual(delays, want) {
		t.Fatalf("sleep delays = %v, want %v", delays, want)
	}
}
