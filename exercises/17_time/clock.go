// Part 7: injected clocks and a monotonic stopwatch.
package timeutil

import "time"

// Clock is the injected source of "now". Production code passes SystemClock{};
// tests pass a fake.
type Clock interface {
	Now() time.Time
}

// SystemClock reads the real clock. Its readings carry a monotonic reading, so
// differences between them are unaffected by wall-clock adjustments.
type SystemClock struct{}

// Now returns the current time.
func (SystemClock) Now() time.Time {
	return time.Time{}
}

// Stopwatch measures elapsed time between Start and Stop. The zero value is
// ready to use and reads the SystemClock when Clock is nil.
type Stopwatch struct {
	// Clock is the source of time. Nil means SystemClock{}.
	Clock Clock
	// Engineer-owned: running state, start reading, last measurement.
}

// Start begins a measurement. It returns [ErrAlreadyRunning] if the stopwatch is running.
func (s *Stopwatch) Start() error {
	return errNotImplemented
}

// Stop ends the measurement and returns the time since Start. It returns
// [ErrNotRunning] if the stopwatch is not running.
func (s *Stopwatch) Stop() (time.Duration, error) {
	return 0, errNotImplemented
}

// Elapsed returns the live reading while running, the value from the last Stop
// after stopping, and 0 before the first Start.
func (s *Stopwatch) Elapsed() time.Duration {
	return 0
}
