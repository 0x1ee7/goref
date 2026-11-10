package timeutil_test

import (
	"errors"
	"testing"
	"time"

	timeutil "goref/exercises/17_time"
)

// fakeClock is a settable Clock. Its readings have no monotonic component, so
// Stopwatch differences fall back to wall-clock subtraction.
type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time { return c.now }

func TestPart7SystemClock(t *testing.T) {
	t.Run("reads the current time", func(t *testing.T) {
		c := timeutil.SystemClock{}.Now()
		if c.IsZero() {
			t.Fatal("SystemClock.Now() returned the zero time")
		}
		if d := time.Since(c); d < -time.Minute || d > time.Minute {
			t.Errorf("SystemClock.Now() is %v away from real time", d)
		}
	})
}

func TestPart7Stopwatch(t *testing.T) {
	t.Run("elapsed tracks the clock while running and is zero before start", func(t *testing.T) {
		clk := &fakeClock{now: utc(t, "2026-06-01T12:00")}
		sw := &timeutil.Stopwatch{Clock: clk}
		if got := sw.Elapsed(); got != 0 {
			t.Fatalf("Elapsed before Start = %v, want 0", got)
		}
		if err := sw.Start(); err != nil {
			t.Fatalf("Start error = %v", err)
		}
		clk.now = clk.now.Add(2 * time.Second)
		if got := sw.Elapsed(); got != 2*time.Second {
			t.Errorf("Elapsed while running = %v, want 2s", got)
		}
	})

	t.Run("stop freezes the reading", func(t *testing.T) {
		clk := &fakeClock{now: utc(t, "2026-06-01T12:00")}
		sw := &timeutil.Stopwatch{Clock: clk}
		if err := sw.Start(); err != nil {
			t.Fatalf("Start error = %v", err)
		}
		clk.now = clk.now.Add(2500 * time.Millisecond)
		got, err := sw.Stop()
		if err != nil {
			t.Fatalf("Stop error = %v", err)
		}
		if got != 2500*time.Millisecond {
			t.Errorf("Stop = %v, want 2.5s", got)
		}
		clk.now = clk.now.Add(10 * time.Second)
		if e := sw.Elapsed(); e != 2500*time.Millisecond {
			t.Errorf("Elapsed after Stop = %v, want 2.5s", e)
		}
	})

	t.Run("start while running is ErrAlreadyRunning", func(t *testing.T) {
		sw := &timeutil.Stopwatch{Clock: &fakeClock{now: utc(t, "2026-06-01T12:00")}}
		if err := sw.Start(); err != nil {
			t.Fatalf("first Start error = %v", err)
		}
		if err := sw.Start(); !errors.Is(err, timeutil.ErrAlreadyRunning) {
			t.Errorf("second Start error = %v, want ErrAlreadyRunning", err)
		}
	})

	t.Run("stop when not running is ErrNotRunning", func(t *testing.T) {
		sw := &timeutil.Stopwatch{Clock: &fakeClock{now: utc(t, "2026-06-01T12:00")}}
		if _, err := sw.Stop(); !errors.Is(err, timeutil.ErrNotRunning) {
			t.Errorf("Stop before Start error = %v, want ErrNotRunning", err)
		}
	})

	t.Run("restart measures a new interval", func(t *testing.T) {
		clk := &fakeClock{now: utc(t, "2026-06-01T12:00")}
		sw := &timeutil.Stopwatch{Clock: clk}
		if err := sw.Start(); err != nil {
			t.Fatalf("Start error = %v", err)
		}
		clk.now = clk.now.Add(2 * time.Second)
		if _, err := sw.Stop(); err != nil {
			t.Fatalf("Stop error = %v", err)
		}
		if err := sw.Start(); err != nil {
			t.Fatalf("second Start error = %v", err)
		}
		clk.now = clk.now.Add(time.Second)
		if got := sw.Elapsed(); got != time.Second {
			t.Errorf("Elapsed after restart = %v, want 1s", got)
		}
	})

	t.Run("zero value uses the system clock", func(t *testing.T) {
		var sw timeutil.Stopwatch
		if err := sw.Start(); err != nil {
			t.Fatalf("Start error = %v", err)
		}
		time.Sleep(10 * time.Millisecond)
		got, err := sw.Stop()
		if err != nil {
			t.Fatalf("Stop error = %v", err)
		}
		if got < 10*time.Millisecond || got > time.Minute {
			t.Errorf("Stop = %v, want between 10ms and 1m", got)
		}
	})
}
