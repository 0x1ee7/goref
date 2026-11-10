package slicesmaps

import (
	"errors"
	"time"
)

// TTLCache is a bounded least-recently-used cache whose entries expire a fixed
// duration after they are written. It is not safe for concurrent use.
type TTLCache[K comparable, V any] struct{}

// NewTTLCache returns an empty cache holding at most maxsize live entries, each
// valid for ttl after it is written. A nil now uses time.Now. maxsize < 1 or
// ttl <= 0 yields ErrInvalidConfig.
func NewTTLCache[K comparable, V any](maxsize int, ttl time.Duration, now func() time.Time) (*TTLCache[K, V], error) {
	return nil, errors.New("not implemented")
}

// Get returns the value for key if it is live. A hit makes the entry most
// recently used; a miss (absent or expired) returns false and is counted.
func (c *TTLCache[K, V]) Get(key K) (V, bool) {
	var zero V
	return zero, false
}

// Put inserts or replaces key. Replacing refreshes the TTL and recency. Expired
// entries are purged, then least-recently-used entries are evicted to fit maxsize.
func (c *TTLCache[K, V]) Put(key K, value V) {}

// Contains reports whether key is live. It changes neither recency nor counters.
func (c *TTLCache[K, V]) Contains(key K) bool {
	return false
}

// Keys returns the live keys ordered least recently used first. It does not
// change recency.
func (c *TTLCache[K, V]) Keys() []K {
	return nil
}

// Len returns the number of live entries.
func (c *TTLCache[K, V]) Len() int {
	return 0
}

// Hits returns the number of Get calls that found a live entry.
func (c *TTLCache[K, V]) Hits() int {
	return 0
}

// Misses returns the number of Get calls that found no live entry.
func (c *TTLCache[K, V]) Misses() int {
	return 0
}
