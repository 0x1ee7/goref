package ctxflow

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// ResourcePool hands out at most size resources at a time. Idle resources are
// reused; the factory runs only when no idle resource is available and a slot
// is free. It is safe for concurrent use.
type ResourcePool[R any] struct {
	factory func(ctx context.Context) (R, error)
	closeFn func(R) error

	// slots is a semaphore: one token per live (idle or leased) resource slot.
	slots chan struct{}
	// done is closed by Close to wake blocked waiters.
	done chan struct{}

	mu     sync.Mutex
	idle   []R
	closed bool
}

// NewResourcePool returns a pool that holds at most size resources. size must
// be at least 1.
func NewResourcePool[R any](size int, factory func(ctx context.Context) (R, error), closeFn func(R) error) (*ResourcePool[R], error) {
	if size < 1 {
		return nil, fmt.Errorf("%w: pool size must be >= 1, got %d", ErrInvalidArgument, size)
	}
	return &ResourcePool[R]{
		factory: factory,
		closeFn: closeFn,
		slots:   make(chan struct{}, size),
		done:    make(chan struct{}),
	}, nil
}

// Acquire returns an idle resource or creates one, waiting for a free slot if
// the pool is full. A waiter that gives up returns context.Cause(ctx) and
// takes no slot. A closed pool returns ErrClosed.
func (p *ResourcePool[R]) Acquire(ctx context.Context) (R, error) {
	var zero R
	select {
	case <-p.done:
		return zero, ErrClosed
	default:
	}
	select {
	case p.slots <- struct{}{}:
	case <-ctx.Done():
		return zero, context.Cause(ctx)
	case <-p.done:
		return zero, ErrClosed
	}

	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		<-p.slots
		return zero, ErrClosed
	}
	if n := len(p.idle); n > 0 {
		r := p.idle[n-1]
		p.idle = p.idle[:n-1]
		p.mu.Unlock()
		return r, nil
	}
	p.mu.Unlock()

	r, err := p.factory(ctx)
	if err != nil {
		<-p.slots
		return zero, err
	}
	return r, nil
}

// Release returns r to the pool. After Close, r is closed instead of pooled.
// Errors from closing r in that case are discarded, since Release has no
// error return.
func (p *ResourcePool[R]) Release(r R) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		_ = p.closeFn(r)
		<-p.slots
		return
	}
	p.idle = append(p.idle, r)
	p.mu.Unlock()
	<-p.slots
}

// Close closes every idle resource and wakes blocked waiters with ErrClosed.
// Leased resources are closed when released. A second Close returns nil.
func (p *ResourcePool[R]) Close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	idle := p.idle
	p.idle = nil
	close(p.done)
	p.mu.Unlock()

	var errs []error
	for _, r := range idle {
		if err := p.closeFn(r); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
