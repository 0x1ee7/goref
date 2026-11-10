package ctxflow_test

import (
	"context"
	"errors"
	"io"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"goref/exercises/15_context_and_cancellation"
)

// blockUntilDone is a run function for components that live until stopped.
func blockUntilDone(ctx context.Context) error {
	<-ctx.Done()
	return context.Cause(ctx)
}

func TestPart3Component(t *testing.T) {
	t.Run("Wait returns the run result and Cause is nil", func(t *testing.T) {
		errRun := errors.New("run failed")
		c := ctxflow.Start(t.Context(), func(context.Context) error { return errRun })
		if c == nil {
			t.Fatal("Start returned nil")
		}
		if err, ok := callWithin(t, c.Wait); !ok || !errors.Is(err, errRun) {
			t.Errorf("Wait = %v, want errRun", err)
		}
		if cause := c.Cause(); cause != nil {
			t.Errorf("Cause after clean return = %v, want nil", cause)
		}
	})

	t.Run("Stop cancels run with the given cause", func(t *testing.T) {
		errMaintenance := errors.New("maintenance window")
		c := ctxflow.Start(t.Context(), blockUntilDone)
		if c == nil {
			t.Fatal("Start returned nil")
		}
		c.Stop(errMaintenance)
		if err, ok := callWithin(t, c.Wait); !ok || !errors.Is(err, errMaintenance) {
			t.Errorf("Wait = %v, want errMaintenance", err)
		}
		if cause := c.Cause(); !errors.Is(cause, errMaintenance) {
			t.Errorf("Cause = %v, want errMaintenance", cause)
		}
	})

	t.Run("the first stop wins", func(t *testing.T) {
		errFirst := errors.New("first")
		c := ctxflow.Start(t.Context(), blockUntilDone)
		if c == nil {
			t.Fatal("Start returned nil")
		}
		c.Stop(errFirst)
		c.Stop(errors.New("second"))
		callWithin(t, c.Wait)
		if cause := c.Cause(); !errors.Is(cause, errFirst) {
			t.Errorf("Cause = %v, want errFirst", cause)
		}
	})

	t.Run("Stop(nil) cancels with context.Canceled", func(t *testing.T) {
		c := ctxflow.Start(t.Context(), blockUntilDone)
		if c == nil {
			t.Fatal("Start returned nil")
		}
		c.Stop(nil)
		if err, ok := callWithin(t, c.Wait); !ok || !errors.Is(err, context.Canceled) {
			t.Errorf("Wait = %v, want context.Canceled", err)
		}
	})

	t.Run("parent cancellation stops the component with the parent's cause", func(t *testing.T) {
		errShutdown := errors.New("process shutdown")
		parent, cancel := context.WithCancelCause(context.Background())
		c := ctxflow.Start(parent, blockUntilDone)
		if c == nil {
			t.Fatal("Start returned nil")
		}
		cancel(errShutdown)
		if err, ok := callWithin(t, c.Wait); !ok || !errors.Is(err, errShutdown) {
			t.Errorf("Wait = %v, want errShutdown", err)
		}
	})
}

