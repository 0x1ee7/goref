package goroutines_test

import (
	"context"
	"errors"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/sync/errgroup"
	"goref/exercises/14_goroutines_and_sync"
)

func TestPart5QueueInvalidCapacity(t *testing.T) {
	_, err := goroutines.NewBoundedQueue[int](0)
	if !errors.Is(err, goroutines.ErrInvalidCapacity) {
		t.Errorf("NewBoundedQueue(0) error = %v, want ErrInvalidCapacity", err)
	}
}

func TestPart5QueueFIFO(t *testing.T) {
	q, err := goroutines.NewBoundedQueue[int](3)
	if err != nil {
		t.Fatalf("NewBoundedQueue() error = %v", err)
	}
	for _, v := range []int{1, 2, 3} {
		if err := q.Put(v); err != nil {
			t.Fatalf("Put(%d) error = %v", v, err)
		}
	}
	if q.Len() != 3 {
		t.Errorf("Len() = %d, want 3", q.Len())
	}
	var got []int
	for range 3 {
		v, ok := q.Get()
		if !ok {
			t.Fatalf("Get() ok = false with items queued")
		}
		got = append(got, v)
	}
	if diff := cmp.Diff([]int{1, 2, 3}, got); diff != "" {
		t.Errorf("Get() order mismatch (-want +got):\n%s", diff)
	}
	if q.Len() != 0 {
		t.Errorf("Len() = %d after draining, want 0", q.Len())
	}
}

func TestPart5QueuePutBlocksWhenFull(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		q, err := goroutines.NewBoundedQueue[int](2)
		if err != nil {
			t.Fatalf("NewBoundedQueue() error = %v", err)
		}
		var puts atomic.Int64
		producerErr := make(chan error, 1)
		go func() {
			for i := 1; i <= 3; i++ {
				if err := q.Put(i); err != nil {
					producerErr <- err
					return
				}
				puts.Add(1)
			}
			producerErr <- nil
		}()

		// With capacity 2 the third Put must be durably blocked.
		synctest.Wait()
		if n := puts.Load(); n != 2 {
			t.Fatalf("completed Puts = %d before any Get, want 2 (queue full)", n)
		}
		if v, ok := q.Get(); !ok || v != 1 {
			t.Fatalf("Get() = %d, %v; want 1, true", v, ok)
		}
		synctest.Wait()
		if n := puts.Load(); n != 3 {
			t.Errorf("completed Puts = %d after one Get, want 3", n)
		}
		if err := recvOrFail(t, producerErr, "producer"); err != nil {
			t.Errorf("producer Put error = %v", err)
		}
	})
}

func TestPart5QueueCloseWakesBlockedGet(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		q, err := goroutines.NewBoundedQueue[int](1)
		if err != nil {
			t.Fatalf("NewBoundedQueue() error = %v", err)
		}
		type res struct {
			v  int
			ok bool
		}
		out := make(chan res, 1)
		go func() {
			v, ok := q.Get()
			out <- res{v, ok}
		}()
		synctest.Wait()
		q.Close()
		r := recvOrFail(t, out, "blocked Get after Close")
		if r.ok {
			t.Errorf("Get() after Close on empty queue = %d, true; want zero, false", r.v)
		}
	})
}

func TestPart5QueueCloseWakesBlockedPut(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		q, err := goroutines.NewBoundedQueue[int](1)
		if err != nil {
			t.Fatalf("NewBoundedQueue() error = %v", err)
		}
		if err := q.Put(1); err != nil {
			t.Fatalf("Put(1) error = %v", err)
		}
		putErr := make(chan error, 1)
		go func() {
			putErr <- q.Put(2)
		}()
		synctest.Wait()
		q.Close()
		if err := recvOrFail(t, putErr, "blocked Put after Close"); !errors.Is(err, goroutines.ErrQueueClosed) {
			t.Errorf("blocked Put() error = %v, want ErrQueueClosed", err)
		}
	})
}

