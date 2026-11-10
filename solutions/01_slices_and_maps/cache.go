package slicesmaps

import (
	"container/list"
	"fmt"
	"time"
)

// entry is one cached value with its absolute expiry instant.
type entry[K comparable, V any] struct {
	key       K
	value     V
	expiresAt time.Time
}

// TTLCache is a bounded least-recently-used cache whose entries expire a fixed
// duration after they are written. It is not safe for concurrent use.
//
// The list runs from least recently used (front) to most recently used (back).
// Expired entries stay in the list until a purge, but never count as live.
type TTLCache[K comparable, V any] struct {
	maxsize int
	ttl     time.Duration
	now     func() time.Time
	ll      *list.List
	index   map[K]*list.Element
	hits    int
	misses  int
}

// NewTTLCache returns an empty cache holding at most maxsize live entries, each
// valid for ttl after it is written. A nil now uses time.Now. maxsize < 1 or
// ttl <= 0 yields ErrInvalidConfig.
func NewTTLCache[K comparable, V any](maxsize int, ttl time.Duration, now func() time.Time) (*TTLCache[K, V], error) {
	if maxsize < 1 {
		return nil, fmt.Errorf("%w: maxsize %d", ErrInvalidConfig, maxsize)
	}
	if ttl <= 0 {
		return nil, fmt.Errorf("%w: ttl %v", ErrInvalidConfig, ttl)
	}
	if now == nil {
		now = time.Now
	}
	return &TTLCache[K, V]{
		maxsize: maxsize,
		ttl:     ttl,
		now:     now,
		ll:      list.New(),
		index:   make(map[K]*list.Element),
	}, nil
}

// Get returns the value for key if it is live. A hit makes the entry most
// recently used and does not extend its TTL. An expired entry is removed.
func (c *TTLCache[K, V]) Get(key K) (V, bool) {
	var zero V
	el, ok := c.index[key]
	if !ok || !c.live(el, c.now()) {
		if ok {
			c.remove(el)
		}
		c.misses++
		return zero, false
	}
	c.ll.MoveToBack(el)
	c.hits++
	return c.entryOf(el).value, true
}

// Put inserts or replaces key. Replacing refreshes the TTL and recency. Expired
// entries are purged first, then least-recently-used entries are evicted until
// the cache fits maxsize.
func (c *TTLCache[K, V]) Put(key K, value V) {
	now := c.now()
	c.purge(now)
	if el, ok := c.index[key]; ok {
		e := c.entryOf(el)
		e.value = value
		e.expiresAt = now.Add(c.ttl)
		c.ll.MoveToBack(el)
	} else {
		c.index[key] = c.ll.PushBack(&entry[K, V]{key: key, value: value, expiresAt: now.Add(c.ttl)})
	}
	for c.ll.Len() > c.maxsize {
		c.remove(c.ll.Front())
	}
}

// Contains reports whether key is live. It changes neither recency nor counters.
func (c *TTLCache[K, V]) Contains(key K) bool {
	el, ok := c.index[key]
	return ok && c.live(el, c.now())
}

// Keys returns the live keys, least recently used first. It does not change recency.
func (c *TTLCache[K, V]) Keys() []K {
	now := c.now()
	var keys []K
	for el := c.ll.Front(); el != nil; el = el.Next() {
		if c.live(el, now) {
			keys = append(keys, c.entryOf(el).key)
		}
	}
	return keys
}

// Len returns the number of live entries.
func (c *TTLCache[K, V]) Len() int {
	now := c.now()
	n := 0
	for el := c.ll.Front(); el != nil; el = el.Next() {
		if c.live(el, now) {
			n++
		}
	}
	return n
}

// Hits returns the number of Get calls that found a live entry.
func (c *TTLCache[K, V]) Hits() int { return c.hits }

// Misses returns the number of Get calls that found no live entry.
func (c *TTLCache[K, V]) Misses() int { return c.misses }

// live reports whether el has not yet reached its expiry instant.
func (c *TTLCache[K, V]) live(el *list.Element, now time.Time) bool {
	return now.Before(c.entryOf(el).expiresAt)
}

// purge removes every expired entry.
func (c *TTLCache[K, V]) purge(now time.Time) {
	for el := c.ll.Front(); el != nil; {
		next := el.Next()
		if !c.live(el, now) {
			c.remove(el)
		}
		el = next
	}
}

// remove deletes el from both the list and the index.
func (c *TTLCache[K, V]) remove(el *list.Element) {
	delete(c.index, c.entryOf(el).key)
	c.ll.Remove(el)
}

func (c *TTLCache[K, V]) entryOf(el *list.Element) *entry[K, V] {
	return el.Value.(*entry[K, V])
}