func TestPart3RunWithCleanup(t *testing.T) {
	t.Run("body runs before cleanup, both once", func(t *testing.T) {
		var order []string
		err := ctxflow.RunWithCleanup(t.Context(), time.Second,
			func(context.Context) error { order = append(order, "body"); return nil },
			func(context.Context) error { order = append(order, "cleanup"); return nil },
		)
		if err != nil {
			t.Fatalf("RunWithCleanup error = %v", err)
		}
		if len(order) != 2 || order[0] != "body" || order[1] != "cleanup" {
			t.Errorf("order = %v, want [body cleanup]", order)
		}
	})

	t.Run("body and cleanup errors are both reported", func(t *testing.T) {
		errBody := errors.New("body")
		errClean := errors.New("cleanup")
		err := ctxflow.RunWithCleanup(t.Context(), time.Second,
			func(context.Context) error { return errBody },
			func(context.Context) error { return errClean },
		)
		if !errors.Is(err, errBody) || !errors.Is(err, errClean) {
			t.Errorf("error = %v, want both errBody and errClean", err)
		}
	})

	t.Run("cleanup runs after cancellation with a context that is not cancelled", func(t *testing.T) {
		parent, cancel := context.WithCancel(context.Background())
		var cleanupSawErr error
		var cleanupRan atomic.Int32
		err := ctxflow.RunWithCleanup(parent, time.Second,
			func(c context.Context) error {
				cancel()
				<-c.Done()
				return c.Err()
			},
			func(c context.Context) error {
				cleanupRan.Add(1)
				cleanupSawErr = c.Err()
				return nil
			},
		)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("error = %v, want context.Canceled from body", err)
		}
		if n := cleanupRan.Load(); n != 1 {
			t.Errorf("cleanup ran %d times, want 1", n)
		}
		if cleanupSawErr != nil {
			t.Errorf("cleanup context already done: %v", cleanupSawErr)
		}
	})

	t.Run("a stuck cleanup is bounded by cleanupTimeout", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			start := time.Now()
			err := ctxflow.RunWithCleanup(context.Background(), 50*time.Millisecond,
				func(context.Context) error { return nil },
				func(c context.Context) error {
					<-c.Done()
					return c.Err()
				},
			)
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("error = %v, want DeadlineExceeded from cleanup", err)
			}
			if elapsed := time.Since(start); elapsed != 50*time.Millisecond {
				t.Errorf("elapsed = %v, want 50ms", elapsed)
			}
		})
	})

	t.Run("already-cancelled context skips body but still runs cleanup", func(t *testing.T) {
		errCause := errors.New("drained")
		parent, cancel := context.WithCancelCause(context.Background())
		cancel(errCause)
		var bodyRan, cleanupRan atomic.Int32
		err := ctxflow.RunWithCleanup(parent, time.Second,
			func(context.Context) error { bodyRan.Add(1); return nil },
			func(context.Context) error { cleanupRan.Add(1); return nil },
		)
		if bodyRan.Load() != 0 {
			t.Errorf("body ran on a cancelled context")
		}
		if cleanupRan.Load() != 1 {
			t.Errorf("cleanup ran %d times, want 1", cleanupRan.Load())
		}
		if !errors.Is(err, errCause) {
			t.Errorf("error = %v, want errCause", err)
		}
	})

	t.Run("non-positive cleanupTimeout is rejected before anything runs", func(t *testing.T) {
		var ran atomic.Int32
		err := ctxflow.RunWithCleanup(t.Context(), 0,
			func(context.Context) error { ran.Add(1); return nil },
			func(context.Context) error { ran.Add(1); return nil },
		)
		if !errors.Is(err, ctxflow.ErrInvalidArgument) {
			t.Errorf("error = %v, want ErrInvalidArgument", err)
		}
		if ran.Load() != 0 {
			t.Errorf("%d functions ran, want 0", ran.Load())
		}
	})
}

// closer records Close calls and signals each one.
type closer struct {
	calls  atomic.Int32
	err    error
	closed chan struct{}
}

func newCloser(err error) *closer {
	return &closer{err: err, closed: make(chan struct{}, 4)}
}

func (c *closer) Close() error {
	c.calls.Add(1)
	c.closed <- struct{}{}
	return c.err
}

var _ io.Closer = (*closer)(nil)

func TestPart3CloseOnDone(t *testing.T) {
	t.Run("closes exactly once when the context ends", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		c := newCloser(nil)
		ctxflow.CloseOnDone(ctx, c, nil)
		cancel()
		if _, ok := recv(t, (<-chan struct{})(c.closed)); !ok {
			return
		}
		if n := c.calls.Load(); n != 1 {
			t.Errorf("Close called %d times, want 1", n)
		}
	})

	t.Run("stop before cancellation prevents the close", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		c := newCloser(nil)
		stop := ctxflow.CloseOnDone(ctx, c, nil)
		if stop == nil {
			t.Fatal("stop is nil")
		}
		if !stop() {
			t.Errorf("stop() = false before cancellation, want true")
		}
		cancel()
		if n := c.calls.Load(); n != 0 {
			t.Errorf("Close called %d times after stop, want 0", n)
		}
	})

	t.Run("stop after the close reports false", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		c := newCloser(nil)
		stop := ctxflow.CloseOnDone(ctx, c, nil)
		if stop == nil {
			t.Fatal("stop is nil")
		}
		cancel()
		if _, ok := recv(t, (<-chan struct{})(c.closed)); !ok {
			return
		}
		if stop() {
			t.Errorf("stop() = true after the close ran, want false")
		}
	})

	t.Run("a Close error is passed to onErr", func(t *testing.T) {
		errFlush := errors.New("flush failed")
		ctx, cancel := context.WithCancel(context.Background())
		c := newCloser(errFlush)
		gotErr := make(chan error, 1)
		ctxflow.CloseOnDone(ctx, c, func(err error) { gotErr <- err })
		cancel()
		err, ok := recv(t, (<-chan error)(gotErr))
		if !ok {
			return
		}
		if !errors.Is(err, errFlush) {
			t.Errorf("onErr got %v, want errFlush", err)
		}
	})

	t.Run("nil onErr is allowed", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		c := newCloser(errors.New("ignored"))
		ctxflow.CloseOnDone(ctx, c, nil)
		cancel()
		if _, ok := recv(t, (<-chan struct{})(c.closed)); !ok {
			return
		}
		if n := c.calls.Load(); n != 1 {
			t.Errorf("Close called %d times, want 1", n)
		}
	})
}
