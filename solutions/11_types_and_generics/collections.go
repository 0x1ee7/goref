package typed

import (
	"iter"
	"maps"
	"slices"
)

// Set is a set of comparable values. The zero value is an empty set ready to
// use. Mutating methods have pointer receivers; reads take values.
type Set[T comparable] struct {
	m map[T]struct{}
}

// NewSet returns a set containing items.
func NewSet[T comparable](items ...T) Set[T] {
	s := Set[T]{m: make(map[T]struct{}, len(items))}
	s.Add(items...)
	return s
}

// Add inserts items; duplicates are ignored.
func (s *Set[T]) Add(items ...T) {
	if s.m == nil {
		s.m = make(map[T]struct{}, len(items))
	}
	for _, item := range items {
		s.m[item] = struct{}{}
	}
}

// Remove deletes item and reports whether it was present.
func (s *Set[T]) Remove(item T) bool {
	_, ok := s.m[item]
	delete(s.m, item)
	return ok
}

// Has reports whether item is in the set.
func (s Set[T]) Has(item T) bool {
	_, ok := s.m[item]
	return ok
}

// Len returns the number of elements.
func (s Set[T]) Len() int { return len(s.m) }

// All yields every element. Order is unspecified.
func (s Set[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		for item := range s.m {
			if !yield(item) {
				return
			}
		}
	}
}

// Clone returns an independent copy.
func (s Set[T]) Clone() Set[T] { return Set[T]{m: maps.Clone(s.m)} }

// Union returns a new set with the elements of s or other.
func (s Set[T]) Union(other Set[T]) Set[T] {
	out := s.Clone()
	for item := range other.All() {
		out.Add(item)
	}
	return out
}

// Intersect returns a new set with the elements present in both sets.
func (s Set[T]) Intersect(other Set[T]) Set[T] {
	var out Set[T]
	for item := range s.All() {
		if other.Has(item) {
			out.Add(item)
		}
	}
	return out
}

// Difference returns a new set with the elements of s that are not in other.
func (s Set[T]) Difference(other Set[T]) Set[T] {
	var out Set[T]
	for item := range s.All() {
		if !other.Has(item) {
			out.Add(item)
		}
	}
	return out
}

// Equal reports whether both sets hold exactly the same elements.
func (s Set[T]) Equal(other Set[T]) bool {
	if s.Len() != other.Len() {
		return false
	}
	for item := range s.All() {
		if !other.Has(item) {
			return false
		}
	}
	return true
}

// OrderedMap is a map that remembers insertion order. The zero value is an
// empty map ready to use.
type OrderedMap[K comparable, V any] struct {
	keys []K
	vals map[K]V
}

// Set stores value under key. Overwriting keeps the key's original position.
func (m *OrderedMap[K, V]) Set(key K, value V) {
	if m.vals == nil {
		m.vals = make(map[K]V)
	}
	if _, ok := m.vals[key]; !ok {
		m.keys = append(m.keys, key)
	}
	m.vals[key] = value
}

// Get returns the value for key and whether it is present.
func (m OrderedMap[K, V]) Get(key K) (V, bool) {
	v, ok := m.vals[key]
	return v, ok
}

// Delete removes key and reports whether it was present. A later Set of the
// same key appends it at the end.
func (m *OrderedMap[K, V]) Delete(key K) bool {
	if _, ok := m.vals[key]; !ok {
		return false
	}
	delete(m.vals, key)
	i := slices.Index(m.keys, key)
	m.keys = slices.Delete(m.keys, i, i+1)
	return true
}

// Len returns the number of entries.
func (m OrderedMap[K, V]) Len() int { return len(m.vals) }

// Keys yields the keys in insertion order.
func (m OrderedMap[K, V]) Keys() iter.Seq[K] {
	return func(yield func(K) bool) {
		for _, k := range m.keys {
			if !yield(k) {
				return
			}
		}
	}
}

// All yields key/value pairs in insertion order.
func (m OrderedMap[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for _, k := range m.keys {
			if !yield(k, m.vals[k]) {
				return
			}
		}
	}
}

// Index is a generic type alias: Index[V] is identical to OrderedMap[string, V],
// not a new named type.
type Index[V any] = OrderedMap[string, V]
