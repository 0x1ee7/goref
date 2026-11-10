package timeutil_test

import (
	"errors"
	"testing"
	"time"

	timeutil "goref/exercises/17_time"

	"github.com/google/go-cmp/cmp"
)

func TestPart5Occurrences(t *testing.T) {
	berlin := zone(t, "Europe/Berlin")
	mondayNine := timeutil.RecurrenceRule{
		Start:    utc(t, "2026-03-23T00:00"),
		Weekday:  time.Monday,
		Hour:     9,
		Location: berlin,
	}

	cases := []struct {
		name  string
		rule  timeutil.RecurrenceRule
		count int
		want  []string
	}{
		{"Monday 09:00 keeps its wall clock across spring DST", mondayNine, 3, []string{
			"2026-03-23T09:00:00+01:00",
			"2026-03-30T09:00:00+02:00",
			"2026-04-06T09:00:00+02:00",
		}},
		{"first occurrence is the next matching weekday", withStart(mondayNine, "2026-03-22T00:00"), 1, []string{
			"2026-03-23T09:00:00+01:00",
		}},
		{"gap shifts forward by default", gapRule(berlin, time.Sunday, 2, 30, "2026-03-22T00:00"), 2, []string{
			"2026-03-22T02:30:00+01:00",
			"2026-03-29T03:30:00+02:00",
		}},
		{"gap skip drops the occurrence without counting it", withPolicy(gapRule(berlin, time.Sunday, 2, 30, "2026-03-22T00:00"), timeutil.GapSkip, timeutil.OverlapEarlier), 2, []string{
			"2026-03-22T02:30:00+01:00",
			"2026-04-05T02:30:00+02:00",
		}},
		{"overlap earlier by default", gapRule(berlin, time.Sunday, 2, 30, "2026-10-18T00:00"), 2, []string{
			"2026-10-18T02:30:00+02:00",
			"2026-10-25T02:30:00+02:00",
		}},
		{"overlap later", withPolicy(gapRule(berlin, time.Sunday, 2, 30, "2026-10-18T00:00"), timeutil.GapShiftForward, timeutil.OverlapLater), 2, []string{
			"2026-10-18T02:30:00+02:00",
			"2026-10-25T02:30:00+01:00",
		}},
		{"overlap skip drops the ambiguous occurrence", withPolicy(gapRule(berlin, time.Sunday, 2, 30, "2026-10-18T00:00"), timeutil.GapShiftForward, timeutil.OverlapSkip), 2, []string{
			"2026-10-18T02:30:00+02:00",
			"2026-11-01T02:30:00+01:00",
		}},
		{"UTC has no transitions", timeutil.RecurrenceRule{
			Start: utc(t, "2026-03-23T00:00"), Weekday: time.Monday, Hour: 9, Location: time.UTC,
		}, 2, []string{
			"2026-03-23T09:00:00Z",
			"2026-03-30T09:00:00Z",
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.rule.Occurrences(tc.count)
			if err != nil {
				t.Fatalf("Occurrences(%d) error = %v", tc.count, err)
			}
			if diff := cmpStrings(tc.want, rfcs(got)); diff != "" {
				t.Errorf("Occurrences(%d) mismatch (-want +got):\n%s", tc.count, diff)
			}
		})
	}

	t.Run("sixty weeks stay at 09:00 Monday local time in order", func(t *testing.T) {
		rule := withStart(mondayNine, "2026-01-05T00:00")
		got, err := rule.Occurrences(60)
		if err != nil {
			t.Fatalf("Occurrences(60) error = %v", err)
		}
		if len(got) != 60 {
			t.Fatalf("len = %d, want 60", len(got))
		}
		for i, o := range got {
			if o.Hour() != 9 || o.Minute() != 0 || o.Weekday() != time.Monday {
				t.Errorf("occurrence %d = %s, want Monday 09:00 local", i, rfc(o))
			}
			if i > 0 && !o.After(got[i-1]) {
				t.Errorf("occurrence %d = %s is not after %s", i, rfc(o), rfc(got[i-1]))
			}
		}
	})

	t.Run("zero count is empty and one count is one", func(t *testing.T) {
		none, err := mondayNine.Occurrences(0)
		if err != nil || len(none) != 0 {
			t.Errorf("Occurrences(0) = %v, %v; want empty, nil", none, err)
		}
		one, err := mondayNine.Occurrences(1)
		if err != nil || len(one) != 1 {
			t.Errorf("Occurrences(1) = %v, %v; want one occurrence", one, err)
		}
	})

	t.Run("invalid rules and counts are ErrInvalid", func(t *testing.T) {
		bad := map[string]timeutil.RecurrenceRule{
			"nil location": {Start: utc(t, "2026-03-23T00:00"), Weekday: time.Monday, Hour: 9},
			"weekday 7":    {Start: utc(t, "2026-03-23T00:00"), Weekday: time.Weekday(7), Hour: 9, Location: berlin},
			"hour 24":      {Start: utc(t, "2026-03-23T00:00"), Weekday: time.Monday, Hour: 24, Location: berlin},
			"minute 60":    {Start: utc(t, "2026-03-23T00:00"), Weekday: time.Monday, Minute: 60, Location: berlin},
			"unknown gap":  {Start: utc(t, "2026-03-23T00:00"), Weekday: time.Monday, Hour: 9, Location: berlin, Gap: timeutil.GapPolicy(9)},
		}
		for name, rule := range bad {
			if _, err := rule.Occurrences(1); !errors.Is(err, timeutil.ErrInvalid) {
				t.Errorf("%s: error = %v, want ErrInvalid", name, err)
			}
		}
		if _, err := mondayNine.Occurrences(-1); !errors.Is(err, timeutil.ErrInvalid) {
			t.Errorf("negative count: error = %v, want ErrInvalid", err)
		}
	})
}

func withStart(r timeutil.RecurrenceRule, s string) timeutil.RecurrenceRule {
	r.Start = fixtureUTC(s)
	return r
}

func withPolicy(r timeutil.RecurrenceRule, gap timeutil.GapPolicy, overlap timeutil.OverlapPolicy) timeutil.RecurrenceRule {
	r.Gap = gap
	r.Overlap = overlap
	return r
}

// gapRule is a weekly rule at hh:mm on weekday, starting at start.
func gapRule(loc *time.Location, weekday time.Weekday, hh, mm int, start string) timeutil.RecurrenceRule {
	return timeutil.RecurrenceRule{
		Start:    fixtureUTC(start),
		Weekday:  weekday,
		Hour:     hh,
		Minute:   mm,
		Location: loc,
	}
}

// fixtureUTC parses a constant "2006-01-02T15:04" fixture as UTC. A parse
// failure is a bug in the test table, so it panics.
func fixtureUTC(s string) time.Time {
	t, err := time.Parse("2006-01-02T15:04", s)
	if err != nil {
		panic(err)
	}
	return t
}

func cmpStrings(want, got []string) string {
	return cmp.Diff(want, got)
}
