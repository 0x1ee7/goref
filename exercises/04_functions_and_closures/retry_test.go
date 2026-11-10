package funcs_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	funcs "goref/exercises/04_functions_and_closures"
)

var errTransient = errors.New("transient")

// recordSleep returns a Sleeper that records every requested delay and never waits.
func recordSleep(log *[]time.Duration) funcs.Sleeper {
	return func(ctx context.Context, d time.Duration) error {
		*log = append(*log, d)
		return nil
	}
}

func TestPart5Retry(t *testing.T) {
	// backoff(n) = n * 10ms, so attempt 1 waits 10ms and attempt 2 waits 20ms.
	backoff := func(attempt int) time.Duration { return time.Duration(attempt) * 10 * time.Millisecond }

	t.Run("returns nil after a later success, sleeping between attempts", func(t *testing.T) {
		var sleeps []time.Duration
		var calls int
		fn := func(context.Context) error {
			calls++
			if calls < 3 {
				return errTransient
			}
			return nil
		}
		if err := funcs.Retry(t.Context(), 5, backoff, recordSleep(&sleeps), fn); err != nil {
			t.Fatalf("Retry = %v, want nil", err)
		}
		if calls != 3 {
			t.Errorf("fn called %d times, want 3", calls)
		}
		if diff := cmp.Diff([]time.Duration{10 * time.Millisecond, 20 * time.Millisecond}, sleeps); diff != "" {
			t.Errorf("sleeps mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("no sleep after a first-try success", func(t *testing.T) {
		var sleeps []time.Duration
		var calls int
		fn := func(context.Context) error { calls++; return nil }
		if err := funcs.Retry(t.Context(), 3, backoff, recordSleep(&sleeps), fn); err != nil {
			t.Fatalf("Retry = %v, want nil", err)
		}
		if calls != 1 || len(sleeps) != 0 {
			t.Errorf("calls=%d sleeps=%v; want calls=1 and no sleeps", calls, sleeps)
		}
	})

	t.Run("exhaustion wraps the last error, with no sleep after the final attempt", func(t *testing.T) {
		var sleeps []time.Duration
		var attempts []int
		bo := func(attempt int) time.Duration { attempts = append(attempts, attempt); return 0 }
		var calls int
		fn := func(context.Context) error {
			calls++
			return fmt.Errorf("try %d: %w", calls, errTransient)
		}
		err := funcs.Retry(t.Context(), 3, bo, recordSleep(&sleeps), fn)
		if err == nil {
			t.Fatal("Retry = nil, want exhaustion error")
		}
		if !strings.Contains(err.Error(), "3 attempts") || !strings.Contains(err.Error(), "try 3") {
			t.Errorf("err = %q, want the attempt count and the last error", err)
		}
		if calls != 3 {
			t.Errorf("fn called %d times, want 3", calls)
		}
		if diff := cmp.Diff([]int{1, 2}, attempts); diff != "" {
			t.Errorf("backoff attempts mismatch (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff([]time.Duration{0, 0}, sleeps); diff != "" {
			t.Errorf("zero delays must still sleep (-want +got):\n%s", diff)
		}
	})

	t.Run("a permanent error stops at once", func(t *testing.T) {
		errBad := errors.New("bad request")
		var sleeps []time.Duration
		var calls int
		fn := func(context.Context) error { calls++; return funcs.Permanent(errBad) }
		err := funcs.Retry(t.Context(), 5, backoff, recordSleep(&sleeps), fn)

		if calls != 1 {
			t.Errorf("fn called %d times, want 1", calls)
		}
		if !errors.Is(err, errBad) {
			t.Errorf("err = %v, want it to match errBad", err)
		}
		if len(sleeps) != 0 {
			t.Errorf("slept %v before giving up on a permanent error", sleeps)
		}
		if funcs.Permanent(nil) != nil {
			t.Errorf("Permanent(nil) must be nil")
		}
	})

	t.Run("attempts below one is rejected without calling fn", func(t *testing.T) {
		for _, n := range []int{0, -1} {
			var calls int
			fn := func(context.Context) error { calls++; return nil }
			err := funcs.Retry(t.Context(), n, backoff, recordSleep(new([]time.Duration)), fn)
			if !errors.Is(err, funcs.ErrInvalidAttempts) {
				t.Errorf("attempts=%d: err = %v, want ErrInvalidAttempts", n, err)
			}
			if calls != 0 {
				t.Errorf("attempts=%d: fn called %d times, want 0", n, calls)
			}
		}
	})

	t.Run("a done context stops retrying before the next attempt", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		var calls int
		fn := func(context.Context) error {
			calls++
			cancel()
			return errTransient
		}
		noSleep := func(context.Context, time.Duration) error { return nil }
		err := funcs.Retry(ctx, 5, backoff, noSleep, fn)

		if !errors.Is(err, context.Canceled) {
			t.Errorf("err = %v, want context.Canceled joined in", err)
		}
		if !errors.Is(err, errTransient) {
			t.Errorf("err = %v, want the last attempt's error joined in", err)
		}
		if calls != 1 {
			t.Errorf("fn called %d times, want 1", calls)
		}
	})

	t.Run("a sleep error is joined with the last attempt's error", func(t *testing.T) {
		errSleep := errors.New("interrupted")
		sleep := func(context.Context, time.Duration) error { return errSleep }
		var calls int
		fn := func(context.Context) error { calls++; return errTransient }
		err := funcs.Retry(t.Context(), 3, backoff, sleep, fn)

		if !errors.Is(err, errSleep) || !errors.Is(err, errTransient) {
			t.Errorf("err = %v, want both the sleep error and errTransient", err)
		}
		if calls != 1 {
			t.Errorf("fn called %d times, want 1", calls)
		}
	})

	t.Run("fn receives the caller's context", func(t *testing.T) {
		type key struct{}
		ctx := context.WithValue(t.Context(), key{}, "v")
		var calls int
		var got any
		fn := func(c context.Context) error {
			calls++
			got = c.Value(key{})
			return nil
		}
		if err := funcs.Retry(ctx, 1, backoff, recordSleep(new([]time.Duration)), fn); err != nil {
			t.Fatalf("Retry = %v, want nil", err)
		}
		if calls != 1 || got != "v" {
			t.Errorf("calls=%d ctx value=%v; want 1 call carrying \"v\"", calls, got)
		}
	})
}

func TestPart5Timed(t *testing.T) {
	t0 := time.Unix(1_700_000_000, 0)

	t.Run("returns results unchanged and logs once", func(t *testing.T) {
		clk := &fakeClock{now: t0, step: 250 * time.Millisecond}
		var lines []string
		logf := func(format string, args ...any) {
			lines = append(lines, fmt.Sprintf(format, args...))
		}
		square := funcs.Timed("square", clk.Now, logf, func(n int) (int, error) { return n * n, nil })

		got, err := square(6)
		if err != nil || got != 36 {
			t.Errorf("square(6) = %d, %v; want 36, <nil>", got, err)
		}
		if diff := cmp.Diff([]string{"square took 250ms err=<nil>"}, lines); diff != "" {
			t.Errorf("log lines mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("logs the error and returns it unchanged", func(t *testing.T) {
		clk := &fakeClock{now: t0, step: 250 * time.Millisecond}
		var lines []string
		logf := func(format string, args ...any) {
			lines = append(lines, fmt.Sprintf(format, args...))
		}
		errBoom := errors.New("boom")
		fetch := funcs.Timed("fetch", clk.Now, logf, func(id string) (string, error) {
			return "", errBoom
		})

		_, err := fetch("x")
		if err != errBoom {
			t.Errorf("err = %v, want the original errBoom", err)
		}
		if diff := cmp.Diff([]string{"fetch took 250ms err=boom"}, lines); diff != "" {
			t.Errorf("log lines mismatch (-want +got):\n%s", diff)
		}
	})
}
