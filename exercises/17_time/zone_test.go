package timeutil_test

import (
	"errors"
	"testing"
	"time"

	timeutil "goref/exercises/17_time"
)

func TestPart1Canonical(t *testing.T) {
	t.Run("strips the monotonic reading so == agrees with Equal", func(t *testing.T) {
		now := time.Now()
		if now == now.Round(0) {
			t.Fatal("precondition: time.Now carries a monotonic reading that Round(0) removes")
		}
		c := timeutil.Canonical(now)
		if !c.Equal(now) {
			t.Fatalf("Canonical(now) = %s, want the same instant as %s", rfc(c), rfc(now))
		}
		if c != timeutil.Canonical(now.Round(0)) {
			t.Errorf("Canonical differs with and without the monotonic reading: %#v vs %#v",
				c, timeutil.Canonical(now.Round(0)))
		}
		if c.Location() != time.UTC {
			t.Errorf("Canonical location = %v, want UTC", c.Location())
		}
	})

	t.Run("equal instants in different zones become == after Canonical", func(t *testing.T) {
		berlin := zone(t, "Europe/Berlin")
		a := local(t, berlin, "2026-06-01T12:00")
		b := utc(t, "2026-06-01T10:00")
		if a == b {
			t.Fatal("precondition: values in different zones are not == before Canonical")
		}
		ca, cb := timeutil.Canonical(a), timeutil.Canonical(b)
		if ca != cb {
			t.Errorf("Canonical(%s) != Canonical(%s)", rfc(a), rfc(b))
		}
		if got := rfc(ca); got != "2026-06-01T10:00:00Z" {
			t.Errorf("Canonical(%s) = %s, want 2026-06-01T10:00:00Z", rfc(a), got)
		}
	})
}

func TestPart1LoadZone(t *testing.T) {
	t.Run("loads IANA names", func(t *testing.T) {
		for _, name := range []string{"Europe/Berlin", "America/New_York", "UTC"} {
			loc, err := timeutil.LoadZone(name)
			if err != nil {
				t.Errorf("LoadZone(%q) error = %v", name, err)
				continue
			}
			if loc.String() != name {
				t.Errorf("LoadZone(%q).String() = %q", name, loc.String())
			}
		}
	})

	t.Run("rejects machine-dependent and empty names", func(t *testing.T) {
		for _, name := range []string{"", "Local"} {
			if _, err := timeutil.LoadZone(name); !errors.Is(err, timeutil.ErrUnknownZone) {
				t.Errorf("LoadZone(%q) error = %v, want ErrUnknownZone", name, err)
			}
		}
	})

	t.Run("unknown and traversal names wrap ErrUnknownZone", func(t *testing.T) {
		for _, name := range []string{"Mars/Olympus", "../Europe/Berlin"} {
			if _, err := timeutil.LoadZone(name); !errors.Is(err, timeutil.ErrUnknownZone) {
				t.Errorf("LoadZone(%q) error = %v, want ErrUnknownZone", name, err)
			}
		}
	})
}

