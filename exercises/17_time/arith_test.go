package timeutil_test

import (
	"errors"
	"math"
	"testing"
	"time"

	timeutil "goref/exercises/17_time"
)

func TestPart3AddCalendarDays(t *testing.T) {
	berlin := zone(t, "Europe/Berlin")
	cases := []struct {
		name  string
		start string
		days  int
		want  string
	}{
		{"keeps wall clock into a spring-forward gap", "2026-03-28T02:30", 1, "2026-03-29T03:30:00+02:00"},
		{"ordinary day keeps the winter offset", "2026-03-27T12:00", 1, "2026-03-28T12:00:00+01:00"},
		{"fall-back overlap uses the earlier occurrence", "2026-10-24T02:30", 1, "2026-10-25T02:30:00+02:00"},
		{"negative days across month end", "2026-03-01T09:00", -1, "2026-02-28T09:00:00+01:00"},
		{"zero days keeps the value", "2026-06-01T12:00", 0, "2026-06-01T12:00:00+02:00"},
		{"year rollover", "2026-12-31T23:15", 1, "2027-01-01T23:15:00+01:00"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := timeutil.AddCalendarDays(local(t, berlin, tc.start), tc.days)
			if rfc(got) != tc.want {
				t.Errorf("AddCalendarDays(%s, %d) = %s, want %s", tc.start, tc.days, rfc(got), tc.want)
			}
			if got.Location().String() != "Europe/Berlin" {
				t.Errorf("result location = %q, want Europe/Berlin", got.Location())
			}
		})
	}

	t.Run("calendar day differs from 24 elapsed hours across DST", func(t *testing.T) {
		start := local(t, berlin, "2026-03-28T12:00")
		if got := rfc(start.Add(24 * time.Hour)); got != "2026-03-29T13:00:00+02:00" {
			t.Fatalf("precondition: start.Add(24h) = %s", got)
		}
		if got := rfc(timeutil.AddCalendarDays(start, 1)); got != "2026-03-29T12:00:00+02:00" {
			t.Errorf("AddCalendarDays(+1 day) = %s, want 2026-03-29T12:00:00+02:00", got)
		}
	})
}

func TestPart3ElapsedBetween(t *testing.T) {
	berlin := zone(t, "Europe/Berlin")

	t.Run("absolute difference across spring DST is 23h", func(t *testing.T) {
		got, err := timeutil.ElapsedBetween(local(t, berlin, "2026-03-28T12:00"), local(t, berlin, "2026-03-29T12:00"))
		if err != nil {
			t.Fatalf("ElapsedBetween error = %v", err)
		}
		if got != 23*time.Hour {
			t.Errorf("ElapsedBetween = %v, want 23h0m0s", got)
		}
	})

	t.Run("reversed arguments give a negative duration", func(t *testing.T) {
		got, err := timeutil.ElapsedBetween(local(t, berlin, "2026-03-29T12:00"), local(t, berlin, "2026-03-28T12:00"))
		if err != nil {
			t.Fatalf("ElapsedBetween error = %v", err)
		}
		if got != -23*time.Hour {
			t.Errorf("ElapsedBetween reversed = %v, want -23h0m0s", got)
		}
	})

	t.Run("keeps sub-millisecond precision", func(t *testing.T) {
		start := utc(t, "2026-01-01T00:00")
		got, err := timeutil.ElapsedBetween(start, start.Add(1500*time.Microsecond))
		if err != nil {
			t.Fatalf("ElapsedBetween error = %v", err)
		}
		if got != 1500*time.Microsecond {
			t.Errorf("ElapsedBetween = %v, want 1.5ms", got)
		}
	})

	t.Run("200 years fits in a Duration", func(t *testing.T) {
		got, err := timeutil.ElapsedBetween(utc(t, "2000-01-01T00:00"), utc(t, "2200-01-01T00:00"))
		if err != nil {
			t.Fatalf("ElapsedBetween error = %v", err)
		}
		// 200*365 days + 49 leap days (2000..2196, excluding 2100)
		if want := 1753176 * time.Hour; got != want {
			t.Errorf("ElapsedBetween = %v, want %v", got, want)
		}
	})

	t.Run("span over 292 years is ErrOverflow", func(t *testing.T) {
		_, err := timeutil.ElapsedBetween(utc(t, "1900-01-01T00:00"), utc(t, "2200-01-01T00:00"))
		if !errors.Is(err, timeutil.ErrOverflow) {
			t.Errorf("error = %v, want ErrOverflow", err)
		}
	})

	t.Run("year 1 to 2026 is ErrOverflow", func(t *testing.T) {
		start := time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)
		_, err := timeutil.ElapsedBetween(start, utc(t, "2026-01-01T00:00"))
		if !errors.Is(err, timeutil.ErrOverflow) {
			t.Errorf("error = %v, want ErrOverflow", err)
		}
	})
}

func TestPart3SafeAdd(t *testing.T) {
	const maxD = time.Duration(math.MaxInt64)
	const minD = time.Duration(math.MinInt64)

	t.Run("adds ordinary durations", func(t *testing.T) {
		got, err := timeutil.SafeAdd(time.Second, 2*time.Second)
		if err != nil {
			t.Fatalf("SafeAdd error = %v", err)
		}
		if got != 3*time.Second {
			t.Errorf("SafeAdd = %v, want 3s", got)
		}
	})

	t.Run("mixed signs cannot overflow", func(t *testing.T) {
		got, err := timeutil.SafeAdd(maxD, minD)
		if err != nil {
			t.Fatalf("SafeAdd error = %v", err)
		}
		if got != -1 {
			t.Errorf("SafeAdd(max, min) = %v, want -1ns", got)
		}
	})

	t.Run("positive overflow is ErrOverflow", func(t *testing.T) {
		if _, err := timeutil.SafeAdd(maxD, 1); !errors.Is(err, timeutil.ErrOverflow) {
			t.Errorf("SafeAdd(max, 1ns) error = %v, want ErrOverflow", err)
		}
	})

	t.Run("negative overflow is ErrOverflow", func(t *testing.T) {
		if _, err := timeutil.SafeAdd(minD, -1); !errors.Is(err, timeutil.ErrOverflow) {
			t.Errorf("SafeAdd(min, -1ns) error = %v, want ErrOverflow", err)
		}
	})
}
