package typed_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/go-cmp/cmp"

	"goref/exercises/11_types_and_generics"
)

type userCreated struct{ Email string }

type orderPlaced struct{ Total int }

type ping struct{}

// record returns a handler that appends label to *log.
func record[E any](log *[]string, label string) typed.Handler[E] {
	return func(_ context.Context, _ E) error {
		*log = append(*log, label)
		return nil
	}
}

func TestPart6DeliveryOrder(t *testing.T) {
	var bus typed.Bus
	var log []string
	for _, label := range []string{"a", "b", "c"} {
		typed.Subscribe(&bus, record[userCreated](&log, label))
	}
	n, err := typed.Publish(t.Context(), &bus, userCreated{Email: "ada@example.com"})
	if err != nil || n != 3 {
		t.Fatalf("Publish = (%d, %v), want (3, nil)", n, err)
	}
	if diff := cmp.Diff([]string{"a", "b", "c"}, log); diff != "" {
		t.Errorf("delivery order mismatch (-want +got):\n%s", diff)
	}
}

func TestPart6TypeIsolation(t *testing.T) {
	var bus typed.Bus
	var log []string
	typed.Subscribe(&bus, record[userCreated](&log, "user"))

	if n, err := typed.Publish(t.Context(), &bus, userCreated{}); err != nil || n != 1 {
		t.Fatalf("Publish(userCreated) = (%d, %v), want (1, nil)", n, err)
	}
	if n, err := typed.Publish(t.Context(), &bus, orderPlaced{Total: 5}); err != nil || n != 0 {
		t.Errorf("Publish(orderPlaced) = (%d, %v), want (0, nil)", n, err)
	}
	if diff := cmp.Diff([]string{"user"}, log); diff != "" {
		t.Errorf("handlers called (-want +got):\n%s", diff)
	}
}

func TestPart6InterfaceTypeArgumentsAreKeyedSeparately(t *testing.T) {
	// Routing uses the exact type argument: a handler for `any` does not see
	// events published as userCreated, and vice versa.
	var bus typed.Bus
	var log []string
	typed.Subscribe(&bus, record[userCreated](&log, "concrete"))
	typed.Subscribe(&bus, record[any](&log, "any"))

	if n, err := typed.Publish(t.Context(), &bus, userCreated{}); err != nil || n != 1 {
		t.Fatalf("Publish(userCreated) = (%d, %v), want (1, nil)", n, err)
	}
	if n, err := typed.Publish[any](t.Context(), &bus, userCreated{}); err != nil || n != 1 {
		t.Fatalf("Publish[any] = (%d, %v), want (1, nil)", n, err)
	}
	if diff := cmp.Diff([]string{"concrete", "any"}, log); diff != "" {
		t.Errorf("handlers called (-want +got):\n%s", diff)
	}
}

func TestPart6CancelRemovesOnlyItsRegistration(t *testing.T) {
	var bus typed.Bus
	var log []string
	cancelFirst := typed.Subscribe(&bus, record[orderPlaced](&log, "first"))
	typed.Subscribe(&bus, record[orderPlaced](&log, "second"))

	if n, err := typed.Publish(t.Context(), &bus, orderPlaced{}); err != nil || n != 2 {
		t.Fatalf("Publish before cancel = (%d, %v), want (2, nil)", n, err)
	}
	cancelFirst()
	cancelFirst() // idempotent: must not remove anything else
	if n, err := typed.Publish(t.Context(), &bus, orderPlaced{}); err != nil || n != 1 {
		t.Errorf("Publish after cancel = (%d, %v), want (1, nil)", n, err)
	}
	if diff := cmp.Diff([]string{"first", "second", "second"}, log); diff != "" {
		t.Errorf("handlers called (-want +got):\n%s", diff)
	}
}

func TestPart6SameHandlerTwiceIsTwoRegistrations(t *testing.T) {
	var bus typed.Bus
	var calls atomic.Int64
	h := func(context.Context, ping) error {
		calls.Add(1)
		return nil
	}
	cancelOne := typed.Subscribe(&bus, typed.Handler[ping](h))
	typed.Subscribe(&bus, typed.Handler[ping](h))

	if n, err := typed.Publish(t.Context(), &bus, ping{}); err != nil || n != 2 {
		t.Fatalf("Publish with two registrations = (%d, %v), want (2, nil)", n, err)
	}
	cancelOne()
	if n, err := typed.Publish(t.Context(), &bus, ping{}); err != nil || n != 1 {
		t.Errorf("Publish after one cancel = (%d, %v), want (1, nil)", n, err)
	}
	if got := calls.Load(); got != 3 {
		t.Errorf("handler calls = %d, want 3", got)
	}
}

func TestPart6HandlerErrorsAreJoined(t *testing.T) {
	var bus typed.Bus
	errA := errors.New("A failed")
	errB := errors.New("B failed")
	var log []string
	typed.Subscribe(&bus, func(context.Context, orderPlaced) error { return errA })
	typed.Subscribe(&bus, record[orderPlaced](&log, "ok"))
	typed.Subscribe(&bus, func(context.Context, orderPlaced) error { return errB })

	n, err := typed.Publish(t.Context(), &bus, orderPlaced{})
	if n != 3 {
		t.Errorf("invoked = %d, want 3: a failing handler must not stop dispatch", n)
	}
	if !errors.Is(err, errA) || !errors.Is(err, errB) {
		t.Errorf("Publish error = %v, want both errA and errB joined", err)
	}
	if diff := cmp.Diff([]string{"ok"}, log); diff != "" {
		t.Errorf("successful handler calls (-want +got):\n%s", diff)
	}
}

