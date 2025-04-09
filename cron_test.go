package cronjob

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Run("default configuration", func(t *testing.T) {
		c, err := New()
		require.NoError(t, err)
		assert.NotNil(t, c)
	})

	t.Run("with invalid backoff configuration", func(t *testing.T) {
		_, err := New(WithRetryBackoff(3, 2*time.Second, 1*time.Second))
		assert.Error(t, err)
	})
}

func TestBasicJobOperations(t *testing.T) {
	c, _ := New()
	const testJobID = "job1"
	const spec = "* * * * *"

	t.Run("add and get job", func(t *testing.T) {
		err := c.Add(testJobID, spec, func() error { return nil })
		require.NoError(t, err)

		s, ok := c.Get(testJobID)
		assert.True(t, ok)
		assert.Equal(t, spec, s)
	})

	t.Run("remove job", func(t *testing.T) {
		err := c.Remove(testJobID)
		assert.NoError(t, err)

		_, ok := c.Get(testJobID)
		assert.False(t, ok)
	})
}

func TestErrorConditions(t *testing.T) {
	c, _ := New()

	t.Run("empty job ID", func(t *testing.T) {
		err := c.Add("", "* * * * *", func() error { return nil })
		assert.Equal(t, ErrJobIdEmpty, err)
	})

	t.Run("empty cron spec", func(t *testing.T) {
		err := c.Add("job1", "", func() error { return nil })
		assert.Error(t, err)
	})

	t.Run("duplicate job ID", func(t *testing.T) {
		c.Add("dup", "* * * * *", func() error { return nil })
		err := c.Add("dup", "* * * * *", func() error { return nil })
		assert.Equal(t, ErrJobIdAlreadyExists, err)
	})

	t.Run("remove non-existent job", func(t *testing.T) {
		err := c.Remove("nonexistent")
		assert.Equal(t, ErrJobNotFound, err)
	})
}

func TestConcurrentOperations(t *testing.T) {
	c, _ := New()
	const parallelCount = 100
	var wg sync.WaitGroup

	t.Run("parallel adds", func(t *testing.T) {
		wg.Add(parallelCount)
		for i := 0; i < parallelCount; i++ {
			go func(idx int) {
				defer wg.Done()
				c.Add(fmt.Sprintf("job%d", idx), "* * * * *", func() error { return nil })
			}(i)
		}
		wg.Wait()

		assert.Len(t, c.Jobs(), parallelCount)
	})

	t.Run("parallel removes", func(t *testing.T) {
		wg.Add(parallelCount)
		for i := 0; i < parallelCount; i++ {
			go func(idx int) {
				defer wg.Done()
				c.Remove(fmt.Sprintf("job%d", idx))
			}(i)
		}
		wg.Wait()

		assert.Empty(t, c.Jobs())
	})
}

func TestBatchOperations(t *testing.T) {
	c, _ := New()
	validBatch := []BatchFunc{
		{"batch1", "* * * * *", func() error { return nil }},
		{"batch2", "* * * * *", func() error { return nil }},
	}

	t.Run("successful batch", func(t *testing.T) {
		err := c.AddBatch(validBatch)
		require.NoError(t, err)
		assert.Len(t, c.Jobs(), 2)
	})

	t.Run("rollback on partial failure", func(t *testing.T) {
		invalidBatch := append(validBatch, BatchFunc{"", "* * * * *", func() error { return nil }})
		err := c.AddBatch(invalidBatch)
		require.Error(t, err)
		assert.Len(t, c.Jobs(), 2) // Original batch remains
	})
}

func TestRetryMechanisms(t *testing.T) {
	c, _ := New(
		WithRetry(3, 100*time.Millisecond), // 100ms
		WithEnableSeconds(),
	)

	var attemptCounter int
	err := c.Add("retryJob", "*/1 * * * * *", func() error {
		attemptCounter++
		return errors.New("simulated error")
	})
	require.NoError(t, err)

	c.Start()
	defer c.Stop()

	time.Sleep(2 * time.Second)
	assert.Equal(t, 6, attemptCounter)
}

func TestBackoffRetry(t *testing.T) {
	c, _ := New(
		WithEnableSeconds(),
		WithRetryBackoff(3, 10*time.Millisecond, 100*time.Millisecond),
	)

	var attempts []time.Time
	err := c.Add("backoffJob", "*/1 * * * * *", func() error {
		attempts = append(attempts, time.Now())
		return errors.New("simulated error")
	})
	require.NoError(t, err)

	c.Start()
	defer c.Stop()

	time.Sleep(300 * time.Millisecond)
	require.Len(t, attempts, 3)
	assert.True(t, attempts[1].Sub(attempts[0]) >= 10*time.Millisecond)
	assert.True(t, attempts[2].Sub(attempts[1]) >= 20*time.Millisecond)
}

func TestPanicRecovery(t *testing.T) {
	c, _ := New()

	err := c.Add("panicJob", "* * * * *", func() error {
		panic("simulated panic")
	})
	require.NoError(t, err)

	c.Start()
	defer c.Stop()

	// Should not crash
	time.Sleep(100 * time.Millisecond)
}

func TestUpsertOperation(t *testing.T) {
	c, _ := New()
	const jobID = "upsertJob"
	originalSpec := "* * * * *"
	newSpec := "*/2 * * * *"

	t.Run("insert new job", func(t *testing.T) {
		err := c.Upsert(jobID, originalSpec, func() error { return nil })
		require.NoError(t, err)
		s, _ := c.Get(jobID)
		assert.Equal(t, originalSpec, s)
	})

	t.Run("update existing job", func(t *testing.T) {
		err := c.Upsert(jobID, newSpec, func() error { return nil })
		require.NoError(t, err)
		s, _ := c.Get(jobID)
		assert.Equal(t, newSpec, s)
	})
}

func TestTimeZoneSupport(t *testing.T) {
	loc, _ := time.LoadLocation("America/New_York")
	c, _ := New(WithLocation(loc))

	err := c.Add("tzJob", "0 12 * * *", func() error { return nil })
	require.NoError(t, err)

	s, _ := c.Get("tzJob")
	assert.Equal(t, "0 12 * * *", s)
}
