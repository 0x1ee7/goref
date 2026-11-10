package perf

// Map returns a new slice whose i-th element is f(xs[i]). f is called once per
// element, in order. The result has len(xs) elements and is allocated once.
func Map[T, U any](xs []T, f func(T) U) []U {
	out := make([]U, len(xs))
	for i, x := range xs {
		out[i] = f(x)
	}
	return out
}

// Concat returns a new slice holding the elements of chunks in order. The result
// never aliases any input chunk. It returns nil when the total length is zero.
// For a non-empty total it allocates exactly once.
func Concat[T any](chunks ...[]T) []T {
	total := 0
	for _, c := range chunks {
		total += len(c)
	}
	if total == 0 {
		return nil
	}
	out := make([]T, 0, total)
	for _, c := range chunks {
		out = append(out, c...)
	}
	return out
}

// Batch splits xs into consecutive chunks of at most n elements; only the last
// chunk may be shorter. Chunks are views into xs, not copies, and each chunk's
// capacity is limited to its length, so appending to a chunk reallocates instead
// of overwriting the next chunk. Batch panics if n < 1. It allocates once for the
// outer slice.
func Batch[T any](xs []T, n int) [][]T {
	if n < 1 {
		panic("perf: Batch size must be at least 1")
	}
	count := 0
	if len(xs) > 0 {
		// Ceiling division without the n+len overflow that (len+n-1)/n would risk.
		count = (len(xs)-1)/n + 1
	}
	out := make([][]T, 0, count)
	for rest := xs; len(rest) > 0; {
		k := min(n, len(rest))
		out = append(out, rest[:k:k])
		rest = rest[k:]
	}
	return out
}

// Dedupe returns the first occurrence of each distinct value in xs, in input
// order. The input is not modified and the result never aliases it.
func Dedupe[T comparable](xs []T) []T {
	seen := make(map[T]struct{}, len(xs))
	out := make([]T, 0, len(xs))
	for _, x := range xs {
		if _, dup := seen[x]; dup {
			continue
		}
		seen[x] = struct{}{}
		out = append(out, x)
	}
	return out
}
