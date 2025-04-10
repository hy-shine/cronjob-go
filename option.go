package cronjob

import (
	"time"

	cronlib "github.com/robfig/cron/v3"
)

// Option defines a functional option for configuring the cron scheduler
type Option func(*cronConf)

// WithCronSeconds enables the cron parser to interpret the first field as seconds
func WithCronSeconds() Option {
	return func(opt *cronConf) {
		opt.enableSeconds = true
	}
}

// WithSkipIfJobRunning skips an invocation of the Job if a previous invocation is
// still running. It logs skips to the given logger at Info level.
func WithSkipIfJobRunning() Option {
	return func(opt *cronConf) {
		opt.skipIfJobRunning = true
	}
}

// WithLogger sets a custom logger for the cron scheduler. The provided logger
// must be safe for concurrent use by multiple goroutines as it may be called
// simultaneously from different jobs.
func WithLogger(logger cronlib.Logger) Option {
	return func(opt *cronConf) {
		opt.logger = logger
	}
}

// WithLocation sets the time zone for the cron scheduler
func WithLocation(loc *time.Location) Option {
	return func(opt *cronConf) {
		opt.location = loc
	}
}

// WithRetry sets the retry count and wait duration for the cron scheduler
func WithRetry(retry uint, wait time.Duration) Option {
	return func(opt *cronConf) {
		opt.retry = retry
		opt.wait = wait
		opt.retryMode = retryModeRegular
	}
}

// WithRetryBackoff configures exponential backoff retry behavior for failed jobs.
// It takes three parameters:
//   - retry: The maximum number of retry attempts
//   - initialWait: The initial wait duration between retries
//   - maxWait: The maximum wait duration between retries
//
// The wait time doubles with each retry attempt (initialWait * 2^i).
func WithRetryBackoff(retry uint, initialWait, maxWait time.Duration) Option {
	return func(opt *cronConf) {
		opt.retry = retry
		opt.wait = maxWait
		opt.initialWait = initialWait
		opt.retryMode = retryModeBackoff
	}
}
