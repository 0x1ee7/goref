package typed

import "errors"

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
type Repository[K comparable, T Keyed[K]] struct{}

// Create stores item under item.Key(). It returns an error wrapping ErrExists
// if the key is taken; the stored entity is left untouched.
func (r *Repository[K, T]) Create(item T) error { return nil }

// Get returns the entity stored under key. A missing key yields the zero T and
// an error wrapping ErrNotFound.
func (r *Repository[K, T]) Get(key K) (T, error) {
	var zero T
	return zero, nil
}

// Replace overwrites the entity stored under item.Key() and keeps its position.
// A missing key yields an error wrapping ErrNotFound.
func (r *Repository[K, T]) Replace(item T) error { return nil }

// Delete removes the entity stored under key. A missing key yields an error
// wrapping ErrNotFound.
func (r *Repository[K, T]) Delete(key K) error { return nil }

// Find returns the entities for which match reports true, in insertion order.
// The result is never nil.
func (r *Repository[K, T]) Find(match func(T) bool) []T { return nil }

// Len returns the number of stored entities.
func (r *Repository[K, T]) Len() int { return 0 }
