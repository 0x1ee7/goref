package slicesmaps

import "iter"

// Head returns the first n elements of s, with n clamped to [0, len(s)].
//
// The result is a view sharing s's backing array, with capacity limited to its
// length, so appending to it reallocates instead of overwriting s[n:].
func Head[T any](s []T, n int) []T {
	return nil
}

// Batches yields consecutive, non-overlapping sub-slices of items with at most
// size elements each; the last batch may be shorter. Each batch is a view into
// items with capacity limited to its length. size < 1 panics.
func Batches[T any](items []T, size int) iter.Seq[[]T] {
	return func(yield func([]T) bool) {}
}

// FilterInPlace keeps the elements of s for which keep returns true, preserving
// their order and reusing s's backing array. Vacated slots at the end of s are
// zeroed. The returned slice aliases s.
func FilterInPlace[T any](s []T, keep func(T) bool) []T {
	return nil
}
