package funcs_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"

	funcs "goref/exercises/04_functions_and_closures"
)

func TestPart6HandlerFunc(t *testing.T) {
	t.Run("Handle calls the function and returns its error", func(t *testing.T) {
		errBoom := errors.New("boom")
		var got funcs.Event
		var h funcs.Handler = funcs.HandlerFunc(func(ctx context.Context, e funcs.Event) error {
			got = e
			return errBoom
		})
		want := funcs.Event{Kind: "k", Payload: "p"}
		if err := h.Handle(context.Background(), want); err != errBoom {
			t.Errorf("Handle = %v, want errBoom unchanged", err)
		}
		if got != want {
			t.Errorf("handler received %+v, want %+v", got, want)
		}
	})

	t.Run("a method value adapts to HandlerFunc", func(t *testing.T) {
		rec := &funcs.Recorder{}
		h := funcs.HandlerFunc(rec.Record)
		e := funcs.Event{Kind: "tick", Payload: "1"}
		if err := h.Handle(context.Background(), e); err != nil {
			t.Fatalf("Handle = %v, want nil", err)
		}
		if diff := cmp.Diff([]funcs.Event{e}, rec.Events()); diff != "" {
			t.Errorf("recorded events mismatch (-want +got):\n%s", diff)
		}
	})
}

func TestPart6Fanout(t *testing.T) {
	ctx := context.Background()

	t.Run("delivers to every handler in order and joins errors", func(t *testing.T) {
		var order []string
		errFirst := errors.New("first failed")
		errThird := errors.New("third failed")
		h := funcs.Fanout(
			funcs.HandlerFunc(func(context.Context, funcs.Event) error { order = append(order, "a"); return errFirst }),
			funcs.HandlerFunc(func(context.Context, funcs.Event) error { order = append(order, "b"); return nil }),
			funcs.HandlerFunc(func(context.Context, funcs.Event) error { order = append(order, "c"); return errThird }),
		)
		err := h.Handle(ctx, funcs.Event{Kind: "x"})
		if diff := cmp.Diff([]string{"a", "b", "c"}, order); diff != "" {
			t.Errorf("delivery order mismatch (-want +got):\n%s", diff)
		}
		if !errors.Is(err, errFirst) || !errors.Is(err, errThird) {
			t.Errorf("err = %v, want both handler errors joined", err)
		}
	})

	t.Run("nil when every handler succeeds", func(t *testing.T) {
		var calls int
		h := funcs.Fanout(funcs.HandlerFunc(func(context.Context, funcs.Event) error { calls++; return nil }))
		if err := h.Handle(ctx, funcs.Event{Kind: "x"}); err != nil {
			t.Errorf("Handle = %v, want nil", err)
		}
		if calls != 1 {
			t.Errorf("handler called %d times, want 1", calls)
		}
	})
}

func TestPart6Only(t *testing.T) {
	ctx := context.Background()

	t.Run("forwards only events of the matching kind", func(t *testing.T) {
		var got []string
		h := funcs.Only("order.created", funcs.HandlerFunc(func(_ context.Context, e funcs.Event) error {
			got = append(got, e.Payload)
			return nil
		}))
		if err := h.Handle(ctx, funcs.Event{Kind: "order.created", Payload: "1"}); err != nil {
			t.Errorf("matching Handle = %v, want nil", err)
		}
		if err := h.Handle(ctx, funcs.Event{Kind: "order.deleted", Payload: "2"}); err != nil {
			t.Errorf("non-matching Handle = %v, want nil", err)
		}
		if diff := cmp.Diff([]string{"1"}, got); diff != "" {
			t.Errorf("forwarded payloads mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("returns the inner handler's error for matching events", func(t *testing.T) {
		errBoom := errors.New("boom")
		h := funcs.Only("k", funcs.HandlerFunc(func(context.Context, funcs.Event) error { return errBoom }))
		if err := h.Handle(ctx, funcs.Event{Kind: "k"}); err != errBoom {
			t.Errorf("Handle = %v, want errBoom unchanged", err)
		}
	})
}

type counter struct{ n int }

func (c *counter) Bump(_ context.Context, e funcs.Event) error {
	c.n += len(e.Payload)
	return nil
}

func TestPart6BindMethod(t *testing.T) {
	t.Run("binds a method expression to a receiver", func(t *testing.T) {
		rec := &funcs.Recorder{}
		h := funcs.BindMethod(rec, (*funcs.Recorder).Record)
		e := funcs.Event{Kind: "bound", Payload: "yes"}
		if err := h.Handle(context.Background(), e); err != nil {
			t.Fatalf("Handle = %v, want nil", err)
		}
		if diff := cmp.Diff([]funcs.Event{e}, rec.Events()); diff != "" {
			t.Errorf("recorded events mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("each bound handler uses its own receiver", func(t *testing.T) {
		a, b := &counter{}, &counter{}
		ha := funcs.BindMethod(a, (*counter).Bump)
		hb := funcs.BindMethod(b, (*counter).Bump)
		ctx := context.Background()
		_ = ha.Handle(ctx, funcs.Event{Payload: "abc"})
		_ = hb.Handle(ctx, funcs.Event{Payload: "x"})
		_ = ha.Handle(ctx, funcs.Event{Payload: "de"})
		if a.n != 5 || b.n != 1 {
			t.Errorf("counters a=%d b=%d; want a=5 b=1", a.n, b.n)
		}
	})
}

func TestPart6Recorder(t *testing.T) {
	ctx := context.Background()

	t.Run("zero value records in order and Events returns a copy", func(t *testing.T) {
		var r funcs.Recorder
		e1 := funcs.Event{Kind: "a", Payload: "1"}
		e2 := funcs.Event{Kind: "b", Payload: "2"}
		_ = r.Record(ctx, e1)
		_ = r.Record(ctx, e2)

		got := r.Events()
		if diff := cmp.Diff([]funcs.Event{e1, e2}, got); diff != "" {
			t.Fatalf("events mismatch (-want +got):\n%s", diff)
		}
		got[0].Kind = "mutated"
		if again := r.Events(); again[0] != e1 {
			t.Errorf("mutating the returned slice changed the recorder: %+v", again[0])
		}
	})

	t.Run("concurrent records are all kept", func(t *testing.T) {
		var r funcs.Recorder
		const n = 100
		var wg sync.WaitGroup
		for i := range n {
			wg.Go(func() {
				_ = r.Record(ctx, funcs.Event{Kind: "c", Payload: string(rune('a' + i%26))})
			})
		}
		wg.Wait()
		if got := len(r.Events()); got != n {
			t.Errorf("recorded %d events, want %d", got, n)
		}
	})
}
