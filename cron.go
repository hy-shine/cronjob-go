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
	Get(jobId string) (info JobInfo, ok bool)

	// Jobs returns a list of all job IDs
	Jobs() []string
	// Len returns the number of jobs
	Len() int

	// Remove deletes a job with the given ID
	Remove(jobId ...string) error

	// Clear removes all jobs from the cron scheduler
	Clear()

	// Start begins the cron scheduler
	Start()

	// Stop gracefully shuts down the cron scheduler
	Stop()
}

// cronConf holds configuration options for the cron scheduler
// cronConf holds the configuration options for the cron scheduler
type cronConf struct {
	// enableSeconds determines if the cron parser should interpret the first field as seconds
	enableSeconds bool

	// logger is the custom logger instance for the cron scheduler
	logger cronlib.Logger

	// location specifies the time zone for the cron scheduler
	location *time.Location

	// skipIfJobRunning determines if a job should be skipped if it is still running
	skipIfJobRunning bool

	// retry specifies the number of retry attempts for failed jobs
	retry uint
	// retryMode defines the retry strategy (regular or backoff)
	retryMode string
	// initialWait is the initial wait duration before retries (used in backoff mode)
	initialWait time.Duration
	// wait is the wait duration between retries
	wait time.Duration
}

// JobInfo represents a single scheduled job
type JobInfo struct {
	JobId   string          // Unique identifier for the job
	Spec    string          // Cron expression
	entryId cronlib.EntryID // Internal cron entry ID
}

// cronJobImpl is the concrete implementation of the CronJober interface
type cronJobImpl struct {
	mu         sync.RWMutex        // Mutex for thread-safe access
	jobs       map[string]*JobInfo // Map of job IDs to cronJob instances
	cronClient *cronlib.Cron       // Underlying cron scheduler
	randGen    *rand.Rand          // Random number generator
	cronConf                       // Embedded configuration
}

// New creates a new cron scheduler instance with optional configuration
func New(opts ...Option) (CronJober, error) {
	instance := &cronJobImpl{
		jobs: make(map[string]*JobInfo),
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

	jobWrapper := []cronlib.JobWrapper{cronlib.Recover(instance.logger)}
	if instance.skipIfJobRunning {
		jobWrapper = append(jobWrapper, cronlib.SkipIfStillRunning(instance.logger))
	}
	optList = append(optList, cronlib.WithLogger(instance.logger), cronlib.WithChain(jobWrapper...))

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

	j.mu.RLock()
	_, exists := j.jobs[jobId]
	j.mu.RUnlock()
	if exists {
		return ErrJobIdAlreadyExists
	}

	j.mu.Lock()
	defer j.mu.Unlock()

	if _, exists := j.jobs[jobId]; exists {
		return ErrJobIdAlreadyExists
	}

	wrappedFunc := func() { j.runWithRetry(jobId, spec, f) }
	entryId, err := j.cronClient.AddFunc(spec, wrappedFunc)
	if err != nil {
		return err
	}

	j.jobs[jobId] = &JobInfo{
		JobId:   jobId,
		Spec:    spec,
		entryId: entryId,
	}
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
		j.jobs[job.JobId] = &JobInfo{
			JobId:   job.JobId,
			Spec:    job.Spec,
			entryId: entryId,
		}
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

	j.jobs[jobId] = &JobInfo{
		JobId:   jobId,
		Spec:    spec,
		entryId: entryId,
	}

	return nil
}

// Get retrieves the cron spec for a given job ID
func (j *cronJobImpl) Get(jobId string) (JobInfo, bool) {
	j.mu.RLock()
	defer j.mu.RUnlock()

	info, ok := j.jobs[jobId]
	if !ok {
		return JobInfo{}, false
	}

	return *info, true
}

func (j *cronJobImpl) runWithRetry(jobId, spec string, f func() error) {
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

// Remove deletes one or more jobs with the given IDs. It removes each job from both the internal
// job map and the cron scheduler. If any of the provided job IDs are not found, it returns
// ErrJobNotFound and stops processing further jobs. If the jobIds slice is empty, it returns
// nil immediately without making any changes.
func (j *cronJobImpl) Remove(jobIds ...string) error {
	if len(jobIds) == 0 {
		return nil
	}

	j.mu.Lock()
	defer j.mu.Unlock()

	for i := range jobIds {
		if _, ok := j.jobs[jobIds[i]]; !ok {
			return fmt.Errorf("jobId %s %w", jobIds[i], ErrJobNotFound)
		}
	}
	for i := range jobIds {
		job, _ := j.jobs[jobIds[i]]
		j.cronClient.Remove(job.entryId)
		delete(j.jobs, jobIds[i])
	}

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

// Len returns the number of jobs currently scheduled in the cron scheduler.
func (j *cronJobImpl) Len() int {
	j.mu.RLock()
	defer j.mu.RUnlock()

	return len(j.jobs)
}

// Clear removes all jobs from the cron scheduler. It safely handles concurrent
// access by acquiring a lock before performing the cleanup.
func (j *cronJobImpl) Clear() {
	j.mu.Lock()
	defer j.mu.Unlock()

	for jobId := range j.jobs {
		j.cronClient.Remove(j.jobs[jobId].entryId)
		delete(j.jobs, jobId)
	}
}

// Start begins the cron scheduler
func (j *cronJobImpl) Start() {
	j.cronClient.Start()
}

// Stop gracefully shuts down the cron scheduler
func (j *cronJobImpl) Stop() {
	j.cronClient.Stop()
}
