package goroutines_test

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync/atomic"
	"testing"

	"github.com/google/go-cmp/cmp"
	"goref/exercises/14_goroutines_and_sync"
)

func double(_ context.Context, n int) (int, error) { return n * 2, nil }

func TestPart6ParallelMapOrderedResults(t *testing.T) {
	items := make([]int, 100)
	want := make([]int, len(items))
	for i := range items {
		items[i] = i
		want[i] = 2 * i
	}
	for _, workers := range []int{1, 4, 7, 0} {
		t.Run(fmt.Sprintf("workers=%d", workers), func(t *testing.T) {
			var got []int
			var err error
			callWithin(t, "ParallelMap", func() {
				got, err = goroutines.ParallelMap(t.Context(), workers, items, double)
			})
			if err != nil {
				t.Fatalf("ParallelMap() error = %v", err)
			}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("ParallelMap() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPart6ParallelMapEmptyInput(t *testing.T) {
	var calls atomic.Int64
	got, err := goroutines.ParallelMap(t.Context(), 4, []int{}, func(_ context.Context, n int) (int, error) {
		calls.Add(1)
		return n, nil
	})
	if err != nil {
		t.Fatalf("ParallelMap(empty) error = %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Errorf("ParallelMap(empty) = %#v, want non-nil empty slice", got)
	}
	if n := calls.Load(); n != 0 {
		t.Errorf("fn called %d times for empty input, want 0", n)
	}
}

func TestPart6ParallelMapFirstErrorWins(t *testing.T) {
	items := make([]int, 50)
	for i := range items {
		items[i] = i
	}
	var got []int
	var err error
	callWithin(t, "ParallelMap", func() {
		got, err = goroutines.ParallelMap(t.Context(), 4, items, func(_ context.Context, n int) (int, error) {
			if n == 3 {
				return 0, errBoom
			}
			return n, nil
		})
	})
	if !errors.Is(err, errBoom) {
		t.Fatalf("ParallelMap() error = %v, want errBoom", err)
	}
	if got != nil {
		t.Errorf("ParallelMap() results = %v on error, want nil", got)
	}
}

func TestPart6ParallelMapCancelledBeforeStart(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	var calls atomic.Int64
	var got []int
	var err error
	callWithin(t, "ParallelMap", func() {
		got, err = goroutines.ParallelMap(ctx, 3, []int{1, 2, 3}, func(_ context.Context, n int) (int, error) {
			calls.Add(1)
			return n, nil
		})
	})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("ParallelMap() error = %v, want context.Canceled", err)
	}
	if got != nil {
		t.Errorf("ParallelMap() results = %v, want nil", got)
	}
	if n := calls.Load(); n != 0 {
		t.Errorf("fn called %d times after cancellation, want 0", n)
	}
}

func TestPart6ParallelMapCancelMidRunStopsTaking(t *testing.T) {
	// One worker: fn(0) cancels the context. Workers must not start item 1.
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var calls atomic.Int64
	var err error
	var got []int
	callWithin(t, "ParallelMap", func() {
		got, err = goroutines.ParallelMap(ctx, 1, []int{0, 1, 2, 3}, func(_ context.Context, n int) (int, error) {
			calls.Add(1)
			cancel()
			return n, nil
		})
	})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("ParallelMap() error = %v, want context.Canceled", err)
	}
	if got != nil {
		t.Errorf("ParallelMap() results = %v, want nil", got)
	}
	if n := calls.Load(); n != 1 {
		t.Errorf("fn called %d times, want 1 (no new items after cancellation)", n)
	}
}

func TestPart6ParallelMapBoundedWorkers(t *testing.T) {
	const workers = 4
	items := make([]int, 12)
	for i := range items {
		items[i] = i
	}
	// The gate opens only when all four workers are inside fn at once.
	gt := newGate(workers)
	var err error
	var got []int
	callWithin(t, "ParallelMap", func() {
		got, err = goroutines.ParallelMap(t.Context(), workers, items, func(ctx context.Context, n int) (int, error) {
			if err := gt.arrive(ctx); err != nil {
				return 0, err
			}
			return n, nil
		})
	})
	if err != nil {
		t.Fatalf("ParallelMap() error = %v", err)
	}
	if len(got) != len(items) {
		t.Errorf("ParallelMap() returned %d results, want %d", len(got), len(items))
	}
}

func TestPart6ParallelMapDefaultWorkers(t *testing.T) {
	// workers <= 0 means GOMAXPROCS. One item per proc must all be in flight at
	// once, so the gate opens only if the default really is GOMAXPROCS.
	procs := runtime.GOMAXPROCS(0)
	for _, workers := range []int{0, -1} {
		t.Run(fmt.Sprintf("workers=%d", workers), func(t *testing.T) {
			items := make([]int, procs)
			for i := range items {
				items[i] = i
			}
			gt := newGate(procs)
			var err error
			callWithin(t, "ParallelMap", func() {
				_, err = goroutines.ParallelMap(t.Context(), workers, items, func(ctx context.Context, n int) (int, error) {
					if err := gt.arrive(ctx); err != nil {
						return 0, err
					}
					return n, nil
				})
			})
			if err != nil {
				t.Fatalf("ParallelMap() error = %v", err)
			}
		})
	}
}

func TestPart6ParallelMapNoGoroutineLeak(t *testing.T) {
	baseline := runtime.NumGoroutine()
	items := make([]int, 64)
	for i := range items {
		items[i] = i
	}
	var (
		got []int
		err error
	)
	callWithin(t, "ParallelMap", func() {
		got, err = goroutines.ParallelMap(t.Context(), 8, items, double)
	})
	if err != nil || len(got) != len(items) {
		t.Fatalf("ParallelMap() = %d results, err %v; want %d results and nil error", len(got), err, len(items))
	}
	settleGoroutines(t, baseline)
}
