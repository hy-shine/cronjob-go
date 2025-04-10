# CronJob Package

[简体中文](README_zh-CN.md) | [English](README.md)

[![Go Reference](https://pkg.go.dev/badge/github.com/hy-shine/cronjob-go.svg)](https://pkg.go.dev/github.com/hy-shine/cronjob-go) [![Go Report Card](https://goreportcard.com/badge/github.com/hy-shine/cronjob-go)](https://goreportcard.com/report/github.com/hy-shine/cronjob-go) [![LICENSE](https://img.shields.io/github/license/hy-shine/cronjob-go)](https://github.com/hy-shine/cronjob-go/blob/master/LICENSE)

A thread-safe wrapper around the [robfig/cron/v3](https://github.com/robfig/cron) library for managing scheduled jobs with unique identifiers.

## Features

*   **Job Management:** Add, update, remove, and query jobs using unique string IDs.
*   **Thread Safety:** All operations are safe for concurrent use via `sync.RWMutex`.
*   **Graceful Lifecycle:** Start and stop the scheduler gracefully.
*   **Flexible Scheduling:** Supports standard cron specifications and optional seconds field precision.
*   **Timezone Support:** Configure jobs to run in specific timezones.
*   **Configurable Logging:** Integrate with your existing logging infrastructure using `cronlib.Logger`.
*   **Flexible Retries:** Built-in support for retrying failed jobs with configurable strategies.
*   **Skip Concurrent Runs:** Option to prevent a job from starting if its previous invocation is still running.
*   **Batch Operations:** Add multiple jobs atomically (`AddBatch`).
*   **Clear All Jobs:** Remove all scheduled jobs (`Clear`).

## Installation

```bash
go get github.com/hy-shine/cronjob-go
```

## Quick Start

```go
import (
	"fmt"

	"github.com/hy-shine/cronjob-go"
)

func main() {
	cron, err := cronjob.New(
		cronjob.WithCronSeconds(),
	)
	if err != nil {
		panic(err)
	}

	// Add a job that runs every 5 seconds
	err = cron.Add("job1", "*/5 * * * * *", func() error {
		fmt.Info("Running job1")
		return nil
	})
	if err != nil {
		fmt.Println("Failed to add job1", "error", err)
	}

	// Add job2
	err = cron.Add("job2", "@every 10s", func() error {
		fmt.Info("Running job2")
		return nil
	})
	if err != nil {
		fmt.Error("Failed to add job2", "error", err)
	}

	// Start the scheduler
	cron.Start()
	fmt.Info("Cron scheduler started")

	// Ensure scheduler stops gracefully on exit
	defer cron.Stop()

	select {}
}
```

## API Reference

### Configuration Options

Pass these to `cronjob.New()`:

*   `cronjob.WithCronSeconds()`: Enables the seconds field in cron specs (e.g., `* * * * * *`).
*   `cronjob.WithSkipIfJobRunning()`: Prevents a job from running if its previous instance is still active.
*   `cronjob.WithLogger(logger cronlib.Logger)`: Sets a custom logger.
*   `cronjob.WithLocation(loc *time.Location)`: Sets the timezone for interpreting schedules (default: `time.Local`).
*   `cronjob.WithRetry(retry uint, wait time.Duration)`: Configures regular retries (fixed `wait` duration). `retry` is the number of attempts *after* the initial failure.
*   `cronjob.WithRetryBackoff(retry uint, initialWait, maxWait time.Duration)`: Configures exponential backoff retries. Wait time starts at `initialWait`, doubles each time (with jitter), up to `maxWait`.

### Predefined Errors

*   `cronjob.ErrJobNotFound`: The specified job ID does not exist.
*   `cronjob.ErrJobIdEmpty`: An empty string was provided as a job ID.
*   `cronjob.ErrSpecEmpty`: An empty string was provided as a cron spec.
*   `cronjob.ErrJobIdAlreadyExists`: Attempted to add a job with an ID that is already in use.

## Advanced Usage

### Batch Operations

Add multiple jobs in a single, atomic operation. If any job fails validation or addition, the entire batch is rolled back.

```go
jobs := []cronjob.BatchFunc{
	{JobId: "batchJob1", Spec: "0 0 * * *", Func: func() error { fmt.Println("Batch Job 1"); return nil }},
	{JobId: "batchJob2", Spec: "@hourly", Func: func() error { fmt.Println("Batch Job 2"); return nil }},
}
err := cron.AddBatch(jobs)
if err != nil {
	// Handle error
}
```

### Retry Mechanisms

**Regular Retry**

```go
cron, _ := cronjob.New(cronjob.WithRetry(5, 10*time.Second))
```

**Exponential Backoff Retry**

```go
cron, _ := cronjob.New(cronjob.WithRetryBackoff(3, 1*time.Second, 30*time.Second))
```

### Updating Jobs

Modify the schedule or function of an existing job, or add it if it doesn't exist.

```go
// Change job1 to run every 10 seconds
err := cron.Upsert("job1", "*/10 * * * * *", func() error {
	logger.Info("Running updated job1")
	return nil
})
```

## Best Practices

1. Always call Stop() when done (preferably with defer)
2. Use meaningful job IDs for easier management
3. Handle errors in job functions properly
4. For time-sensitive jobs, enable seconds precision
5. Set appropriate retry counts based on job criticality

## Contributing

Contributions are welcome! Please feel free to submit issues or pull requests.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
