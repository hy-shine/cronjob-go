package cronjob

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

var testFunc = func() error { return nil }

func TestNew(t *testing.T) {
	c, err := New()
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}
	if c == nil {
		t.Error("New() returned nil")
	}
}

func TestAdd(t *testing.T) {
	c, _ := New()

	tests := []struct {
		name    string
		jobId   string
		spec    string
		wantErr error
	}{
		{"valid job", "job1", "* * * * *", nil},
		{"empty jobId", "", "* * * * *", ErrJobIdEmpty},
		{"empty spec", "job2", "", ErrSpecEmpty},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := c.Add(tt.jobId, tt.spec, func() error { return nil })
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Add() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestAddBatch(t *testing.T) {
	c, _ := New()

	tests := []struct {
		name    string
		jobs    []BatchFunc
		wantErr error
	}{
		{
			"valid batch",
			[]BatchFunc{
				{"job1", "* * * * *", func() error { return nil }},
				{"job2", "* * * * *", func() error { return nil }},
			},
			nil,
		},
		{
			"duplicate jobId",
			[]BatchFunc{
				{"job1", "* * * * *", func() error { return nil }},
				{"job1", "* * * * *", func() error { return nil }},
			},
			ErrJobIdAlreadyExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := c.AddBatch(tt.jobs)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("AddBatch() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestUpsert(t *testing.T) {
	c, _ := New()

	// Add initial job
	c.Add("job1", "* * * * *", func() error { return nil })

	tests := []struct {
		name    string
		jobId   string
		spec    string
		wantErr error
	}{
		{"update existing", "job1", "*/5 * * * *", nil},
		{"insert new", "job2", "* * * * *", nil},
		{"empty jobId", "", "* * * * *", ErrJobIdEmpty},
		{"empty spec", "job3", "", ErrSpecEmpty},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := c.Upsert(tt.jobId, tt.spec, func() error { return nil })
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Upsert() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestGet(t *testing.T) {
	c, _ := New()
	c.Add("job1", "* * * * *", func() error { return nil })

	tests := []struct {
		name   string
		jobId  string
		want   string
		wantOk bool
	}{
		{"existing job", "job1", "* * * * *", true},
		{"non-existent job", "job2", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := c.Get(tt.jobId)
			if got != tt.want || ok != tt.wantOk {
				t.Errorf("Get() = (%v, %v), want (%v, %v)", got, ok, tt.want, tt.wantOk)
			}
		})
	}
}

func TestRemove(t *testing.T) {
	c, _ := New()
	c.Add("job1", "* * * * *", func() error { return nil })

	tests := []struct {
		name    string
		jobId   string
		wantErr error
	}{
		{"existing job", "job1", nil},
		{"non-existent job", "job2", ErrJobNotFound},
		{"empty jobId", "", ErrJobIdEmpty},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := c.Remove(tt.jobId)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Remove() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestJobs(t *testing.T) {
	c, _ := New()
	c.Add("job1", "* * * * *", func() error { return nil })
	c.Add("job2", "* * * * *", func() error { return nil })

	got := c.Jobs()
	if len(got) != 2 {
		t.Errorf("Jobs() returned %d jobs, want 2", len(got))
	}
}

func TestStartStop(t *testing.T) {
	c, _ := New(WithEnableSeconds())
	var wg sync.WaitGroup
	wg.Add(1)

	c.Add("job1", "*/3 * * * * *", func() error {
		wg.Done()
		return nil
	})

	c.Start()
	defer c.Stop()

	select {
	case <-wait(&wg):
	case <-time.After(5 * time.Second):
		t.Error("Job did not run within expected time")
	}
}

func TestConcurrentAccess(t *testing.T) {
	c, _ := New()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c.Add(fmt.Sprintf("job%d", i), "* * * * *", func() error { return nil })
		}(i)
	}

	wg.Wait()

	if len(c.Jobs()) != 100 {
		t.Errorf("Expected 100 jobs, got %d", len(c.Jobs()))
	}
}

func wait(wg *sync.WaitGroup) chan struct{} {
	ch := make(chan struct{})
	go func() {
		wg.Wait()
		close(ch)
	}()
	return ch
}
