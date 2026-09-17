package test

import (
	"errors"
	"math/rand"
	"time"
)

// RetryPolicy controls bounded retries for operations that may fail transiently.
// MaxAttempts includes the initial operation attempt.
type RetryPolicy struct {
	MaxAttempts    int
	InitialDelay   time.Duration
	MaxDelay       time.Duration
	JitterFraction float64
	Sleep          func(time.Duration)
	Random         func() float64
}

// Execute runs operation until it succeeds, returns a terminal error, or
// exhausts the configured attempts. The retryable callback classifies errors
// so callers can avoid retrying permanent failures.
func (p RetryPolicy) Execute(operation func() error, retryable func(error) bool) error {
	if operation == nil {
		return errors.New("retry operation is nil")
	}

	maxAttempts := p.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 1
	}

	sleep := p.Sleep
	if sleep == nil {
		sleep = time.Sleep
	}

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err := operation()
		if err == nil {
			return nil
		}
		if attempt == maxAttempts || retryable == nil || !retryable(err) {
			return err
		}

		if delay := p.delay(attempt); delay > 0 {
			sleep(delay)
		}
	}

	return errors.New("retry attempts exhausted")
}

func (p RetryPolicy) delay(attempt int) time.Duration {
	if p.InitialDelay <= 0 {
		return 0
	}

	delay := p.InitialDelay
	for i := 1; i < attempt; i++ {
		if delay > time.Duration(1<<63-1)/2 {
			delay = time.Duration(1<<63 - 1)
			break
		}
		delay *= 2
	}
	if p.MaxDelay > 0 && delay > p.MaxDelay {
		delay = p.MaxDelay
	}

	jitterFraction := p.JitterFraction
	if jitterFraction < 0 {
		jitterFraction = 0
	}
	if jitterFraction > 1 {
		jitterFraction = 1
	}
	if jitterFraction == 0 {
		return delay
	}

	random := p.Random
	if random == nil {
		random = rand.Float64
	}
	value := random()
	if value < 0 {
		value = 0
	}
	if value > 1 {
		value = 1
	}

	factor := 1 + (2*value-1)*jitterFraction
	delay = time.Duration(float64(delay) * factor)
	if p.MaxDelay > 0 && delay > p.MaxDelay {
		return p.MaxDelay
	}
	return delay
}
