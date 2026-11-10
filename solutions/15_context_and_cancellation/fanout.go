package ctxflow

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

// FetchAll runs every source concurrently and returns the values in source
// order. The first failure cancels the remaining sources, and FetchAll returns
// only after all of them have returned. If the parent ctx ended, the error is
// its cause rather than the sources' cancellation errors.
func FetchAll[T any](ctx context.Context, sources []Source[T]) ([]T, error) {
	g, gctx := errgroup.WithContext(ctx)
	values := make([]T, len(sources))
	for i, src := range sources {
		g.Go(func() error {
			v, err := src.Fetch(gctx)
			if err != nil {
				return err
			}
			values[i] = v
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		if ctx.Err() != nil {
			return nil, context.Cause(ctx)
		}
		return nil, err
	}
	return values, nil
}

// Hedge runs attempt 0 immediately and starts attempt n+1 whenever delay
// passes without a result or attempt n fails. The first success wins: the
// losing attempts are cancelled and Hedge waits for them before returning.
// When every attempt fails, the errors are joined in attempt order.
func Hedge[T any](ctx context.Context, delay time.Duration, attempts int, fetch func(ctx context.Context, attempt int) (T, error)) (T, error) {
	var zero T
	if attempts < 1 || delay < 0 {
		return zero, fmt.Errorf("%w: attempts must be >= 1 and delay >= 0", ErrInvalidArgument)
	}

	type outcome struct {
		attempt int
		value   T
		err     error
	}

	ctx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	defer wg.Wait()
	defer cancel()

	// Buffered for every attempt so losers never block after Hedge returns.
	results := make(chan outcome, attempts)
	launch := func(attempt int) {
		wg.Go(func() {
			v, err := fetch(ctx, attempt)
			results <- outcome{attempt, v, err}
		})
	}

	hedge := time.NewTimer(delay)
	defer hedge.Stop()
	errs := make([]error, attempts)
	launch(0)
	next, failed := 1, 0
	for {
		select {
		case <-ctx.Done():
			return zero, context.Cause(ctx)
		case <-hedge.C:
			if next < attempts {
				launch(next)
				next++
				hedge.Reset(delay)
			}
		case o := <-results:
			if o.err == nil {
				return o.value, nil
			}
			errs[o.attempt] = o.err
			failed++
			if failed == attempts {
				return zero, errors.Join(errs...)
			}
			if next < attempts {
				launch(next)
				next++
				hedge.Reset(delay)
			}
		}
	}
}
