package funcs

import (
	"context"
	"errors"
	"time"
)

// ErrInvalidAttempts is returned by Retry when attempts is less than 1.
var ErrInvalidAttempts = errors.New("funcs: attempts must be >= 1")

// Backoff returns the delay after a failed attempt. attempt is 1-based: it is
// the number of the attempt that just failed.
type Backoff func(attempt int) time.Duration

// Sleeper waits for d or until ctx is done, returning a non-nil error if the
// wait was cut short. Production code passes a timer-based sleeper; tests
// inject a fake.
type Sleeper func(ctx context.Context, d time.Duration) error

// Permanent marks err as not worth retrying. Retry returns it immediately. The
// returned error still matches err with errors.Is and errors.As. Permanent(nil)
// is nil.
func Permanent(err error) error { return nil }

// Retry calls fn(ctx) up to attempts times and returns nil on the first
// success.
//   - attempts < 1 returns ErrInvalidAttempts without calling fn.
//   - A Permanent error is returned immediately, with no sleep.
//   - After a retryable failure on attempt i < attempts, sleep(ctx, backoff(i))
//     is called, even when the delay is 0. It is never called after success or
//     after the last attempt. If sleep returns an error, Retry returns
//     errors.Join(lastErr, sleepErr).
//   - Before each attempt after the first, a done ctx returns
//     errors.Join(lastErr, ctx.Err()) without calling fn.
//   - When all attempts fail, the result is "retry: gave up after N attempts: %w"
//     wrapping the last error.
func Retry(ctx context.Context, attempts int, backoff Backoff, sleep Sleeper, fn func(context.Context) error) error {
	return nil
}

// Timed wraps fn so that each call reads clock before and after the call and
// logs once with logf("%s took %v err=%v", name, elapsed, err). Results are
// returned unchanged. Panics propagate without logging.
func Timed[In, Out any](name string, clock func() time.Time, logf func(format string, args ...any), fn func(In) (Out, error)) func(In) (Out, error) {
	return func(in In) (Out, error) {
		var zero Out
		return zero, nil
	}
}
