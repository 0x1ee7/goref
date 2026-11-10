package iterators_test

import (
	"context"
	"errors"
	"iter"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	iterators "goref/exercises/05_iterators"
)

// waitFor fails the test if done is not closed within two seconds.
func waitFor(t *testing.T, what string, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("%s: timed out after 2s", what)
	}
}

func TestPart8FromChannel(t *testing.T) {
	t.Run("yields values until the channel is closed", func(t *testing.T) {
		ch := make(chan int, 3)
		ch <- 1
		ch <- 2
		ch <- 3
		close(ch)
		got, err := drain(t, iterators.FromChannel(t.Context(), ch))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if diff := cmp.Diff([]int{1, 2, 3}, got); diff != "" {
			t.Errorf("FromChannel mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("cancelled context yields ctx.Err once", func(t *testing.T) {
		ch := make(chan int) // never written, never closed
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		got, err := drain(t, iterators.FromChannel(ctx, ch))
		if len(got) != 0 {
			t.Errorf("values = %v, want none", got)
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("err = %v, want context.Canceled", err)
		}
	})

	t.Run("break returns without waiting for the channel to close", func(t *testing.T) {
		ch := make(chan int, 2)
		ch <- 1
		ch <- 2 // deliberately left open
		var got []int
		done := make(chan struct{})
		go func() {
			defer close(done)
			for v, err := range iterators.FromChannel(t.Context(), ch) {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
					return
				}
				got = append(got, v)
				if len(got) == 2 {
					break
				}
			}
		}()
		waitFor(t, "FromChannel after break", done)
		if diff := cmp.Diff([]int{1, 2}, got); diff != "" {
			t.Errorf("got mismatch (-want +got):\n%s", diff)
		}
	})
}

func TestPart8ToChannel(t *testing.T) {
	t.Run("delivers every value and closes", func(t *testing.T) {
		var got []int
		for v := range iterators.ToChannel(t.Context(), ints(3)) {
			got = append(got, v)
		}
		if diff := cmp.Diff([]int{1, 2, 3}, got); diff != "" {
			t.Errorf("ToChannel mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("cancel stops the producer goroutine and runs seq cleanup", func(t *testing.T) {
		stopped := make(chan struct{})
		var seq iter.Seq[int] = func(yield func(int) bool) {
			defer close(stopped)
			for i := 1; ; i++ {
				if !yield(i) {
					return
				}
			}
		}
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		ch := iterators.ToChannel(ctx, seq)
		for want := 1; want <= 3; want++ {
			v, ok := <-ch
			if !ok || v != want {
				t.Fatalf("receive = (%d, %t), want (%d, true)", v, ok, want)
			}
		}
		cancel()
		waitFor(t, "producer cleanup after cancel", stopped)

		drained := make(chan struct{})
		go func() {
			defer close(drained)
			for range ch {
			}
		}()
		waitFor(t, "channel closed after cancel", drained)
	})
}
