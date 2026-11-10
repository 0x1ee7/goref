package ctxflow

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ErrCallTimeout is the per-result error for a source that exceeded its own
// budget. A source's own errors (including its deadline errors) are reported
// unchanged.
var ErrCallTimeout = errors.New("ctxflow: source call timed out")

// Source is a named upstream call.
type Source[T any] struct {
	Name  string
	Fetch func(ctx context.Context) (T, error)
}

// Result is the outcome of one source in an Aggregate call.
type Result[T any] struct {
	Name  string
	Value T
	Err   error
}

type sourceNameKey struct{}

// WithSource returns a context that carries the source name.
func WithSource(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, sourceNameKey{}, name)
}

// SourceFrom returns the source name carried by ctx, if any.
func SourceFrom(ctx context.Context) (string, bool) {
	name, ok := ctx.Value(sourceNameKey{}).(string)
	return name, ok
}

// Aggregate calls every source concurrently, each under its own perCall
// budget, and returns one Result per source in input order. A failing source
// does not fail the call. If ctx ends, the sources are cancelled, the partial
// results are still returned, and the error is context.Cause(ctx).
func Aggregate[T any](ctx context.Context, perCall time.Duration, sources []Source[T]) ([]Result[T], error) {
	if perCall <= 0 {
		return nil, fmt.Errorf("%w: per-call budget must be positive, got %v", ErrInvalidArgument, perCall)
	}
	results := make([]Result[T], len(sources))
	var wg sync.WaitGroup
	for i, src := range sources {
		wg.Go(func() {
			results[i] = callSource(ctx, perCall, src)
		})
	}
	wg.Wait()
	if ctx.Err() != nil {
		return results, context.Cause(ctx)
	}
	return results, nil
}

// callSource runs one source under its budget. The budget's cause is
// ErrCallTimeout, so an error seen after the budget expired is attributed to
// the budget rather than to the source.
func callSource[T any](parent context.Context, perCall time.Duration, src Source[T]) Result[T] {
	ctx, cancel := context.WithTimeoutCause(parent, perCall, ErrCallTimeout)
	defer cancel()
	v, err := src.Fetch(WithSource(ctx, src.Name))
	if err != nil && context.Cause(ctx) == ErrCallTimeout {
		err = ErrCallTimeout
	}
	return Result[T]{Name: src.Name, Value: v, Err: err}
}
