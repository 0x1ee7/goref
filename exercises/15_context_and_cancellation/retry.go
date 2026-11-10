package ctxflow

import (
	"context"
	"time"
)

// Retrier retries an operation with capped exponential backoff and jitter. The
// zero value is not usable: Attempts must be at least 1.
type Retrier struct {
	// Attempts is the total number of calls, including the first.
	Attempts int
	// Base is the nominal wait before the first retry; it doubles per retry.
	Base time.Duration
	// Max caps the nominal wait of any single retry.
	Max time.Duration
	// Retryable reports whether an error is worth another attempt. nil retries all errors.
	Retryable func(error) bool
	// Jitter maps the nominal wait to the actual wait, which must lie in
	// [0, nominal]. nil selects full jitter (uniform in [0, nominal]).
	Jitter func(nominal time.Duration) time.Duration
}

// Do calls op until it succeeds, a non-retryable error occurs, the attempts run
// out, or ctx ends.
//
// Before retry n (n = 1 for the first retry) Do waits Jitter(min(Max, Base*2^(n-1))),
// and never after the final attempt. A non-retryable error is returned unchanged
// at once. When attempts are exhausted the last error is returned unchanged. If
// ctx is done before an attempt, no attempt is made and the result is
// context.Cause(ctx); if it ends during a wait, the result is
// errors.Join(lastErr, context.Cause(ctx)). Attempts < 1, Base < 0 or Max < 0
// wraps ErrInvalidArgument. An attempt that fails because ctx is done is not
// retried.
func (r Retrier) Do(ctx context.Context, op func(ctx context.Context) error) error {
	return nil
}
