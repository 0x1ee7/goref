package ctxflow

import (
	"context"
	"errors"
	"math/rand/v2"
	"time"
)

// Retrier retries an operation with capped exponential backoff.
//
// Before retry n (n >= 1) it waits Jitter(nominal), where nominal is
// min(Max, Base*2^(n-1)). With a nil Jitter it uses full jitter: a uniform
// random wait in [0, nominal). Attempts below 1 are treated as 1. The zero
// Retrier makes a single attempt.
type Retrier struct {
	Attempts int
	Base     time.Duration
	Max      time.Duration
	// Retryable reports whether an error is worth another attempt. A nil
	// Retryable retries every error.
	Retryable func(error) bool
	// Jitter maps a nominal wait to the actual wait. A nil Jitter means full jitter.
	Jitter func(nominal time.Duration) time.Duration
}

// Do calls op until it succeeds, the error is not retryable, or the attempts
// run out. A context that is already done before an attempt returns
// context.Cause(ctx) without calling op. If the context ends during a wait,
// Do returns the last error joined with the cause.
func (r Retrier) Do(ctx context.Context, op func(ctx context.Context) error) error {
	for attempt := 1; ; attempt++ {
		if ctx.Err() != nil {
			return context.Cause(ctx)
		}
		opErr := op(ctx)
		if opErr == nil {
			return nil
		}
		if r.Retryable != nil && !r.Retryable(opErr) {
			return opErr
		}
		if attempt >= r.Attempts {
			return opErr
		}
		if err := sleep(ctx, r.delay(attempt)); err != nil {
			return errors.Join(opErr, err)
		}
	}
}

// delay returns the wait before retry number attempt (1-based).
func (r Retrier) delay(attempt int) time.Duration {
	nominal := r.Base
	for range attempt - 1 {
		// Stop doubling once the next step would reach Max. This also
		// keeps very large attempt counts from overflowing.
		if nominal >= r.Max/2 {
			nominal = r.Max
			break
		}
		nominal *= 2
	}
	nominal = min(nominal, r.Max)
	if r.Jitter != nil {
		return r.Jitter(nominal)
	}
	if nominal <= 0 {
		return 0
	}
	return rand.N(nominal)
}

// sleep waits for d or until ctx ends, whichever comes first.
func sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return context.Cause(ctx)
	}
}
