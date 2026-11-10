package interfaces_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/go-cmp/cmp"
	"goref/exercises/08_interfaces"
)

// recorder is a Notifier that remembers events and returns a fixed error.
type recorder struct {
	err    error
	mu     sync.Mutex
	events []interfaces.Event
}

func (r *recorder) Notify(_ context.Context, ev interfaces.Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, ev)
	return r.err
}

func (r *recorder) calls() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.events)
}

func TestPart5NotifierFunc(t *testing.T) {
	t.Run("adapter forwards the event and the error", func(t *testing.T) {
		sentinel := errors.New("channel down")
		var got interfaces.Event
		f := interfaces.NotifierFunc(func(_ context.Context, ev interfaces.Event) error {
			got = ev
			return sentinel
		})
		ev := interfaces.Event{Kind: "deploy", Message: "v1"}
		err := f.Notify(t.Context(), ev)
		if !errors.Is(err, sentinel) {
			t.Errorf("Notify err = %v, want sentinel", err)
		}
		if got != ev {
			t.Errorf("function received %+v, want %+v", got, ev)
		}
	})
}

func TestPart5Dispatch(t *testing.T) {
	ev := interfaces.Event{Kind: "deploy", Message: "v2"}

	t.Run("delivers the event to every notifier", func(t *testing.T) {
		a, b := &recorder{}, &recorder{}
		if err := interfaces.NewDispatcher([]interfaces.Notifier{a, b}).Dispatch(t.Context(), ev); err != nil {
			t.Fatalf("Dispatch: %v", err)
		}
		for name, r := range map[string]*recorder{"a": a, "b": b} {
			if diff := cmp.Diff([]interfaces.Event{ev}, r.events); diff != "" {
				t.Errorf("notifier %s events (-want +got):\n%s", name, diff)
			}
		}
	})

	t.Run("all succeeding returns nil", func(t *testing.T) {
		a, b := &recorder{}, &recorder{}
		err := interfaces.NewDispatcher([]interfaces.Notifier{a, b}).Dispatch(t.Context(), ev)
		if err != nil {
			t.Errorf("Dispatch = %v, want nil", err)
		}
		if a.calls() != 1 || b.calls() != 1 {
			t.Errorf("calls = %d, %d; want 1, 1", a.calls(), b.calls())
		}
	})

	t.Run("failures are joined in registration order", func(t *testing.T) {
		e2 := errors.New("second failed")
		e3 := errors.New("third failed")
		ns := []interfaces.Notifier{&recorder{}, &recorder{err: e2}, &recorder{err: e3}}
		err := interfaces.NewDispatcher(ns).Dispatch(t.Context(), ev)
		if !errors.Is(err, e2) || !errors.Is(err, e3) {
			t.Fatalf("Dispatch err = %v, want both failures", err)
		}
		msg := err.Error()
		i2, i3 := strings.Index(msg, "second failed"), strings.Index(msg, "third failed")
		if i2 < 0 || i3 < 0 || i2 > i3 {
			t.Errorf("error text %q: want %q before %q", msg, "second failed", "third failed")
		}
	})

	t.Run("notifier slice is copied at construction", func(t *testing.T) {
		a, b := &recorder{}, &recorder{}
		list := []interfaces.Notifier{a}
		d := interfaces.NewDispatcher(list)
		list[0] = b // caller mutates its slice afterwards
		if err := d.Dispatch(t.Context(), ev); err != nil {
			t.Fatalf("Dispatch: %v", err)
		}
		if a.calls() != 1 || b.calls() != 0 {
			t.Errorf("calls a=%d b=%d; want a=1 b=0", a.calls(), b.calls())
		}
	})

	t.Run("canceled context reaches notifiers", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		watch := interfaces.NotifierFunc(func(ctx context.Context, _ interfaces.Event) error {
			return ctx.Err()
		})
		err := interfaces.NewDispatcher([]interfaces.Notifier{watch}).Dispatch(ctx, ev)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Dispatch err = %v, want context.Canceled", err)
		}
	})

	t.Run("default is unlimited concurrency", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			var inFlight, peak atomic.Int64
			slow := interfaces.NotifierFunc(func(context.Context, interfaces.Event) error {
				n := inFlight.Add(1)
				if n > peak.Load() {
					peak.Store(n)
				}
				time.Sleep(time.Second)
				inFlight.Add(-1)
				return nil
			})
			d := interfaces.NewDispatcher([]interfaces.Notifier{slow, slow, slow, slow})
			start := time.Now()
			if err := d.Dispatch(t.Context(), ev); err != nil {
				t.Fatalf("Dispatch: %v", err)
			}
			if got := peak.Load(); got != 4 {
				t.Errorf("peak in-flight = %d, want 4", got)
			}
			if got := time.Since(start); got != time.Second {
				t.Errorf("elapsed = %v, want 1s", got)
			}
		})
	})

	t.Run("WithConcurrency caps in-flight notifiers", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			var inFlight, peak atomic.Int64
			slow := interfaces.NotifierFunc(func(context.Context, interfaces.Event) error {
				n := inFlight.Add(1)
				if n > peak.Load() {
					peak.Store(n)
				}
				time.Sleep(time.Second)
				inFlight.Add(-1)
				return nil
			})
			d := interfaces.NewDispatcher([]interfaces.Notifier{slow, slow, slow, slow}, interfaces.WithConcurrency(2))
			start := time.Now()
			if err := d.Dispatch(t.Context(), ev); err != nil {
				t.Fatalf("Dispatch: %v", err)
			}
			if got := peak.Load(); got != 2 {
				t.Errorf("peak in-flight = %d, want 2", got)
			}
			if got := time.Since(start); got != 2*time.Second {
				t.Errorf("elapsed = %v, want 2s (two waves of two)", got)
			}
		})
	})

	t.Run("cancellation while waiting for a slot returns", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			block := interfaces.NotifierFunc(func(ctx context.Context, _ interfaces.Event) error {
				<-ctx.Done()
				return ctx.Err()
			})
			time.AfterFunc(time.Second, cancel)
			d := interfaces.NewDispatcher([]interfaces.Notifier{block, block}, interfaces.WithConcurrency(1))
			err := d.Dispatch(ctx, ev)
			if !errors.Is(err, context.Canceled) {
				t.Errorf("Dispatch err = %v, want context.Canceled", err)
			}
		})
	})
}
