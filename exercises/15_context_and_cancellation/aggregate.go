package ctxflow

import (
	"context"
	"errors"
	"time"
)

// ErrCallTimeout is the Err of a Result whose per-call budget expired while the
// source was still running. A source's own timeout error is NOT ErrCallTimeout.
var ErrCallTimeout = errors.New("per-call timeout")

// Source is a named upstream call. Fetch must honour the context it receives.
type Source[T any] struct {
	Name  string
	Fetch func(ctx context.Context) (T, error)
}

// Result is the outcome of one Source. Value is meaningful only when Err is nil.
type Result[T any] struct {
	Name  string
	Value T
	Err   error
}

// WithSource returns a context derived from ctx that carries the source name.
// Use an unexported key type; the key must not be a string.
func WithSource(ctx context.Context, name string) context.Context {
	return ctx
}

// SourceFrom returns the name stored by WithSource, if any.
func SourceFrom(ctx context.Context) (string, bool) {
	return "", false
}

// Aggregate calls every source concurrently, each under its own perCall budget,
// and returns one Result per source in the order of sources.
//
// Partial failure is data, not an error: a failing or slow source yields a
// Result with Err set while the others still succeed. The returned error is
// non-nil only for invalid input (ErrInvalidArgument, perCall <= 0) or when
// ctx itself is done (context.Cause(ctx)); in that case the results are still
// returned for every source. Aggregate waits for every source to return.
func Aggregate[T any](ctx context.Context, perCall time.Duration, sources []Source[T]) ([]Result[T], error) {
	return nil, nil
}
