package funcs

import (
	"context"
	"sync"
	"time"
)

// MemoStats counts memo activity. Each Get is counted once, as a Hit or a Miss.
type MemoStats struct {
	Hits   int
	Misses int
	Loads  int
	Errors int
}

type memoEntry[V any] struct {
	val     V
	expires time.Time
}

// inflightLoad is one in-progress load shared by every concurrent Get for its key.
type inflightLoad[V any] struct {
	done chan struct{}
	val  V
	err  error
}

// Memo caches results of a load function per key, for a fixed time-to-live.
// Concurrent Gets for the same missing key share one load (single-flight).
// Errors are not cached.
type Memo[K comparable, V any] struct {
	ttl   time.Duration
	clock func() time.Time
	load  func(ctx context.Context, key K) (V, error)

	mu       sync.Mutex
	entries  map[K]memoEntry[V]
	inflight map[K]*inflightLoad[V]
	stats    MemoStats
}

// NewMemo returns a Memo that stores values for ttl, measured by clock. A ttl of
// zero or less means entries never expire. load runs in the goroutine of the
// Get that found no usable entry, with that Get's context.
func NewMemo[K comparable, V any](ttl time.Duration, clock func() time.Time, load func(ctx context.Context, key K) (V, error)) *Memo[K, V] {
	return &Memo[K, V]{
		ttl:      ttl,
		clock:    clock,
		load:     load,
		entries:  make(map[K]memoEntry[V]),
		inflight: make(map[K]*inflightLoad[V]),
	}
}

// Get returns the cached value for key, loading it if there is no fresh entry.
// A caller that joins an in-flight load stops waiting when ctx is done, but the
// load continues for the others. On error Get returns the zero V.
func (m *Memo[K, V]) Get(ctx context.Context, key K) (V, error) {
	m.mu.Lock()
	if e, ok := m.entries[key]; ok && (m.ttl <= 0 || m.clock().Before(e.expires)) {
		m.stats.Hits++
		m.mu.Unlock()
		return e.val, nil
	}
	m.stats.Misses++
	if c, ok := m.inflight[key]; ok {
		m.mu.Unlock()
		return awaitLoad(ctx, c)
	}
	c := &inflightLoad[V]{done: make(chan struct{})}
	m.inflight[key] = c
	m.stats.Loads++
	m.mu.Unlock()

	val, err := m.load(ctx, key)

	m.mu.Lock()
	delete(m.inflight, key)
	if err != nil {
		m.stats.Errors++
		var zero V
		val = zero
	} else {
		m.entries[key] = memoEntry[V]{val: val, expires: m.clock().Add(m.ttl)}
	}
	c.val, c.err = val, err
	m.mu.Unlock()
	close(c.done)

	return val, err
}

func awaitLoad[V any](ctx context.Context, c *inflightLoad[V]) (V, error) {
	select {
	case <-c.done:
		return c.val, c.err
	case <-ctx.Done():
		var zero V
		return zero, ctx.Err()
	}
}

// Forget drops the cached entry for key. A load already in flight is unaffected.
func (m *Memo[K, V]) Forget(key K) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.entries, key)
}

// Stats returns a snapshot of the counters.
func (m *Memo[K, V]) Stats() MemoStats {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stats
}
