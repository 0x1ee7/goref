package goroutines

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"
)

// Errors returned by Scheduler.
var (
	ErrInvalidInterval = errors.New("scheduler: interval must be > 0")
	ErrNilJob          = errors.New("scheduler: job must not be nil")
	ErrDuplicateJob    = errors.New("scheduler: duplicate job name")
	ErrStarted         = errors.New("scheduler: already started")
	ErrStopped         = errors.New("scheduler: stopped")
)

// Job is one unit of periodic work. It must return promptly once ctx is done.
type Job func(ctx context.Context) error

// JobError records a failed run of a named job.
type JobError struct {
	Name string
	Err  error
}

// Error returns "job <name>: <err>".
func (e JobError) Error() string {
	return fmt.Sprintf("job %s: %v", e.Name, e.Err)
}

// Unwrap returns the job's error.
func (e JobError) Unwrap() error {
	return e.Err
}

type scheduledJob struct {
	name     string
	interval time.Duration
	job      Job
}

type schedulerState int

const (
	stateIdle schedulerState = iota
	stateStarted
	stateStopped
)

// Scheduler runs named jobs periodically, one goroutine per job. The zero value
// is ready to use. It is safe for concurrent use. A Scheduler must not be copied
// after first use.
type Scheduler struct {
	mu     sync.Mutex
	state  schedulerState
	names  map[string]bool
	jobs   []scheduledJob
	cancel context.CancelFunc
	wg     sync.WaitGroup
	errs   []JobError
}

// Add registers a job that runs every interval. It returns ErrInvalidInterval
// when interval <= 0, ErrNilJob, ErrDuplicateJob for a name already registered,
// and ErrStarted once Start has been called.
func (s *Scheduler) Add(name string, interval time.Duration, job Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != stateIdle {
		return ErrStarted
	}
	if interval <= 0 {
		return ErrInvalidInterval
	}
	if job == nil {
		return ErrNilJob
	}
	if s.names[name] {
		return ErrDuplicateJob
	}
	if s.names == nil {
		s.names = make(map[string]bool)
	}
	s.names[name] = true
	s.jobs = append(s.jobs, scheduledJob{name: name, interval: interval, job: job})
	return nil
}

// Start launches one goroutine per job. Each job runs immediately, then waits
// interval after each run finishes, then runs again. Job contexts derive from
// ctx. It returns ErrStarted if already started and ErrStopped after Stop.
func (s *Scheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch s.state {
	case stateStarted:
		return ErrStarted
	case stateStopped:
		return ErrStopped
	}
	ctx, s.cancel = context.WithCancel(ctx)
	s.state = stateStarted
	for _, j := range s.jobs {
		s.wg.Go(func() {
			s.run(ctx, j)
		})
	}
	return nil
}

// run executes j until ctx is done.
func (s *Scheduler) run(ctx context.Context, j scheduledJob) {
	for ctx.Err() == nil {
		if err := j.job(ctx); err != nil && !(errors.Is(err, context.Canceled) && ctx.Err() != nil) {
			s.mu.Lock()
			s.errs = append(s.errs, JobError{Name: j.name, Err: err})
			s.mu.Unlock()
		}
		timer := time.NewTimer(j.interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

// Stop cancels the job contexts, which ends every sleep, and waits for every job
// goroutine to exit. Once Stop returns nil no job runs again. It returns ctx.Err()
// if ctx ends before the jobs have exited; jobs keep being waited for by later
// calls to Stop. Stop before Start, and repeated calls, are no-ops.
func (s *Scheduler) Stop(ctx context.Context) error {
	s.mu.Lock()
	switch s.state {
	case stateIdle:
		s.mu.Unlock()
		return nil
	case stateStarted:
		s.cancel()
		s.state = stateStopped
	}
	s.mu.Unlock()

	exited := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(exited)
	}()
	select {
	case <-exited:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Errors returns a copy of the failures recorded so far, in the order they
// happened. A run that returns an error wrapping context.Canceled because the
// scheduler is stopping is not recorded.
func (s *Scheduler) Errors() []JobError {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.errs)
}
