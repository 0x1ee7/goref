package typed

import (
	"errors"
	"fmt"
	"slices"
	"sync"
)

// ErrNotFound reports a missing entity.
var ErrNotFound = errors.New("not found")

// ErrExists reports a Create for a key that is already stored.
var ErrExists = errors.New("already exists")

// Keyed is implemented by entities that know their own key.
type Keyed[K comparable] interface {
	Key() K
}

// Repository is an in-memory store of entities keyed by K, preserving
// insertion order. The zero value is ready to use. It is safe for concurrent
// use and must not be copied after first use.
type Repository[K comparable, T Keyed[K]] struct {
	mu    sync.RWMutex
	items map[K]T
	order []K
}

// Create stores item under item.Key(). It returns an error wrapping ErrExists
// if the key is taken; the stored entity is left untouched.
func (r *Repository[K, T]) Create(item T) error {
	key := item.Key()
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.items[key]; ok {
		return fmt.Errorf("create %v: %w", key, ErrExists)
	}
	if r.items == nil {
		r.items = make(map[K]T)
	}
	r.items[key] = item
	r.order = append(r.order, key)
	return nil
}

// Get returns the entity stored under key. A missing key yields the zero T and
// an error wrapping ErrNotFound.
func (r *Repository[K, T]) Get(key K) (T, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, ok := r.items[key]
	if !ok {
		var zero T
		return zero, fmt.Errorf("get %v: %w", key, ErrNotFound)
	}
	return item, nil
}

// Replace overwrites the entity stored under item.Key() and keeps its position.
// A missing key yields an error wrapping ErrNotFound.
func (r *Repository[K, T]) Replace(item T) error {
	key := item.Key()
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.items[key]; !ok {
		return fmt.Errorf("replace %v: %w", key, ErrNotFound)
	}
	r.items[key] = item
	return nil
}

// Delete removes the entity stored under key. A missing key yields an error
// wrapping ErrNotFound.
func (r *Repository[K, T]) Delete(key K) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.items[key]; !ok {
		return fmt.Errorf("delete %v: %w", key, ErrNotFound)
	}
	delete(r.items, key)
	i := slices.Index(r.order, key)
	r.order = slices.Delete(r.order, i, i+1)
	return nil
}

// Find returns the entities for which match reports true, in insertion order.
// The result is never nil. match runs under the read lock and must not call
// back into the repository.
func (r *Repository[K, T]) Find(match func(T) bool) []T {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]T, 0)
	for _, key := range r.order {
		if item := r.items[key]; match(item) {
			out = append(out, item)
		}
	}
	return out
}

// Len returns the number of stored entities.
func (r *Repository[K, T]) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.items)
}
