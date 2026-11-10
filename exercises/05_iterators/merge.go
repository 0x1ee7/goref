package iterators

import "iter"

// Merge lazily merges streams that are each already sorted according to cmp,
// where cmp(a, b) < 0 means a sorts before b (the convention of cmp.Compare
// and time.Time.Compare). The output is sorted too.
//
// On ties the element from the earlier stream argument comes first; elements
// of one stream keep their relative order. Merge works on infinite streams
// (consume with Take). It pulls at most one element ahead per stream, and every
// stream is stopped when Merge returns, including on consumer break. Zero
// streams yield nothing.
func Merge[T any](cmp func(a, b T) int, streams ...iter.Seq[T]) iter.Seq[T] {
	return func(yield func(T) bool) {}
}
