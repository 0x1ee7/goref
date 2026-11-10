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
	return time.Now()
}

// Stopwatch measures elapsed time between Start and Stop. The zero value is
// ready to use and reads the SystemClock when Clock is nil.
type Stopwatch struct {
	// Clock is the source of time. Nil means SystemClock{}.
	Clock Clock

	running bool
	started time.Time
	last    time.Duration
}

func (s *Stopwatch) clock() Clock {
	if s.Clock == nil {
		return SystemClock{}
	}
	return s.Clock
}

// Start begins a measurement. It returns [ErrAlreadyRunning] if the stopwatch is running.
func (s *Stopwatch) Start() error {
	if s.running {
		return ErrAlreadyRunning
	}
	s.started = s.clock().Now()
	s.running = true
	return nil
}

// Stop ends the measurement and returns the time since Start. It returns
// [ErrNotRunning] if the stopwatch is not running.
func (s *Stopwatch) Stop() (time.Duration, error) {
	if !s.running {
		return 0, ErrNotRunning
	}
	s.last = s.clock().Now().Sub(s.started)
	s.running = false
	return s.last, nil
}

// Elapsed returns the live reading while running, the value from the last Stop
// after stopping, and 0 before the first Start.
func (s *Stopwatch) Elapsed() time.Duration {
	if s.running {
		return s.clock().Now().Sub(s.started)
	}
	return s.last
}
