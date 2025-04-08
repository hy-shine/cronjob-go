# CronJob Package

A thread-safe wrapper around the robfig/cron library for managing scheduled jobs with unique identifiers.

## Features

- Job management with unique IDs
- Thread-safe operations using sync.RWMutex
- Graceful start/stop functionality
- Comprehensive error handling
- Flexible configuration options:
  - Seconds precision
  - Custom logger
  - Timezone support
  - Retry mechanism
- Batch job operations

## Installation

```bash
go get github.com/hy-shine/cronjob
```

## Quick Start

```go
package main

import (
	"fmt"
	
	"github.com/hy-shine/cronjob"
)

func main() {
	// Create a new cron scheduler
	cron, _ := cronjob.New()
	
	// Add a job
	cron.Add("job1", "* * * * *", func() error {
		fmt.Println("Running job1")
		return nil
	})
	
	// Start the scheduler
	cron.Start()
	defer cron.Stop()
}
```

## API Reference

### CronJober Interface

```go
type CronJober interface {
	// Add schedules a new job
	Add(jobId, spec string, f func() error) error
	// AddBatch schedules multiple jobs (atomic operation)
	AddBatch(jobs []BatchFunc) error
	// Upsert updates or creates a job
	Upsert(jobId, spec string, f func() error) error
	// Get retrieves a job's cron spec
	Get(jobId string) (spec string, ok bool)
	// Jobs lists all job IDs
	Jobs() []string
	// Remove deletes a job
	Remove(jobId string) error
	// Start begins the scheduler
	Start()
	// Stop gracefully shuts down the scheduler
	Stop()
}
```

### Configuration Options

```go
// WithEnableSeconds enables seconds precision
cronjob.WithEnableSeconds()

// WithLogger sets a custom logger
cronjob.WithLogger(logger)

// WithLocation sets the timezone
cronjob.WithLocation(loc)

// WithRetry configures retry behavior
cronjob.WithRetry(retryCount, waitDuration)
```

### Error Types

```go
ErrJobNotFound       // Job does not exist
ErrJobIdEmpty        // Empty job ID provided  
ErrSpecEmpty         // Empty cron spec provided
ErrJobIdAlreadyExists // Job ID already exists
```

## Advanced Usage

### Batch Operations

```go
jobs := []cronjob.BatchFunc{
	{
		JobId: "job1",
		Spec:  "*/5 * * * *",
		Func:  func() error { /* ... */ },
	},
	{
		JobId: "job2", 
		Spec:  "0 * * * *",
		Func:  func() error { /* ... */ },
	},
}

err := cron.AddBatch(jobs)
```

### Retry Mechanism

Jobs automatically retry on failure (default: 1 retry with 1 second wait):

```go
cron, _ := cronjob.New(
	cronjob.WithRetry(3, 2*time.Second)
)
```

## Best Practices

1. Always call `Stop()` when done (preferably with defer)
2. Use meaningful job IDs for easier management
3. Handle errors in job functions properly
4. For time-sensitive jobs, enable seconds precision
5. Set appropriate retry counts based on job criticality
