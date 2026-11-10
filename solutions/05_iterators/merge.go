package iterators

import "iter"

// Merge performs a k-way merge of streams that are each already sorted by cmp.
// Ties go to the earlier stream. Each stream is pulled at most one element
// ahead of what has been yielded, so infinite streams are fine. Breaking out
// stops every stream. With no streams it yields nothing.
func Merge[T any](cmp func(a, b T) int, streams ...iter.Seq[T]) iter.Seq[T] {
	return func(yield func(T) bool) {
		type source struct {
			next func() (T, bool)
			stop func()
			cur  T
			ok   bool
		}
		srcs := make([]*source, 0, len(streams))
		defer func() {
			for _, s := range srcs {
				s.stop()
			}
		}()
		for _, seq := range streams {
			next, stop := iter.Pull(seq)
			s := &source{next: next, stop: stop}
			srcs = append(srcs, s)
			s.cur, s.ok = next()
		}
		for {
			best := -1
			for i, s := range srcs {
				if !s.ok {
					continue
				}
				if best < 0 || cmp(s.cur, srcs[best].cur) < 0 {
					best = i
				}
			}
			if best < 0 {
				return
			}
			s := srcs[best]
			if !yield(s.cur) {
				return
			}
			s.cur, s.ok = s.next()
		}
	}
}
