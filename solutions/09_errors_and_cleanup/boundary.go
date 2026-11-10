package support

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
)

// PanicError is what Guard returns when the guarded function panics. Value is
// the recovered value and Stack is the goroutine stack at the panic.
type PanicError struct {
	Value any
	Stack []byte
}

// Error renders the panic value.
func (e *PanicError) Error() string { return fmt.Sprintf("panic: %v", e.Value) }

// Unwrap returns Value when it is an error, so errors.Is reaches it. Otherwise
// it returns nil.
func (e *PanicError) Unwrap() error {
	err, _ := e.Value.(error)
	return err
}

// Guard calls fn and converts a panic into a *PanicError. It is meant for
// boundaries such as a request handler or a goroutine entry point. A plain
// error from fn is returned unchanged.
func Guard(fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			// debug.Stack is taken inside the deferred call, so it still
			// shows the frame that panicked.
			err = &PanicError{Value: r, Stack: debug.Stack()}
		}
	}()
	return fn()
}

// Must returns v. If err is non-nil, Must panics with err. Use it for wiring
// that cannot fail at runtime, such as a regular expression literal in a
// package variable.
func Must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

// RunJob runs job with ctx and returns its error.
//
// If job fails after ctx is done and context.Cause(ctx) is a cause that the
// job error does not already match, the result also matches the cause. It
// wraps both with two %w verbs. A job that succeeds returns nil. A panic in
// job propagates.
func RunJob(ctx context.Context, job func(context.Context) error) error {
	err := job(ctx)
	if err == nil || ctx.Err() == nil {
		return err
	}
	cause := context.Cause(ctx)
	if cause == nil || errors.Is(err, cause) {
		return err
	}
	return fmt.Errorf("run job: %w (cause: %w)", err, cause)
}
