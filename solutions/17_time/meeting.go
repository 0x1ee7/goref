// Part 5: recurring meetings with an explicit DST policy.
package timeutil

import (
	"errors"
	"fmt"
	"time"
)

// RecurrenceRule describes a weekly meeting at a local wall-clock time in
// Location. The zero value is invalid because Location is nil.
type RecurrenceRule struct {
	// Start is the earliest date considered. Its wall-clock date, read in its own location, is used.
	Start time.Time
	// Weekday is the weekday of every occurrence.
	Weekday time.Weekday
	// Hour (0-23) and Minute (0-59) are the local wall-clock time.
	Hour, Minute int
	// Location is the zone the wall-clock time is read in.
	Location *time.Location
	// Gap decides what happens when the wall-clock time falls in a DST gap.
	Gap GapPolicy
	// Overlap decides what happens when the wall-clock time falls in a DST overlap.
	Overlap OverlapPolicy
}

// Occurrences returns the next count occurrences, in ascending order, on or
// after Start's date. Occurrences dropped by GapSkip or OverlapSkip do not
// count toward count. It returns [ErrInvalid] for a negative count, a nil
// Location, a Weekday outside Sunday..Saturday, an Hour or Minute out of range,
// or an unknown policy.
func (r RecurrenceRule) Occurrences(count int) ([]time.Time, error) {
	if err := r.validate(count); err != nil {
		return nil, err
	}

	out := make([]time.Time, 0, count)
	day := civilUTC(r.Start)
	// Advance to the first date that falls on the requested weekday.
	day = day.AddDate(0, 0, (int(r.Weekday)-int(day.Weekday())+7)%7)

	for len(out) < count {
		wall := time.Date(day.Year(), day.Month(), day.Day(), r.Hour, r.Minute, 0, 0, time.UTC)
		t, err := resolve(wall, r.Location, r.Gap, r.Overlap)
		switch {
		case err == nil:
			out = append(out, t)
		case errors.Is(err, ErrNonexistent), errors.Is(err, ErrAmbiguous):
			// Dropped by policy; it does not count toward count.
		default:
			return nil, err
		}
		day = day.AddDate(0, 0, 7)
	}
	return out, nil
}

func (r RecurrenceRule) validate(count int) error {
	switch {
	case count < 0:
		return fmt.Errorf("%w: negative count %d", ErrInvalid, count)
	case r.Location == nil:
		return fmt.Errorf("%w: nil location", ErrInvalid)
	case r.Weekday < time.Sunday || r.Weekday > time.Saturday:
		return fmt.Errorf("%w: weekday %d", ErrInvalid, int(r.Weekday))
	case r.Hour < 0 || r.Hour > 23:
		return fmt.Errorf("%w: hour %d", ErrInvalid, r.Hour)
	case r.Minute < 0 || r.Minute > 59:
		return fmt.Errorf("%w: minute %d", ErrInvalid, r.Minute)
	}
	return checkPolicies(r.Gap, r.Overlap)
}
