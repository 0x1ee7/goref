package typed

import "iter"

// Set is a set of comparable values. The zero value is an empty set ready to
// use. Mutating methods have pointer receivers; reads take values.
type Set[T comparable] struct{}

// NewSet returns a set containing items.
func NewSet[T comparable](items ...T) Set[T] { return Set[T]{} }

// Add inserts items; duplicates are ignored.
func (s *Set[T]) Add(items ...T) {}

// Remove deletes item and reports whether it was present.
func (s *Set[T]) Remove(item T) bool { return false }

// Has reports whether item is in the set.
func (s Set[T]) Has(item T) bool { return false }

// Len returns the number of elements.
func (s Set[T]) Len() int { return 0 }

// All yields every element. Order is unspecified.
func (s Set[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {}
}

// Clone returns an independent copy.
func (s Set[T]) Clone() Set[T] { return Set[T]{} }

// Union returns a new set with the elements of s or other.
func (s Set[T]) Union(other Set[T]) Set[T] { return Set[T]{} }

// Intersect returns a new set with the elements present in both sets.
func (s Set[T]) Intersect(other Set[T]) Set[T] { return Set[T]{} }

// Difference returns a new set with the elements of s that are not in other.
func (s Set[T]) Difference(other Set[T]) Set[T] { return Set[T]{} }

// Equal reports whether both sets hold exactly the same elements.
func (s Set[T]) Equal(other Set[T]) bool { return false }

// OrderedMap is a map that remembers insertion order. The zero value is an
// empty map ready to use.
type OrderedMap[K comparable, V any] struct{}

// Set stores value under key. Overwriting keeps the key's original position.
func (m *OrderedMap[K, V]) Set(key K, value V) {}

// Get returns the value for key and whether it is present.
func (m OrderedMap[K, V]) Get(key K) (V, bool) {
	var zero V
	return zero, false
}

// Delete removes key and reports whether it was present. A later Set of the
// same key appends it at the end.
func (m *OrderedMap[K, V]) Delete(key K) bool { return false }

// Len returns the number of entries.
func (m OrderedMap[K, V]) Len() int { return 0 }

// Keys yields the keys in insertion order.
func (m OrderedMap[K, V]) Keys() iter.Seq[K] {
	return func(yield func(K) bool) {}
}

// All yields key/value pairs in insertion order.
func (m OrderedMap[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {}
}

// Index is a generic type alias: Index[V] is identical to OrderedMap[string, V],
// not a new named type.
type Index[V any] = OrderedMap[string, V]
