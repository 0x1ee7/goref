package iterators

import (
	"context"
	"iter"
)

// FromChannel yields values received from ch until ch is closed. If ctx is
// done first, it yields (zero, ctx.Err()) once and ends. Breaking out of the
// loop returns immediately; no goroutine is involved.
func FromChannel[T any](ctx context.Context, ch <-chan T) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		for {
			select {
			case <-ctx.Done():
				var zero T
				yield(zero, ctx.Err())
				return
			case v, ok := <-ch:
				if !ok {
					return
				}
				if !yield(v, nil) {
					return
				}
			}
		}
	}
}

// ToChannel runs seq in a goroutine and sends each value on the returned
// channel. The channel is closed when seq finishes or ctx is done. When ctx is
// done the goroutine stops ranging over seq, which runs seq's cleanup. A
// consumer that stops receiving should cancel ctx, or the goroutine blocks.
func ToChannel[T any](ctx context.Context, seq iter.Seq[T]) <-chan T {
	out := make(chan T)
	go func() {
		defer close(out)
		for v := range seq {
			select {
			case out <- v:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}
