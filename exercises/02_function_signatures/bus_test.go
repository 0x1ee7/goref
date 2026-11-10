package signatures_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"goref/exercises/02_function_signatures"
)

// recordTo returns a Handler that appends "name:[payload]" to *tr.
func recordTo(tr *[]string, name string) signatures.Handler {
	return func(_ context.Context, payload ...any) error {
		*tr = append(*tr, fmt.Sprintf("%s:%v", name, payload))
		return nil
	}
}

func mustSubscribe(t *testing.T, bus *signatures.Bus, topic string, h signatures.Handler) func() {
	t.Helper()
	cancel, err := bus.Subscribe(topic, h)
	if err != nil {
		t.Fatalf("Subscribe(%q): unexpected error: %v", topic, err)
	}
	if cancel == nil {
		t.Fatalf("Subscribe(%q) returned nil cancel without error", topic)
	}
	return cancel
}

// publish runs Publish with a deadline so that a deadlocking implementation
// fails the test instead of hanging the whole package.
func publish(t *testing.T, ctx context.Context, bus *signatures.Bus, topic string, payload ...any) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- bus.Publish(ctx, topic, payload...) }()
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatalf("Publish(%q) did not return within 5s (deadlock?)", topic)
		return nil
	}
}

type collector struct{ got []string }

func (c *collector) Handle(_ context.Context, payload ...any) error {
	c.got = append(c.got, fmt.Sprintf("%v", payload))
	return nil
}

func TestPart5Delivery(t *testing.T) {
	t.Run("subscription order and variadic payload", func(t *testing.T) {
		var bus signatures.Bus
		var got []string
		mustSubscribe(t, &bus, "orders", recordTo(&got, "first"))
		mustSubscribe(t, &bus, "orders", recordTo(&got, "second"))
		if err := publish(t, t.Context(), &bus, "orders", 1, "x"); err != nil {
			t.Fatalf("Publish unexpected error: %v", err)
		}
		want := []string{"first:[1 x]", "second:[1 x]"}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("deliveries mismatch (-want +got):\n%s", diff)
		}
	})
	t.Run("spread slice", func(t *testing.T) {
		var bus signatures.Bus
		var got []string
		mustSubscribe(t, &bus, "t", recordTo(&got, "h"))
		args := []any{"x", 2}
		if err := publish(t, t.Context(), &bus, "t", args...); err != nil {
			t.Fatalf("Publish unexpected error: %v", err)
		}
		if want := []string{"h:[x 2]"}; !cmp.Equal(want, got) {
			t.Errorf("deliveries = %q, want %q", got, want)
		}
	})
	t.Run("no payload delivers an empty slice", func(t *testing.T) {
		var bus signatures.Bus
		var got []string
		mustSubscribe(t, &bus, "t", recordTo(&got, "h"))
		if err := publish(t, t.Context(), &bus, "t"); err != nil {
			t.Fatalf("Publish unexpected error: %v", err)
		}
		if want := []string{"h:[]"}; !cmp.Equal(want, got) {
			t.Errorf("deliveries = %q, want %q", got, want)
		}
	})
	t.Run("topics are isolated", func(t *testing.T) {
		var bus signatures.Bus
		var got []string
		mustSubscribe(t, &bus, "orders", recordTo(&got, "h"))
		if err := publish(t, t.Context(), &bus, "billing"); err != nil {
			t.Fatalf("Publish(billing) unexpected error: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("subscriber to orders received billing publish: %q", got)
		}
		if err := publish(t, t.Context(), &bus, "orders"); err != nil {
			t.Fatalf("Publish(orders) unexpected error: %v", err)
		}
		if want := []string{"h:[]"}; !cmp.Equal(want, got) {
			t.Errorf("deliveries = %q, want %q", got, want)
		}
	})
	t.Run("publish with no subscribers returns nil", func(t *testing.T) {
		var bus signatures.Bus
		if err := publish(t, t.Context(), &bus, "nobody-listens", 1); err != nil {
			t.Fatalf("Publish with no subscribers: unexpected error: %v", err)
		}
		var got []string
		mustSubscribe(t, &bus, "nobody-listens", recordTo(&got, "late"))
		if err := publish(t, t.Context(), &bus, "nobody-listens", 2); err != nil {
			t.Fatalf("Publish unexpected error: %v", err)
		}
		if want := []string{"late:[2]"}; !cmp.Equal(want, got) {
			t.Errorf("deliveries = %q, want %q", got, want)
		}
	})
	t.Run("method value handler", func(t *testing.T) {
		var bus signatures.Bus
		c := &collector{}
		mustSubscribe(t, &bus, "t", c.Handle)
		if err := publish(t, t.Context(), &bus, "t", "a", 1); err != nil {
			t.Fatalf("Publish unexpected error: %v", err)
		}
		if want := []string{"[a 1]"}; !cmp.Equal(want, c.got) {
			t.Errorf("collector got %q, want %q", c.got, want)
		}
	})
}

