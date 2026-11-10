package ctxflow_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/go-cmp/cmp"
	"goref/exercises/15_context_and_cancellation"
)

func TestPart2FetchAll(t *testing.T) {
	t.Run("returns values in source order", func(t *testing.T) {
		srcs := []ctxflow.Source[int]{
			{Name: "a", Fetch: func(context.Context) (int, error) { return 1, nil }},
			{Name: "b", Fetch: func(context.Context) (int, error) { return 2, nil }},
			{Name: "c", Fetch: func(context.Context) (int, error) { return 3, nil }},
		}
		got, err := ctxflow.FetchAll(t.Context(), srcs)
		if err != nil {
			t.Fatalf("FetchAll error = %v", err)
		}
		if diff := cmp.Diff([]int{1, 2, 3}, got); diff != "" {
			t.Errorf("values mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("empty input gives an empty result and nil error", func(t *testing.T) {
		got, err := ctxflow.FetchAll[int](t.Context(), nil)
		if err != nil {
			t.Fatalf("FetchAll error = %v", err)
		}
		if got == nil || len(got) != 0 {
			t.Errorf("got %#v, want empty non-nil slice", got)
		}
	})

	t.Run("first error is returned and cancels siblings, which return before FetchAll does", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			var sawCancel atomic.Bool
			srcs := []ctxflow.Source[int]{
				{Name: "fails", Fetch: func(context.Context) (int, error) {
					time.Sleep(10 * time.Millisecond)
					return 0, errBoom
				}},
				{Name: "waits", Fetch: func(c context.Context) (int, error) {
					<-c.Done()
					sawCancel.Store(true)
					return 0, c.Err()
				}},
			}
			got, err := ctxflow.FetchAll(context.Background(), srcs)
			if !errors.Is(err, errBoom) {
				t.Errorf("FetchAll error = %v, want errBoom", err)
			}
			if errors.Is(err, context.Canceled) {
				t.Errorf("FetchAll error = %v: the sibling's cancellation must not win", err)
			}
			if got != nil {
				t.Errorf("values = %v on error, want nil", got)
			}
			if !sawCancel.Load() {
				t.Errorf("sibling had not observed cancellation when FetchAll returned")
			}
		})
	})

	t.Run("parent cancellation surfaces as the error", func(t *testing.T) {
		ctx, cancel := context.WithCancelCause(context.Background())
		defer cancel(nil)
		errStop := errors.New("stop")
		srcs := []ctxflow.Source[int]{
			{Name: "a", Fetch: func(c context.Context) (int, error) {
				cancel(errStop)
				<-c.Done()
				return 0, c.Err()
			}},
		}
		done := make(chan error, 1)
		go func() {
			_, err := ctxflow.FetchAll(ctx, srcs)
			done <- err
		}()
		err, ok := recv(t, (<-chan error)(done))
		if !ok {
			return
		}
		if !errors.Is(err, errStop) {
			t.Errorf("FetchAll error = %v, want errStop", err)
		}
	})
}

func TestPart2Hedge(t *testing.T) {
	t.Run("fast primary wins without starting a hedge", func(t *testing.T) {
		var calls atomic.Int32
		got, err := ctxflow.Hedge(t.Context(), time.Hour, 3, func(context.Context, int) (string, error) {
			calls.Add(1)
			return "primary", nil
		})
		if err != nil || got != "primary" {
			t.Fatalf("Hedge = (%q, %v), want (primary, nil)", got, err)
		}
		if n := calls.Load(); n != 1 {
			t.Errorf("calls = %d, want 1", n)
		}
	})

	t.Run("slow primary is hedged, the hedge wins, and the loser is cancelled", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			var calls atomic.Int32
			var primaryCancelled atomic.Bool
			fetch := func(c context.Context, attempt int) (string, error) {
				calls.Add(1)
				if attempt == 0 {
					<-c.Done()
					primaryCancelled.Store(true)
					return "", c.Err()
				}
				time.Sleep(10 * time.Millisecond)
				return "hedge", nil
			}
			got, err := ctxflow.Hedge(context.Background(), 50*time.Millisecond, 2, fetch)
			if err != nil || got != "hedge" {
				t.Fatalf("Hedge = (%q, %v), want (hedge, nil)", got, err)
			}
			if n := calls.Load(); n != 2 {
				t.Errorf("calls = %d, want 2", n)
			}
			if !primaryCancelled.Load() {
				t.Errorf("losing attempt was not cancelled before Hedge returned")
			}
		})
	})

	t.Run("a failed attempt starts the next one without waiting for delay", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			start := time.Now()
			got, err := ctxflow.Hedge(context.Background(), time.Hour, 2, func(_ context.Context, attempt int) (int, error) {
				if attempt == 0 {
					return 0, errBoom
				}
				return 42, nil
			})
			if err != nil || got != 42 {
				t.Fatalf("Hedge = (%d, %v), want (42, nil)", got, err)
			}
			if elapsed := time.Since(start); elapsed != 0 {
				t.Errorf("elapsed = %v, want 0 (failure must not wait for the hedge delay)", elapsed)
			}
		})
	})

	t.Run("all attempts failing returns every error", func(t *testing.T) {
		errs := []error{errors.New("e0"), errors.New("e1"), errors.New("e2")}
		_, err := ctxflow.Hedge(t.Context(), 0, 3, func(_ context.Context, attempt int) (int, error) {
			return 0, errs[attempt]
		})
		if err == nil {
			t.Fatal("Hedge error = nil, want joined attempt errors")
		}
		for _, want := range errs {
			if !errors.Is(err, want) {
				t.Errorf("Hedge error = %v, missing %v", err, want)
			}
		}
	})

	t.Run("parent deadline returns the cause", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			errBudget := errors.New("budget spent")
			ctx, cancel := context.WithTimeoutCause(context.Background(), 30*time.Millisecond, errBudget)
			defer cancel()
			_, err := ctxflow.Hedge(ctx, 10*time.Millisecond, 3, func(c context.Context, _ int) (int, error) {
				<-c.Done()
				return 0, c.Err()
			})
			if !errors.Is(err, errBudget) {
				t.Errorf("Hedge error = %v, want errBudget", err)
			}
		})
	})

	t.Run("attempts below one and negative delay are rejected", func(t *testing.T) {
		fetch := func(context.Context, int) (int, error) { return 1, nil }
		if _, err := ctxflow.Hedge(t.Context(), time.Second, 0, fetch); !errors.Is(err, ctxflow.ErrInvalidArgument) {
			t.Errorf("attempts=0: error = %v, want ErrInvalidArgument", err)
		}
		if _, err := ctxflow.Hedge(t.Context(), -time.Second, 1, fetch); !errors.Is(err, ctxflow.ErrInvalidArgument) {
			t.Errorf("delay<0: error = %v, want ErrInvalidArgument", err)
		}
	})
}
