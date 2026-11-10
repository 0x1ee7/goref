package scope

import (
	"fmt"
	"sync"
	"time"
)

// clock is the package time source. Tests replace it through SetClock.
var clock = time.Now

// SetClock replaces the package clock read by Now and ExpiresAt. It returns a
// restore function that puts back the clock that was in place when SetClock
// was called. Calling restore more than once is harmless. A nil now selects
// time.Now. SetClock is not safe to call while other goroutines call Now.
func SetClock(now func() time.Time) (restore func()) {
	if now == nil {
		now = time.Now
	}
	prev := clock
	clock = now
	var once sync.Once
	return func() {
		once.Do(func() { clock = prev })
	}
}

// Now returns the time according to the package clock.
func Now() time.Time { return clock() }

// ExpiresAt returns Now() plus ttl. A non-positive ttl returns ErrInvalidArgument.
func ExpiresAt(ttl time.Duration) (time.Time, error) {
	if ttl <= 0 {
		return time.Time{}, fmt.Errorf("%w: ttl %v", ErrInvalidArgument, ttl)
	}
	return Now().Add(ttl), nil
}

// Issuer computes token expiries from an injected clock.
type Issuer struct {
	now func() time.Time
}

// NewIssuer returns an Issuer that reads time from now. A nil now means the
// package clock (Now), so SetClock applies to it.
func NewIssuer(now func() time.Time) *Issuer {
	if now == nil {
		now = Now
	}
	return &Issuer{now: now}
}

// ExpiresAt returns the issuer clock's current time plus ttl. A non-positive
// ttl returns ErrInvalidArgument.
func (i *Issuer) ExpiresAt(ttl time.Duration) (time.Time, error) {
	if ttl <= 0 {
		return time.Time{}, fmt.Errorf("%w: ttl %v", ErrInvalidArgument, ttl)
	}
	return i.now().Add(ttl), nil
}
