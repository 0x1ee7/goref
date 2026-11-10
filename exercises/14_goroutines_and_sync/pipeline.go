package goroutines

import (
	"context"

	"golang.org/x/sync/errgroup"
)

// Source starts a goroutine in g that sends items on the returned channel in
// order and then closes it. It stops early, returning ctx.Err(), when ctx is
// done. The channel is unbuffered and is closed however the goroutine ends, so
// readers never hang.
func Source[T any](ctx context.Context, g *errgroup.Group, items []T) <-chan T {
	return nil
}

// Map starts workers goroutines in g that apply fn to every value received
// from in and send the results on the returned channel. Results are not in
// input order. workers < 1 is treated as 1. The output channel is closed once
// in is drained or the stage stops, after all workers have finished. A worker
// stops, returning ctx.Err(), when ctx is done; an fn error is returned from
// g.Wait and cancels ctx for every stage using it.
func Map[In, Out any](ctx context.Context, g *errgroup.Group, in <-chan In, workers int, fn func(context.Context, In) (Out, error)) <-chan Out {
	return nil
}

// Collect receives from in until it is closed or ctx is done and returns the
// values received, in order. It never blocks past ctx's cancellation.
func Collect[T any](ctx context.Context, in <-chan T) []T {
	return nil
}
