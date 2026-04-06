// Package errors provides error definitions for the cronjob package.
package errors

import "errors"

// Sentinel errors for cronjob operations.
var (
	// ErrJobNotFound indicates the requested job does not exist.
	ErrJobNotFound = errors.New("job not found")

	// ErrJobIdEmpty indicates an empty job ID was provided.
	ErrJobIdEmpty = errors.New("job id is empty")

	// ErrSpecEmpty indicates an empty cron spec was provided.
	ErrSpecEmpty = errors.New("cron spec is empty")

	// ErrJobIdAlreadyExists indicates a job with the same ID already exists.
	ErrJobIdAlreadyExists = errors.New("job id already exists")

	// ErrJobInfoNil indicates nil job info was provided.
	ErrJobInfoNil = errors.New("job info is nil")

	// ErrInvalidConfig indicates invalid configuration.
	ErrInvalidConfig = errors.New("invalid configuration")
)
