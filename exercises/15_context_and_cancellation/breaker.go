package ctxflow

import (
	"context"
	"errors"
	"time"
)

// ErrOpen is returned by Breaker.Do when the call is rejected without running.
var ErrOpen = errors.New("circuit open")

// BreakerState is the state of a Breaker.
type BreakerState int

const (
	// StateClosed lets every call through and counts consecutive failures.
	StateClosed BreakerState = iota
	// StateOpen rejects every call until the cooldown has passed.
	StateOpen
	// StateHalfOpen admits exactly one probe call at a time.
	StateHalfOpen
)

// String returns "closed", "open" or "half-open".
func (s BreakerState) String() string { return "" }

// Breaker is a circuit breaker: closed, open, and half-open with a single probe.
type Breaker struct{}

// NewBreaker opens after threshold consecutive failures and stays open for
// cooldown, reading time from now (nil means time.Now). threshold < 1 or
// cooldown < 0 wraps ErrInvalidArgument.
func NewBreaker(threshold int, cooldown time.Duration, now func() time.Time) (*Breaker, error) {
	return nil, nil
}

// State reports the current state. An open breaker whose cooldown has elapsed
// reads as StateHalfOpen.
func (b *Breaker) State() BreakerState { return StateClosed }

// Do runs op if the breaker admits it, and returns op's error unchanged.
//
// Any error counts as a failure, except errors caused by ctx being done: such a
// call is neutral and changes no failure count. A rejected call returns ErrOpen
// without calling op. In half-open state, a probe that succeeds closes the
// breaker, and a probe that fails reopens it with a fresh cooldown. A neutral
// probe leaves the breaker half-open so another probe may run. If ctx is already
// done, Do returns context.Cause(ctx) without calling op.
func (b *Breaker) Do(ctx context.Context, op func(ctx context.Context) error) error {
	return nil
}
