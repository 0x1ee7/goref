package slicesmaps_test

import (
	"errors"
	"testing"
	"time"

	"goref/exercises/01_slices_and_maps"
)

func newLimiter(t *testing.T, limit int, window time.Duration, clk *fakeClock) *slicesmaps.SlidingWindowLimiter {
	t.Helper()
	l, err := slicesmaps.NewSlidingWindowLimiter(limit, window, clk.Now)
	if err != nil {
		t.Fatalf("NewSlidingWindowLimiter(%d, %v): %v", limit, window, err)
	}
	return l
}

func TestPart4Constructor(t *testing.T) {
	clk := newClock()
	tests := []struct {
		name    string
		limit   int
		window  time.Duration
		wantErr bool
	}{
		{"zero limit", 0, time.Minute, true},
		{"negative limit", -1, time.Minute, true},
		{"zero window", 3, 0, true},
		{"negative window", 3, -time.Second, true},
		{"valid", 3, time.Minute, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			l, err := slicesmaps.NewSlidingWindowLimiter(tc.limit, tc.window, clk.Now)
			if tc.wantErr {
				if !errors.Is(err, slicesmaps.ErrInvalidConfig) {
					t.Fatalf("constructor error = %v, want ErrInvalidConfig", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("constructor: unexpected error %v", err)
			}
			if !l.Allow("a") {
				t.Fatal("first request on a fresh limiter was denied")
			}
		})
	}
}

func TestPart4AllowUpToLimit(t *testing.T) {
	t.Run("admits limit requests then denies", func(t *testing.T) {
		l := newLimiter(t, 3, time.Minute, newClock())
		for i := 1; i <= 3; i++ {
			if !l.Allow("a") {
				t.Fatalf("request %d denied, want admitted", i)
			}
		}
		if l.Allow("a") {
			t.Fatal("4th request admitted, want denied")
		}
	})
	t.Run("denied requests are not recorded", func(t *testing.T) {
		clk := newClock()
		l := newLimiter(t, 2, 10*time.Second, clk)
		if !l.Allow("a") || !l.Allow("a") {
			t.Fatal("first two requests should be admitted")
		}
		clk.Advance(5 * time.Second)
		if l.Allow("a") {
			t.Fatal("request at t+5s admitted, want denied")
		}
		clk.Advance(5 * time.Second) // t+10s: both admitted events have left
		if got := l.Remaining("a"); got != 2 {
			t.Fatalf("Remaining(a) = %d at t+10s, want 2: denied request must not be recorded", got)
		}
	})
	t.Run("window slides instead of resetting", func(t *testing.T) {
		clk := newClock()
		l := newLimiter(t, 2, 10*time.Second, clk)
		if !l.Allow("a") {
			t.Fatal("request at t+0 denied")
		}
		clk.Advance(6 * time.Second)
		if !l.Allow("a") {
			t.Fatal("request at t+6s denied")
		}
		clk.Advance(4 * time.Second) // t+10s: only the t+0 event has left
		if !l.Allow("a") {
			t.Fatal("request at t+10s denied: oldest event should have left the window")
		}
		if l.Allow("a") {
			t.Fatal("window should be full again at t+10s")
		}
	})
	t.Run("clients are independent", func(t *testing.T) {
		l := newLimiter(t, 1, time.Minute, newClock())
		if !l.Allow("a") {
			t.Fatal("client a first request denied")
		}
		if !l.Allow("b") {
			t.Fatal("client b first request denied: clients must not share a budget")
		}
		if l.Allow("a") {
			t.Fatal("client a second request admitted, want denied")
		}
	})
	t.Run("exact window boundary has left", func(t *testing.T) {
		clk := newClock()
		l := newLimiter(t, 1, 10*time.Second, clk)
		if !l.Allow("a") {
			t.Fatal("first request denied")
		}
		clk.Advance(10*time.Second - time.Nanosecond)
		if l.Allow("a") {
			t.Fatal("event 1ns younger than window has already left")
		}
		clk.Advance(time.Nanosecond)
		if !l.Allow("a") {
			t.Fatal("event exactly window old still counted")
		}
	})
}

func TestPart4Remaining(t *testing.T) {
	t.Run("unseen client has full allowance", func(t *testing.T) {
		l := newLimiter(t, 3, time.Minute, newClock())
		if got := l.Remaining("nobody"); got != 3 {
			t.Fatalf("Remaining(nobody) = %d, want 3", got)
		}
	})
	t.Run("decreases with admitted requests", func(t *testing.T) {
		l := newLimiter(t, 3, time.Minute, newClock())
		if !l.Allow("a") {
			t.Fatal("first request denied")
		}
		if got := l.Remaining("a"); got != 2 {
			t.Fatalf("Remaining(a) = %d, want 2", got)
		}
		if !l.Allow("a") {
			t.Fatal("second request denied")
		}
		if got := l.Remaining("a"); got != 1 {
			t.Fatalf("Remaining(a) = %d, want 1", got)
		}
	})
	t.Run("queries do not create state", func(t *testing.T) {
		clk := newClock()
		l := newLimiter(t, 2, 10*time.Second, clk)
		if !l.Allow("a") {
			t.Fatal("request for a denied")
		}
		if got := l.Remaining("ghost"); got != 2 {
			t.Fatalf("Remaining(ghost) = %d, want 2", got)
		}
		_ = l.RetryAfter("ghost")
		clk.Advance(10 * time.Second)
		if got := l.Prune(); got != 1 {
			t.Fatalf("Prune() dropped %d clients, want 1: queries must not create state for ghost", got)
		}
	})
}

func TestPart4RetryAfter(t *testing.T) {
	t.Run("zero while capacity remains", func(t *testing.T) {
		l := newLimiter(t, 2, 10*time.Second, newClock())
		if !l.Allow("a") {
			t.Fatal("first request denied")
		}
		if got := l.Remaining("a"); got != 1 {
			t.Fatalf("Remaining(a) = %d, want 1", got)
		}
		if got := l.RetryAfter("a"); got != 0 {
			t.Fatalf("RetryAfter(a) = %v, want 0", got)
		}
	})
	t.Run("time until oldest event leaves", func(t *testing.T) {
		clk := newClock()
		l := newLimiter(t, 2, 10*time.Second, clk)
		if !l.Allow("a") {
			t.Fatal("request at t+0 denied")
		}
		clk.Advance(4 * time.Second)
		if !l.Allow("a") {
			t.Fatal("request at t+4s denied")
		}
		if got := l.Remaining("a"); got != 0 {
			t.Fatalf("Remaining(a) = %d at t+4s, want 0", got)
		}
		if got, want := l.RetryAfter("a"), 6*time.Second; got != want {
			t.Fatalf("RetryAfter(a) = %v at t+4s, want %v", got, want)
		}
	})
}

func TestPart4Prune(t *testing.T) {
	t.Run("drops clients with no live events", func(t *testing.T) {
		clk := newClock()
		l := newLimiter(t, 2, 10*time.Second, clk)
		if !l.Allow("a") || !l.Allow("b") {
			t.Fatal("initial requests denied")
		}
		clk.Advance(5 * time.Second)
		if !l.Allow("b") {
			t.Fatal("b request at t+5s denied")
		}
		clk.Advance(5 * time.Second) // a has no live events; b still has one
		if got := l.Prune(); got != 1 {
			t.Fatalf("Prune() = %d, want 1", got)
		}
		if got := l.Prune(); got != 0 {
			t.Fatalf("second Prune() = %d, want 0", got)
		}
	})
	t.Run("keeps clients with live events", func(t *testing.T) {
		l := newLimiter(t, 2, time.Minute, newClock())
		if !l.Allow("a") {
			t.Fatal("request denied")
		}
		if got := l.Prune(); got != 0 {
			t.Fatalf("Prune() = %d, want 0", got)
		}
		if got := l.Remaining("a"); got != 1 {
			t.Fatalf("Remaining(a) = %d after Prune, want 1", got)
		}
	})
}

func TestPart4RingBuffer(t *testing.T) {
	t.Run("long run keeps admitting at a steady rate", func(t *testing.T) {
		clk := newClock()
		l := newLimiter(t, 2, 10*time.Second, clk)
		for i := range 1000 {
			if !l.Allow("a") {
				t.Fatalf("request %d denied at steady rate of one per 5s", i)
			}
			clk.Advance(5 * time.Second)
		}
	})
	t.Run("partial expiry frees only old slots", func(t *testing.T) {
		clk := newClock()
		l := newLimiter(t, 3, 10*time.Second, clk)
		if !l.Allow("a") {
			t.Fatal("request at t+0 denied")
		}
		clk.Advance(2 * time.Second)
		if !l.Allow("a") {
			t.Fatal("request at t+2s denied")
		}
		clk.Advance(2 * time.Second)
		if !l.Allow("a") {
			t.Fatal("request at t+4s denied")
		}
		if l.Allow("a") {
			t.Fatal("full window admitted a request at t+4s")
		}
		clk.Advance(6 * time.Second) // t+10s: only the t+0 event has left
		if got := l.Remaining("a"); got != 1 {
			t.Fatalf("Remaining(a) at t+10s = %d, want 1", got)
		}
		if !l.Allow("a") {
			t.Fatal("request at t+10s denied")
		}
		if l.Allow("a") {
			t.Fatal("window full again but request admitted")
		}
	})
}
