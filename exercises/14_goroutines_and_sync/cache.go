package goroutines

import (
	"context"
	"errors"
)

// ErrInvalidCapacity is returned by NewLRU, NewCache and NewBoundedQueue when
// the capacity is < 1.
var ErrInvalidCapacity = errors.New("cache: capacity must be >= 1")

// LRU is a fixed-capacity, least-recently-used map. It is safe for concurrent use.
type LRU[K comparable, V any] struct {
	// engineer-owned fields
}

// NewLRU returns an LRU that holds at most capacity entries.
func NewLRU[K comparable, V any](capacity int) (*LRU[K, V], error) {
	return nil, errors.New("not implemented")
}

// Get returns the value for key and marks it most recently used.
func (c *LRU[K, V]) Get(key K) (V, bool) {
	var zero V
	return zero, false
}

// Peek returns the value for key without changing its recency.
func (c *LRU[K, V]) Peek(key K) (V, bool) {
	var zero V
	return zero, false
}

// Put inserts or updates key and marks it most recently used. When the cache
// grows beyond its capacity, the least recently used entry is evicted.
func (c *LRU[K, V]) Put(key K, value V) {}

// Len returns the number of entries.
func (c *LRU[K, V]) Len() int {
	return 0
}

// Loader loads the value for key. It must return promptly once ctx is done.
type Loader[V any] func(ctx context.Context, key string) (V, error)

// Cache is an LRU in front of a Loader with single-flight loading: concurrent
// GetOrLoad calls for the same missing key share one Loader call. It is safe
// for concurrent use.
type Cache[V any] struct {
	// engineer-owned fields
}

// NewCache returns a Cache that holds at most capacity values and loads misses
// with load. It returns ErrInvalidCapacity if capacity < 1.
func NewCache[V any](capacity int, load Loader[V]) (*Cache[V], error) {
	return nil, errors.New("not implemented")
}

// GetOrLoad returns the cached value for key, or loads it. Only one load per
// key is in flight at a time. A caller whose ctx ends stops waiting and gets
// ctx.Err(), but the load it joined continues and its result is still cached.
// Loader errors are returned to every waiter and are not cached.
func (c *Cache[V]) GetOrLoad(ctx context.Context, key string) (V, error) {
	var zero V
	return zero, errors.New("not implemented")
}

// Loads returns how many times the Loader has been invoked.
func (c *Cache[V]) Loads() int64 {
	return 0
}