func TestPart1Localize(t *testing.T) {
	berlin := zone(t, "Europe/Berlin")

	// Wall clocks are written in UTC on purpose: only the fields matter, not the zone of wall.
	cases := []struct {
		name    string
		wall    string
		gap     timeutil.GapPolicy
		overlap timeutil.OverlapPolicy
		want    string
		wantErr error
	}{
		{name: "ordinary summer time", wall: "2026-06-01T12:00", want: "2026-06-01T12:00:00+02:00"},
		{name: "ordinary winter time", wall: "2026-01-15T08:00", want: "2026-01-15T08:00:00+01:00"},
		{name: "gap shifts forward to 03:30 CEST", wall: "2026-03-29T02:30",
			gap: timeutil.GapShiftForward, want: "2026-03-29T03:30:00+02:00"},
		{name: "gap skip reports ErrNonexistent", wall: "2026-03-29T02:30",
			gap: timeutil.GapSkip, wantErr: timeutil.ErrNonexistent},
		{name: "01:59 is before the gap and valid", wall: "2026-03-29T01:59",
			gap: timeutil.GapSkip, want: "2026-03-29T01:59:00+01:00"},
		{name: "03:00 is after the gap and valid", wall: "2026-03-29T03:00",
			gap: timeutil.GapSkip, want: "2026-03-29T03:00:00+02:00"},
		{name: "overlap earlier is CEST", wall: "2026-10-25T02:30",
			overlap: timeutil.OverlapEarlier, want: "2026-10-25T02:30:00+02:00"},
		{name: "overlap later is CET", wall: "2026-10-25T02:30",
			overlap: timeutil.OverlapLater, want: "2026-10-25T02:30:00+01:00"},
		{name: "overlap skip reports ErrAmbiguous", wall: "2026-10-25T02:30",
			overlap: timeutil.OverlapSkip, wantErr: timeutil.ErrAmbiguous},
		{name: "01:30 is unambiguous on fall-back day", wall: "2026-10-25T01:30",
			overlap: timeutil.OverlapSkip, want: "2026-10-25T01:30:00+02:00"},
		{name: "03:00 after the overlap is CET", wall: "2026-10-25T03:00",
			overlap: timeutil.OverlapSkip, want: "2026-10-25T03:00:00+01:00"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := timeutil.Localize(utc(t, tc.wall), berlin, tc.gap, tc.overlap)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("Localize(%s) error = %v, want %v", tc.wall, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Localize(%s) error = %v", tc.wall, err)
			}
			if rfc(got) != tc.want {
				t.Errorf("Localize(%s) = %s, want %s", tc.wall, rfc(got), tc.want)
			}
			if got.Location().String() != "Europe/Berlin" {
				t.Errorf("Localize result location = %q, want Europe/Berlin", got.Location())
			}
		})
	}

	t.Run("nil location is ErrInvalid", func(t *testing.T) {
		_, err := timeutil.Localize(utc(t, "2026-06-01T12:00"), nil, timeutil.GapSkip, timeutil.OverlapSkip)
		if !errors.Is(err, timeutil.ErrInvalid) {
			t.Errorf("error = %v, want ErrInvalid", err)
		}
	})

	t.Run("unknown policy values are ErrInvalid", func(t *testing.T) {
		_, err := timeutil.Localize(utc(t, "2026-06-01T12:00"), berlin, timeutil.GapPolicy(7), timeutil.OverlapEarlier)
		if !errors.Is(err, timeutil.ErrInvalid) {
			t.Errorf("gap policy 7: error = %v, want ErrInvalid", err)
		}
		_, err = timeutil.Localize(utc(t, "2026-06-01T12:00"), berlin, timeutil.GapShiftForward, timeutil.OverlapPolicy(7))
		if !errors.Is(err, timeutil.ErrInvalid) {
			t.Errorf("overlap policy 7: error = %v, want ErrInvalid", err)
		}
	})
}

// Go's time.Date and the explicit policy disagree on purpose: time.Date does not
// promise which side of a transition it picks, so the policy must be explicit.
func TestPart1TimeDateVersusPolicy(t *testing.T) {
	berlin := zone(t, "Europe/Berlin")

	t.Run("time.Date in a gap returns a wall clock that was not requested", func(t *testing.T) {
		got := time.Date(2026, 3, 29, 2, 30, 0, 0, berlin)
		if got.Hour() == 2 && got.Minute() == 30 {
			t.Fatalf("time.Date(02:30 in gap) kept 02:30: %s", rfc(got))
		}
		want, err := timeutil.Localize(utc(t, "2026-03-29T02:30"), berlin, timeutil.GapShiftForward, timeutil.OverlapEarlier)
		if err != nil {
			t.Fatalf("Localize: %v", err)
		}
		if rfc(want) != "2026-03-29T03:30:00+02:00" {
			t.Errorf("Localize(02:30 in gap, shift) = %s, want 2026-03-29T03:30:00+02:00", rfc(want))
		}
	})

	t.Run("time.Date in an overlap is one of the two occurrences", func(t *testing.T) {
		got := time.Date(2026, 10, 25, 2, 30, 0, 0, berlin)
		early, err := timeutil.Localize(utc(t, "2026-10-25T02:30"), berlin, timeutil.GapShiftForward, timeutil.OverlapEarlier)
		if err != nil {
			t.Fatalf("Localize earlier: %v", err)
		}
		late, err := timeutil.Localize(utc(t, "2026-10-25T02:30"), berlin, timeutil.GapShiftForward, timeutil.OverlapLater)
		if err != nil {
			t.Fatalf("Localize later: %v", err)
		}
		if late.Sub(early) != time.Hour {
			t.Errorf("later - earlier = %v, want 1h", late.Sub(early))
		}
		if !got.Equal(early) && !got.Equal(late) {
			t.Errorf("time.Date(02:30 in overlap) = %s, not one of %s or %s", rfc(got), rfc(early), rfc(late))
		}
	})
}
