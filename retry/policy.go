// Package retry provides retry policies for job execution.
package retry

import (
	"context"
	"fmt"
	"math/rand"
	"time"
)

// Policy defines the interface for retry behavior.
type Policy interface {
	// Execute runs the given function with retry logic.
	Execute(ctx context.Context, fn func() error) error
}

// Regular implements fixed-interval retry.
type Regular struct {
	maxAttempts int
	wait        time.Duration
}

// NewRegular creates a new Regular retry policy.
func NewRegular(maxAttempts int, wait time.Duration) *Regular {
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	if wait <= 0 {
		wait = 10 * time.Second
	}
	return &Regular{
		maxAttempts: maxAttempts,
		wait:        wait,
	}
}

// Execute runs the function with fixed-interval retry.
func (r *Regular) Execute(ctx context.Context, fn func() error) error {
	var lastErr error
	for i := 0; i < r.maxAttempts; i++ {
		if err := fn(); err == nil {
			return nil
		} else {
			lastErr = err
		}

		if i < r.maxAttempts-1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(r.wait):
			}
		}
	}
	return fmt.Errorf("max retries (%d) exceeded: %w", r.maxAttempts, lastErr)
}

// ExponentialBackoff implements exponential backoff retry with jitter.
type ExponentialBackoff struct {
	maxAttempts int
	initialWait time.Duration
	maxWait     time.Duration
	randGen     *rand.Rand
}

// NewExponentialBackoff creates a new ExponentialBackoff retry policy.
func NewExponentialBackoff(maxAttempts int, initialWait, maxWait time.Duration) *ExponentialBackoff {
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	if initialWait <= 0 {
		initialWait = 1 * time.Second
	}
	if maxWait <= 0 {
		maxWait = 5 * time.Minute
	}
	return &ExponentialBackoff{
		maxAttempts: maxAttempts,
		initialWait: initialWait,
		maxWait:     maxWait,
		randGen:     rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Execute runs the function with exponential backoff retry.
func (e *ExponentialBackoff) Execute(ctx context.Context, fn func() error) error {
	var lastErr error
	for i := 0; i < e.maxAttempts; i++ {
		if err := fn(); err == nil {
			return nil
		} else {
			lastErr = err
		}

		if i < e.maxAttempts-1 {
			backoff := e.calculateBackoff(i)
			jitter := time.Duration(e.randGen.Int63n(int64(backoff / 2)))

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff/2 + jitter):
			}
		}
	}
	return fmt.Errorf("max retries (%d) exceeded: %w", e.maxAttempts, lastErr)
}

func (e *ExponentialBackoff) calculateBackoff(attempt int) time.Duration {
	backoff := e.initialWait * time.Duration(1<<uint(attempt))
	if backoff > e.maxWait {
		backoff = e.maxWait
	}
	return backoff
}
