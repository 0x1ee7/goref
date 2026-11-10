package ctxflow_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"goref/exercises/15_context_and_cancellation"
)

func newWorkerPool(t *testing.T, workers, queue int, handle func(context.Context, int) error) *ctxflow.WorkerPool[int] {
	t.Helper()
	p, err := ctxflow.NewWorkerPool(workers, queue, handle)
	if err != nil {
		t.Fatalf("NewWorkerPool(%d, %d) error = %v", workers, queue, err)
	}
	if p == nil {
		t.Fatal("NewWorkerPool returned a nil pool")
	}
	return p
}

// peakTracker records the maximum number of handlers running at once.
type peakTracker struct {
	running, peak atomic.Int32
}

func (pt *peakTracker) enter() {
	n := pt.running.Add(1)
	for {
		p := pt.peak.Load()
		if n <= p || pt.peak.CompareAndSwap(p, n) {
			return
		}
	}
}

func (pt *peakTracker) leave() { pt.running.Add(-1) }

func TestPart5WorkerPool(t *testing.T) {
	t.Run("shutdown drains every queued job when ctx allows", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			var pt peakTracker
			p := newWorkerPool(t, 2, 4, func(context.Context, int) error {
				pt.enter()
				defer pt.leave()
				time.Sleep(10 * time.Millisecond)
				return nil
			})
			for i := range 10 {
				if err := p.Submit(context.Background(), i); err != nil {
					t.Fatalf("Submit(%d) error = %v", i, err)
				}
			}
			abandoned, err := p.Shutdown(context.Background())
			if abandoned != 0 || err != nil {
				t.Errorf("Shutdown = (%d, %v), want (0, nil)", abandoned, err)
			}
			if n := p.Succeeded(); n != 10 {
				t.Errorf("Succeeded = %d, want 10", n)
			}
			if peak := pt.peak.Load(); peak > 2 {
				t.Errorf("peak concurrency = %d, want <= 2 workers", peak)
			}
		})
	})

	t.Run("workers run concurrently up to the worker count", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			var pt peakTracker
			p := newWorkerPool(t, 3, 10, func(context.Context, int) error {
				pt.enter()
				defer pt.leave()
				time.Sleep(5 * time.Millisecond)
				return nil
			})
			for i := range 12 {
				if err := p.Submit(context.Background(), i); err != nil {
					t.Fatalf("Submit(%d) error = %v", i, err)
				}
			}
			if _, err := p.Shutdown(context.Background()); err != nil {
				t.Fatalf("Shutdown error = %v", err)
			}
			if peak := pt.peak.Load(); peak != 3 {
				t.Errorf("peak concurrency = %d, want exactly 3", peak)
			}
		})
	})

	t.Run("handler errors are counted and the worker keeps going", func(t *testing.T) {
		p := newWorkerPool(t, 1, 10, func(_ context.Context, job int) error {
			if job%2 == 0 {
				return errBoom
			}
			return nil
		})
		for i := range 6 {
			if err := p.Submit(t.Context(), i); err != nil {
				t.Fatalf("Submit(%d) error = %v", i, err)
			}
		}
		if _, err := p.Shutdown(t.Context()); err != nil {
			t.Fatalf("Shutdown error = %v", err)
		}
		if s, f := p.Succeeded(), p.Failed(); s != 3 || f != 3 {
			t.Errorf("Succeeded, Failed = %d, %d; want 3, 3", s, f)
		}
	})

	t.Run("submit after shutdown is rejected with ErrClosed", func(t *testing.T) {
		p := newWorkerPool(t, 1, 1, func(context.Context, int) error { return nil })
		if _, err := p.Shutdown(t.Context()); err != nil {
			t.Fatalf("Shutdown error = %v", err)
		}
		if err := p.Submit(t.Context(), 1); !errors.Is(err, ctxflow.ErrClosed) {
			t.Errorf("Submit after Shutdown error = %v, want ErrClosed", err)
		}
	})

	t.Run("a second shutdown returns ErrClosed", func(t *testing.T) {
		p := newWorkerPool(t, 1, 1, func(context.Context, int) error { return nil })
		if _, err := p.Shutdown(t.Context()); err != nil {
			t.Fatalf("first Shutdown error = %v", err)
		}
		if _, err := p.Shutdown(t.Context()); !errors.Is(err, ctxflow.ErrClosed) {
			t.Errorf("second Shutdown error = %v, want ErrClosed", err)
		}
	})

	t.Run("submit gives up on a full queue when its context ends", func(t *testing.T) {
		started := make(chan struct{}, 1)
		gate := make(chan struct{})
		p := newWorkerPool(t, 1, 1, func(c context.Context, _ int) error {
			started <- struct{}{}
			select {
			case <-gate:
				return nil
			case <-c.Done():
				return c.Err()
			}
		})
		if err := p.Submit(t.Context(), 1); err != nil {
			t.Fatalf("Submit(1) error = %v", err)
		}
		if _, ok := recv(t, (<-chan struct{})(started)); !ok {
			return
		}
		if err := p.Submit(t.Context(), 2); err != nil {
			t.Fatalf("Submit(2) error = %v (queue has room)", err)
		}
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
		defer cancel()
		if err := p.Submit(ctx, 3); !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("Submit(3) on a full queue error = %v, want DeadlineExceeded", err)
		}
		close(gate)
		if _, err := p.Shutdown(t.Context()); err != nil {
			t.Errorf("Shutdown error = %v, want nil", err)
		}
		if n := p.Succeeded(); n != 2 {
			t.Errorf("Succeeded = %d, want 2 (jobs 1 and 2; job 3 never enqueued)", n)
		}
	})

	t.Run("the submitter's context does not cancel the handler", func(t *testing.T) {
		started := make(chan struct{}, 1)
		gate := make(chan struct{})
		var handlerErr atomic.Pointer[error]
		p := newWorkerPool(t, 1, 1, func(c context.Context, _ int) error {
			started <- struct{}{}
			<-gate
			err := c.Err()
			handlerErr.Store(&err)
			return err
		})
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		if err := p.Submit(ctx, 1); err != nil {
			t.Fatalf("Submit error = %v", err)
		}
		if _, ok := recv(t, (<-chan struct{})(started)); !ok {
			return
		}
		cancel()
		close(gate)
		if _, err := p.Shutdown(t.Context()); err != nil {
			t.Fatalf("Shutdown error = %v", err)
		}
		if e := handlerErr.Load(); e == nil || *e != nil {
			t.Errorf("handler saw ctx.Err() = %v, want nil", e)
		}
		if n := p.Succeeded(); n != 1 {
			t.Errorf("Succeeded = %d, want 1", n)
		}
	})

	t.Run("shutdown past its deadline abandons queued jobs and waits for in-flight ones", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			var handled atomic.Int32
			started := make(chan struct{}, 1)
			p := newWorkerPool(t, 1, 8, func(c context.Context, job int) error {
				handled.Add(1)
				if job == 0 {
					started <- struct{}{}
					<-c.Done()
					return c.Err()
				}
				return nil
			})
			for i := range 4 {
				if err := p.Submit(context.Background(), i); err != nil {
					t.Fatalf("Submit(%d) error = %v", i, err)
				}
			}
			if _, ok := recv(t, (<-chan struct{})(started)); !ok {
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer cancel()
			abandoned, err := p.Shutdown(ctx)
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("Shutdown error = %v, want DeadlineExceeded", err)
			}
			if abandoned != 3 {
				t.Errorf("abandoned = %d, want 3 (the queued jobs)", abandoned)
			}
			if n := handled.Load(); n != 1 {
				t.Errorf("handler ran %d times, want 1 (queued jobs must not start)", n)
			}
			if f := p.Failed(); f != 1 {
				t.Errorf("Failed = %d, want 1 (the cancelled in-flight job)", f)
			}
		})
	})

	t.Run("invalid configuration is rejected", func(t *testing.T) {
		h := func(context.Context, int) error { return nil }
		if _, err := ctxflow.NewWorkerPool(0, 1, h); !errors.Is(err, ctxflow.ErrInvalidArgument) {
			t.Errorf("workers=0 error = %v, want ErrInvalidArgument", err)
		}
		if _, err := ctxflow.NewWorkerPool(1, -1, h); !errors.Is(err, ctxflow.ErrInvalidArgument) {
			t.Errorf("queue=-1 error = %v, want ErrInvalidArgument", err)
		}
	})
}
