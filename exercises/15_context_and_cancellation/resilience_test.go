package ctxflow_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/go-cmp/cmp"
	"goref/exercises/15_context_and_cancellation"
)

func TestPart7Retrier(t *testing.T) {
	identity := func(d time.Duration) time.Duration { return d }

	t.Run("success on the first attempt makes exactly one call", func(t *testing.T) {
		var calls atomic.Int32
		r := ctxflow.Retrier{Attempts: 3, Base: time.Millisecond, Max: time.Millisecond, Jitter: identity}
		err := r.Do(t.Context(), func(context.Context) error {
			calls.Add(1)
			return nil
		})
		if err != nil {
			t.Fatalf("Do error = %v", err)
		}
		if n := calls.Load(); n != 1 {
			t.Errorf("calls = %d, want 1", n)
		}
	})

	t.Run("retries until success with a nil Retryable", func(t *testing.T) {
		var calls atomic.Int32
		r := ctxflow.Retrier{Attempts: 5, Base: time.Millisecond, Max: time.Millisecond, Jitter: identity}
		err := r.Do(t.Context(), func(context.Context) error {
			if calls.Add(1) < 3 {
				return errBoom
			}
			return nil
		})
		if err != nil {
			t.Fatalf("Do error = %v, want nil", err)
		}
		if n := calls.Load(); n != 3 {
			t.Errorf("calls = %d, want 3", n)
		}
	})

	t.Run("backoff doubles and is capped at Max, with no wait after the last attempt", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			var nominals []time.Duration
			r := ctxflow.Retrier{
				Attempts: 5,
				Base:     10 * time.Millisecond,
				Max:      35 * time.Millisecond,
				Jitter: func(d time.Duration) time.Duration {
					nominals = append(nominals, d)
					return d
				},
			}
			start := time.Now()
			err := r.Do(context.Background(), func(context.Context) error { return errBoom })
			if !errors.Is(err, errBoom) {
				t.Errorf("Do error = %v, want errBoom", err)
			}
			if diff := cmp.Diff([]time.Duration{10, 20, 35, 35}, nominalsMs(nominals)); diff != "" {
				t.Errorf("nominal waits mismatch (-want +got):\n%s", diff)
			}
			if elapsed := time.Since(start); elapsed != 100*time.Millisecond {
				t.Errorf("elapsed = %v, want 100ms", elapsed)
			}
		})
	})

	t.Run("very many attempts do not overflow the backoff", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			var calls atomic.Int32
			r := ctxflow.Retrier{Attempts: 200, Base: time.Millisecond, Max: 10 * time.Millisecond, Jitter: identity}
			start := time.Now()
			err := r.Do(context.Background(), func(context.Context) error {
				calls.Add(1)
				return errBoom
			})
			if !errors.Is(err, errBoom) {
				t.Errorf("Do error = %v, want errBoom", err)
			}
			if n := calls.Load(); n != 200 {
				t.Errorf("calls = %d, want 200", n)
			}
			if elapsed := time.Since(start); elapsed != 1965*time.Millisecond {
				t.Errorf("elapsed = %v, want 1.965s (waits of 1,2,4,8ms, then 195 of 10ms)", elapsed)
			}
		})
	})

	t.Run("a non-retryable error stops immediately", func(t *testing.T) {
		var calls atomic.Int32
		r := ctxflow.Retrier{
			Attempts:  5,
			Base:      time.Millisecond,
			Max:       time.Millisecond,
			Jitter:    identity,
			Retryable: func(error) bool { return false },
		}
		err := r.Do(t.Context(), func(context.Context) error {
			calls.Add(1)
			return errBoom
		})
		if !errors.Is(err, errBoom) {
			t.Errorf("Do error = %v, want errBoom", err)
		}
		if n := calls.Load(); n != 1 {
			t.Errorf("calls = %d, want 1", n)
		}
	})

	t.Run("a context already done runs no attempt and returns its cause", func(t *testing.T) {
		errCause := errors.New("shutting down")
		ctx, cancel := context.WithCancelCause(context.Background())
		cancel(errCause)
		var calls atomic.Int32
		r := ctxflow.Retrier{Attempts: 3, Base: time.Millisecond, Max: time.Millisecond, Jitter: identity}
		err := r.Do(ctx, func(context.Context) error {
			calls.Add(1)
			return nil
		})
		if !errors.Is(err, errCause) {
			t.Errorf("Do error = %v, want errCause", err)
		}
		if n := calls.Load(); n != 0 {
			t.Errorf("calls = %d, want 0", n)
		}
	})

	t.Run("cancellation during a wait joins the last error with the cause", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			errBudget := errors.New("budget spent")
			ctx, cancel := context.WithTimeoutCause(context.Background(), 50*time.Millisecond, errBudget)
			defer cancel()
			var calls atomic.Int32
			r := ctxflow.Retrier{Attempts: 3, Base: 100 * time.Millisecond, Max: 100 * time.Millisecond, Jitter: identity}
			err := r.Do(ctx, func(context.Context) error {
				calls.Add(1)
				return errBoom
			})
			if !errors.Is(err, errBudget) {
				t.Errorf("Do error = %v, want errBudget", err)
			}
			if !errors.Is(err, errBoom) {
				t.Errorf("Do error = %v, want the last attempt's errBoom too", err)
			}
			if n := calls.Load(); n != 1 {
				t.Errorf("calls = %d, want 1 (the wait is cut short)", n)
			}
		})
	})
}

