package retry

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegular_Execute_Success(t *testing.T) {
	policy := NewRegular(3, 10*time.Millisecond)

	callCount := 0
	err := policy.Execute(context.Background(), func() error {
		callCount++
		if callCount < 2 {
			return errors.New("temporary error")
		}
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, 2, callCount)
}

func TestRegular_Execute_MaxRetriesExceeded(t *testing.T) {
	policy := NewRegular(3, 10*time.Millisecond)

	err := policy.Execute(context.Background(), func() error {
		return errors.New("permanent error")
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "max retries")
}

func TestRegular_Execute_ContextCancellation(t *testing.T) {
	policy := NewRegular(10, 1*time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := policy.Execute(ctx, func() error {
		return errors.New("error")
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled))
}

func TestRegular_Execute_FirstAttemptSuccess(t *testing.T) {
	policy := NewRegular(3, 10*time.Millisecond)

	callCount := 0
	err := policy.Execute(context.Background(), func() error {
		callCount++
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, 1, callCount)
}

func TestRegular_NewRegular_Defaults(t *testing.T) {
	policy := NewRegular(0, 0)
	assert.Equal(t, 1, policy.maxAttempts)
	assert.Equal(t, 10*time.Second, policy.wait)
}

func TestExponentialBackoff_Execute_Success(t *testing.T) {
	policy := NewExponentialBackoff(3, 10*time.Millisecond, 100*time.Millisecond)

	callCount := 0
	err := policy.Execute(context.Background(), func() error {
		callCount++
		if callCount < 2 {
			return errors.New("temporary error")
		}
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, 2, callCount)
}

func TestExponentialBackoff_Execute_MaxRetriesExceeded(t *testing.T) {
	policy := NewExponentialBackoff(3, 10*time.Millisecond, 100*time.Millisecond)

	err := policy.Execute(context.Background(), func() error {
		return errors.New("permanent error")
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "max retries")
}

func TestExponentialBackoff_Execute_ContextCancellation(t *testing.T) {
	policy := NewExponentialBackoff(10, 1*time.Second, 5*time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := policy.Execute(ctx, func() error {
		return errors.New("error")
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled))
}

func TestExponentialBackoff_Execute_BackoffIncreases(t *testing.T) {
	policy := NewExponentialBackoff(3, 20*time.Millisecond, 200*time.Millisecond)

	var timestamps []time.Time
	err := policy.Execute(context.Background(), func() error {
		timestamps = append(timestamps, time.Now())
		return errors.New("error")
	})

	require.Error(t, err)
	require.Len(t, timestamps, 3)

	// Verify backoff increases (with tolerance for jitter)
	gap1 := timestamps[1].Sub(timestamps[0])
	gap2 := timestamps[2].Sub(timestamps[1])
	assert.True(t, gap2 >= gap1, "backoff should increase: gap1=%v, gap2=%v", gap1, gap2)
}

func TestExponentialBackoff_NewExponentialBackoff_Defaults(t *testing.T) {
	policy := NewExponentialBackoff(0, 0, 0)
	assert.Equal(t, 1, policy.maxAttempts)
	assert.Equal(t, 1*time.Second, policy.initialWait)
	assert.Equal(t, 5*time.Minute, policy.maxWait)
}

func TestExponentialBackoff_CalculateBackoff(t *testing.T) {
	policy := NewExponentialBackoff(5, 10*time.Millisecond, 100*time.Millisecond)

	// First attempt: 10ms
	assert.Equal(t, 10*time.Millisecond, policy.calculateBackoff(0))

	// Second attempt: 20ms
	assert.Equal(t, 20*time.Millisecond, policy.calculateBackoff(1))

	// Third attempt: 40ms
	assert.Equal(t, 40*time.Millisecond, policy.calculateBackoff(2))

	// Fourth attempt: 80ms
	assert.Equal(t, 80*time.Millisecond, policy.calculateBackoff(3))

	// Fifth attempt: capped at maxWait
	assert.Equal(t, 100*time.Millisecond, policy.calculateBackoff(4))
}
