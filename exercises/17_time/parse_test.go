package timeutil_test

import (
	"errors"
	"testing"
	"time"

	timeutil "goref/exercises/17_time"
)

func TestPart2ParseTimestamp(t *testing.T) {
	ok := []struct {
		name string
		in   string
		want string // RFC 3339 with nanoseconds, in UTC
	}{
		{"RFC 3339 with positive offset", "2026-03-29T12:00:00+02:00", "2026-03-29T10:00:00Z"},
		{"RFC 3339 with negative offset", "2026-03-29T05:00:00-05:00", "2026-03-29T10:00:00Z"},
		{"Z suffix", "2026-03-29T10:00:00Z", "2026-03-29T10:00:00Z"},
		{"fractional seconds kept", "2026-03-29T12:00:00.123456789+02:00", "2026-03-29T10:00:00.123456789Z"},
		{"surrounding whitespace trimmed", "  2026-03-29T10:00:00Z\n", "2026-03-29T10:00:00Z"},
		{"RFC 2822 with offset", "Sun, 29 Mar 2026 12:00:00 +0200", "2026-03-29T10:00:00Z"},
		{"epoch seconds", "1767225600", "2026-01-01T00:00:00Z"},
		{"negative epoch seconds", "-1500", "1969-12-31T23:35:00Z"},
		{"epoch milliseconds by magnitude", "1767225600123", "2026-01-01T00:00:00.123Z"},
		{"1e11 is the first millisecond value", "100000000000", "1973-03-03T09:46:40Z"},
	}
	for _, tc := range ok {
		t.Run(tc.name, func(t *testing.T) {
			got, err := timeutil.ParseTimestamp(tc.in)
			if err != nil {
				t.Fatalf("ParseTimestamp(%q) error = %v", tc.in, err)
			}
			if got.Location() != time.UTC {
				t.Errorf("ParseTimestamp(%q) location = %v, want UTC", tc.in, got.Location())
			}
			if rfc(got) != tc.want {
				t.Errorf("ParseTimestamp(%q) = %s, want %s", tc.in, rfc(got), tc.want)
			}
		})
	}

	bad := []struct {
		name string
		in   string
		want error
	}{
		{"empty", "", timeutil.ErrInvalidTimestamp},
		{"whitespace only", "   ", timeutil.ErrInvalidTimestamp},
		{"naive datetime has no offset", "2026-03-29T10:00:00", timeutil.ErrInvalidTimestamp},
		{"date only", "2026-03-29", timeutil.ErrInvalidTimestamp},
		{"impossible day", "2026-02-30T10:00:00Z", timeutil.ErrInvalidTimestamp},
		{"not a timestamp", "yesterday", timeutil.ErrInvalidTimestamp},
		{"fractional epoch is not accepted", "1767225600.5", timeutil.ErrInvalidTimestamp},
		{"RFC 2822 weekday must match the date", "Mon, 29 Mar 2026 12:00:00 +0200", timeutil.ErrInvalidTimestamp},
		{"RFC 2822 -0000 offset is unknown", "Sun, 29 Mar 2026 12:00:00 -0000", timeutil.ErrUnknownOffset},
		{"RFC 3339 -00:00 offset is unknown", "2026-03-29T10:00:00-00:00", timeutil.ErrUnknownOffset},
	}
	for _, tc := range bad {
		t.Run("rejects "+tc.name, func(t *testing.T) {
			if _, err := timeutil.ParseTimestamp(tc.in); !errors.Is(err, tc.want) {
				t.Errorf("ParseTimestamp(%q) error = %v, want %v", tc.in, err, tc.want)
			}
		})
	}
}

func TestPart2FromEpoch(t *testing.T) {
	cases := []struct {
		name string
		in   int64
		want string
	}{
		{"seconds", 1767225600, "2026-01-01T00:00:00Z"},
		{"negative seconds", -1500, "1969-12-31T23:35:00Z"},
		{"milliseconds", 1767225600123, "2026-01-01T00:00:00.123Z"},
		{"first millisecond magnitude", 100_000_000_000, "1973-03-03T09:46:40Z"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := rfc(timeutil.FromEpoch(tc.in)); got != tc.want {
				t.Errorf("FromEpoch(%d) = %s, want %s", tc.in, got, tc.want)
			}
		})
	}
}

func TestPart2EpochMilli(t *testing.T) {
	berlin := zone(t, "Europe/Berlin")

	t.Run("truncates positive sub-millisecond part", func(t *testing.T) {
		if got := timeutil.EpochMilli(time.Unix(1, 999_500_000)); got != 1999 {
			t.Errorf("EpochMilli(1.9995s) = %d, want 1999", got)
		}
	})

	t.Run("rounds pre-epoch instants toward the past", func(t *testing.T) {
		// -1.5ms: Duration.Milliseconds would truncate toward zero and give -1.
		if got := timeutil.EpochMilli(time.Unix(0, -1_500_000)); got != -2 {
			t.Errorf("EpochMilli(-1.5ms) = %d, want -2", got)
		}
		if got := timeutil.EpochMilli(time.Unix(-1, 500_000_000)); got != -500 {
			t.Errorf("EpochMilli(-0.5s) = %d, want -500", got)
		}
	})

	t.Run("depends on the instant, not the zone", func(t *testing.T) {
		if got := timeutil.EpochMilli(time.Unix(1, 999_500_000).In(berlin)); got != 1999 {
			t.Errorf("EpochMilli in Berlin = %d, want 1999", got)
		}
	})

	t.Run("round trips through FromEpoch", func(t *testing.T) {
		const ms = 1767225600123
		if got := timeutil.EpochMilli(timeutil.FromEpoch(ms)); got != ms {
			t.Errorf("EpochMilli(FromEpoch(%d)) = %d", ms, got)
		}
	})
}
