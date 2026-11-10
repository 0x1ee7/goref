package funcs

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ErrInvalidAttempts is returned by Retry when attempts is less than one.
var ErrInvalidAttempts = errors.New("funcs: attempts must be >= 1")

// Backoff returns the delay to wait after the given failed attempt. attempt is
// 1-based.
type Backoff func(attempt int) time.Duration

// Sleeper waits for d or until ctx is done. Injecting it keeps retries testable
// without real time.
type Sleeper func(ctx context.Context, d time.Duration) error

type permanentError struct{ err error }

func (p *permanentError) Error() string { return p.err.Error() }
func (p *permanentError) Unwrap() error { return p.err }

// Permanent marks err as not worth retrying. Retry returns it immediately.
// errors.Is and errors.As still see the wrapped error. Permanent(nil) is nil.
func Permanent(err error) error {
	if err == nil {
		return nil
	}
	return &permanentError{err: err}
}

// Retry calls fn up to attempts times. Between failed attempts it calls
// sleep(ctx, backoff(n)), including for zero delays. It does not sleep after a
// success or after the last attempt. A Permanent error stops the loop at once.
// If ctx is done before another attempt, Retry returns the last error joined
// with ctx.Err() and does not call fn. Exhaustion is reported as
// "retry: gave up after N attempts" wrapping the last error.
func Retry(ctx context.Context, attempts int, backoff Backoff, sleep Sleeper, fn func(context.Context) error) error {
	if attempts < 1 {
		return ErrInvalidAttempts
	}
	for attempt := 1; ; attempt++ {
		err := fn(ctx)
		if err == nil {
			return nil
		}
		var perm *permanentError
		if errors.As(err, &perm) {
			return err
		}
		if attempt == attempts {
			return fmt.Errorf("retry: gave up after %d attempts: %w", attempts, err)
		}
		if serr := sleep(ctx, backoff(attempt)); serr != nil {
			return errors.Join(err, serr)
		}
		if cerr := ctx.Err(); cerr != nil {
			return errors.Join(err, cerr)
		}
	}
}

// Timed wraps fn so each call logs its name, elapsed time, and error once. The
// results are returned unchanged. A panic propagates without a log line.
func Timed[In, Out any](name string, clock func() time.Time, logf func(format string, args ...any), fn func(In) (Out, error)) func(In) (Out, error) {
	return func(in In) (Out, error) {
		start := clock()
		out, err := fn(in)
		logf("%s took %v err=%v", name, clock().Sub(start), err)
		return out, err
	}
}
