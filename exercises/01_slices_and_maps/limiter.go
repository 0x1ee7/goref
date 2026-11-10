package slicesmaps

import (
	"errors"
	"time"
)

// SlidingWindowLimiter allows at most limit requests per client in any trailing
// window. It is not safe for concurrent use.
type SlidingWindowLimiter struct{}

// NewSlidingWindowLimiter returns a limiter that allows limit requests per
// client in any window. A nil now uses time.Now. limit < 1 or window <= 0
// yields ErrInvalidConfig.
func NewSlidingWindowLimiter(limit int, window time.Duration, now func() time.Time) (*SlidingWindowLimiter, error) {
	return nil, errors.New("not implemented")
}

// Allow records and admits a request from client if fewer than limit requests
// are live in the window. A denied request is not recorded.
func (l *SlidingWindowLimiter) Allow(client string) bool {
	return false
}

// Remaining returns how many more requests client may make right now.
func (l *SlidingWindowLimiter) Remaining(client string) int {
	return 0
}

// RetryAfter returns how long client must wait before a request is admitted.
// It is zero when Remaining is positive.
func (l *SlidingWindowLimiter) RetryAfter(client string) time.Duration {
	return 0
}

// Prune drops state for clients with no live events and returns how many
// clients were dropped.
func (l *SlidingWindowLimiter) Prune() int {
	return 0
}
