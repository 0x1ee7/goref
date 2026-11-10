package ctxflow

import (
	"context"
	"time"
)

// FetchAll runs every source concurrently and returns their values in source
// order. It is all-or-nothing: the first error cancels the context handed to
// the other sources and is the error returned. FetchAll returns only after every
// source has returned, so no goroutine outlives the call. Empty input returns an
// empty, non-nil slice and a nil error.
func FetchAll[T any](ctx context.Context, sources []Source[T]) ([]T, error) {
	return nil, nil
}

// Hedge races attempts against each other to cut tail latency. Attempt 0 starts
// immediately. Every delay without an answer starts the next attempt, and a
// failed attempt starts the next one at once. Up to attempts calls are made in
// total; fetch receives the attempt index.
//
// The first success wins: the other attempts' contexts are cancelled and Hedge
// waits for them to return before it returns. If every attempt fails, the error
// is errors.Join of all attempt errors in attempt order. If ctx ends first, the
// result is context.Cause(ctx). attempts < 1 or delay < 0 wraps ErrInvalidArgument.
func Hedge[T any](ctx context.Context, delay time.Duration, attempts int, fetch func(ctx context.Context, attempt int) (T, error)) (T, error) {
	var zero T
	return zero, nil
}
