// Package functional is the exercise for 06 · Functional programming with
// generics. Implement every function; see README.md for the spec.
package functional

import "iter"

// Map returns a new slice holding f applied to each element of xs. The result
// is never nil: an empty input yields an empty slice.
func Map[T, U any](xs []T, f func(T) U) []U {
	return nil
}

// Filter returns the elements of xs for which keep reports true, in input
// order. The result is a fresh slice that never aliases xs and is never nil.
func Filter[T any](xs []T, keep func(T) bool) []T {
	return nil
}

// Reduce folds xs from left to right, starting from init.
func Reduce[T, A any](xs []T, init A, f func(A, T) A) A {
	var zero A
	return zero
}

// TryReduce is Reduce for a step that can fail. It stops at the first error and
// returns the zero A alongside it; f is not called again after a failure.
func TryReduce[T, A any](xs []T, init A, f func(A, T) (A, error)) (A, error) {
	var zero A
	return zero, nil
}

// GroupBy buckets xs by key. Elements keep their input order inside each
// bucket. The result is never nil, so an empty input yields an empty map.
func GroupBy[T any, K comparable](xs []T, key func(T) K) map[K][]T {
	return nil
}

// MapSeq lazily applies f to each element of seq. Nothing is pulled from seq
// until the result is ranged over, and an early break stops upstream too.
func MapSeq[T, U any](seq iter.Seq[T], f func(T) U) iter.Seq[U] {
	return func(yield func(U) bool) {}
}

// FilterSeq lazily yields the elements of seq for which keep reports true.
func FilterSeq[T any](seq iter.Seq[T], keep func(T) bool) iter.Seq[T] {
	return func(yield func(T) bool) {}
}

// ReduceSeq folds seq like Reduce, consuming it completely.
func ReduceSeq[T, A any](seq iter.Seq[T], init A, f func(A, T) A) A {
	var zero A
	return zero
}

// Enumerate pairs each element of seq with its zero-based position.
func Enumerate[T any](seq iter.Seq[T]) iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {}
}
