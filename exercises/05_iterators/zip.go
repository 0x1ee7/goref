package iterators

import "iter"

// Zip pairs elements of a and b until either sequence is exhausted. Zip pulls
// from b only after a has produced an element, so a shorter a never causes an
// extra element to be drawn from b. Both sequences are stopped (via iter.Pull's
// stop) when Zip returns, including on an early consumer break.
func Zip[A, B any](a iter.Seq[A], b iter.Seq[B]) iter.Seq2[A, B] {
	return func(yield func(A, B) bool) {}
}

// Peekable adds one-element lookahead to an iter.Seq. Construct it with
// NewPeekable. Close must be called to release the underlying iterator; Close
// is idempotent.
type Peekable[T any] struct{}

// NewPeekable wraps seq. Nothing is pulled from seq until the first Peek or Next.
func NewPeekable[T any](seq iter.Seq[T]) *Peekable[T] {
	return &Peekable[T]{}
}

// Peek returns the next element without consuming it. ok is false when the
// sequence is exhausted or the Peekable is closed.
func (p *Peekable[T]) Peek() (v T, ok bool) {
	return v, false
}

// Next returns and consumes the next element. ok is false when the sequence is
// exhausted or the Peekable is closed; repeated calls keep returning false.
func (p *Peekable[T]) Next() (v T, ok bool) {
	return v, false
}

// Close stops the underlying iterator. Peek and Next return zero, false after
// Close.
func (p *Peekable[T]) Close() {}
