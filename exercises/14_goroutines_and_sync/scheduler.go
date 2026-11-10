package goroutines

import (
	"context"
	"errors"
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
	return ""
}

// Unwrap returns the job's error.
func (e JobError) Unwrap() error {
	return e.Err
}

// Scheduler runs named jobs periodically, one goroutine per job. The zero value
// is ready to use. It is safe for concurrent use. A Scheduler must not be copied
// after first use.
type Scheduler struct {
	// engineer-owned fields
}

// Add registers a job that runs every interval. It returns ErrInvalidInterval
// when interval <= 0, ErrNilJob, ErrDuplicateJob for a name already registered,
// and ErrStarted once Start has been called.
func (s *Scheduler) Add(name string, interval time.Duration, job Job) error {
	return errors.New("not implemented")
}

// Start launches one goroutine per job. Each job runs immediately, then waits
// interval after each run finishes, then runs again. Job contexts derive from
// ctx. It returns ErrStarted if already started and ErrStopped after Stop.
func (s *Scheduler) Start(ctx context.Context) error {
	return errors.New("not implemented")
}

// Stop cancels the job contexts, which ends every sleep, and waits for every job
// goroutine to exit. Once Stop returns nil no job runs again. It returns ctx.Err()
// if ctx ends before the jobs have exited; jobs keep being waited for by later
// calls to Stop. Stop before Start, and repeated calls, are no-ops.
func (s *Scheduler) Stop(ctx context.Context) error {
	return errors.New("not implemented")
}

// Errors returns a copy of the failures recorded so far, in the order they
// happened. A run that returns an error wrapping context.Canceled because the
// scheduler is stopping is not recorded.
func (s *Scheduler) Errors() []JobError {
	return nil
}
