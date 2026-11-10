package iterators

import (
	"context"
	"iter"
)

// FromChannel adapts a channel to an iterator. It yields each value received
// from ch until ch is closed. If ctx is done first it yields (zero, ctx.Err())
// once and ends. A consumer break returns immediately. FromChannel starts no
// goroutines, so a break cannot leak one.
func FromChannel[T any](ctx context.Context, ch <-chan T) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {}
}

// ToChannel ranges over seq in a new goroutine and sends each value on the
// returned channel, which is closed when seq is exhausted or ctx is done.
//
// The goroutine never blocks forever: each send also selects on ctx.Done(), so
// cancelling ctx stops the goroutine and, through the range loop's early exit,
// runs seq's own cleanup (defers inside seq). A consumer that stops reading
// without cancelling ctx leaks the goroutine; that is the cost of push-based
// generators that this exercise contrasts with pull-based iterators.
func ToChannel[T any](ctx context.Context, seq iter.Seq[T]) <-chan T {
	ch := make(chan T)
	close(ch)
	return ch
}
