package support

import "errors"

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
func (e *TaskError) Error() string { return "" }

// Unwrap returns the task's error.
func (e *TaskError) Unwrap() error { return nil }

// BatchError aggregates the failures of a batch. Total is the number of tasks
// that were run. Failures are in task order.
type BatchError struct {
	Total    int
	Failures []*TaskError
}

// Error renders the failure count, for example "2 of 5 tasks failed".
func (e *BatchError) Error() string { return "" }

// Unwrap returns each failure, so errors.As and errors.Is reach every task.
func (e *BatchError) Unwrap() []error { return nil }

// RunBatch runs every task in order and returns the results keyed by name.
//
// It does not stop at the first failure. If any task fails, RunBatch returns
// a nil map and a *BatchError with every failure. Duplicate names are rejected
// before any task runs, with an error matching ErrDuplicateTask. A panic in a
// task propagates and stops the batch.
func RunBatch[T any](tasks []Task[T]) (map[string]T, error) { return nil, nil }

// Triage splits a batch failure by IsTransient. Each side keeps the original
// Total. A side with no failures is nil. A nil input gives nil, nil.
func Triage(b *BatchError) (transient, permanent *BatchError) { return nil, nil }
