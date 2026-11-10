package ctxflow

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ErrOpen is returned by Breaker.Do when the call is rejected without running
// the operation: the circuit is open, or a probe is already in flight.
var ErrOpen = errors.New("ctxflow: circuit open")

// BreakerState is the state of a circuit breaker.
type BreakerState int

const (
	// StateClosed lets every call through and counts consecutive failures.
	StateClosed BreakerState = iota
	// StateOpen rejects every call until the cooldown has passed.
	StateOpen
	// StateHalfOpen admits exactly one probe call at a time.
	StateHalfOpen
)

// String returns the lower-case state name.
func (s BreakerState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	}
	return fmt.Sprintf("BreakerState(%d)", int(s))
}

// Breaker stops calling an upstream that keeps failing. It opens after
// threshold consecutive failures, waits cooldown, then admits one probe. A
// probe that succeeds closes it, and one that fails reopens it.
//
// An error caused by the caller's own context ending is neutral. It neither
// counts as a failure nor resets the count. It is safe for concurrent use.
type Breaker struct {
	threshold int
	cooldown  time.Duration
	now       func() time.Time

	mu       sync.Mutex
	state    BreakerState
	failures int
	openedAt time.Time
	probing  bool
}

// NewBreaker returns a closed breaker. threshold must be at least 1 and
// cooldown positive. now supplies the clock.
func NewBreaker(threshold int, cooldown time.Duration, now func() time.Time) (*Breaker, error) {
	if threshold < 1 || cooldown <= 0 || now == nil {
		return nil, fmt.Errorf("%w: threshold must be >= 1, cooldown > 0, and now non-nil", ErrInvalidArgument)
	}
	return &Breaker{threshold: threshold, cooldown: cooldown, now: now}, nil
}

// refresh moves an open breaker to half-open once the cooldown has elapsed.
// The caller holds mu.
func (b *Breaker) refresh() {
	if b.state == StateOpen && !b.now().Before(b.openedAt.Add(b.cooldown)) {
		b.state = StateHalfOpen
		b.probing = false
	}
}

// open trips the breaker and starts a fresh cooldown. The caller holds mu.
func (b *Breaker) open() {
	b.state = StateOpen
	b.openedAt = b.now()
	b.failures = 0
}

// State reports the current state, moving open to half-open when the cooldown
// has elapsed.
func (b *Breaker) State() BreakerState {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.refresh()
	return b.state
}

// Do runs op if the breaker admits the call and records its outcome. It
// returns ErrOpen without running op when the call is rejected, and
// context.Cause(ctx) without running op when ctx is already done.
func (b *Breaker) Do(ctx context.Context, op func(ctx context.Context) error) error {
	if ctx.Err() != nil {
		return context.Cause(ctx)
	}

	b.mu.Lock()
	b.refresh()
	probe := false
	switch b.state {
	case StateOpen:
		b.mu.Unlock()
		return ErrOpen
	case StateHalfOpen:
		if b.probing {
			b.mu.Unlock()
			return ErrOpen
		}
		b.probing = true
		probe = true
	}
	b.mu.Unlock()

	err := op(ctx)

	b.mu.Lock()
	defer b.mu.Unlock()
	if probe {
		b.probing = false
	}
	switch {
	case err == nil:
		if probe || b.state == StateClosed {
			b.state = StateClosed
			b.failures = 0
		}
	case ctx.Err() != nil:
		// Caller-caused: neutral. A neutral probe leaves the breaker half-open.
	case probe:
		b.open()
	default:
		b.failures++
		if b.failures >= b.threshold {
			b.open()
		}
	}
	return err
}
