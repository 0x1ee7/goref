package scope_test

import (
	"errors"
	"testing"
	"time"

	"goref/exercises/03_packages_and_scope"
)

// clockOverride installs fn as the package clock until the test ends.
func clockOverride(t *testing.T, fn func() time.Time) {
	t.Helper()
	restore := scope.SetClock(fn)
	t.Cleanup(restore)
}

// checkRealNow fails unless the package clock reads the real time.
func checkRealNow(t *testing.T) {
	t.Helper()
	now := scope.Now()
	d := time.Since(now)
	if d < 0 {
		d = -d
	}
	if d > time.Minute {
		t.Fatalf("Now() = %v; want the real clock (within a minute of time.Now)", now)
	}
}

var fixedNow = time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)

func TestPart5SetClock(t *testing.T) {
	t.Run("override is read by Now and ExpiresAt", func(t *testing.T) {
		clockOverride(t, func() time.Time { return fixedNow })
		if got := scope.Now(); !got.Equal(fixedNow) {
			t.Fatalf("Now() = %v; want %v", got, fixedNow)
		}
		got, err := scope.ExpiresAt(time.Hour)
		if err != nil {
			t.Fatalf("ExpiresAt(1h): unexpected error: %v", err)
		}
		if want := fixedNow.Add(time.Hour); !got.Equal(want) {
			t.Errorf("ExpiresAt(1h) = %v; want %v", got, want)
		}
	})

	t.Run("restore reinstates the real clock", func(t *testing.T) {
		restore := scope.SetClock(func() time.Time { return fixedNow })
		if got := scope.Now(); !got.Equal(fixedNow) {
			t.Fatalf("Now() = %v; want %v before restore", got, fixedNow)
		}
		restore()
		checkRealNow(t)
	})

	t.Run("restore is idempotent", func(t *testing.T) {
		restore := scope.SetClock(func() time.Time { return fixedNow })
		if got := scope.Now(); !got.Equal(fixedNow) {
			t.Fatalf("Now() = %v; want %v before restore", got, fixedNow)
		}
		restore()
		restore()
		checkRealNow(t)
	})

	t.Run("nil selects the real clock", func(t *testing.T) {
		clockOverride(t, func() time.Time { return fixedNow })
		if got := scope.Now(); !got.Equal(fixedNow) {
			t.Fatalf("Now() = %v; want %v before SetClock(nil)", got, fixedNow)
		}
		restore := scope.SetClock(nil)
		t.Cleanup(restore)
		checkRealNow(t)
	})
}

func TestPart5ScopedRestore(t *testing.T) {
	outer := fixedNow
	inner := fixedNow.Add(24 * time.Hour)

	t.Run("nested overrides unwind in order", func(t *testing.T) {
		restoreOuter := scope.SetClock(func() time.Time { return outer })
		restoreInner := scope.SetClock(func() time.Time { return inner })
		if got := scope.Now(); !got.Equal(inner) {
			t.Fatalf("Now() = %v; want inner %v", got, inner)
		}
		restoreInner()
		if got := scope.Now(); !got.Equal(outer) {
			t.Errorf("Now() after inner restore = %v; want outer %v", got, outer)
		}
		restoreOuter()
		checkRealNow(t)
	})

	t.Run("restore runs after a panic", func(t *testing.T) {
		func() {
			defer func() { _ = recover() }()
			restore := scope.SetClock(func() time.Time { return fixedNow })
			defer restore()
			if got := scope.Now(); !got.Equal(fixedNow) {
				t.Errorf("Now() = %v; want %v inside the scope", got, fixedNow)
			}
			panic("boom")
		}()
		checkRealNow(t)
	})
}

func TestPart5Issuer(t *testing.T) {
	t.Run("injected clock", func(t *testing.T) {
		iss := scope.NewIssuer(func() time.Time { return fixedNow })
		if iss == nil {
			t.Fatal("NewIssuer returned nil")
		}
		got, err := iss.ExpiresAt(15 * time.Minute)
		if err != nil {
			t.Fatalf("ExpiresAt(15m): unexpected error: %v", err)
		}
		if want := fixedNow.Add(15 * time.Minute); !got.Equal(want) {
			t.Errorf("ExpiresAt(15m) = %v; want %v", got, want)
		}
	})

	t.Run("injected clock ignores package override", func(t *testing.T) {
		iss := scope.NewIssuer(func() time.Time { return fixedNow })
		clockOverride(t, func() time.Time { return fixedNow.Add(100 * time.Hour) })
		got, err := iss.ExpiresAt(time.Minute)
		if err != nil {
			t.Fatalf("ExpiresAt(1m): unexpected error: %v", err)
		}
		if want := fixedNow.Add(time.Minute); !got.Equal(want) {
			t.Errorf("ExpiresAt(1m) = %v; want %v (injected clock, not the package clock)", got, want)
		}
	})

	t.Run("nil clock follows the package clock", func(t *testing.T) {
		iss := scope.NewIssuer(nil)
		if iss == nil {
			t.Fatal("NewIssuer(nil) returned nil")
		}
		clockOverride(t, func() time.Time { return fixedNow })
		got, err := iss.ExpiresAt(time.Minute)
		if err != nil {
			t.Fatalf("ExpiresAt(1m): unexpected error: %v", err)
		}
		if want := fixedNow.Add(time.Minute); !got.Equal(want) {
			t.Errorf("ExpiresAt(1m) = %v; want %v", got, want)
		}
	})

	t.Run("non-positive ttl rejected", func(t *testing.T) {
		iss := scope.NewIssuer(func() time.Time { return fixedNow })
		if _, err := iss.ExpiresAt(time.Second); err != nil {
			t.Fatalf("ExpiresAt(1s): unexpected error: %v", err)
		}
		for _, ttl := range []time.Duration{0, -time.Second} {
			if _, err := iss.ExpiresAt(ttl); !errors.Is(err, scope.ErrInvalidArgument) {
				t.Errorf("ExpiresAt(%v) error = %v; want ErrInvalidArgument", ttl, err)
			}
		}
	})

	t.Run("package ExpiresAt", func(t *testing.T) {
		clockOverride(t, func() time.Time { return fixedNow })
		got, err := scope.ExpiresAt(time.Hour)
		if err != nil {
			t.Fatalf("ExpiresAt(1h): unexpected error: %v", err)
		}
		if want := fixedNow.Add(time.Hour); !got.Equal(want) {
			t.Errorf("ExpiresAt(1h) = %v; want %v", got, want)
		}
		if _, err := scope.ExpiresAt(0); !errors.Is(err, scope.ErrInvalidArgument) {
			t.Errorf("ExpiresAt(0) error = %v; want ErrInvalidArgument", err)
		}
	})
}
