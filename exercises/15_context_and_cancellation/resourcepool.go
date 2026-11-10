package ctxflow

import "context"

// ResourcePool hands out at most size live resources at a time. Idle resources
// are reused before factory is called again.
type ResourcePool[R any] struct{}

// NewResourcePool returns a pool that creates resources with factory and closes
// them with closeFn. size < 1 wraps ErrInvalidArgument.
func NewResourcePool[R any](size int, factory func(ctx context.Context) (R, error), closeFn func(R) error) (*ResourcePool[R], error) {
	return nil, nil
}

// Acquire returns an idle resource, or creates one if the pool is below size, or
// waits until one is released or ctx is done. A factory error is returned to the
// caller and its slot is freed. After Close, Acquire returns ErrClosed. A waiting
// Acquire that gives up because ctx is done returns context.Cause(ctx) and must
// not consume a slot.
func (p *ResourcePool[R]) Acquire(ctx context.Context) (R, error) {
	var zero R
	return zero, nil
}

// Release returns a resource obtained from Acquire to the pool. Once the pool is
// closed, Release closes the resource instead. Call it exactly once per Acquire.
func (p *ResourcePool[R]) Release(r R) {}

// Close closes all idle resources and makes later Acquire calls fail with
// ErrClosed. Waiting Acquire calls fail with ErrClosed too. Leased resources are
// closed when they are released. Close returns the joined closeFn errors and is
// a no-op with a nil result the second time.
func (p *ResourcePool[R]) Close() error { return nil }
