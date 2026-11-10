package ctxflow

import (
	"context"
	"time"
)

// TokenBucket admits events at a sustained rate with bursts of up to burst.
// It starts full. Refill is computed lazily from the injected clock, so no
// background goroutine or ticker is needed.
type TokenBucket struct{}

// NewTokenBucket returns a bucket that refills at rate tokens per second, holds
// at most burst tokens, and reads time from now (nil means time.Now).
// rate <= 0 or burst < 1 wraps ErrInvalidArgument.
func NewTokenBucket(rate float64, burst int, now func() time.Time) (*TokenBucket, error) {
	return nil, nil
}

// Allow takes one token if one is available and reports whether it did. It never blocks.
func (b *TokenBucket) Allow() bool { return false }

// Wait blocks until one token is available and takes it. The sleep is a real
// timer sized to the deficit, so Wait is meaningful with the real clock (or
// under testing/synctest), not with a frozen injected clock. If ctx ends first
// it returns context.Cause(ctx) and takes no token.
func (b *TokenBucket) Wait(ctx context.Context) error { return nil }
