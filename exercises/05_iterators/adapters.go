// Package iterators teaches Go 1.23+ iterators: iter.Seq/iter.Seq2,
// range-over-func, iter.Pull, and lazy pipelines over readers, pages and
// channels. Every function here must be lazy: it reads no further ahead than
// the consumer asks for, and it honours an early break from the consumer.
package iterators

import (
	"cmp"
	"iter"
)

// Map lazily applies f to each element of seq.
func Map[T, U any](seq iter.Seq[T], f func(T) U) iter.Seq[U] {
	return func(yield func(U) bool) {}
}

// Filter lazily yields the elements of seq for which keep returns true.
func Filter[T any](seq iter.Seq[T], keep func(T) bool) iter.Seq[T] {
	return func(yield func(T) bool) {}
}

// Take yields at most n elements of seq. Take must not pull an element from
// seq beyond the n-th one. If n <= 0 it yields nothing and does not range over
// seq at all.
func Take[T any](seq iter.Seq[T], n int) iter.Seq[T] {
	return func(yield func(T) bool) {}
}

// Chunk groups seq into slices of exactly size elements; the final slice may be
// shorter. Each yielded slice is a fresh allocation that the consumer may keep.
// Chunk panics if size < 1.
func Chunk[T any](seq iter.Seq[T], size int) iter.Seq[[]T] {
	return func(yield func([]T) bool) {}
}

// Window yields every sliding window of size consecutive elements of seq, each
// as a fresh slice (windows never alias one another or the internal buffer).
// Partial windows are never yielded. Window panics if size < 1.
func Window[T any](seq iter.Seq[T], size int) iter.Seq[[]T] {
	return func(yield func([]T) bool) {}
}

// MovingAverage yields the mean of each full window of n consecutive values.
// It maintains a running sum in O(1) per value rather than re-summing each
// window. Partial windows are never yielded. MovingAverage panics if n < 1.
func MovingAverage(seq iter.Seq[float64], n int) iter.Seq[float64] {
	return func(yield func(float64) bool) {}
}

// Concat yields every element of seqs[0], then of seqs[1], and so on. Later
// sequences are not ranged over until earlier ones are exhausted, and a
// consumer break stops the current sequence and skips all remaining ones.
func Concat[T any](seqs ...iter.Seq[T]) iter.Seq[T] {
	return func(yield func(T) bool) {}
}

// Enumerate pairs each element of seq with its zero-based position.
func Enumerate[T any](seq iter.Seq[T]) iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {}
}

// SortedEntries yields the entries of m in ascending key order. The map is
// read when iteration starts, so later mutations of m during iteration are
// not required to be visible.
func SortedEntries[K cmp.Ordered, V any](m map[K]V) iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {}
}