func TestPart5QueueCloseDrainsThenStops(t *testing.T) {
	q, err := goroutines.NewBoundedQueue[string](4)
	if err != nil {
		t.Fatalf("NewBoundedQueue() error = %v", err)
	}
	for _, s := range []string{"a", "b"} {
		if err := q.Put(s); err != nil {
			t.Fatalf("Put(%s) error = %v", s, err)
		}
	}
	q.Close()
	q.Close() // idempotent

	if err := q.Put("c"); !errors.Is(err, goroutines.ErrQueueClosed) {
		t.Errorf("Put() after Close error = %v, want ErrQueueClosed", err)
	}
	for _, want := range []string{"a", "b"} {
		v, ok := q.Get()
		if !ok || v != want {
			t.Errorf("Get() = %q, %v; want %q, true (queued items survive Close)", v, ok, want)
		}
	}
	if v, ok := q.Get(); ok || v != "" {
		t.Errorf("Get() on drained closed queue = %q, %v; want zero, false", v, ok)
	}
}

func TestPart5QueueConcurrentProducersConsumers(t *testing.T) {
	q, err := goroutines.NewBoundedQueue[int](8)
	if err != nil {
		t.Fatalf("NewBoundedQueue() error = %v", err)
	}
	const producers, perProducer, consumers = 4, 250, 3
	var producersWG sync.WaitGroup
	for p := range producers {
		producersWG.Go(func() {
			for i := range perProducer {
				if err := q.Put(p*perProducer + i); err != nil {
					t.Errorf("Put() error = %v", err)
					return
				}
			}
		})
	}
	var consumersWG sync.WaitGroup
	sums := make([]int, consumers)
	counts := make([]int, consumers)
	for c := range consumers {
		consumersWG.Go(func() {
			for {
				v, ok := q.Get()
				if !ok {
					return
				}
				sums[c] += v
				counts[c]++
			}
		})
	}
	callWithin(t, "producers", producersWG.Wait)
	q.Close()
	callWithin(t, "consumers", consumersWG.Wait)

	total, n := 0, 0
	for c := range consumers {
		total += sums[c]
		n += counts[c]
	}
	const items = producers * perProducer
	if n != items || total != items*(items-1)/2 {
		t.Errorf("consumed %d items summing to %d; want %d items summing to %d", n, total, items, items*(items-1)/2)
	}
}

func TestPart5PipelineMapCollect(t *testing.T) {
	items := make([]int, 20)
	want := make([]int, 20)
	for i := range items {
		items[i] = i + 1
		want[i] = (i + 1) * (i + 1)
	}
	g, ctx := errgroup.WithContext(t.Context())
	src := goroutines.Source(ctx, g, items)
	squares := goroutines.Map(ctx, g, src, 4, func(_ context.Context, n int) (int, error) {
		return n * n, nil
	})
	var got []int
	callWithin(t, "pipeline", func() {
		got = goroutines.Collect(ctx, squares)
	})
	if err := g.Wait(); err != nil {
		t.Fatalf("g.Wait() error = %v", err)
	}
	slices.Sort(got) // Map is unordered
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("pipeline output mismatch (-want +got):\n%s", diff)
	}
}

func TestPart5PipelineWorkersBelowOneMeansOne(t *testing.T) {
	g, ctx := errgroup.WithContext(t.Context())
	src := goroutines.Source(ctx, g, []int{1, 2, 3, 4, 5})
	out := goroutines.Map(ctx, g, src, 0, func(_ context.Context, n int) (int, error) {
		return -n, nil
	})
	var got []int
	callWithin(t, "pipeline", func() {
		got = goroutines.Collect(ctx, out)
	})
	if err := g.Wait(); err != nil {
		t.Fatalf("g.Wait() error = %v", err)
	}
	slices.Sort(got)
	if diff := cmp.Diff([]int{-5, -4, -3, -2, -1}, got); diff != "" {
		t.Errorf("Map(workers=0) output mismatch (-want +got):\n%s", diff)
	}
}

