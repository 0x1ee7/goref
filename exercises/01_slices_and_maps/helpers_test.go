package slicesmaps_test

import "time"

// fakeClock is a deterministic clock for TTL and sliding-window tests.
type fakeClock struct{ now time.Time }

func newClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
}

// Now returns the current fake time.
func (c *fakeClock) Now() time.Time { return c.now }

// Advance moves the fake time forward by d.
func (c *fakeClock) Advance(d time.Duration) { c.now = c.now.Add(d) }

// isEven is a shared predicate for FilterInPlace tests.
func isEven(n int) bool { return n%2 == 0 }
