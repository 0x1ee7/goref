package timeutil_test

import (
	"testing"
	"time"
)

// zone loads a fixture zone with time.LoadLocation. Fixtures deliberately avoid
// the code under test.
func zone(t testing.TB, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("load fixture zone %q: %v", name, err)
	}
	return loc
}

// local parses "2006-01-02T15:04" as a wall-clock time in loc.
func local(t testing.TB, loc *time.Location, s string) time.Time {
	t.Helper()
	v, err := time.ParseInLocation("2006-01-02T15:04", s, loc)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return v
}

// utc parses "2006-01-02T15:04" as UTC.
func utc(t testing.TB, s string) time.Time {
	t.Helper()
	return local(t, time.UTC, s)
}

// rfc renders t with its offset ("Z" for UTC) and nanoseconds when present.
func rfc(t time.Time) string {
	return t.Format(time.RFC3339Nano)
}

// rfcs renders each time with rfc.
func rfcs(ts []time.Time) []string {
	out := make([]string, 0, len(ts))
	for _, t := range ts {
		out = append(out, rfc(t))
	}
	return out
}
