package goroutines

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
)

// ParallelMap applies fn to every item using workers goroutines and returns the
// results in input order. It is meant for CPU-bound work. workers <= 0 means
// runtime.GOMAXPROCS(0). Empty input returns an empty slice without calling fn.
//
// If fn returns an error, workers stop taking new items, the error is returned
// and the results are nil. If ctx is done before all items are processed, it
// returns (nil, ctx.Err()). ParallelMap returns only after every worker exits.
func ParallelMap[T, R any](ctx context.Context, workers int, items []T, fn func(context.Context, T) (R, error)) ([]R, error) {
	if len(items) == 0 {
		return []R{}, nil
	}
	if workers <= 0 {
		workers = runtime.GOMAXPROCS(0)
	}
	workers = min(workers, len(items))

	// Derived context lets the first fn error stop sibling workers.
	parent := ctx
	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	results := make([]R, len(items))
	var (
		next      atomic.Int64
		completed atomic.Int64
		errOnce   sync.Once
		firstErr  error
		wg        sync.WaitGroup
	)
	for range workers {
		wg.Go(func() {
			for {
				if ctx.Err() != nil {
					return
				}
				i := int(next.Add(1) - 1)
				if i >= len(items) {
					return
				}
				r, err := fn(ctx, items[i])
				if err != nil {
					errOnce.Do(func() {
						firstErr = err
						cancel()
					})
					return
				}
				results[i] = r
				completed.Add(1)
			}
		})
	}
	wg.Wait()

	// After Wait, firstErr and results are safe to read.
	if firstErr != nil {
		return nil, firstErr
	}
	if int(completed.Load()) < len(items) {
		return nil, parent.Err()
	}
	return results, nil
}
