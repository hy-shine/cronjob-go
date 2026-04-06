// Package job provides job management interfaces and implementations.
package job

import (
	cronlib "github.com/robfig/cron/v3"
)

// Info represents a single scheduled job.
type Info struct {
	JobId   string
	Spec    string
	EntryId cronlib.EntryID
}

// Repository defines the interface for job storage operations.
type Repository interface {
	// Add adds a new job, returns error if already exists.
	Add(info *Info) error

	// Get retrieves job info by ID.
	Get(jobId string) (*Info, bool)

	// Remove deletes a job by ID.
	Remove(jobId string) error

	// List returns all job IDs.
	List() []string

	// Len returns the count of jobs.
	Len() int

	// Clear removes all jobs.
	Clear()

	// Exists checks if a job ID exists.
	Exists(jobId string) bool
}