// nominalsMs converts durations to whole milliseconds for readable diffs.
func nominalsMs(ds []time.Duration) []time.Duration {
	out := make([]time.Duration, len(ds))
	for i, d := range ds {
		out[i] = d / time.Millisecond
	}
	return out
}

func TestPart7TokenBucket(t *testing.T) {
	t.Run("a full bucket allows burst calls then refuses", func(t *testing.T) {
		now := time.Unix(0, 0)
		tb, err := ctxflow.NewTokenBucket(10, 2, func() time.Time { return now })
		if err != nil {
			t.Fatalf("NewTokenBucket error = %v", err)
		}
		if tb == nil {
			t.Fatal("NewTokenBucket returned nil")
		}
		got := []bool{tb.Allow(), tb.Allow(), tb.Allow()}
		if diff := cmp.Diff([]bool{true, true, false}, got); diff != "" {
			t.Errorf("Allow sequence mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("tokens refill at the configured rate", func(t *testing.T) {
		now := time.Unix(0, 0)
		tb, err := ctxflow.NewTokenBucket(10, 2, func() time.Time { return now })
		if err != nil || tb == nil {
			t.Fatalf("NewTokenBucket = (%v, %v)", tb, err)
		}
		tb.Allow()
		tb.Allow()
		now = now.Add(100 * time.Millisecond)
		got := []bool{tb.Allow(), tb.Allow()}
		if diff := cmp.Diff([]bool{true, false}, got); diff != "" {
			t.Errorf("after 100ms at 10/s, Allow sequence mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("refill is capped at burst", func(t *testing.T) {
		now := time.Unix(0, 0)
		tb, err := ctxflow.NewTokenBucket(10, 2, func() time.Time { return now })
		if err != nil || tb == nil {
			t.Fatalf("NewTokenBucket = (%v, %v)", tb, err)
		}
		tb.Allow()
		tb.Allow()
		now = now.Add(10 * time.Second)
		got := []bool{tb.Allow(), tb.Allow(), tb.Allow()}
		if diff := cmp.Diff([]bool{true, true, false}, got); diff != "" {
			t.Errorf("after long idle, Allow sequence mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("Wait paces calls to the rate", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			tb, err := ctxflow.NewTokenBucket(10, 1, time.Now)
			if err != nil || tb == nil {
				t.Fatalf("NewTokenBucket = (%v, %v)", tb, err)
			}
			start := time.Now()
			for i := range 3 {
				if err := tb.Wait(context.Background()); err != nil {
					t.Fatalf("Wait %d error = %v", i, err)
				}
			}
			if elapsed := time.Since(start); elapsed != 200*time.Millisecond {
				t.Errorf("3 waits took %v, want 200ms (first is free, then 100ms each)", elapsed)
			}
		})
	})

	t.Run("Wait returns the context cause when it ends first", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			errBudget := errors.New("budget spent")
			tb, err := ctxflow.NewTokenBucket(1, 1, time.Now)
			if err != nil || tb == nil {
				t.Fatalf("NewTokenBucket = (%v, %v)", tb, err)
			}
			if err := tb.Wait(context.Background()); err != nil {
				t.Fatalf("first Wait error = %v", err)
			}
			ctx, cancel := context.WithTimeoutCause(context.Background(), 10*time.Millisecond, errBudget)
			defer cancel()
			start := time.Now()
			if err := tb.Wait(ctx); !errors.Is(err, errBudget) {
				t.Errorf("Wait error = %v, want errBudget", err)
			}
			if elapsed := time.Since(start); elapsed != 10*time.Millisecond {
				t.Errorf("Wait returned after %v, want 10ms", elapsed)
			}
		})
	})

	t.Run("non-positive rate or burst is rejected", func(t *testing.T) {
		now := func() time.Time { return time.Unix(0, 0) }
		if _, err := ctxflow.NewTokenBucket(0, 1, now); !errors.Is(err, ctxflow.ErrInvalidArgument) {
			t.Errorf("rate=0 error = %v, want ErrInvalidArgument", err)
		}
		if _, err := ctxflow.NewTokenBucket(1, 0, now); !errors.Is(err, ctxflow.ErrInvalidArgument) {
			t.Errorf("burst=0 error = %v, want ErrInvalidArgument", err)
		}
	})
}

func TestPart7Breaker(t *testing.T) {
	noop := func(context.Context) error { return nil }
	fail := func(context.Context) error { return errBoom }

	newTestBreaker := func(t *testing.T, threshold int, cooldown time.Duration, now *time.Time) *ctxflow.Breaker {
		t.Helper()
		b, err := ctxflow.NewBreaker(threshold, cooldown, func() time.Time { return *now })
		if err != nil {
			t.Fatalf("NewBreaker error = %v", err)
		}
		if b == nil {
			t.Fatal("NewBreaker returned nil")
		}
		return b
	}

	t.Run("failures below the threshold keep the breaker closed", func(t *testing.T) {
		now := time.Unix(0, 0)
		b := newTestBreaker(t, 3, time.Minute, &now)
		var calls atomic.Int32
		op := func(context.Context) error { calls.Add(1); return errBoom }
		b.Do(t.Context(), op)
		b.Do(t.Context(), op)
		if n := calls.Load(); n != 2 {
			t.Errorf("op ran %d times, want 2 (breaker must admit calls while closed)", n)
		}
		if s := b.State(); s != ctxflow.StateClosed {
			t.Errorf("State = %v after 2 failures, want closed", s)
		}
	})

	t.Run("reaching the threshold opens the breaker and open calls fail fast", func(t *testing.T) {
		now := time.Unix(0, 0)
		b := newTestBreaker(t, 3, time.Minute, &now)
		for range 3 {
			b.Do(t.Context(), fail)
		}
		if s := b.State(); s != ctxflow.StateOpen {
			t.Fatalf("State = %v after 3 failures, want open", s)
		}
		var calls atomic.Int32
		err := b.Do(t.Context(), func(context.Context) error {
			calls.Add(1)
			return nil
		})
		if !errors.Is(err, ctxflow.ErrOpen) {
			t.Errorf("Do on open breaker error = %v, want ErrOpen", err)
		}
		if n := calls.Load(); n != 0 {
			t.Errorf("op ran %d times on an open breaker, want 0", n)
		}
	})

	t.Run("a success resets the consecutive failure count", func(t *testing.T) {
		now := time.Unix(0, 0)
		b := newTestBreaker(t, 3, time.Minute, &now)
		var calls atomic.Int32
		fails := func(context.Context) error { calls.Add(1); return errBoom }
		succeeds := func(context.Context) error { calls.Add(1); return nil }
		b.Do(t.Context(), fails)
		b.Do(t.Context(), fails)
		b.Do(t.Context(), succeeds)
		b.Do(t.Context(), fails)
		b.Do(t.Context(), fails)
		if n := calls.Load(); n != 5 {
			t.Errorf("op ran %d times, want 5 (all calls admitted while closed)", n)
		}
		if s := b.State(); s != ctxflow.StateClosed {
			t.Errorf("State = %v, want closed (failures were not consecutive)", s)
		}
	})

	t.Run("after the cooldown the breaker is half-open and a success closes it", func(t *testing.T) {
		now := time.Unix(0, 0)
		b := newTestBreaker(t, 1, time.Minute, &now)
		b.Do(t.Context(), fail)
		now = now.Add(time.Minute)
		if s := b.State(); s != ctxflow.StateHalfOpen {
			t.Fatalf("State = %v after cooldown, want half-open", s)
		}
		if err := b.Do(t.Context(), noop); err != nil {
			t.Fatalf("probe error = %v", err)
		}
		if s := b.State(); s != ctxflow.StateClosed {
			t.Errorf("State after successful probe = %v, want closed", s)
		}
	})

	t.Run("half-open admits one probe and rejects concurrent calls", func(t *testing.T) {
		now := time.Unix(0, 0)
		b := newTestBreaker(t, 1, time.Minute, &now)
		b.Do(t.Context(), fail)
		now = now.Add(time.Minute)

		started := make(chan struct{}, 1)
		gate := make(chan struct{})
		release := sync.OnceFunc(func() { close(gate) })
		defer release()
		probeErr := make(chan error, 1)
		go func() {
			probeErr <- b.Do(t.Context(), func(context.Context) error {
				started <- struct{}{}
				<-gate
				return nil
			})
		}()
		if _, ok := recv(t, (<-chan struct{})(started)); !ok {
			return
		}
		var secondCalls atomic.Int32
		err := b.Do(t.Context(), func(context.Context) error {
			secondCalls.Add(1)
			return nil
		})
		if !errors.Is(err, ctxflow.ErrOpen) {
			t.Errorf("second call during probe error = %v, want ErrOpen", err)
		}
		if n := secondCalls.Load(); n != 0 {
			t.Errorf("second op ran %d times, want 0", n)
		}
		release()
		if err, ok := recv(t, (<-chan error)(probeErr)); ok && err != nil {
			t.Errorf("probe error = %v, want nil", err)
		}
		if s := b.State(); s != ctxflow.StateClosed {
			t.Errorf("State after probe success = %v, want closed", s)
		}
	})

	t.Run("a failed probe reopens with a fresh cooldown", func(t *testing.T) {
		now := time.Unix(0, 0)
		b := newTestBreaker(t, 1, time.Minute, &now)
		b.Do(t.Context(), fail)
		now = now.Add(time.Minute)
		b.Do(t.Context(), fail)
		now = now.Add(time.Minute - time.Second)
		if s := b.State(); s != ctxflow.StateOpen {
			t.Errorf("State %v just before fresh cooldown ends, want open", s)
		}
		now = now.Add(time.Second)
		if s := b.State(); s != ctxflow.StateHalfOpen {
			t.Errorf("State %v after fresh cooldown, want half-open", s)
		}
	})

	t.Run("cancellation-caused errors are neutral", func(t *testing.T) {
		now := time.Unix(0, 0)
		b := newTestBreaker(t, 2, time.Minute, &now)
		for range 3 {
			ctx, cancel := context.WithCancel(t.Context())
			err := b.Do(ctx, func(c context.Context) error {
				cancel()
				return c.Err()
			})
			if !errors.Is(err, context.Canceled) {
				t.Errorf("Do error = %v, want context.Canceled", err)
			}
		}
		if s := b.State(); s != ctxflow.StateClosed {
			t.Errorf("State = %v, want closed (cancellations are not upstream failures)", s)
		}
	})

	t.Run("a neutral probe leaves the breaker half-open for the next probe", func(t *testing.T) {
		now := time.Unix(0, 0)
		b := newTestBreaker(t, 1, time.Minute, &now)
		b.Do(t.Context(), fail)
		now = now.Add(time.Minute)
		ctx, cancel := context.WithCancel(t.Context())
		b.Do(ctx, func(c context.Context) error {
			cancel()
			return c.Err()
		})
		if s := b.State(); s != ctxflow.StateHalfOpen {
			t.Errorf("State after neutral probe = %v, want half-open", s)
		}
		if err := b.Do(t.Context(), noop); err != nil {
			t.Errorf("next probe error = %v, want nil", err)
		}
		if s := b.State(); s != ctxflow.StateClosed {
			t.Errorf("State after successful probe = %v, want closed", s)
		}
	})

	t.Run("states have distinct, non-empty names", func(t *testing.T) {
		names := map[string]bool{}
		for _, s := range []ctxflow.BreakerState{ctxflow.StateClosed, ctxflow.StateOpen, ctxflow.StateHalfOpen} {
			n := s.String()
			if n == "" || names[n] {
				t.Errorf("state name %q is empty or duplicated", n)
			}
			names[n] = true
		}
	})

	t.Run("invalid threshold or cooldown is rejected", func(t *testing.T) {
		now := func() time.Time { return time.Unix(0, 0) }
		if _, err := ctxflow.NewBreaker(0, time.Second, now); !errors.Is(err, ctxflow.ErrInvalidArgument) {
			t.Errorf("threshold=0 error = %v, want ErrInvalidArgument", err)
		}
		if _, err := ctxflow.NewBreaker(1, 0, now); !errors.Is(err, ctxflow.ErrInvalidArgument) {
			t.Errorf("cooldown=0 error = %v, want ErrInvalidArgument", err)
		}
	})
}
