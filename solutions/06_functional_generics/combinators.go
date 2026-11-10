// Package functional is the reference solution for 06 · Functional programming
// with generics.
package functional

import "iter"

// Map returns a new slice holding f applied to each element of xs. The result
// is never nil: an empty input yields an empty slice.
func Map[T, U any](xs []T, f func(T) U) []U {
	out := make([]U, 0, len(xs))
	for _, x := range xs {
		out = append(out, f(x))
	}
	return out
}

// Filter returns the elements of xs for which keep reports true, in input
// order. The result is a fresh slice that never aliases xs and is never nil.
func Filter[T any](xs []T, keep func(T) bool) []T {
	out := make([]T, 0, len(xs))
	for _, x := range xs {
		if keep(x) {
			out = append(out, x)
		}
	}
	return out
}

// Reduce folds xs from left to right, starting from init.
func Reduce[T, A any](xs []T, init A, f func(A, T) A) A {
	acc := init
	for _, x := range xs {
		acc = f(acc, x)
	}
	return acc
}

// TryReduce is Reduce for a step that can fail. It stops at the first error and
// returns the zero A alongside it; f is not called again after a failure.
func TryReduce[T, A any](xs []T, init A, f func(A, T) (A, error)) (A, error) {
	acc := init
	for _, x := range xs {
		next, err := f(acc, x)
		if err != nil {
			var zero A
			return zero, err
		}
		acc = next
	}
	return acc, nil
}

// GroupBy buckets xs by key. Elements keep their input order inside each
// bucket. The result is never nil, so an empty input yields an empty map.
func GroupBy[T any, K comparable](xs []T, key func(T) K) map[K][]T {
	groups := make(map[K][]T)
	for _, x := range xs {
		k := key(x)
		groups[k] = append(groups[k], x)
	}
	return groups
}

// MapSeq lazily applies f to each element of seq. Nothing is pulled from seq
// until the result is ranged over, and an early break stops upstream too.
func MapSeq[T, U any](seq iter.Seq[T], f func(T) U) iter.Seq[U] {
	return func(yield func(U) bool) {
		for x := range seq {
			if !yield(f(x)) {
				return
			}
		}
	}
}

// FilterSeq lazily yields the elements of seq for which keep reports true.
func FilterSeq[T any](seq iter.Seq[T], keep func(T) bool) iter.Seq[T] {
	return func(yield func(T) bool) {
		for x := range seq {
			if keep(x) && !yield(x) {
				return
			}
		}
	}
}

// ReduceSeq folds seq like Reduce, consuming it completely.
func ReduceSeq[T, A any](seq iter.Seq[T], init A, f func(A, T) A) A {
	acc := init
	for x := range seq {
		acc = f(acc, x)
	}
	return acc
}

// Enumerate pairs each element of seq with its zero-based position.
func Enumerate[T any](seq iter.Seq[T]) iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		i := 0
		for x := range seq {
			if !yield(i, x) {
				return
			}
			i++
		}
	}
}