func TestPart6SnapshotSemantics(t *testing.T) {
	t.Run("subscribe during dispatch misses current event", func(t *testing.T) {
		var bus typed.Bus
		var log []string
		typed.Subscribe(&bus, func(ctx context.Context, e ping) error {
			log = append(log, "A")
			if len(log) == 1 {
				typed.Subscribe(&bus, record[ping](&log, "B"))
			}
			return nil
		})

		if n, err := typed.Publish(t.Context(), &bus, ping{}); err != nil || n != 1 {
			t.Fatalf("first Publish = (%d, %v), want (1, nil)", n, err)
		}
		if n, err := typed.Publish(t.Context(), &bus, ping{}); err != nil || n != 2 {
			t.Errorf("second Publish = (%d, %v), want (2, nil)", n, err)
		}
		if diff := cmp.Diff([]string{"A", "A", "B"}, log); diff != "" {
			t.Errorf("calls (-want +got):\n%s", diff)
		}
	})

	t.Run("cancel during dispatch skips the cancelled handler", func(t *testing.T) {
		var bus typed.Bus
		var log []string
		var cancelB func()
		typed.Subscribe(&bus, func(context.Context, ping) error {
			log = append(log, "A")
			cancelB()
			return nil
		})
		cancelB = typed.Subscribe(&bus, record[ping](&log, "B"))

		if n, err := typed.Publish(t.Context(), &bus, ping{}); err != nil || n != 1 {
			t.Fatalf("Publish = (%d, %v), want (1, nil): B was cancelled mid-dispatch", n, err)
		}
		if n, err := typed.Publish(t.Context(), &bus, ping{}); err != nil || n != 1 {
			t.Errorf("second Publish = (%d, %v), want (1, nil)", n, err)
		}
		if diff := cmp.Diff([]string{"A", "A"}, log); diff != "" {
			t.Errorf("calls (-want +got):\n%s", diff)
		}
	})
}

type traceKey struct{}

func TestPart6ContextHandling(t *testing.T) {
	t.Run("handlers receive the publish context", func(t *testing.T) {
		var bus typed.Bus
		var got []string
		typed.Subscribe(&bus, func(ctx context.Context, e ping) error {
			v, _ := ctx.Value(traceKey{}).(string)
			got = append(got, v)
			return nil
		})
		ctx := context.WithValue(t.Context(), traceKey{}, "trace-1")
		if n, err := typed.Publish(ctx, &bus, ping{}); err != nil || n != 1 {
			t.Fatalf("Publish = (%d, %v), want (1, nil)", n, err)
		}
		if diff := cmp.Diff([]string{"trace-1"}, got); diff != "" {
			t.Errorf("context values seen (-want +got):\n%s", diff)
		}
	})

	t.Run("already cancelled context invokes nothing", func(t *testing.T) {
		var bus typed.Bus
		var calls atomic.Int64
		typed.Subscribe(&bus, func(context.Context, ping) error {
			calls.Add(1)
			return nil
		})
		if _, err := typed.Publish(t.Context(), &bus, ping{}); err != nil {
			t.Fatalf("Publish with live context: %v", err)
		}
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		n, err := typed.Publish(ctx, &bus, ping{})
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Publish with cancelled ctx error = %v, want context.Canceled", err)
		}
		if n != 0 || calls.Load() != 1 {
			t.Errorf("invoked=%d total calls=%d, want 0 and 1", n, calls.Load())
		}
	})

	t.Run("cancellation mid-dispatch stops later handlers", func(t *testing.T) {
		var bus typed.Bus
		var log []string
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		typed.Subscribe(&bus, func(context.Context, ping) error {
			log = append(log, "first")
			cancel()
			return nil
		})
		typed.Subscribe(&bus, record[ping](&log, "second"))

		n, err := typed.Publish(ctx, &bus, ping{})
		if n != 1 || !errors.Is(err, context.Canceled) {
			t.Errorf("Publish = (%d, %v), want (1, context.Canceled)", n, err)
		}
		if diff := cmp.Diff([]string{"first"}, log); diff != "" {
			t.Errorf("calls (-want +got):\n%s", diff)
		}
	})
}

func TestPart6ZeroBusConcurrentUse(t *testing.T) {
	var bus typed.Bus
	var handled atomic.Int64
	typed.Subscribe(&bus, func(context.Context, ping) error {
		handled.Add(1)
		return nil
	})

	var invoked atomic.Int64
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			n, err := typed.Publish(t.Context(), &bus, ping{})
			if err != nil {
				t.Errorf("Publish: %v", err)
			}
			invoked.Add(int64(n))
		})
	}
	// Concurrent subscribe/cancel of an unrelated event type must not disturb routing.
	for i := range 10 {
		wg.Go(func() {
			cancel := typed.Subscribe(&bus, func(context.Context, orderPlaced) error {
				return fmt.Errorf("unexpected delivery %d", i)
			})
			cancel()
		})
	}
	wg.Wait()

	if invoked.Load() != 20 {
		t.Errorf("total invoked across publishes = %d, want 20", invoked.Load())
	}
	if handled.Load() != invoked.Load() {
		t.Errorf("handler calls = %d, invoked = %d; they must agree", handled.Load(), invoked.Load())
	}
}
