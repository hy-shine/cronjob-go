package job

import (
	"fmt"
	"sync"

	"github.com/hy-shine/cronjob-go/errors"
)

// InMemoryRepository implements Repository using an in-memory map.
type InMemoryRepository struct {
	mu   sync.RWMutex
	jobs map[string]*Info
}

// NewInMemoryRepository creates a new in-memory repository.
func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{
		jobs: make(map[string]*Info),
	}
}

// Add adds a new job, returns error if already exists.
func (r *InMemoryRepository) Add(info *Info) error {
	if info == nil {
		return errors.ErrJobInfoNil
	}
	if info.JobId == "" {
		return errors.ErrJobIdEmpty
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.jobs[info.JobId]; exists {
		return fmt.Errorf("jobId %s: %w", info.JobId, errors.ErrJobIdAlreadyExists)
	}

	r.jobs[info.JobId] = info
	return nil
}

// Get retrieves job info by ID.
func (r *InMemoryRepository) Get(jobId string) (*Info, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	info, ok := r.jobs[jobId]
	if !ok {
		return nil, false
	}
	return info, true
}

// Remove deletes a job by ID.
func (r *InMemoryRepository) Remove(jobId string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.jobs[jobId]; !ok {
		return fmt.Errorf("jobId %s: %w", jobId, errors.ErrJobNotFound)
	}

	delete(r.jobs, jobId)
	return nil
}

// List returns all job IDs.
func (r *InMemoryRepository) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := make([]string, 0, len(r.jobs))
	for id := range r.jobs {
		ids = append(ids, id)
	}
	return ids
}

// Len returns the count of jobs.
func (r *InMemoryRepository) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.jobs)
}

// Clear removes all jobs.
func (r *InMemoryRepository) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.jobs = make(map[string]*Info)
}

// Exists checks if a job ID exists.
func (r *InMemoryRepository) Exists(jobId string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.jobs[jobId]
	return ok
}
