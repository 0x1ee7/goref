package ctxflow

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// tokenEpsilon absorbs float error so that exactly 1/rate of refill is enough
// for one token.
const tokenEpsilon = 1e-9

// TokenBucket is a token-bucket rate limiter. It starts full with burst tokens
// and refills at rate tokens per second, up to burst. It is safe for concurrent
// use.
type TokenBucket struct {
	rate  float64 // tokens per second
	burst float64
	now   func() time.Time

	mu     sync.Mutex
	tokens float64
	last   time.Time
}

// NewTokenBucket returns a limiter that refills at rate tokens per second with
// capacity burst. now supplies the clock, which makes Allow deterministic in
// tests. rate must be positive and burst at least 1.
func NewTokenBucket(rate float64, burst int, now func() time.Time) (*TokenBucket, error) {
	if rate <= 0 || burst < 1 || now == nil {
		return nil, fmt.Errorf("%w: rate must be > 0, burst >= 1, and now non-nil", ErrInvalidArgument)
	}
	return &TokenBucket{
		rate:   rate,
		burst:  float64(burst),
		now:    now,
		tokens: float64(burst),
		last:   now(),
	}, nil
}

// refill adds the tokens earned since the last call. The caller holds mu.
func (tb *TokenBucket) refill() {
	t := tb.now()
	if elapsed := t.Sub(tb.last); elapsed > 0 {
		tb.tokens = min(tb.burst, tb.tokens+elapsed.Seconds()*tb.rate)
	}
	tb.last = t
}

// Allow takes a token if one is available and reports whether it did.
func (tb *TokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.refill()
	if tb.tokens+tokenEpsilon >= 1 {
		tb.tokens--
		return true
	}
	return false
}

// Wait blocks until a token is available, then takes it. It uses the real
// timer for the wait, so it is paced by the clock the bucket was built with
// only when that clock is time.Now. If ctx ends first, it returns
// context.Cause(ctx).
func (tb *TokenBucket) Wait(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return context.Cause(ctx)
		}
		tb.mu.Lock()
		tb.refill()
		if tb.tokens+tokenEpsilon >= 1 {
			tb.tokens--
			tb.mu.Unlock()
			return nil
		}
		need := time.Duration((1 - tb.tokens) / tb.rate * float64(time.Second))
		tb.mu.Unlock()

		t := time.NewTimer(need)
		select {
		case <-t.C:
		case <-ctx.Done():
			t.Stop()
			return context.Cause(ctx)
		}
	}
}
