package funcs_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	funcs "goref/exercises/04_functions_and_closures"
)

func TestPart7Memo(t *testing.T) {
	t0 := time.Unix(1_000_000, 0)
	const ttl = 10 * time.Second

	// countingLoader returns its 1-based call number, so a test can tell a
	// cached value from a reloaded one.
	countingLoader := func(calls *atomic.Int64) func(context.Context, string) (int, error) {
		return func(context.Context, string) (int, error) {
			return int(calls.Add(1)), nil
		}
	}

	t.Run("a fresh entry is served from cache", func(t *testing.T) {
		clk := &fakeClock{now: t0}
		var calls atomic.Int64
		m := funcs.NewMemo[string, int](ttl, clk.Now, countingLoader(&calls))
		for range 2 {
			if got, err := m.Get(t.Context(), "a"); err != nil || got != 1 {
				t.Errorf("Get(a) = %d, %v; want 1, <nil>", got, err)
			}
		}
		want := funcs.MemoStats{Hits: 1, Misses: 1, Loads: 1}
		if diff := cmp.Diff(want, m.Stats()); diff != "" {
			t.Errorf("stats mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("an entry expires exactly at its ttl", func(t *testing.T) {
		clk := &fakeClock{now: t0}
		var calls atomic.Int64
		m := funcs.NewMemo[string, int](ttl, clk.Now, countingLoader(&calls))
		ctx := t.Context()

		got := make([]int, 0, 3)
		for _, step := range []time.Duration{0, ttl - time.Nanosecond, time.Nanosecond} {
			clk.Advance(step)
			v, _ := m.Get(ctx, "a")
			got = append(got, v)
		}
		if diff := cmp.Diff([]int{1, 1, 2}, got); diff != "" {
			t.Errorf("values mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("ttl of zero or less never expires", func(t *testing.T) {
		clk := &fakeClock{now: t0}
		var calls atomic.Int64
		m := funcs.NewMemo[string, int](0, clk.Now, countingLoader(&calls))
		if got, _ := m.Get(t.Context(), "a"); got != 1 {
			t.Errorf("first Get = %d, want 1", got)
		}
		clk.Advance(1000 * time.Hour)
		if got, _ := m.Get(t.Context(), "a"); got != 1 {
			t.Errorf("Get after 1000h = %d, want cached 1", got)
		}
		if n := calls.Load(); n != 1 {
			t.Errorf("loader ran %d times, want 1", n)
		}
	})

	t.Run("errors are not cached and the zero value is returned", func(t *testing.T) {
		clk := &fakeClock{now: t0}
		errBoom := errors.New("boom")
		var calls atomic.Int64
		load := func(context.Context, string) (int, error) {
			if n := calls.Add(1); n == 1 {
				return 0, errBoom
			} else {
				return int(n), nil
			}
		}
		m := funcs.NewMemo[string, int](ttl, clk.Now, load)

		got, err := m.Get(t.Context(), "a")
		if !errors.Is(err, errBoom) || got != 0 {
			t.Errorf("first Get = %d, %v; want 0 and errBoom", got, err)
		}
		got, err = m.Get(t.Context(), "a")
		if err != nil || got != 2 {
			t.Errorf("second Get = %d, %v; want 2, <nil> (error must not be cached)", got, err)
		}
		want := funcs.MemoStats{Misses: 2, Loads: 2, Errors: 1}
		if diff := cmp.Diff(want, m.Stats()); diff != "" {
			t.Errorf("stats mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("Forget makes the next Get load again", func(t *testing.T) {
		clk := &fakeClock{now: t0}
		var calls atomic.Int64
		m := funcs.NewMemo[string, int](ttl, clk.Now, countingLoader(&calls))
		if got, _ := m.Get(t.Context(), "a"); got != 1 {
			t.Fatalf("first Get = %d, want 1", got)
		}
		m.Forget("a")
		if got, _ := m.Get(t.Context(), "a"); got != 2 {
			t.Errorf("Get after Forget = %d, want reloaded 2", got)
		}
	})

	t.Run("keys are cached independently", func(t *testing.T) {
		clk := &fakeClock{now: t0}
		var calls atomic.Int64
		m := funcs.NewMemo[string, int](ttl, clk.Now, countingLoader(&calls))
		ctx := t.Context()

		var got []int
		for _, k := range []string{"a", "b", "a", "b"} {
			v, _ := m.Get(ctx, k)
			got = append(got, v)
		}
		if diff := cmp.Diff([]int{1, 2, 1, 2}, got); diff != "" {
			t.Errorf("values mismatch (-want +got):\n%s", diff)
		}
		if s := m.Stats(); s.Loads != 2 || s.Hits != 2 {
			t.Errorf("stats = %+v, want Loads 2 and Hits 2", s)
		}
	})

	t.Run("the load receives the context of the Get that started it", func(t *testing.T) {
		type ctxKey struct{}
		clk := &fakeClock{now: t0}
		var seen string
		var calls atomic.Int64
		load := func(ctx context.Context, _ string) (int, error) {
			calls.Add(1)
			seen, _ = ctx.Value(ctxKey{}).(string)
			return 7, nil
		}
		m := funcs.NewMemo[string, int](ttl, clk.Now, load)
		ctx := context.WithValue(t.Context(), ctxKey{}, "req-42")

		got, err := m.Get(ctx, "k")
		if err != nil || got != 7 {
			t.Errorf("Get = %d, %v; want 7, <nil>", got, err)
		}
		if calls.Load() != 1 || seen != "req-42" {
			t.Errorf("loads=%d ctx value=%q; want 1 load carrying %q", calls.Load(), seen, "req-42")
		}
	})

	t.Run("concurrent Gets for one key share one load", func(t *testing.T) {
		clk := &fakeClock{now: t0}
		started := make(chan struct{})
		release := make(chan struct{})
		var once sync.Once
		closeRelease := func() { once.Do(func() { close(release) }) }
		t.Cleanup(closeRelease)

		var calls atomic.Int64
		load := func(context.Context, string) (int, error) {
			if calls.Add(1) == 1 {
				close(started)
			}
			<-release
			return 42, nil
		}
		m := funcs.NewMemo[string, int](ttl, clk.Now, load)

		type result struct {
			v   int
			err error
		}
		const n = 50
		results := make(chan result, n)
		get := func() {
			v, err := m.Get(t.Context(), "k")
			results <- result{v, err}
		}

		var wg sync.WaitGroup
		wg.Go(get)
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatal("load never started")
		}
		for range n - 1 {
			wg.Go(get)
		}
		closeRelease()

		done := make(chan struct{})
		go func() { wg.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("Gets did not finish after the load was released")
		}
		close(results)

		count := 0
		for r := range results {
			count++
			if r.err != nil || r.v != 42 {
				t.Errorf("Get = %d, %v; want 42, <nil>", r.v, r.err)
			}
		}
		if count != n {
			t.Errorf("got %d results, want %d", count, n)
		}
		if got := calls.Load(); got != 1 {
			t.Errorf("load ran %d times, want 1", got)
		}
		if s := m.Stats(); s.Loads != 1 || s.Hits+s.Misses != n {
			t.Errorf("stats = %+v, want Loads 1 and Hits+Misses %d", s, n)
		}
	})

	t.Run("generic over struct values", func(t *testing.T) {
		type quote struct {
			Symbol string
			Cents  int
		}
		clk := &fakeClock{now: t0}
		var calls atomic.Int64
		load := func(_ context.Context, key string) (quote, error) {
			calls.Add(1)
			return quote{Symbol: key, Cents: 150}, nil
		}
		m := funcs.NewMemo[string, quote](ttl, clk.Now, load)
		want := quote{Symbol: "ACME", Cents: 150}
		for range 2 {
			got, err := m.Get(t.Context(), "ACME")
			if err != nil || got != want {
				t.Errorf("Get = %+v, %v; want %+v, <nil>", got, err, want)
			}
		}
		if calls.Load() != 1 {
			t.Errorf("loader ran %d times, want 1", calls.Load())
		}
	})
}
