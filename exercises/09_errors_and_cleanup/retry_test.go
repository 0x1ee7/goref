package support_test

import (
	"context"
	"errors"
	"fmt"
	"syscall"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	support "goref/exercises/09_errors_and_cleanup"
)

// overrideError lets a test pick the Transient() and Timeout() answers.
type overrideError struct {
	msg       string
	transient bool
	timeout   bool
}

func (e overrideError) Error() string   { return e.msg }
func (e overrideError) Transient() bool { return e.transient }
func (e overrideError) Timeout() bool   { return e.timeout }

func TestPart6IsTransient(t *testing.T) {
	t.Run("timeouts are transient, plain errors are not", func(t *testing.T) {
		if !support.IsTransient(timeoutError{"slow"}) {
			t.Errorf("IsTransient(timeout) = false, want true")
		}
		if support.IsTransient(errors.New("boom")) {
			t.Errorf("IsTransient(plain) = true, want false")
		}
	})

	t.Run("nil is not transient", func(t *testing.T) {
		if !support.IsTransient(timeoutError{"slow"}) {
			t.Errorf("IsTransient(timeout) = false, want true")
		}
		if support.IsTransient(nil) {
			t.Errorf("IsTransient(nil) = true, want false")
		}
	})

	t.Run("explicit Transient() beats Timeout()", func(t *testing.T) {
		if support.IsTransient(overrideError{msg: "x", transient: false, timeout: true}) {
			t.Errorf("Transient()=false must win over Timeout()=true")
		}
		if !support.IsTransient(overrideError{msg: "x", transient: true, timeout: false}) {
			t.Errorf("Transient()=true must make the error transient")
		}
	})

	t.Run("errnos are transient through wrapping", func(t *testing.T) {
		errnos := []syscall.Errno{
			syscall.EAGAIN, syscall.EBUSY, syscall.EINTR,
			syscall.ECONNRESET, syscall.ECONNREFUSED, syscall.ECONNABORTED,
		}
		for _, e := range errnos {
			t.Run(e.Error(), func(t *testing.T) {
				if !support.IsTransient(fmt.Errorf("dial: %w", e)) {
					t.Errorf("IsTransient(wrapped %v) = false, want true", e)
				}
			})
		}
	})

	t.Run("context errors are never transient", func(t *testing.T) {
		if support.IsTransient(context.Canceled) || support.IsTransient(context.DeadlineExceeded) {
			t.Errorf("context errors must not be transient")
		}
		if !support.IsTransient(timeoutError{"slow"}) {
			t.Errorf("IsTransient(timeout) = false, want true")
		}
	})

	t.Run("join is searched", func(t *testing.T) {
		if !support.IsTransient(errors.Join(errors.New("boom"), timeoutError{"slow"})) {
			t.Errorf("joined transient error = false, want true")
		}
		if support.IsTransient(errors.Join(errors.New("a"), errors.New("b"))) {
			t.Errorf("joined permanent errors = true, want false")
		}
	})
}

func TestPart6Retry(t *testing.T) {
	var sleeps []time.Duration
	sleep := func(d time.Duration) { sleeps = append(sleeps, d) }
	cfg := func(attempts int) support.RetryConfig {
		sleeps = nil
		return support.RetryConfig{Attempts: attempts, BaseDelay: 10 * time.Millisecond, Sleep: sleep}
	}

	t.Run("transient failures then success, with doubling backoff", func(t *testing.T) {
		calls := 0
		got, err := support.Retry(cfg(3), func() (string, error) {
			calls++
			if calls < 3 {
				return "", timeoutError{"slow"}
			}
			return "ok", nil
		})
		if err != nil {
			t.Fatalf("Retry() error = %v", err)
		}
		if got != "ok" {
			t.Errorf("Retry() = %q, want %q", got, "ok")
		}
		if calls != 3 {
			t.Errorf("calls = %d, want 3", calls)
		}
		want := []time.Duration{10 * time.Millisecond, 20 * time.Millisecond}
		if diff := cmp.Diff(want, sleeps); diff != "" {
			t.Errorf("sleeps mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("permanent error is returned unchanged at once", func(t *testing.T) {
		boom := errors.New("boom")
		calls := 0
		_, err := support.Retry(cfg(5), func() (string, error) {
			calls++
			return "", boom
		})
		if err != boom {
			t.Fatalf("Retry() error = %v, want the permanent error unchanged", err)
		}
		if calls != 1 {
			t.Errorf("calls = %d, want 1", calls)
		}
		if len(sleeps) != 0 {
			t.Errorf("slept %v, want no sleep for a permanent error", sleeps)
		}
	})

	t.Run("exhausted transient errors become RetryExhausted", func(t *testing.T) {
		want := timeoutError{"busy"}
		calls := 0
		_, err := support.Retry(cfg(3), func() (string, error) {
			calls++
			return "", want
		})
		ex, ok := errors.AsType[*support.RetryExhausted](err)
		if !ok {
			t.Fatalf("error = %v (%T), want *RetryExhausted", err, err)
		}
		if ex.Attempts != 3 {
			t.Errorf("Attempts = %d, want 3", ex.Attempts)
		}
		if !errors.Is(err, want) {
			t.Errorf("errors.Is(err, last error) = false, want true")
		}
		if calls != 3 {
			t.Errorf("calls = %d, want 3", calls)
		}
		// No sleep after the last attempt: only between attempts.
		wantSleeps := []time.Duration{10 * time.Millisecond, 20 * time.Millisecond}
		if diff := cmp.Diff(wantSleeps, sleeps); diff != "" {
			t.Errorf("sleeps mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("custom Classify decides what is retried", func(t *testing.T) {
		boom := errors.New("boom")
		calls := 0
		cfg := cfg(3)
		cfg.Classify = func(err error) bool { return errors.Is(err, boom) }
		got, err := support.Retry(cfg, func() (string, error) {
			calls++
			if calls < 3 {
				return "", boom
			}
			return "ok", nil
		})
		if err != nil || got != "ok" {
			t.Fatalf("Retry() = (%q, %v), want (%q, nil)", got, err, "ok")
		}
		if calls != 3 {
			t.Errorf("calls = %d, want 3", calls)
		}
	})

	t.Run("Classify false makes the error permanent", func(t *testing.T) {
		want := timeoutError{"slow"}
		calls := 0
		cfg := cfg(4)
		cfg.Classify = func(error) bool { return false }
		_, err := support.Retry(cfg, func() (string, error) {
			calls++
			return "", want
		})
		if !errors.Is(err, want) {
			t.Fatalf("Retry() error = %v, want the error unchanged", err)
		}
		if _, ok := errors.AsType[*support.RetryExhausted](err); ok {
			t.Errorf("error is *RetryExhausted, want the unchanged error")
		}
		if calls != 1 {
			t.Errorf("calls = %d, want 1", calls)
		}
	})

	t.Run("attempts below one is an error and fn is not called", func(t *testing.T) {
		calls := 0
		_, err := support.Retry(cfg(0), func() (string, error) {
			calls++
			return "ok", nil
		})
		if err == nil {
			t.Fatal("Retry(Attempts: 0) error = nil, want an error")
		}
		if calls != 0 {
			t.Errorf("fn called %d times, want 0", calls)
		}
	})
}
