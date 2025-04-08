// Package cronjob provides a thread-safe wrapper around the robfig/cron library
// for managing scheduled jobs with unique identifiers. It offers:
// - Job management with unique IDs
// - Thread-safe operations using sync.RWMutex
// - Graceful start/stop functionality
// - Comprehensive error handling for common scenarios
// - Flexible configuration options including:
//   - Seconds precision
//   - Custom logger
//   - Timezone support
//
// - Batch job addition capability
//
// The package implements the CronJober interface which provides methods for:
// - Adding individual jobs (Add)
// - Adding multiple jobs in batch (AddBatch)
// - Updating or inserting jobs (Upsert)
// - Retrieving job information (Get)
// - Listing all jobs (Jobs)
// - Removing jobs (Remove)
// - Starting and stopping the scheduler (Start, Stop)
//
// Example usage:
//
//	cron, _ := cronjob.New()
//	cron.Add("job1", "* * * * *", func() { fmt.Println("Running job1") })
//	cron.Start()
//	defer cron.Stop()
package cronjob

import (
	"errors"
	"fmt"
	"sync"
	"time"

	cronlib "github.com/robfig/cron/v3"
)

var (
	// ErrJobNotFound indicates the requested job does not exist
	ErrJobNotFound = errors.New("job not found")
	// ErrJobIdEmpty indicates an empty job ID was provided
	ErrJobIdEmpty = errors.New("job id is empty")
	// ErrSpecEmpty indicates an empty cron spec was provided
	ErrSpecEmpty = errors.New("cron spec is empty")
	// ErrJobIdAlreadyExists indicates a job with the same ID already exists
	ErrJobIdAlreadyExists = errors.New("job id already exists")
)

// BatchFunc represents a batch job configuration containing:
// - JobId: Unique identifier for the job
// - Spec: The cron schedule specification
// - Func: The function to execute
type BatchFunc struct {
	JobId string
	Spec  string
	Func  func() error
}

// CronJober defines the interface for managing cron jobs
type CronJober interface {
	// Add schedules a new job with the given ID, cron spec, and function
	Add(jobId, spec string, f func() error) error

	// AddBatch schedules multiple jobs from a slice of BatchFunc.
	// If any validation fails, the entire batch is rejected.
	// If adding any job fails, all previously added jobs in the batch are rolled back.
	// Returns nil if all jobs were successfully added, or an error if any validation or addition fails.
	AddBatch(m []BatchFunc) error

	// Upsert updates an existing job or creates a new one if it doesn't exist
	Upsert(jobId, spec string, f func() error) error

	// Get retrieves the cron spec for a given job ID
	Get(jobId string) (spec string, ok bool)
	// Jobs returns a list of all job IDs
	Jobs() []string
	// Remove deletes a job with the given ID
	Remove(jobId string) error

	// Start begins the cron scheduler
	Start()

	// Stop gracefully shuts down the cron scheduler
	Stop()
}

// cronConf holds configuration options for the cron scheduler
type cronConf struct {
	// Whether to enable seconds precision in cron specs
	enableSeconds bool
	// Custom logger for the cron scheduler
	logger cronlib.Logger
	// Time zone for the cron scheduler
	location *time.Location

	retry uint
	wait  time.Duration
}

// Option defines a functional option for configuring the cron scheduler
type Option func(*cronConf)

// WithEnableSeconds enables the cron parser to interpret the first field as seconds
func WithEnableSeconds() Option {
	return func(opt *cronConf) {
		opt.enableSeconds = true
	}
}

// WithLogger sets a custom logger for the cron scheduler
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
	}
}

// cronJob represents a single scheduled job
type cronJob struct {
	jobId   string          // Unique identifier for the job
	spec    string          // Cron expression
	entryId cronlib.EntryID // Internal cron entry ID
}

// cronJobImpl is the concrete implementation of the CronJober interface
type cronJobImpl struct {
	mu         sync.RWMutex        // Mutex for thread-safe access
	jobs       map[string]*cronJob // Map of job IDs to cronJob instances
	cronClient *cronlib.Cron       // Underlying cron scheduler
	logger     cronlib.Logger

	retry uint
	wait  time.Duration
}

// New creates a new cron scheduler instance with optional configuration
func New(opts ...Option) (CronJober, error) {
	var c cronConf
	for _, opt := range opts {
		opt(&c)
	}

	var optList []cronlib.Option
	if c.enableSeconds {
		optList = append(optList, cronlib.WithSeconds())
	}
	if c.location != nil {
		optList = append(optList, cronlib.WithLocation(c.location))
	}
	if c.logger == nil {
		c.logger = cronlib.DefaultLogger
	}
	if c.retry == 0 {
		c.retry = 1
	}
	if c.wait == 0 {
		c.wait = time.Second
	}

	optList = append(optList, cronlib.WithLogger(c.logger))
	clinet := cronlib.New(optList...)

	return &cronJobImpl{
		jobs:       make(map[string]*cronJob),
		cronClient: clinet,
		logger:     c.logger,
		retry:      c.retry,
		wait:       c.wait,
	}, nil
}

