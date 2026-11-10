package support

import (
	"context"
	"errors"
	"fmt"
	"syscall"
	"time"
)

// transientErrnos are the OS errors that usually clear up on their own.
var transientErrnos = []syscall.Errno{
	syscall.EAGAIN,
	syscall.EBUSY,
	syscall.EINTR,
	syscall.ECONNABORTED,
	syscall.ECONNREFUSED,
	syscall.ECONNRESET,
}

// IsTransient reports whether retrying the operation that produced err might
// succeed.
//
// The first error in the chain that implements Transient() bool decides the
// answer. Otherwise a timeout (Timeout() bool true) or one of the connection
// and resource errnos (EAGAIN, EBUSY, EINTR, ECONNABORTED, ECONNREFUSED,
// ECONNRESET) is transient. context.Canceled and context.DeadlineExceeded are
// never transient, even though DeadlineExceeded reports Timeout. A nil error is
// not transient.
func IsTransient(err error) bool {
	if err == nil {
		return false
	}
	// Checked first: a context error must not be retried, even when a caller
	// has marked its wrapper as transient.
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var tr interface{ Transient() bool }
	if errors.As(err, &tr) {
		return tr.Transient()
	}
	var to interface{ Timeout() bool }
	if errors.As(err, &to) && to.Timeout() {
		return true
	}
	for _, e := range transientErrnos {
		if errors.Is(err, e) {
			return true
		}
	}
	return false
}

// RetryExhausted reports that Retry gave up on a transient error. Err is the
// last error and is reachable through Unwrap.
type RetryExhausted struct {
	Attempts int
	Err      error
}

// Error renders the attempt count and the last error.
func (e *RetryExhausted) Error() string {
	return fmt.Sprintf("gave up after %d attempts: %v", e.Attempts, e.Err)
}

// Unwrap returns the last error.
func (e *RetryExhausted) Unwrap() error { return e.Err }

// RetryConfig controls Retry.
type RetryConfig struct {
	// Attempts is the total number of calls, including the first. It must be
	// at least 1.
	Attempts int
	// BaseDelay is the sleep after the first failed attempt. It doubles after
	// each further failure.
	BaseDelay time.Duration
	// Sleep waits between attempts. Nil means time.Sleep.
	Sleep func(time.Duration)
	// Classify reports whether an error is worth retrying. Nil means
	// IsTransient.
	Classify func(error) bool
}

// Retry calls fn until it succeeds, fails permanently, or the attempts run
// out.
//
// A permanent error is returned unchanged and immediately. A transient error
// on the last attempt is returned as a *RetryExhausted. The result on failure
// is the zero T. Attempts below 1 is an error and fn is not called.
func Retry[T any](cfg RetryConfig, fn func() (T, error)) (T, error) {
	var zero T
	if cfg.Attempts < 1 {
		return zero, fmt.Errorf("retry: attempts must be at least 1, got %d", cfg.Attempts)
	}
	sleep := cfg.Sleep
	if sleep == nil {
		sleep = time.Sleep
	}
	classify := cfg.Classify
	if classify == nil {
		classify = IsTransient
	}

	delay := cfg.BaseDelay
	for attempt := 1; ; attempt++ {
		v, err := fn()
		if err == nil {
			return v, nil
		}
		if !classify(err) {
			return zero, err
		}
		if attempt == cfg.Attempts {
			return zero, &RetryExhausted{Attempts: attempt, Err: err}
		}
		sleep(delay)
		delay *= 2
	}
}
