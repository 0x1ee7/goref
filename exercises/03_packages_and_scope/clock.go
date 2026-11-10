package scope

import (
	"errors"
	"time"
)

// SetClock replaces the package clock read by Now and ExpiresAt. It returns a
// restore function that puts back the clock that was in place when SetClock
// was called. Calling restore more than once is harmless. A nil now selects
// time.Now. SetClock is not safe to call while other goroutines call Now.
func SetClock(now func() time.Time) (restore func()) {
	return func() {}
}

// Now returns the time according to the package clock.
func Now() time.Time { return time.Time{} }

// ExpiresAt returns Now() plus ttl. A non-positive ttl returns ErrInvalidArgument.
func ExpiresAt(ttl time.Duration) (time.Time, error) {
	return time.Time{}, errors.New("not implemented")
}

// Issuer computes token expiries from an injected clock.
type Issuer struct {
	// The unexported fields are yours to declare.
}

// NewIssuer returns an Issuer that reads time from now. A nil now means the
// package clock (Now), so SetClock applies to it.
func NewIssuer(now func() time.Time) *Issuer {
	return &Issuer{}
}

// ExpiresAt returns the issuer clock's current time plus ttl. A non-positive
// ttl returns ErrInvalidArgument.
func (i *Issuer) ExpiresAt(ttl time.Duration) (time.Time, error) {
	return time.Time{}, errors.New("not implemented")
}
