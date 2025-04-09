// Package cronjob provides a thread-safe wrapper around the robfig/cron library
// for managing scheduled jobs with unique identifiers.
//
// Features:
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
//
//	f := func() error {
//			fmt.Println("Running job")
//			return nil
//	}
//	cron.Add("job1", "* * * * *", f)
//	cron.Start()
//	defer cron.Stop()
package cronjob

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	cronlib "github.com/robfig/cron/v3"
)

const (
	// retryModeRegular indicates regular retry behavior
	retryModeRegular = "regular"
	// retryModeBackoff indicates exponential backoff retry behavior
	retryModeBackoff = "backoff"
)

const (
	initialWaitDuration    = 1 * time.Second
	defaultWaitDuration    = 10 * time.Second
	defaultMaxWaitDuration = 5 * time.Minute
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

// BatchFunc represents a batch job configuration
type BatchFunc struct {
	JobId string       // Unique identifier for the job in scheduler
	Spec  string       // The cron schedule specification
	Func  func() error // The function to execute
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
	enableSeconds bool           // Whether to enable seconds precision in cron specs
	logger        cronlib.Logger // Custom logger for the cron scheduler
	location      *time.Location // Time zone for the cron scheduler

	retry       uint          // Number of retry attempts
	retryMode   string        // Retry mode (regular or backoff)
	initialWait time.Duration // Initial wait duration for retries
	wait        time.Duration // Wait duration between retries
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
	randGen    *rand.Rand          // Random number generator
	cronConf                       // Embedded configuration
}

// New creates a new cron scheduler instance with optional configuration
func New(opts ...Option) (CronJober, error) {
	instance := &cronJobImpl{
		jobs: make(map[string]*cronJob),
		cronConf: cronConf{
			retry:       1,
			logger:      cronlib.DefaultLogger,
			retryMode:   retryModeRegular,
			initialWait: initialWaitDuration,
			wait:        defaultWaitDuration,
		},
		randGen: rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	for _, opt := range opts {
		opt(&instance.cronConf)
	}

	var optList []cronlib.Option
	if instance.enableSeconds {
		optList = append(optList, cronlib.WithSeconds())
	}
	if instance.location != nil {
		optList = append(optList, cronlib.WithLocation(instance.location))
	}
	if instance.retryMode == retryModeBackoff {
		if instance.wait < instance.initialWait {
			return nil, errors.New("wait must be greater than initTime")
		}
		if instance.initialWait == 0 {
			instance.initialWait = initialWaitDuration
		}
		if instance.wait == 0 {
			instance.wait = defaultMaxWaitDuration
		}
	}
	if instance.retry == 0 {
		instance.retry = 1
	}
	if instance.wait == 0 {
		instance.wait = defaultWaitDuration
	}
	optList = append(optList, cronlib.WithLogger(instance.logger))

	instance.cronClient = cronlib.New(optList...)

	return instance, nil
}

func checkJob(jobId string, spec string) error {
	if jobId == "" {
		return ErrJobIdEmpty
	}
	if spec == "" {
		return fmt.Errorf("cron spec cannot be empty")
	}
	return nil
}

// Add schedules a new job with the given ID, cron spec, and function
func (j *cronJobImpl) Add(jobId, spec string, f func() error) error {
	if err := checkJob(jobId, spec); err != nil {
		return err
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
		if err := checkJob(job.JobId, job.Spec); err != nil {
			return err
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
	if err := checkJob(jobId, spec); err != nil {
		return err
	}

	j.mu.Lock()
	defer j.mu.Unlock()

	wrappedFunc := func() { j.runWithRetry(jobId, spec, f) }

	// Capture existing entry before modification
	existingJob, exists := j.jobs[jobId]
	if exists {
		// Remove existing entry before adding new one
		j.cronClient.Remove(existingJob.entryId)
	}

	entryId, err := j.cronClient.AddFunc(spec, wrappedFunc)
	if err != nil {
		return fmt.Errorf("failed to add updated job: %w", err)
	}

	j.jobs[jobId] = &cronJob{
		jobId:   jobId,
		spec:    spec,
		entryId: entryId,
	}

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

	var lastErr error
	var waitTime time.Duration
	for i := 0; i < int(j.retry); i++ {
		if err := f(); err == nil {
			j.logger.Info("job run success", "jobId", jobId, "spec", spec)
			return
		} else {
			lastErr = err
		}

		j.logger.Error(lastErr, "job run failed", "jobId", jobId, "spec", spec)

		if j.retryMode == retryModeBackoff {
			// Calculate exponential backoff with jitter
			backoff := min(j.initialWait*(1<<i), j.wait) // Exponential backoff: initialWait * 2^i
			halfBackoff := backoff >> 1
			jitter := time.Duration(j.randGen.Int63n(int64(halfBackoff))) // Random jitter up to halfBackoff
			waitTime = halfBackoff + jitter                               // Apply jitter but don't exceed max wait
		} else {
			waitTime = j.wait
		}
		time.Sleep(waitTime)
	}

	if lastErr != nil {
		j.logger.Error(lastErr, "job run failed after retries", "jobId", jobId, "spec", spec)
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
