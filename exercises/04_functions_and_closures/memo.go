package funcs

import (
	"context"
	"time"
)

// MemoStats are counters for a Memo.
type MemoStats struct {
	Hits   int // Get answered from a fresh cached value
	Misses int // every other Get, including ones that waited on an in-flight load
	Loads  int // load invocations
	Errors int // load invocations that returned an error
}

// Memo caches the results of load per key with a TTL. It is safe for
// concurrent use.
type Memo[K comparable, V any] struct{}

// NewMemo returns a Memo. A ttl <= 0 disables expiry. An entry stored at time t
// is fresh while clock() < t+ttl.
func NewMemo[K comparable, V any](ttl time.Duration, clock func() time.Time, load func(ctx context.Context, key K) (V, error)) *Memo[K, V] {
	return &Memo[K, V]{}
}

// Get returns the cached value for key or loads it. Concurrent Gets for the
// same key during one load share that load's result and error. Errors are not
// cached. The load receives the ctx of the Get that started it. A waiter whose
// ctx ends returns ctx.Err() while the load continues for the others.
func (m *Memo[K, V]) Get(ctx context.Context, key K) (V, error) {
	var zero V
	return zero, nil
}

// Forget drops the cached value for key so the next Get loads again. An
// in-flight load is not affected.
func (m *Memo[K, V]) Forget(key K) {}

// Stats returns a snapshot of the counters.
func (m *Memo[K, V]) Stats() MemoStats { return MemoStats{} }
