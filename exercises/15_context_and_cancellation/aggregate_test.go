package ctxflow_test

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/go-cmp/cmp"
	"goref/exercises/15_context_and_cancellation"
)

var errBoom = errors.New("boom")

func okSource(name, value string) ctxflow.Source[string] {
	return ctxflow.Source[string]{Name: name, Fetch: func(context.Context) (string, error) { return value, nil }}
}

func TestPart1Aggregate(t *testing.T) {
	t.Run("values come back in source order", func(t *testing.T) {
		srcs := []ctxflow.Source[string]{okSource("users", "u"), okSource("orders", "o"), okSource("prices", "p")}
		got, err := ctxflow.Aggregate(t.Context(), time.Second, srcs)
		if err != nil {
			t.Fatalf("Aggregate error = %v, want nil", err)
		}
		want := []ctxflow.Result[string]{
			{Name: "users", Value: "u"},
			{Name: "orders", Value: "o"},
			{Name: "prices", Value: "p"},
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("results mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("a failing source is reported per result, not as an error", func(t *testing.T) {
		srcs := []ctxflow.Source[string]{
			okSource("a", "A"),
			{Name: "b", Fetch: func(context.Context) (string, error) { return "", errBoom }},
			okSource("c", "C"),
		}
		got, err := ctxflow.Aggregate(t.Context(), time.Second, srcs)
		if err != nil {
			t.Fatalf("Aggregate error = %v, want nil", err)
		}
		if len(got) != 3 {
			t.Fatalf("got %d results, want 3", len(got))
		}
		if !errors.Is(got[1].Err, errBoom) || got[1].Name != "b" {
			t.Errorf("result[1] = %+v, want Name b with errBoom", got[1])
		}
		if got[0].Err != nil || got[0].Value != "A" || got[2].Err != nil || got[2].Value != "C" {
			t.Errorf("healthy sources affected by failure: %+v", got)
		}
	})

	t.Run("each factory is called exactly once", func(t *testing.T) {
		var calls [3]atomic.Int32
		var srcs []ctxflow.Source[int]
		for i := range calls {
			srcs = append(srcs, ctxflow.Source[int]{Name: fmt.Sprint(i), Fetch: func(context.Context) (int, error) {
				calls[i].Add(1)
				return i, nil
			}})
		}
		if _, err := ctxflow.Aggregate(t.Context(), time.Second, srcs); err != nil {
			t.Fatalf("Aggregate error = %v", err)
		}
		for i := range calls {
			if n := calls[i].Load(); n != 1 {
				t.Errorf("source %d called %d times, want 1", i, n)
			}
		}
	})

	t.Run("slow source times out while the others return partial results", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			slow := ctxflow.Source[string]{Name: "slow", Fetch: func(ctx context.Context) (string, error) {
				<-ctx.Done()
				return "", ctx.Err()
			}}
			got, err := ctxflow.Aggregate(context.Background(), 100*time.Millisecond, []ctxflow.Source[string]{okSource("fast", "F"), slow})
			if err != nil {
				t.Fatalf("Aggregate error = %v, want nil", err)
			}
			if len(got) != 2 {
				t.Fatalf("got %d results, want 2", len(got))
			}
			if got[0].Err != nil || got[0].Value != "F" {
				t.Errorf("fast = %+v, want value F", got[0])
			}
			if !errors.Is(got[1].Err, ctxflow.ErrCallTimeout) {
				t.Errorf("slow.Err = %v, want ErrCallTimeout", got[1].Err)
			}
		})
	})

	t.Run("sources run concurrently under their own budgets", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			var srcs []ctxflow.Source[string]
			for _, name := range []string{"a", "b", "c"} {
				srcs = append(srcs, ctxflow.Source[string]{Name: name, Fetch: func(context.Context) (string, error) {
					time.Sleep(80 * time.Millisecond)
					return name, nil
				}})
			}
			start := time.Now()
			got, err := ctxflow.Aggregate(context.Background(), 100*time.Millisecond, srcs)
			elapsed := time.Since(start)
			if err != nil {
				t.Fatalf("Aggregate error = %v", err)
			}
			if len(got) != 3 {
				t.Fatalf("got %d results, want 3", len(got))
			}
			for _, r := range got {
				if r.Err != nil || r.Value != r.Name {
					t.Errorf("result %+v, want success", r)
				}
			}
			if elapsed != 80*time.Millisecond {
				t.Errorf("elapsed = %v, want 80ms (sources must run concurrently)", elapsed)
			}
		})
	})

	t.Run("a source's own deadline error is not reported as a call timeout", func(t *testing.T) {
		src := ctxflow.Source[string]{Name: "upstream", Fetch: func(context.Context) (string, error) {
			return "", fmt.Errorf("upstream: %w", context.DeadlineExceeded)
		}}
		got, err := ctxflow.Aggregate(t.Context(), time.Minute, []ctxflow.Source[string]{src})
		if err != nil {
			t.Fatalf("Aggregate error = %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("got %d results, want 1", len(got))
		}
		if errors.Is(got[0].Err, ctxflow.ErrCallTimeout) {
			t.Errorf("Err = %v, must not be ErrCallTimeout", got[0].Err)
		}
		if !errors.Is(got[0].Err, context.DeadlineExceeded) {
			t.Errorf("Err = %v, want the source's own DeadlineExceeded", got[0].Err)
		}
	})

	t.Run("parent cancellation returns the parent's cause and cancels sources", func(t *testing.T) {
		errDraining := errors.New("draining")
		ctx, cancel := context.WithCancelCause(context.Background())
		defer cancel(nil)
		started := make(chan struct{}, 1)
		src := ctxflow.Source[string]{Name: "x", Fetch: func(c context.Context) (string, error) {
			started <- struct{}{}
			<-c.Done()
			return "", c.Err()
		}}
		type out struct {
			res []ctxflow.Result[string]
			err error
		}
		done := make(chan out, 1)
		go func() {
			res, err := ctxflow.Aggregate(ctx, time.Minute, []ctxflow.Source[string]{src})
			done <- out{res, err}
		}()
		if _, ok := recv(t, (<-chan struct{})(started)); !ok {
			return
		}
		cancel(errDraining)
		o, ok := recv(t, (<-chan out)(done))
		if !ok {
			return
		}
		if !errors.Is(o.err, errDraining) {
			t.Errorf("Aggregate error = %v, want errDraining", o.err)
		}
		if len(o.res) != 1 {
			t.Fatalf("got %d results, want 1 (results are returned even on cancellation)", len(o.res))
		}
		if !errors.Is(o.res[0].Err, context.Canceled) || errors.Is(o.res[0].Err, ctxflow.ErrCallTimeout) {
			t.Errorf("Err = %v, want context.Canceled and not ErrCallTimeout", o.res[0].Err)
		}
	})

	t.Run("source name travels in the context", func(t *testing.T) {
		src := ctxflow.Source[string]{Name: "geo", Fetch: func(c context.Context) (string, error) {
			name, _ := ctxflow.SourceFrom(c)
			return name, nil
		}}
		got, err := ctxflow.Aggregate(t.Context(), time.Second, []ctxflow.Source[string]{src})
		if err != nil || len(got) != 1 {
			t.Fatalf("Aggregate = (%d results, %v), want 1 result and nil", len(got), err)
		}
		if got[0].Value != "geo" {
			t.Errorf("Fetch saw source name %q, want %q", got[0].Value, "geo")
		}
	})

	t.Run("WithSource round-trips and absence reports false", func(t *testing.T) {
		if name, ok := ctxflow.SourceFrom(ctxflow.WithSource(context.Background(), "billing")); !ok || name != "billing" {
			t.Errorf("SourceFrom(WithSource(bg, billing)) = (%q, %v), want (billing, true)", name, ok)
		}
		if name, ok := ctxflow.SourceFrom(context.Background()); ok || name != "" {
			t.Errorf("SourceFrom(bg) = (%q, %v), want (\"\", false)", name, ok)
		}
	})

	t.Run("non-positive per-call budget is rejected before any source runs", func(t *testing.T) {
		var calls atomic.Int32
		src := ctxflow.Source[int]{Name: "x", Fetch: func(context.Context) (int, error) {
			calls.Add(1)
			return 0, nil
		}}
		for _, d := range []time.Duration{0, -time.Second} {
			_, err := ctxflow.Aggregate(t.Context(), d, []ctxflow.Source[int]{src})
			if !errors.Is(err, ctxflow.ErrInvalidArgument) {
				t.Errorf("perCall=%v: error = %v, want ErrInvalidArgument", d, err)
			}
		}
		if n := calls.Load(); n != 0 {
			t.Errorf("sources called %d times, want 0", n)
		}
	})
}
