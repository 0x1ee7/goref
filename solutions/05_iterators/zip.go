package iterators

import "iter"

// Zip pairs elements of a and b positionally. It stops at the shorter input.
// b is pulled only after a has produced a value, so an exhausted a leaves b
// untouched. Both sources are stopped when iteration ends.
func Zip[A, B any](a iter.Seq[A], b iter.Seq[B]) iter.Seq2[A, B] {
	return func(yield func(A, B) bool) {
		nextA, stopA := iter.Pull(a)
		defer stopA()
		nextB, stopB := iter.Pull(b)
		defer stopB()
		for {
			va, ok := nextA()
			if !ok {
				return
			}
			vb, ok := nextB()
			if !ok {
				return
			}
			if !yield(va, vb) {
				return
			}
		}
	}
}

// Peekable wraps a sequence with one-element lookahead. Nothing is pulled
// until the first Peek or Next call.
type Peekable[T any] struct {
	next   func() (T, bool)
	stop   func()
	peeked T
	has    bool
	done   bool
	closed bool
}

// NewPeekable wraps seq. The sequence is not started until it is first used.
func NewPeekable[T any](seq iter.Seq[T]) *Peekable[T] {
	next, stop := iter.Pull(seq)
	return &Peekable[T]{next: next, stop: stop}
}

// fill makes a value available in p.peeked if one exists and reports whether it does.
func (p *Peekable[T]) fill() bool {
	if p.closed {
		return false
	}
	if p.has {
		return true
	}
	if p.done {
		return false
	}
	v, ok := p.next()
	if !ok {
		p.done = true
		return false
	}
	p.peeked, p.has = v, true
	return true
}

// Peek returns the next value without consuming it.
func (p *Peekable[T]) Peek() (T, bool) {
	if !p.fill() {
		var zero T
		return zero, false
	}
	return p.peeked, true
}

// Next consumes and returns the next value.
func (p *Peekable[T]) Next() (T, bool) {
	v, ok := p.Peek()
	if ok {
		var zero T
		p.peeked, p.has = zero, false
	}
	return v, ok
}

// Close stops the underlying sequence, running its cleanup. It is idempotent;
// after Close, Peek and Next report no value.
func (p *Peekable[T]) Close() {
	if p.closed {
		return
	}
	p.closed = true
	p.stop()
	var zero T
	p.peeked, p.has = zero, false
}
