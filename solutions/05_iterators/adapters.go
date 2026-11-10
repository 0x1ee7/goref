package iterators

import (
	"cmp"
	"iter"
	"maps"
	"slices"
)

// Map returns a lazy sequence of f applied to each element of seq.
func Map[T, U any](seq iter.Seq[T], f func(T) U) iter.Seq[U] {
	return func(yield func(U) bool) {
		for v := range seq {
			if !yield(f(v)) {
				return
			}
		}
	}
}

// Filter returns a lazy sequence of the elements of seq for which keep is true.
func Filter[T any](seq iter.Seq[T], keep func(T) bool) iter.Seq[T] {
	return func(yield func(T) bool) {
		for v := range seq {
			if keep(v) && !yield(v) {
				return
			}
		}
	}
}

// Take yields at most the first n elements of seq. It returns as soon as the
// n-th element has been yielded, so it never pulls element n+1 from seq.
// n <= 0 yields nothing and never ranges over seq.
func Take[T any](seq iter.Seq[T], n int) iter.Seq[T] {
	return func(yield func(T) bool) {
		if n <= 0 {
			return
		}
		taken := 0
		for v := range seq {
			if !yield(v) {
				return
			}
			taken++
			if taken == n {
				return
			}
		}
	}
}

// Chunk groups seq into slices of up to size elements. Every chunk is a fresh
// slice; the final chunk may be shorter. It panics if size < 1.
func Chunk[T any](seq iter.Seq[T], size int) iter.Seq[[]T] {
	if size < 1 {
		panic("iterators.Chunk: size must be >= 1")
	}
	return func(yield func([]T) bool) {
		buf := make([]T, 0, size)
		for v := range seq {
			buf = append(buf, v)
			if len(buf) == size {
				if !yield(buf) {
					return
				}
				buf = make([]T, 0, size)
			}
		}
		if len(buf) > 0 {
			yield(buf)
		}
	}
}

// Window yields every contiguous run of exactly size elements of seq. Each
// window is a fresh copy, so callers may keep it. Partial windows at the end
// are not yielded. It panics if size < 1.
func Window[T any](seq iter.Seq[T], size int) iter.Seq[[]T] {
	if size < 1 {
		panic("iterators.Window: size must be >= 1")
	}
	return func(yield func([]T) bool) {
		var win []T
		for v := range seq {
			win = append(win, v)
			if len(win) > size {
				win = win[1:]
			}
			if len(win) == size && !yield(slices.Clone(win)) {
				return
			}
		}
	}
}

// MovingAverage yields the mean of each full window of n consecutive values.
// It keeps a ring of the last n values and a running sum, so each step is O(1).
// It panics if n < 1.
func MovingAverage(seq iter.Seq[float64], n int) iter.Seq[float64] {
	if n < 1 {
		panic("iterators.MovingAverage: n must be >= 1")
	}
	return func(yield func(float64) bool) {
		ring := make([]float64, n)
		var sum float64
		seen := 0
		for v := range seq {
			slot := seen % n
			if seen >= n {
				sum -= ring[slot]
			}
			ring[slot] = v
			sum += v
			seen++
			if seen >= n && !yield(sum/float64(n)) {
				return
			}
		}
	}
}

// Concat yields the elements of each sequence in order. Sequences after the
// one a consumer stops in are never ranged over.
func Concat[T any](seqs ...iter.Seq[T]) iter.Seq[T] {
	return func(yield func(T) bool) {
		for _, s := range seqs {
			for v := range s {
				if !yield(v) {
					return
				}
			}
		}
	}
}

// Enumerate pairs each element of seq with its zero-based position.
func Enumerate[T any](seq iter.Seq[T]) iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		i := 0
		for v := range seq {
			if !yield(i, v) {
				return
			}
			i++
		}
	}
}

// SortedEntries yields the entries of m in ascending key order. The keys are
// sorted when iteration starts, not when the sequence is created.
func SortedEntries[K cmp.Ordered, V any](m map[K]V) iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for _, k := range slices.Sorted(maps.Keys(m)) {
			if !yield(k, m[k]) {
				return
			}
		}
	}
}
