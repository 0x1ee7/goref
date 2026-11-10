package interfaces

import (
	"bytes"
	"container/list"
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
)

// CacheStats reports cache effectiveness.
type CacheStats struct {
	Hits, Misses, Evictions int64
}

// CacheStore is a bounded, write-through, read-through decorator around a
// BlobStore (Part 3). It deliberately does not implement Lister: a cached
// listing would go stale.
//
// Eviction is FIFO by insertion. Only successful reads are cached. A generation
// counter stops a read that raced with a write from caching stale bytes.
type CacheStore struct {
	inner BlobStore
	max   int

	mu    sync.Mutex
	gen   uint64
	items map[string]*list.Element // key -> element whose Value is *cacheEntry
	order *list.List               // front is the oldest insertion
	stats CacheStats
}

type cacheEntry struct {
	key  string
	data []byte
}

var _ BlobStore = (*CacheStore)(nil)

// NewCacheStore wraps inner with a cache holding at most maxEntries blobs.
// maxEntries must be at least 1.
func NewCacheStore(inner BlobStore, maxEntries int) (*CacheStore, error) {
	if maxEntries < 1 {
		return nil, fmt.Errorf("cache store: maxEntries must be at least 1, got %d", maxEntries)
	}
	return &CacheStore{
		inner: inner,
		max:   maxEntries,
		items: make(map[string]*list.Element, maxEntries),
		order: list.New(),
	}, nil
}

// Get implements BlobStore.
func (c *CacheStore) Get(ctx context.Context, key string) ([]byte, error) {
	c.mu.Lock()
	if el, ok := c.items[key]; ok {
		c.stats.Hits++
		data := bytes.Clone(el.Value.(*cacheEntry).data)
		c.mu.Unlock()
		return data, nil
	}
	c.stats.Misses++
	gen := c.gen
	c.mu.Unlock()

	data, err := c.inner.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	if c.gen == gen {
		c.insertLocked(key, data)
	}
	c.mu.Unlock()
	return data, nil
}

// Put implements BlobStore. The write goes to the inner store first; the cache
// is updated only after it succeeds.
func (c *CacheStore) Put(ctx context.Context, key string, data []byte) error {
	if err := c.inner.Put(ctx, key, data); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.gen++
	c.insertLocked(key, data)
	return nil
}

// Delete implements BlobStore. The entry is dropped whatever the inner result,
// because a failed delete leaves the cache in an unknown state.
func (c *CacheStore) Delete(ctx context.Context, key string) error {
	err := c.inner.Delete(ctx, key)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.gen++
	c.removeLocked(key)
	return err
}

// Stats returns a snapshot of the counters.
func (c *CacheStore) Stats() CacheStats {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stats
}

// insertLocked stores a private copy of data, evicting the oldest entry when
// full. Updating an existing key keeps its FIFO position.
func (c *CacheStore) insertLocked(key string, data []byte) {
	if el, ok := c.items[key]; ok {
		el.Value.(*cacheEntry).data = bytes.Clone(data)
		return
	}
	if c.order.Len() >= c.max {
		oldest := c.order.Remove(c.order.Front()).(*cacheEntry)
		delete(c.items, oldest.key)
		c.stats.Evictions++
	}
	c.items[key] = c.order.PushBack(&cacheEntry{key: key, data: bytes.Clone(data)})
}

func (c *CacheStore) removeLocked(key string) {
	if el, ok := c.items[key]; ok {
		c.order.Remove(el)
		delete(c.items, key)
	}
}

// MetricsSnapshot is a point-in-time copy of Metrics counters.
type MetricsSnapshot struct {
	Gets, Puts, Deletes, Lists, Sizes, Errors int64
}

// Metrics counts operations on instrumented stores. Its zero value is ready to
// use and it is safe for concurrent use.
type Metrics struct {
	gets, puts, deletes, lists, sizes, errs atomic.Int64
}

// Snapshot returns the current counters.
func (m *Metrics) Snapshot() MetricsSnapshot {
	return MetricsSnapshot{
		Gets:    m.gets.Load(),
		Puts:    m.puts.Load(),
		Deletes: m.deletes.Load(),
		Lists:   m.lists.Load(),
		Sizes:   m.sizes.Load(),
		Errors:  m.errs.Load(),
	}
}

// record bumps an operation counter and, for real failures, the error counter.
func (m *Metrics) record(op *atomic.Int64, err error) {
	op.Add(1)
	if err != nil && !errors.Is(err, ErrNotFound) {
		m.errs.Add(1)
	}
}

// meter instruments the three mandatory BlobStore methods.
type meter struct {
	inner BlobStore
	m     *Metrics
}

func (s *meter) Get(ctx context.Context, key string) ([]byte, error) {
	data, err := s.inner.Get(ctx, key)
	s.m.record(&s.m.gets, err)
	return data, err
}

func (s *meter) Put(ctx context.Context, key string, data []byte) error {
	err := s.inner.Put(ctx, key, data)
	s.m.record(&s.m.puts, err)
	return err
}

func (s *meter) Delete(ctx context.Context, key string) error {
	err := s.inner.Delete(ctx, key)
	s.m.record(&s.m.deletes, err)
	return err
}

// listMeter adds a metered Lister capability.
type listMeter struct {
	l Lister
	m *Metrics
}

func (s *listMeter) List(ctx context.Context, prefix string) ([]string, error) {
	keys, err := s.l.List(ctx, prefix)
	s.m.record(&s.m.lists, err)
	return keys, err
}

// sizeMeter adds a metered Sizer capability.
type sizeMeter struct {
	s Sizer
	m *Metrics
}

func (s *sizeMeter) Size(ctx context.Context, key string) (int64, error) {
	n, err := s.s.Size(ctx, key)
	s.m.record(&s.m.sizes, err)
	return n, err
}

// One struct per combination of optional capabilities, so the wrapper
// satisfies exactly the interfaces the inner store does. The embedded
// io.Closer is passed through unmetered.
type (
	instrLister struct {
		meter
		listMeter
	}
	instrSizer struct {
		meter
		sizeMeter
	}
	instrCloser struct {
		meter
		io.Closer
	}
	instrListerSizer struct {
		meter
		listMeter
		sizeMeter
	}
	instrListerCloser struct {
		meter
		listMeter
		io.Closer
	}
	instrSizerCloser struct {
		meter
		sizeMeter
		io.Closer
	}
	instrAll struct {
		meter
		listMeter
		sizeMeter
		io.Closer
	}
)

// Instrument wraps inner so every call is counted in m. The result implements
// Lister, Sizer and io.Closer exactly when inner does (Part 3).
// Errors counts failures other than ErrNotFound. m must be non-nil.
func Instrument(inner BlobStore, m *Metrics) BlobStore {
	base := meter{inner: inner, m: m}
	l, hasL := inner.(Lister)
	s, hasS := inner.(Sizer)
	c, hasC := inner.(io.Closer)
	lm := listMeter{l: l, m: m}
	sm := sizeMeter{s: s, m: m}

	switch {
	case hasL && hasS && hasC:
		return &instrAll{base, lm, sm, c}
	case hasL && hasS:
		return &instrListerSizer{base, lm, sm}
	case hasL && hasC:
		return &instrListerCloser{base, lm, c}
	case hasS && hasC:
		return &instrSizerCloser{base, sm, c}
	case hasL:
		return &instrLister{base, lm}
	case hasS:
		return &instrSizer{base, sm}
	case hasC:
		return &instrCloser{base, c}
	default:
		return &base
	}
}
