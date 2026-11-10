package goroutines

import (
	"container/list"
	"context"
	"errors"
	"sync"
	"sync/atomic"

	"golang.org/x/sync/singleflight"
)

// ErrInvalidCapacity is returned by NewLRU, NewCache and NewBoundedQueue when
// the capacity is < 1.
var ErrInvalidCapacity = errors.New("cache: capacity must be >= 1")

type lruEntry[K comparable, V any] struct {
	key   K
	value V
}

// LRU is a fixed-capacity, least-recently-used map. It is safe for concurrent use.
type LRU[K comparable, V any] struct {
	mu       sync.Mutex
	capacity int
	order    *list.List // front = most recently used
	items    map[K]*list.Element
}

// NewLRU returns an LRU that holds at most capacity entries.
func NewLRU[K comparable, V any](capacity int) (*LRU[K, V], error) {
	if capacity < 1 {
		return nil, ErrInvalidCapacity
	}
	return &LRU[K, V]{
		capacity: capacity,
		order:    list.New(),
		items:    make(map[K]*list.Element, capacity),
	}, nil
}

// Get returns the value for key and marks it most recently used.
func (c *LRU[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[key]
	if !ok {
		var zero V
		return zero, false
	}
	c.order.MoveToFront(el)
	return el.Value.(*lruEntry[K, V]).value, true
}

// Peek returns the value for key without changing its recency.
func (c *LRU[K, V]) Peek(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[key]
	if !ok {
		var zero V
		return zero, false
	}
	return el.Value.(*lruEntry[K, V]).value, true
}

// Put inserts or updates key and marks it most recently used. When the cache
// grows beyond its capacity, the least recently used entry is evicted.
func (c *LRU[K, V]) Put(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[key]; ok {
		el.Value.(*lruEntry[K, V]).value = value
		c.order.MoveToFront(el)
		return
	}
	c.items[key] = c.order.PushFront(&lruEntry[K, V]{key: key, value: value})
	if c.order.Len() > c.capacity {
		oldest := c.order.Back()
		c.order.Remove(oldest)
		delete(c.items, oldest.Value.(*lruEntry[K, V]).key)
	}
}

// Len returns the number of entries.
func (c *LRU[K, V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len()
}

// Loader loads the value for key. It must return promptly once ctx is done.
type Loader[V any] func(ctx context.Context, key string) (V, error)

// Cache is an LRU in front of a Loader with single-flight loading: concurrent
// GetOrLoad calls for the same missing key share one Loader call. It is safe
// for concurrent use.
type Cache[V any] struct {
	lru   *LRU[string, V]
	load  Loader[V]
	group singleflight.Group
	loads atomic.Int64
}

// NewCache returns a Cache that holds at most capacity values and loads misses
// with load. It returns ErrInvalidCapacity if capacity < 1.
func NewCache[V any](capacity int, load Loader[V]) (*Cache[V], error) {
	lru, err := NewLRU[string, V](capacity)
	if err != nil {
		return nil, err
	}
	return &Cache[V]{lru: lru, load: load}, nil
}

// GetOrLoad returns the cached value for key, or loads it. Only one load per
// key is in flight at a time. A caller whose ctx ends stops waiting and gets
// ctx.Err(), but the load it joined continues and its result is still cached.
// Loader errors are returned to every waiter and are not cached.
func (c *Cache[V]) GetOrLoad(ctx context.Context, key string) (V, error) {
	if v, ok := c.lru.Get(key); ok {
		return v, nil
	}
	ch := c.group.DoChan(key, func() (any, error) {
		// Re-check: another flight may have filled the entry since our miss.
		if v, ok := c.lru.Peek(key); ok {
			return v, nil
		}
		c.loads.Add(1)
		// The load must not die with the first caller's context.
		v, err := c.load(context.WithoutCancel(ctx), key)
		if err != nil {
			return nil, err
		}
		c.lru.Put(key, v)
		return v, nil
	})

	var zero V
	select {
	case <-ctx.Done():
		return zero, ctx.Err()
	case r := <-ch:
		if r.Err != nil {
			return zero, r.Err
		}
		v, _ := r.Val.(V)
		return v, nil
	}
}

// Loads returns how many times the Loader has been invoked.
func (c *Cache[V]) Loads() int64 {
	return c.loads.Load()
}
