package ctxflow_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/go-cmp/cmp"
	"goref/exercises/15_context_and_cancellation"
)

func newBroker(t *testing.T, patience time.Duration) *ctxflow.Broker[string] {
	t.Helper()
	b, err := ctxflow.NewBroker[string](patience)
	if err != nil {
		t.Fatalf("NewBroker error = %v", err)
	}
	if b == nil {
		t.Fatal("NewBroker returned a nil broker")
	}
	return b
}

func subscribe(t *testing.T, b *ctxflow.Broker[string], buffer int) *ctxflow.Subscription[string] {
	t.Helper()
	s, err := b.Subscribe(buffer)
	if err != nil {
		t.Fatalf("Subscribe(%d) error = %v", buffer, err)
	}
	if s == nil {
		t.Fatal("Subscribe returned a nil subscription")
	}
	return s
}

func TestPart6Broker(t *testing.T) {
	t.Run("every subscriber receives every message in publish order", func(t *testing.T) {
		b := newBroker(t, time.Second)
		subs := []*ctxflow.Subscription[string]{subscribe(t, b, 10), subscribe(t, b, 10), subscribe(t, b, 10)}
		msgs := []string{"a", "b", "c", "d", "e"}
		for _, m := range msgs {
			if err := b.Publish(t.Context(), m); err != nil {
				t.Fatalf("Publish(%q) error = %v", m, err)
			}
		}
		for i, s := range subs {
			var got []string
			for range msgs {
				m, ok := recv(t, s.C)
				if !ok {
					break
				}
				got = append(got, m)
			}
			if diff := cmp.Diff(msgs, got); diff != "" {
				t.Errorf("subscriber %d mismatch (-want +got):\n%s", i, diff)
			}
		}
	})

	t.Run("Close leaves buffered messages readable, then closes the channel", func(t *testing.T) {
		b := newBroker(t, time.Second)
		s := subscribe(t, b, 3)
		for _, m := range []string{"x", "y"} {
			if err := b.Publish(t.Context(), m); err != nil {
				t.Fatalf("Publish error = %v", err)
			}
		}
		b.Close()
		for _, want := range []string{"x", "y"} {
			if m, ok := recv(t, s.C); !ok || m != want {
				t.Errorf("receive = (%q, %v), want (%q, true)", m, ok, want)
			}
		}
		if _, ok := recv(t, s.C); ok {
			t.Errorf("channel still open after buffered messages were read")
		}
	})

	t.Run("unsubscribe stops delivery, closes the channel and is idempotent", func(t *testing.T) {
		b := newBroker(t, time.Second)
		gone := subscribe(t, b, 4)
		kept := subscribe(t, b, 4)
		gone.Unsubscribe()
		gone.Unsubscribe()
		if err := b.Publish(t.Context(), "after"); err != nil {
			t.Fatalf("Publish error = %v", err)
		}
		if _, ok := recv(t, gone.C); ok {
			t.Errorf("unsubscribed channel delivered a message or stayed open")
		}
		if m, ok := recv(t, kept.C); !ok || m != "after" {
			t.Errorf("remaining subscriber got (%q, %v), want (after, true)", m, ok)
		}
	})

	t.Run("a full subscriber drops the message once patience runs out", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			b := newBroker(t, 10*time.Millisecond)
			slow := subscribe(t, b, 1)
			fast := subscribe(t, b, 10)
			for _, m := range []string{"1", "2", "3"} {
				if err := b.Publish(context.Background(), m); err != nil {
					t.Fatalf("Publish(%q) error = %v", m, err)
				}
			}
			if d := slow.Dropped(); d != 2 {
				t.Errorf("slow.Dropped() = %d, want 2", d)
			}
			if d := fast.Dropped(); d != 0 {
				t.Errorf("fast.Dropped() = %d, want 0", d)
			}
			var got []string
			for range 3 {
				m, ok := recv(t, fast.C)
				if !ok {
					break
				}
				got = append(got, m)
			}
			if diff := cmp.Diff([]string{"1", "2", "3"}, got); diff != "" {
				t.Errorf("fast subscriber mismatch (-want +got):\n%s", diff)
			}
		})
	})

	t.Run("zero patience drops at once instead of waiting", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			b := newBroker(t, 0)
			s := subscribe(t, b, 1)
			start := time.Now()
			for _, m := range []string{"1", "2"} {
				if err := b.Publish(context.Background(), m); err != nil {
					t.Fatalf("Publish error = %v", err)
				}
			}
			if elapsed := time.Since(start); elapsed != 0 {
				t.Errorf("elapsed = %v, want 0", elapsed)
			}
			if d := s.Dropped(); d != 1 {
				t.Errorf("Dropped() = %d, want 1", d)
			}
		})
	})

	t.Run("publish returns the context cause when it ends while waiting", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			b := newBroker(t, time.Hour)
			subscribe(t, b, 0)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
			defer cancel()
			if err := b.Publish(ctx, "stuck"); !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("Publish error = %v, want DeadlineExceeded", err)
			}
		})
	})

	t.Run("unsubscribing while a publish waits neither panics nor fails the publish", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			b := newBroker(t, 50*time.Millisecond)
			s := subscribe(t, b, 0)
			done := make(chan error, 1)
			go func() { done <- b.Publish(context.Background(), "race") }()
			time.Sleep(10 * time.Millisecond)
			s.Unsubscribe()
			if err, ok := recv(t, (<-chan error)(done)); ok && err != nil {
				t.Errorf("Publish error = %v, want nil", err)
			}
		})
	})

	t.Run("concurrent publishers lose nothing when buffers have room", func(t *testing.T) {
		const publishers, each = 4, 25
		b := newBroker(t, time.Second)
		s := subscribe(t, b, publishers*each)
		var wg sync.WaitGroup
		for range publishers {
			wg.Go(func() {
				for range each {
					if err := b.Publish(context.Background(), "m"); err != nil {
						t.Errorf("Publish error = %v", err)
						return
					}
				}
			})
		}
		wg.Wait()
		for i := range publishers * each {
			if _, ok := recv(t, s.C); !ok {
				t.Fatalf("message %d missing", i)
			}
		}
		if d := s.Dropped(); d != 0 {
			t.Errorf("Dropped() = %d, want 0", d)
		}
	})

	t.Run("operations after Close fail with ErrClosed and Close is idempotent", func(t *testing.T) {
		b := newBroker(t, time.Second)
		b.Close()
		b.Close()
		if _, err := b.Subscribe(1); !errors.Is(err, ctxflow.ErrClosed) {
			t.Errorf("Subscribe after Close error = %v, want ErrClosed", err)
		}
		if err := b.Publish(t.Context(), "late"); !errors.Is(err, ctxflow.ErrClosed) {
			t.Errorf("Publish after Close error = %v, want ErrClosed", err)
		}
	})

	t.Run("negative patience or buffer is rejected", func(t *testing.T) {
		if _, err := ctxflow.NewBroker[string](-time.Second); !errors.Is(err, ctxflow.ErrInvalidArgument) {
			t.Errorf("NewBroker(-1s) error = %v, want ErrInvalidArgument", err)
		}
		b := newBroker(t, time.Second)
		if _, err := b.Subscribe(-1); !errors.Is(err, ctxflow.ErrInvalidArgument) {
			t.Errorf("Subscribe(-1) error = %v, want ErrInvalidArgument", err)
		}
	})
}
