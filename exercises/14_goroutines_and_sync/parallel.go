package goroutines

import (
	"context"
	"errors"
)

// ParallelMap applies fn to every item using workers goroutines and returns the
// results in input order. It is meant for CPU-bound work. workers <= 0 means
// runtime.GOMAXPROCS(0). Empty input returns an empty slice without calling fn.
//
// If fn returns an error, workers stop taking new items, the error is returned
// and the results are nil. If ctx is done before all items are processed, it
// returns (nil, ctx.Err()). ParallelMap returns only after every worker exits.
func ParallelMap[T, R any](ctx context.Context, workers int, items []T, fn func(context.Context, T) (R, error)) ([]R, error) {
	return nil, errors.New("not implemented")
}