func TestPart5PipelineWorkersRunInParallel(t *testing.T) {
	// Three workers must all be inside fn at once for the gate to open.
	g, ctx := errgroup.WithContext(t.Context())
	items := make([]int, 9)
	for i := range items {
		items[i] = i
	}
	gt := newGate(3)
	src := goroutines.Source(ctx, g, items)
	out := goroutines.Map(ctx, g, src, 3, func(ctx context.Context, n int) (int, error) {
		if err := gt.arrive(ctx); err != nil {
			return 0, err
		}
		return n, nil
	})
	var got []int
	callWithin(t, "pipeline", func() {
		got = goroutines.Collect(ctx, out)
	})
	if err := g.Wait(); err != nil {
		t.Fatalf("g.Wait() error = %v", err)
	}
	if len(got) != len(items) {
		t.Errorf("Collect() returned %d values, want %d", len(got), len(items))
	}
}

func TestPart5PipelineFnErrorStopsEarly(t *testing.T) {
	const total = 1000
	items := make([]int, total)
	for i := range items {
		items[i] = i + 1
	}
	g, ctx := errgroup.WithContext(t.Context())
	src := goroutines.Source(ctx, g, items)
	out := goroutines.Map(ctx, g, src, 2, func(_ context.Context, n int) (int, error) {
		if n == 5 {
			return 0, errBoom
		}
		return n, nil
	})
	var got []int
	callWithin(t, "pipeline", func() {
		got = goroutines.Collect(ctx, out)
	})
	if err := g.Wait(); !errors.Is(err, errBoom) {
		t.Fatalf("g.Wait() error = %v, want errBoom", err)
	}
	if len(got) >= total {
		t.Errorf("Collect() returned all %d items; error should stop the pipeline early", len(got))
	}
	if slices.Contains(got, 5) {
		t.Errorf("Collect() contains 5, whose fn call failed")
	}
}

func TestPart5CollectStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	in := make(chan int) // never closed: only cancellation can end Collect
	go func() {
		in <- 1
		in <- 2
		in <- 3
		cancel()
	}()
	var got []int
	callWithin(t, "Collect after cancel", func() {
		got = goroutines.Collect(ctx, in)
	})
	if diff := cmp.Diff([]int{1, 2, 3}, got); diff != "" {
		t.Errorf("Collect() mismatch (-want +got):\n%s", diff)
	}
}

func TestPart5SourceStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	g, gctx := errgroup.WithContext(ctx)
	items := make([]int, 100)
	for i := range items {
		items[i] = i
	}
	out := goroutines.Source(gctx, g, items)
	recvOrFail(t, out, "first item")
	recvOrFail(t, out, "second item")
	cancel()
	var err error
	callWithin(t, "g.Wait after cancel", func() {
		err = g.Wait()
	})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("g.Wait() error = %v, want context.Canceled", err)
	}
	// The channel must still be closed so that readers never hang.
	callWithin(t, "drain closed source", func() {
		for range out {
		}
	})
}

func TestPart5PipelineNoGoroutineLeak(t *testing.T) {
	baseline := runtime.NumGoroutine()
	items := make([]int, 50)
	for i := range items {
		items[i] = i
	}
	g, ctx := errgroup.WithContext(t.Context())
	src := goroutines.Source(ctx, g, items)
	out := goroutines.Map(ctx, g, src, 5, func(_ context.Context, n int) (int, error) {
		return n, nil
	})
	var got []int
	callWithin(t, "pipeline", func() {
		got = goroutines.Collect(ctx, out)
		_ = g.Wait()
	})
	if len(got) != len(items) {
		t.Fatalf("pipeline delivered %d items, want %d", len(got), len(items))
	}
	settleGoroutines(t, baseline)
}