// Add schedules a new job with the given ID, cron spec, and function
func (j *cronJobImpl) Add(jobId, spec string, f func() error) error {
	if jobId == "" {
		return ErrJobIdEmpty
	}
	if spec == "" {
		return ErrSpecEmpty
	}

	j.mu.Lock()
	defer j.mu.Unlock()

	_, ok := j.jobs[jobId]
	if ok {
		return ErrJobIdAlreadyExists
	}

	wrappedFunc := func() { j.runWithRetry(jobId, spec, f) }
	entryId, err := j.cronClient.AddFunc(spec, wrappedFunc)
	if err != nil {
		return err
	}

	j.jobs[jobId] = &cronJob{jobId: jobId, spec: spec, entryId: entryId}

	return nil
}

// AddBatch schedules multiple jobs from a slice of BatchFunc.
func (j *cronJobImpl) AddBatch(jobs []BatchFunc) error {
	j.mu.Lock()
	defer j.mu.Unlock()

	for _, job := range jobs {
		if job.JobId == "" {
			return ErrJobIdEmpty
		}
		if job.Spec == "" {
			return ErrSpecEmpty
		}
		_, ok := j.jobs[job.JobId]
		if ok {
			return ErrJobIdAlreadyExists
		}
	}

	var err error
	var entryId cronlib.EntryID
	addedJobs := make(map[string]cronlib.EntryID, 0)
	for _, job := range jobs {
		wrappedFunc := func() { j.runWithRetry(job.JobId, job.Spec, job.Func) }

		entryId, err = j.cronClient.AddFunc(job.Spec, wrappedFunc)
		if err != nil {
			for jobId, entryId := range addedJobs {
				delete(j.jobs, jobId)
				j.cronClient.Remove(entryId)
			}
			return err
		}

		addedJobs[job.JobId] = entryId
		j.jobs[job.JobId] = &cronJob{jobId: job.JobId, spec: job.Spec, entryId: entryId}
	}
	return nil
}

// Upsert updates an existing job or creates a new one if it doesn't exist
func (j *cronJobImpl) Upsert(jobId, spec string, f func() error) error {
	if jobId == "" {
		return ErrJobIdEmpty
	}
	if spec == "" {
		return ErrSpecEmpty
	}

	j.mu.Lock()
	defer j.mu.Unlock()

	wrappedFunc := func() { j.runWithRetry(jobId, spec, f) }
	_, ok := j.jobs[jobId]
	if ok {
		j.cronClient.Remove(j.jobs[jobId].entryId)
	}

	entryId, err := j.cronClient.AddFunc(spec, wrappedFunc)
	if err != nil {
		return err
	}

	j.jobs[jobId] = &cronJob{jobId: jobId, spec: spec, entryId: entryId}

	return nil
}

// Get retrieves the cron spec for a given job ID
func (j *cronJobImpl) Get(jobId string) (string, bool) {
	j.mu.RLock()
	defer j.mu.RUnlock()
	info, ok := j.jobs[jobId]
	if !ok {
		return "", false
	}

	return info.spec, true
}

func (j *cronJobImpl) runWithRetry(jobId, spec string, f func() error) {
	defer func() {
		if err := recover(); err != nil {
			j.logger.Error(fmt.Errorf("run job panic: %v", err), "job run failed", "jobId", jobId, "spec", spec)
		}
	}()

	for i := 0; i < int(j.retry); i++ {
		err := f()
		if err == nil {
			j.logger.Info("job run success", "jobId", jobId, "spec", spec)
			return
		}
		j.logger.Error(err, "job run failed", "jobId", jobId, "spec", spec)
		time.Sleep(j.wait)
	}
}

// Remove deletes a job with the given ID
func (j *cronJobImpl) Remove(jobId string) error {
	if jobId == "" {
		return ErrJobIdEmpty
	}

	j.mu.Lock()
	defer j.mu.Unlock()

	job, ok := j.jobs[jobId]
	if !ok {
		return ErrJobNotFound
	}

	j.cronClient.Remove(job.entryId)
	delete(j.jobs, jobId)

	return nil
}

// Jobs returns a list of all job IDs
func (j *cronJobImpl) Jobs() []string {
	j.mu.RLock()
	defer j.mu.RUnlock()
	jobIds := make([]string, 0, len(j.jobs))
	for jobId := range j.jobs {
		jobIds = append(jobIds, jobId)
	}
	return jobIds
}

// Start begins the cron scheduler
func (j *cronJobImpl) Start() {
	j.cronClient.Start()
}

// Stop gracefully shuts down the cron scheduler
func (j *cronJobImpl) Stop() {
	j.cronClient.Stop()
}
