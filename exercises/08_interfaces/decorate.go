package interfaces

import (
	"context"
	"errors"
)

// CacheStats reports cache effectiveness.
type CacheStats struct {
	Hits, Misses, Evictions int64
}

// CacheStore is a bounded, write-through, read-through decorator around a
// BlobStore (Part 3). It deliberately does not implement Lister: a cached
// listing would go stale.
type CacheStore struct{}

// NewCacheStore wraps inner with a cache holding at most maxEntries blobs.
// maxEntries must be at least 1.
func NewCacheStore(inner BlobStore, maxEntries int) (*CacheStore, error) {
	return nil, errors.New("not implemented")
}

// Get implements BlobStore.
func (c *CacheStore) Get(ctx context.Context, key string) ([]byte, error) { return nil, nil }

// Put implements BlobStore.
func (c *CacheStore) Put(ctx context.Context, key string, data []byte) error { return nil }

// Delete implements BlobStore.
func (c *CacheStore) Delete(ctx context.Context, key string) error { return nil }

// Stats returns a snapshot of the counters.
func (c *CacheStore) Stats() CacheStats { return CacheStats{} }

// MetricsSnapshot is a point-in-time copy of Metrics counters.
type MetricsSnapshot struct {
	Gets, Puts, Deletes, Lists, Sizes, Errors int64
}

// Metrics counts operations on instrumented stores. Its zero value is ready to
// use and it is safe for concurrent use.
type Metrics struct{}

// Snapshot returns the current counters.
func (m *Metrics) Snapshot() MetricsSnapshot { return MetricsSnapshot{} }

// Instrument wraps inner so every call is counted in m. The result implements
// Lister, Sizer and io.Closer exactly when inner does (Part 3).
// Errors counts failures other than ErrNotFound.
func Instrument(inner BlobStore, m *Metrics) BlobStore { return inner }