func TestPart5PayloadIsolation(t *testing.T) {
	t.Run("handler cannot change another handler's payload", func(t *testing.T) {
		var bus signatures.Bus
		var got []string
		mustSubscribe(t, &bus, "t", func(_ context.Context, payload ...any) error {
			payload[0] = "changed"
			return nil
		})
		mustSubscribe(t, &bus, "t", recordTo(&got, "second"))
		if err := publish(t, t.Context(), &bus, "t", "orig"); err != nil {
			t.Fatalf("Publish unexpected error: %v", err)
		}
		if want := []string{"second:[orig]"}; !cmp.Equal(want, got) {
			t.Errorf("second handler saw %q, want %q", got, want)
		}
	})
	t.Run("caller slice is not changed by handlers", func(t *testing.T) {
		var bus signatures.Bus
		var calls int
		mustSubscribe(t, &bus, "t", func(_ context.Context, payload ...any) error {
			calls++
			payload[0] = "changed"
			return nil
		})
		args := []any{"orig"}
		if err := publish(t, t.Context(), &bus, "t", args...); err != nil {
			t.Fatalf("Publish unexpected error: %v", err)
		}
		if calls != 1 {
			t.Fatalf("handler ran %d times, want 1", calls)
		}
		if args[0] != "orig" {
			t.Errorf("caller's slice changed to %v", args[0])
		}
	})
}

func TestPart5Errors(t *testing.T) {
	t.Run("all handlers run and errors are joined", func(t *testing.T) {
		var bus signatures.Bus
		var got []string
		errA := errors.New("a failed")
		errB := errors.New("b failed")
		mustSubscribe(t, &bus, "t", func(_ context.Context, _ ...any) error {
			got = append(got, "h1")
			return errA
		})
		mustSubscribe(t, &bus, "t", recordTo(&got, "h2"))
		mustSubscribe(t, &bus, "t", func(_ context.Context, _ ...any) error {
			got = append(got, "h3")
			return errB
		})
		err := publish(t, t.Context(), &bus, "t")
		if !errors.Is(err, errA) || !errors.Is(err, errB) {
			t.Fatalf("Publish error = %v, want both errA and errB joined", err)
		}
		if want := []string{"h1", "h2:[]", "h3"}; !cmp.Equal(want, got) {
			t.Errorf("handlers ran %q, want %q", got, want)
		}
	})
	t.Run("nil when every handler succeeds", func(t *testing.T) {
		var bus signatures.Bus
		var got []string
		mustSubscribe(t, &bus, "t", recordTo(&got, "h"))
		if err := publish(t, t.Context(), &bus, "t"); err != nil {
			t.Fatalf("Publish error = %v, want nil", err)
		}
		if len(got) != 1 {
			t.Errorf("handler ran %d times, want 1", len(got))
		}
	})
}

