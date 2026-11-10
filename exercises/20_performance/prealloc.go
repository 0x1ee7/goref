package perf

// Map returns a new slice whose i-th element is f(xs[i]). f is called once per
// element, in order. The result has len(xs) elements and is allocated once.
func Map[T, U any](xs []T, f func(T) U) []U {
	return nil
}

// Concat returns a new slice holding the elements of chunks in order. The result
// never aliases any input chunk. It returns nil when the total length is zero.
// For a non-empty total it allocates exactly once.
func Concat[T any](chunks ...[]T) []T {
	return nil
}

// Batch splits xs into consecutive chunks of at most n elements; only the last
// chunk may be shorter. Chunks are views into xs, not copies, and each chunk's
// capacity is limited to its length, so appending to a chunk reallocates instead
// of overwriting the next chunk. Batch panics if n < 1. It allocates once for the
// outer slice.
func Batch[T any](xs []T, n int) [][]T {
	return nil
}

// Dedupe returns the first occurrence of each distinct value in xs, in input
// order. The input is not modified and the result never aliases it.
func Dedupe[T comparable](xs []T) []T {
	return nil
}
