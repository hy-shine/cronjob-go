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
		assert.Equal(t, spec, s.Spec)
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
		assert.True(t, errors.Is(err, ErrJobNotFound))
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
		WithCronSeconds(),
	)

	var attemptCounter int
	err := c.Add("retryJob", "*/1 * * * * *", func() error {
		attemptCounter++
		return errors.New("simulated error")
	})
	require.NoError(t, err)

	c.Start()
	defer c.Stop()

	time.Sleep(time.Second)
	assert.Equal(t, 3, attemptCounter)
}

func TestBackoffRetry(t *testing.T) {
	c, _ := New(
		WithCronSeconds(),
		WithRetryBackoff(3, 20*time.Millisecond, 100*time.Millisecond),
	)

	var attempts []time.Time
	err := c.Add("backoffJob", "* * * * * *", func() error {
		attempts = append(attempts, time.Now())
		return errors.New("simulated error")
	})
	require.NoError(t, err)

	c.Start()
	defer c.Stop()

	time.Sleep(time.Second)
	require.Len(t, attempts, 3)
	fmt.Println(attempts[1].Sub(attempts[0]).Milliseconds(), attempts[2].Sub(attempts[1]))
	assert.True(t, attempts[1].Sub(attempts[0]) >= 10*time.Millisecond)
	assert.True(t, attempts[2].Sub(attempts[1]) >= 20*time.Millisecond)
}

func TestPanicRecovery(t *testing.T) {
	c, _ := New(
		WithCronSeconds(),
	)

	err := c.Add("panicJob", "* * * * * *", func() error {
		panic("simulated panic")
	})
	require.NoError(t, err)

	c.Start()
	defer c.Stop()

	// Should not crash
	time.Sleep(time.Second)
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
		assert.Equal(t, originalSpec, s.Spec)
	})

	t.Run("update existing job", func(t *testing.T) {
		err := c.Upsert(jobID, newSpec, func() error { return nil })
		require.NoError(t, err)
		s, _ := c.Get(jobID)
		assert.Equal(t, newSpec, s.Spec)
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

func TestConcurrencyStress_Upsert(t *testing.T) {
	c, _ := New()
	const workers = 50000
	var wg sync.WaitGroup

	t.Run("high concurrency operations", func(t *testing.T) {
		wg.Add(workers)
		for i := 0; i < workers; i++ {
			go func(idx int) {
				defer wg.Done()
				err := c.Upsert(fmt.Sprintf("job%d", idx%50000), fmt.Sprintf("%d * * * *", (idx+1)%60), func() error {
					return nil
				})
				assert.NoError(t, err)
			}(i)
		}
		wg.Wait()

		assert.Equal(t, c.Len(), 50000)
	})
}

func TestErrorRecovery(t *testing.T) {
	c, _ := New(
		WithRetry(3, 100*time.Millisecond),
		WithCronSeconds(),
	)

	t.Run("permanent failure", func(t *testing.T) {
		var attempts int
		c.Add("permFail", "* * * * * *", func() error {
			attempts++
			return errors.New("permanent error")
		})

		c.Start()
		defer c.Stop()

		time.Sleep(time.Second)
		assert.Equal(t, 3, attempts)
	})
}

func TestConfigurationValidation(t *testing.T) {
	t.Run("invalid logger", func(t *testing.T) {
		c, err := New(WithLogger(nil))
		require.NoError(t, err)
		require.NotNil(t, c)
	})

	t.Run("invalid location", func(t *testing.T) {
		_, err := New(WithLocation(time.FixedZone("Invalid", 24*3600)))
		require.NoError(t, err)
	})
}

func TestBatchEdgeCases(t *testing.T) {
	c, _ := New()

	t.Run("empty batch", func(t *testing.T) {
		err := c.AddBatch(nil)
		require.NoError(t, err)
	})

	t.Run("mixed valid and invalid", func(t *testing.T) {
		batch := []BatchFunc{
			{"valid1", "* * * * *", func() error { return nil }},
			{"", "* * * * *", func() error { return nil }},
			{"valid2", "* * * * *", func() error { return nil }},
		}

		err := c.AddBatch(batch)
		require.Error(t, err)
		assert.Len(t, c.Jobs(), 0)
	})
}

func TestClearOperations(t *testing.T) {
	c, _ := New()

	t.Run("clear populated jobs", func(t *testing.T) {
		c.Add("job1", "* * * * *", func() error { return nil })
		c.Add("job2", "* * * * *", func() error { return nil })

		c.Clear()
		assert.Empty(t, c.Jobs())
	})
}

func TestTimePrecision(t *testing.T) {
	t.Run("seconds precision", func(t *testing.T) {
		c, _ := New(WithCronSeconds())
		var counter int

		c.Add("secJob", "*/1 * * * * *", func() error {
			counter++
			return nil
		})

		c.Start()
		defer c.Stop()

		time.Sleep(2500 * time.Millisecond)
		assert.Equal(t, 2, counter)
	})
}

func BenchmarkAddJobs(b *testing.B) {
	cron, _ := New(WithCronSeconds())
	cron.Start()
	defer cron.Stop()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			jobId := fmt.Sprintf("job-%d", i)
			cron.Add(jobId, "* * * * * *", func() error { return nil })
			i++
		}
	})
}
