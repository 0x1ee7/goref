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

// conn is a fake resource that records how it was closed.
type conn struct {
	id     int
	closed atomic.Bool
}

// connFactory returns a factory that numbers connections from 1 and counts calls.
func connFactory(created *atomic.Int32) func(context.Context) (*conn, error) {
	return func(context.Context) (*conn, error) {
		return &conn{id: int(created.Add(1))}, nil
	}
}

func closeConn(c *conn) error {
	c.closed.Store(true)
	return nil
}

func newPool(t *testing.T, size int, factory func(context.Context) (*conn, error)) *ctxflow.ResourcePool[*conn] {
	t.Helper()
	p, err := ctxflow.NewResourcePool(size, factory, closeConn)
	if err != nil {
		t.Fatalf("NewResourcePool(%d) error = %v", size, err)
	}
	if p == nil {
		t.Fatal("NewResourcePool returned a nil pool")
	}
	return p
}

// mustAcquire acquires a resource and fails the test on error or a nil resource.
func mustAcquire(t *testing.T, p *ctxflow.ResourcePool[*conn], ctx context.Context) *conn {
	t.Helper()
	c, err := p.Acquire(ctx)
	if err != nil {
		t.Fatalf("Acquire error = %v", err)
	}
	if c == nil {
		t.Fatal("Acquire returned a nil resource")
	}
	return c
}

func TestPart4ResourcePool(t *testing.T) {
	t.Run("idle resources are reused before the factory is called again", func(t *testing.T) {
		var created atomic.Int32
		p := newPool(t, 2, connFactory(&created))
		c1 := mustAcquire(t, p, t.Context())
		p.Release(c1)
		c2 := mustAcquire(t, p, t.Context())
		if c2 != c1 {
			t.Errorf("second Acquire returned conn %d, want reused conn %d", c2.id, c1.id)
		}
		if n := created.Load(); n != 1 {
			t.Errorf("factory called %d times, want 1", n)
		}
	})

	t.Run("live resources never exceed size and a waiter gives up on its context", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			var created atomic.Int32
			p := newPool(t, 2, connFactory(&created))
			a := mustAcquire(t, p, context.Background())
			b := mustAcquire(t, p, context.Background())
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer cancel()
			if _, err := p.Acquire(ctx); !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("third Acquire error = %v, want DeadlineExceeded", err)
			}
			if n := created.Load(); n != 2 {
				t.Errorf("factory called %d times, want 2", n)
			}
			p.Release(a)
			p.Release(b)
		})
	})

	t.Run("a release wakes a waiting Acquire", func(t *testing.T) {
		var created atomic.Int32
		p := newPool(t, 1, connFactory(&created))
		held := mustAcquire(t, p, t.Context())
		got := make(chan *conn, 1)
		go func() {
			c, err := p.Acquire(t.Context())
			if err != nil {
				got <- nil
				return
			}
			got <- c
		}()
		p.Release(held)
		c, ok := recv(t, (<-chan *conn)(got))
		if !ok {
			return
		}
		if c == nil || c != held {
			t.Errorf("waiter got conn %v, want the released conn %d", c, held.id)
		}
	})

	t.Run("a factory error is returned and frees its slot", func(t *testing.T) {
		errDial := errors.New("dial refused")
		var attempts atomic.Int32
		factory := func(context.Context) (*conn, error) {
			n := attempts.Add(1)
			if n == 1 {
				return nil, errDial
			}
			return &conn{id: int(n)}, nil
		}
		p := newPool(t, 1, factory)
		if _, err := p.Acquire(t.Context()); !errors.Is(err, errDial) {
			t.Errorf("first Acquire error = %v, want errDial", err)
		}
		if c, err := p.Acquire(t.Context()); err != nil || c == nil {
			t.Errorf("second Acquire = (%v, %v), want a resource and nil (slot must be freed)", c, err)
		}
	})

	t.Run("a cancelled waiter does not consume a slot", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			var created atomic.Int32
			p := newPool(t, 1, connFactory(&created))
			held := mustAcquire(t, p, context.Background())
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
			defer cancel()
			if _, err := p.Acquire(ctx); !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("waiter error = %v, want DeadlineExceeded", err)
			}
			p.Release(held)
			c := mustAcquire(t, p, context.Background())
			if c != held {
				t.Errorf("Acquire returned conn %d, want reused conn %d", c.id, held.id)
			}
		})
	})

	t.Run("Close closes idle resources and rejects later Acquire", func(t *testing.T) {
		var created atomic.Int32
		p := newPool(t, 2, connFactory(&created))
		c := mustAcquire(t, p, t.Context())
		p.Release(c)
		if err := p.Close(); err != nil {
			t.Fatalf("Close error = %v", err)
		}
		if !c.closed.Load() {
			t.Errorf("idle conn was not closed by Close")
		}
		if _, err := p.Acquire(t.Context()); !errors.Is(err, ctxflow.ErrClosed) {
			t.Errorf("Acquire after Close error = %v, want ErrClosed", err)
		}
	})

	t.Run("a resource released after Close is closed, not pooled", func(t *testing.T) {
		var created atomic.Int32
		p := newPool(t, 1, connFactory(&created))
		c := mustAcquire(t, p, t.Context())
		if err := p.Close(); err != nil {
			t.Fatalf("Close error = %v", err)
		}
		if c.closed.Load() {
			t.Fatalf("leased conn closed by Close; it must wait for Release")
		}
		p.Release(c)
		if !c.closed.Load() {
			t.Errorf("conn released after Close was not closed")
		}
	})

	t.Run("a waiter is woken with ErrClosed when the pool closes", func(t *testing.T) {
		var created atomic.Int32
		p := newPool(t, 1, connFactory(&created))
		held := mustAcquire(t, p, t.Context())
		errs := make(chan error, 1)
		go func() {
			_, err := p.Acquire(t.Context())
			errs <- err
		}()
		if err := p.Close(); err != nil {
			t.Fatalf("Close error = %v", err)
		}
		if err, ok := recv(t, (<-chan error)(errs)); ok && !errors.Is(err, ctxflow.ErrClosed) {
			t.Errorf("waiter error = %v, want ErrClosed", err)
		}
		p.Release(held)
	})

	t.Run("second Close is a no-op", func(t *testing.T) {
		var created atomic.Int32
		p := newPool(t, 1, connFactory(&created))
		if err := p.Close(); err != nil {
			t.Fatalf("first Close error = %v", err)
		}
		if err := p.Close(); err != nil {
			t.Errorf("second Close error = %v, want nil", err)
		}
	})

	t.Run("size below one is rejected", func(t *testing.T) {
		_, err := ctxflow.NewResourcePool(0, func(context.Context) (*conn, error) { return nil, nil }, closeConn)
		if !errors.Is(err, ctxflow.ErrInvalidArgument) {
			t.Errorf("NewResourcePool(0) error = %v, want ErrInvalidArgument", err)
		}
	})
}
