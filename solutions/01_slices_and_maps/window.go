package slicesmaps

import "iter"

// Head returns the first n elements of s, with n clamped to [0, len(s)].
//
// The result is a view sharing s's backing array. The full slice expression
// caps its capacity at its length, so appending to it reallocates instead of
// overwriting s[n:].
func Head[T any](s []T, n int) []T {
	n = min(max(n, 0), len(s))
	return s[:n:n]
}

// Batches yields consecutive, non-overlapping sub-slices of items with at most
// size elements each; the last batch may be shorter. Each batch is a view into
// items with capacity limited to its length, so appending to one batch cannot
// clobber the next. size < 1 panics, as slices.Chunk does.
func Batches[T any](items []T, size int) iter.Seq[[]T] {
	if size < 1 {
		panic("slicesmaps: Batches size must be at least 1")
	}
	return func(yield func([]T) bool) {
		for start := 0; start < len(items); start += size {
			end := len(items)
			if size < end-start {
				end = start + size
			}
			if !yield(items[start:end:end]) {
				return
			}
		}
	}
}

// FilterInPlace keeps the elements of s for which keep returns true, preserving
// order and reusing s's backing array. Vacated slots are zeroed so the garbage
// collector can reclaim what they referenced. The returned slice aliases s.
func FilterInPlace[T any](s []T, keep func(T) bool) []T {
	n := 0
	for _, v := range s {
		if keep(v) {
			s[n] = v
			n++
		}
	}
	clear(s[n:])
	return s[:n]
}
