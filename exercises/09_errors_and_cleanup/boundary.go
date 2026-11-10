package support

import "context"

// PanicError is what Guard returns when the guarded function panics. Value is
// the recovered value and Stack is the goroutine stack at the panic.
type PanicError struct {
	Value any
	Stack []byte
}

// Error renders the panic value.
func (e *PanicError) Error() string { return "" }

// Unwrap returns Value when it is an error, so errors.Is reaches it. Otherwise
// it returns nil.
func (e *PanicError) Unwrap() error { return nil }

// Guard calls fn and converts a panic into a *PanicError. It is meant for
// boundaries such as a request handler or a goroutine entry point. A plain
// error from fn is returned unchanged.
func Guard(fn func() error) error { return nil }

// Must returns v. If err is non-nil, Must panics with err. Use it for wiring
// that cannot fail at runtime, such as a regular expression literal in a
// package variable.
func Must[T any](v T, err error) T {
	var zero T
	return zero
}

// RunJob runs job with ctx and returns its error.
//
// If job fails after ctx is done and context.Cause(ctx) is a cause that the
// job error does not already match, the result also matches the cause. It
// wraps both with two %w verbs. A job that succeeds returns nil. A panic in
// job propagates.
func RunJob(ctx context.Context, job func(context.Context) error) error { return nil }
