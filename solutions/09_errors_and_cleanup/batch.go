package support

import (
	"errors"
	"fmt"
)

// ErrDuplicateTask is returned by RunBatch when two tasks share a name.
var ErrDuplicateTask = errors.New("duplicate task name")

// Task is a named unit of work that produces a T.
type Task[T any] struct {
	Name string
	Run  func() (T, error)
}

// TaskError is the failure of one named task. Err is reachable through Unwrap.
type TaskError struct {
	Name string
	Err  error
}

// Error renders the task name and cause.
func (e *TaskError) Error() string { return fmt.Sprintf("task %q: %v", e.Name, e.Err) }

// Unwrap returns the task's error.
func (e *TaskError) Unwrap() error { return e.Err }

// BatchError aggregates the failures of a batch. Total is the number of tasks
// that were run. Failures are in task order.
type BatchError struct {
	Total    int
	Failures []*TaskError
}

// Error renders the failure count, for example "2 of 5 tasks failed".
func (e *BatchError) Error() string {
	return fmt.Sprintf("%d of %d tasks failed", len(e.Failures), e.Total)
}

// Unwrap returns each failure, so errors.As and errors.Is reach every task.
func (e *BatchError) Unwrap() []error {
	errs := make([]error, len(e.Failures))
	for i, f := range e.Failures {
		errs[i] = f
	}
	return errs
}

// RunBatch runs every task in order and returns the results keyed by name.
//
// It does not stop at the first failure. If any task fails, RunBatch returns
// a nil map and a *BatchError with every failure. Duplicate names are rejected
// before any task runs, with an error matching ErrDuplicateTask. A panic in a
// task propagates and stops the batch.
func RunBatch[T any](tasks []Task[T]) (map[string]T, error) {
	seen := make(map[string]struct{}, len(tasks))
	for _, t := range tasks {
		if _, dup := seen[t.Name]; dup {
			return nil, fmt.Errorf("task %q: %w", t.Name, ErrDuplicateTask)
		}
		seen[t.Name] = struct{}{}
	}

	results := make(map[string]T, len(tasks))
	var failures []*TaskError
	for _, t := range tasks {
		v, err := t.Run()
		if err != nil {
			failures = append(failures, &TaskError{Name: t.Name, Err: err})
			continue
		}
		results[t.Name] = v
	}
	if len(failures) > 0 {
		return nil, &BatchError{Total: len(tasks), Failures: failures}
	}
	return results, nil
}

// Triage splits a batch failure by IsTransient. Each side keeps the original
// Total. A side with no failures is nil. A nil input gives nil, nil.
func Triage(b *BatchError) (transient, permanent *BatchError) {
	if b == nil {
		return nil, nil
	}
	for _, f := range b.Failures {
		if IsTransient(f) {
			if transient == nil {
				transient = &BatchError{Total: b.Total}
			}
			transient.Failures = append(transient.Failures, f)
		} else {
			if permanent == nil {
				permanent = &BatchError{Total: b.Total}
			}
			permanent.Failures = append(permanent.Failures, f)
		}
	}
	return transient, permanent
}