func TestPart5Cancel(t *testing.T) {
	t.Run("cancel removes only its own registration", func(t *testing.T) {
		var bus signatures.Bus
		var got []string
		h := recordTo(&got, "h")
		cancelFirst := mustSubscribe(t, &bus, "t", h)
		mustSubscribe(t, &bus, "t", h)
		cancelFirst()
		if err := publish(t, t.Context(), &bus, "t"); err != nil {
			t.Fatalf("Publish unexpected error: %v", err)
		}
		if want := []string{"h:[]"}; !cmp.Equal(want, got) {
			t.Errorf("deliveries = %q, want %q", got, want)
		}
	})
	t.Run("cancel is idempotent", func(t *testing.T) {
		var bus signatures.Bus
		var got []string
		cancel := mustSubscribe(t, &bus, "t", recordTo(&got, "gone"))
		mustSubscribe(t, &bus, "t", recordTo(&got, "kept"))
		cancel()
		cancel()
		if err := publish(t, t.Context(), &bus, "t"); err != nil {
			t.Fatalf("Publish unexpected error: %v", err)
		}
		if want := []string{"kept:[]"}; !cmp.Equal(want, got) {
			t.Errorf("deliveries = %q, want %q", got, want)
		}
	})
	t.Run("handler cancelled during publish still runs in that publish", func(t *testing.T) {
		var bus signatures.Bus
		var got []string
		var cancel2 func()
		mustSubscribe(t, &bus, "t", func(_ context.Context, _ ...any) error {
			got = append(got, "h1")
			cancel2()
			return nil
		})
		cancel2 = mustSubscribe(t, &bus, "t", recordTo(&got, "h2"))
		if err := publish(t, t.Context(), &bus, "t"); err != nil {
			t.Fatalf("first Publish unexpected error: %v", err)
		}
		if want := []string{"h1", "h2:[]"}; !cmp.Equal(want, got) {
			t.Fatalf("first publish ran %q, want %q", got, want)
		}
		got = nil
		if err := publish(t, t.Context(), &bus, "t"); err != nil {
			t.Fatalf("second Publish unexpected error: %v", err)
		}
		if want := []string{"h1"}; !cmp.Equal(want, got) {
			t.Errorf("second publish ran %q, want %q", got, want)
		}
	})
}

func TestPart5Snapshot(t *testing.T) {
	t.Run("handler subscribed during publish waits for the next publish", func(t *testing.T) {
		var bus signatures.Bus
		var got []string
		subscribed := false
		mustSubscribeLater := func() {
			mustSubscribe(t, &bus, "t", recordTo(&got, "h2"))
		}
		mustSubscribe(t, &bus, "t", func(_ context.Context, _ ...any) error {
			got = append(got, "h1")
			if !subscribed {
				subscribed = true
				mustSubscribeLater()
			}
			return nil
		})
		if err := publish(t, t.Context(), &bus, "t"); err != nil {
			t.Fatalf("first Publish unexpected error: %v", err)
		}
		if want := []string{"h1"}; !cmp.Equal(want, got) {
			t.Fatalf("first publish ran %q, want %q", got, want)
		}
		got = nil
		if err := publish(t, t.Context(), &bus, "t"); err != nil {
			t.Fatalf("second Publish unexpected error: %v", err)
		}
		if want := []string{"h1", "h2:[]"}; !cmp.Equal(want, got) {
			t.Errorf("second publish ran %q, want %q", got, want)
		}
	})
}

func TestPart5Context(t *testing.T) {
	t.Run("remaining handlers are skipped once ctx is done", func(t *testing.T) {
		var bus signatures.Bus
		var got []string
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		mustSubscribe(t, &bus, "t", func(_ context.Context, _ ...any) error {
			got = append(got, "h1")
			cancel()
			return nil
		})
		mustSubscribe(t, &bus, "t", recordTo(&got, "h2"))
		err := publish(t, ctx, &bus, "t")
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Publish error = %v, want errors.Is context.Canceled", err)
		}
		if want := []string{"h1"}; !cmp.Equal(want, got) {
			t.Errorf("handlers ran %q, want %q", got, want)
		}
	})
	t.Run("already-cancelled ctx runs nothing", func(t *testing.T) {
		var bus signatures.Bus
		var got []string
		mustSubscribe(t, &bus, "t", recordTo(&got, "h"))
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		err := publish(t, ctx, &bus, "t")
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Publish error = %v, want errors.Is context.Canceled", err)
		}
		if len(got) != 0 {
			t.Errorf("handler ran under a cancelled context: %q", got)
		}
	})
}

func TestPart5Subscribe(t *testing.T) {
	t.Run("nil handler is rejected", func(t *testing.T) {
		var bus signatures.Bus
		_, err := bus.Subscribe("t", nil)
		if !errors.Is(err, signatures.ErrNilHandler) {
			t.Fatalf("Subscribe(nil) error = %v, want errors.Is ErrNilHandler", err)
		}
		var got []string
		mustSubscribe(t, &bus, "t", recordTo(&got, "h"))
		if err := publish(t, t.Context(), &bus, "t"); err != nil {
			t.Fatalf("Publish unexpected error: %v", err)
		}
		if want := []string{"h:[]"}; !cmp.Equal(want, got) {
			t.Errorf("deliveries = %q, want %q", got, want)
		}
	})
}
